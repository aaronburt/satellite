package mqtt

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"strings"
	"sync"
	"time"

	"satellite/internal/actions"
	"satellite/internal/config"
	"satellite/internal/telemetry"

	"github.com/eclipse/paho.golang/paho"
)

type Status string

const (
	StatusDisconnected Status = "Disconnected"
	StatusConnecting   Status = "Connecting"
	StatusConnected    Status = "Connected"
)

type Client struct {
	mu           sync.RWMutex
	cfg          config.Config
	pahoClient   *paho.Client
	netConn      net.Conn
	status       Status
	cancelLoop   context.CancelFunc
	lastSnapshot telemetry.Snapshot
	lastSentTime time.Time
	hasBattery   bool
	registry     *actions.Registry
}

func NewClient() *Client {
	reg := actions.NewRegistry()
	c := &Client{
		status:   StatusDisconnected,
		registry: reg,
	}
	reg.SetStatusProvider(func() string {
		c.mu.RLock()
		defer c.mu.RUnlock()
		if c.lastSnapshot.Media != nil {
			return c.lastSnapshot.Media.Status
		}
		return ""
	})
	return c
}

func (c *Client) SetHasBattery(hasBattery bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.hasBattery = hasBattery
}

func (c *Client) Status() Status {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.status
}

func (c *Client) UpdateConfigAndRestart(cfg config.Config) {
	c.Stop()
	c.Start(cfg)
}

func (c *Client) Start(cfg config.Config) {
	c.mu.Lock()
	c.cfg = cfg
	if cfg.MQTT.Broker == "" {
		c.status = StatusDisconnected
		c.mu.Unlock()
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	c.cancelLoop = cancel
	c.status = StatusConnecting
	c.mu.Unlock()

	go c.runLoop(ctx)
}

func (c *Client) Stop() {
	c.mu.Lock()
	if c.cancelLoop != nil {
		c.cancelLoop()
		c.cancelLoop = nil
	}

	if c.pahoClient != nil && c.netConn != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		prefix := c.cfg.MQTT.TopicPrefix
		if prefix == "" {
			prefix = "satellite"
		}
		statusTopic := fmt.Sprintf("%s/%s/status", prefix, c.cfg.NodeID)
		_, _ = c.pahoClient.Publish(ctx, &paho.Publish{
			Topic:   statusTopic,
			QoS:     1,
			Retain:  true,
			Payload: []byte("offline"),
		})
		cancel()
		_ = c.pahoClient.Disconnect(&paho.Disconnect{ReasonCode: 0})
		_ = c.netConn.Close()
		c.pahoClient = nil
		c.netConn = nil
	}
	c.status = StatusDisconnected
	c.mu.Unlock()
}

func (c *Client) runLoop(ctx context.Context) {
	backoff := 2 * time.Second
	for {
		select {
		case <-ctx.Done():
			return
		default:
			startTime := time.Now()
			err := c.connectAndServe(ctx)
			if err != nil {
				c.mu.Lock()
				c.status = StatusDisconnected
				c.mu.Unlock()
			}

			if time.Since(startTime) >= 10*time.Second {
				backoff = 2 * time.Second
			}

			select {
			case <-ctx.Done():
				return
			case <-time.After(backoff):
				if backoff < 30*time.Second {
					backoff *= 2
				}
			}
		}
	}
}

