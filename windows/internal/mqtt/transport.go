package mqtt

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"satellite/internal/config"
)

func dialBroker(ctx context.Context, cfg config.MQTTConfig, timeout time.Duration) (net.Conn, error) {
	address := strings.TrimSpace(cfg.Broker)
	if !strings.Contains(address, "://") {
		address = "tcp://" + address
	}
	u, err := url.Parse(address)
	if err != nil {
		return nil, err
	}
	if u.Hostname() == "" {
		return nil, fmt.Errorf("MQTT broker hostname is required")
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	tlsConfig := &tls.Config{InsecureSkipVerify: cfg.InsecureTLS}
	switch u.Scheme {
	case "ws", "wss":
		d := websocket.Dialer{HandshakeTimeout: timeout, Subprotocols: []string{"mqtt"}, TLSClientConfig: tlsConfig}
		conn, response, err := d.DialContext(ctx, u.String(), nil)
		if err != nil {
			if response != nil && response.Body != nil {
				response.Body.Close()
			}
			return nil, fmt.Errorf("MQTT WebSocket connection failed: %w", err)
		}
		return &websocketConn{Conn: conn}, nil
	case "tcp", "mqtt", "ssl", "tls", "mqtts":
		port := u.Port()
		secure := u.Scheme == "ssl" || u.Scheme == "tls" || u.Scheme == "mqtts"
		if port == "" {
			port = "1883"
			if secure {
				port = "8883"
			}
		}
		d := &net.Dialer{}
		address = net.JoinHostPort(u.Hostname(), port)
		if secure {
			return (&tls.Dialer{NetDialer: d, Config: tlsConfig}).DialContext(ctx, "tcp", address)
		}
		return d.DialContext(ctx, "tcp", address)
	default:
		return nil, fmt.Errorf("unsupported MQTT broker scheme %q; use tcp, ssl, ws or wss", u.Scheme)
	}
}

type websocketConn struct {
	*websocket.Conn
	sync.Mutex
	reader io.Reader
}

func (c *websocketConn) Read(p []byte) (int, error) {
	for {
		if c.reader == nil {
			kind, reader, err := c.NextReader()
			if err != nil {
				return 0, err
			}
			if kind != websocket.BinaryMessage {
				return 0, fmt.Errorf("MQTT WebSocket requires binary messages")
			}
			c.reader = reader
		}
		n, err := c.reader.Read(p)
		if err == io.EOF {
			c.reader = nil
			if n > 0 {
				return n, nil
			}
			continue
		}
		return n, err
	}
}

func (c *websocketConn) Write(p []byte) (int, error) {
	if err := c.WriteMessage(websocket.BinaryMessage, p); err != nil {
		return 0, err
	}
	return len(p), nil
}

func (c *websocketConn) SetDeadline(t time.Time) error {
	if err := c.SetReadDeadline(t); err != nil {
		return err
	}
	return c.SetWriteDeadline(t)
}