func (c *Client) connectAndServe(ctx context.Context) error {
	c.mu.RLock()
	cfg := c.cfg
	hasBattery := c.hasBattery
	c.mu.RUnlock()

	brokerURL := cfg.MQTT.Broker
	if !strings.Contains(brokerURL, "://") {
		brokerURL = "tcp://" + brokerURL
	}
	parsed, err := url.Parse(brokerURL)
	if err != nil {
		return err
	}

	hostPort := parsed.Host
	if !strings.Contains(hostPort, ":") {
		if parsed.Scheme == "ssl" || parsed.Scheme == "tls" || parsed.Scheme == "mqtts" {
			hostPort += ":8883"
		} else {
			hostPort += ":1883"
		}
	}

	dialer := net.Dialer{Timeout: 5 * time.Second}
	var conn net.Conn
	if parsed.Scheme == "ssl" || parsed.Scheme == "tls" || parsed.Scheme == "mqtts" {
		conn, err = tls.DialWithDialer(&dialer, "tcp", hostPort, &tls.Config{InsecureSkipVerify: cfg.MQTT.InsecureTLS})
	} else {
		conn, err = dialer.DialContext(ctx, "tcp", hostPort)
	}
	if err != nil {
		return err
	}

	connCtx, connCancel := context.WithCancel(ctx)
	defer connCancel()

	connAssigned := false
	defer func() {
		if !connAssigned {
			conn.Close()
		}
	}()

	prefix := cfg.MQTT.TopicPrefix
	if prefix == "" {
		prefix = "satellite"
	}
	statusTopic := fmt.Sprintf("%s/%s/status", prefix, cfg.NodeID)
	commandTopic := fmt.Sprintf("%s/%s/command", prefix, cfg.NodeID)

	router := paho.NewStandardRouter()
	router.RegisterHandler(commandTopic, func(p *paho.Publish) {
		c.mu.RLock()
		allowed := c.cfg.Expose.RemoteLock || c.cfg.Expose.MediaControl
		reg := c.registry
		c.mu.RUnlock()

		if !allowed || reg == nil {
			return
		}

		_ = reg.ExecutePayload(p.Payload)
	})

	pClient := paho.NewClient(paho.ClientConfig{
		Conn:   conn,
		Router: router,
		OnServerDisconnect: func(d *paho.Disconnect) {
			connCancel()
		},
		OnClientError: func(err error) {
			connCancel()
		},
	})

	clientID := cfg.MQTT.ClientID
	if clientID == "" {
		clientID = "satellite-" + cfg.NodeID
	}

	cp := &paho.Connect{
		KeepAlive:  30,
		ClientID:   clientID,
		CleanStart: true,
		WillMessage: &paho.WillMessage{
			Topic:   statusTopic,
			Payload: []byte("offline"),
			QoS:     1,
			Retain:  true,
		},
	}
	if cfg.MQTT.Username != "" {
		cp.Username = cfg.MQTT.Username
		cp.UsernameFlag = true
	}
	if cfg.MQTT.Password != "" {
		cp.Password = []byte(cfg.MQTT.Password)
		cp.PasswordFlag = true
	}

	connectCtx, connectCancel := context.WithTimeout(ctx, 5*time.Second)
	_, err = pClient.Connect(connectCtx, cp)
	connectCancel()
	if err != nil {
		return err
	}

	c.mu.Lock()
	c.netConn = conn
	c.pahoClient = pClient
	c.status = StatusConnected
	connAssigned = true
	c.mu.Unlock()

	pubCtx, pubCancel := context.WithTimeout(ctx, 5*time.Second)
	_, _ = pClient.Publish(pubCtx, &paho.Publish{
		Topic:   statusTopic,
		Payload: []byte("online"),
		QoS:     1,
		Retain:  true,
	})
	pubCancel()

	if cfg.Expose.RemoteLock || cfg.Expose.MediaControl {
		subCtx, subCancel := context.WithTimeout(ctx, 5*time.Second)
		_, _ = pClient.Subscribe(subCtx, &paho.Subscribe{
			Subscriptions: []paho.SubscribeOptions{
				{
					Topic: commandTopic,
					QoS:   1,
				},
			},
		})
		subCancel()
	}

	discoveryItems := GetAllDiscoveryItems(cfg, hasBattery)
	for _, item := range discoveryItems {
		dCtx, dCancel := context.WithTimeout(ctx, 3*time.Second)
		payload := item.Payload
		if !item.ShouldRun {
			payload = []byte("")
		}
		_, _ = pClient.Publish(dCtx, &paho.Publish{
			Topic:   item.Topic,
			Payload: payload,
			QoS:     1,
			Retain:  true,
		})
		dCancel()
	}

	select {
	case <-ctx.Done():
		return nil
	case <-connCtx.Done():
		c.mu.Lock()
		if c.pahoClient == pClient {
			c.pahoClient = nil
			if c.netConn != nil {
				_ = c.netConn.Close()
				c.netConn = nil
			}
			connAssigned = false
		}
		c.status = StatusDisconnected
		c.mu.Unlock()
		return fmt.Errorf("mqtt connection closed")
	}
}

func (c *Client) PublishTelemetry(snap telemetry.Snapshot) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.status != StatusConnected || c.pahoClient == nil {
		return nil
	}

	now := time.Now()
	isHeartbeat := now.Sub(c.lastSentTime) >= 60*time.Second
	if !isHeartbeat && !telemetry.HasSignificantDelta(c.lastSnapshot, snap) {
		return nil
	}

	prefix := c.cfg.MQTT.TopicPrefix
	if prefix == "" {
		prefix = "satellite"
	}
	stateTopic := fmt.Sprintf("%s/%s/state", prefix, c.cfg.NodeID)

	bytes, err := json.Marshal(snap)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err = c.pahoClient.Publish(ctx, &paho.Publish{
		Topic:   stateTopic,
		Payload: bytes,
		QoS:     0,
		Retain:  false,
	})
	if err == nil {
		c.lastSnapshot = snap
		c.lastSentTime = now
	}
	return err
}

func TestConnection(cfg config.MQTTConfig) error {
	if cfg.Broker == "" {
		return fmt.Errorf("broker address cannot be empty")
	}

	brokerURL := cfg.Broker
	if !strings.Contains(brokerURL, "://") {
		brokerURL = "tcp://" + brokerURL
	}
	parsed, err := url.Parse(brokerURL)
	if err != nil {
		return err
	}

	hostPort := parsed.Host
	if !strings.Contains(hostPort, ":") {
		if parsed.Scheme == "ssl" || parsed.Scheme == "tls" || parsed.Scheme == "mqtts" {
			hostPort += ":8883"
		} else {
			hostPort += ":1883"
		}
	}

	dialer := net.Dialer{Timeout: 4 * time.Second}
	var conn net.Conn
	if parsed.Scheme == "ssl" || parsed.Scheme == "tls" || parsed.Scheme == "mqtts" {
		conn, err = tls.DialWithDialer(&dialer, "tcp", hostPort, &tls.Config{InsecureSkipVerify: cfg.InsecureTLS})
	} else {
		conn, err = dialer.Dial("tcp", hostPort)
	}
	if err != nil {
		return err
	}
	defer conn.Close()

	pClient := paho.NewClient(paho.ClientConfig{
		Conn: conn,
	})

	clientID := cfg.ClientID
	if clientID == "" {
		clientID = "satellite-test-conn"
	}

	cp := &paho.Connect{
		KeepAlive:  10,
		ClientID:   clientID,
		CleanStart: true,
	}
	if cfg.Username != "" {
		cp.Username = cfg.Username
		cp.UsernameFlag = true
	}
	if cfg.Password != "" {
		cp.Password = []byte(cfg.Password)
		cp.PasswordFlag = true
	}

	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()

	_, err = pClient.Connect(ctx, cp)
	if err != nil {
		return err
	}

	_ = pClient.Disconnect(&paho.Disconnect{ReasonCode: 0})
	return nil
}
