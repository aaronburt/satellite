package mqtt

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"satellite/internal/config"
)

func TestWebSocketMQTT(t *testing.T) {
	for _, secure := range []bool{false, true} {
		name := "ws"
		if secure {
			name = "wss"
		}
		t.Run(name, func(t *testing.T) {
			broker := startBroker(t, "user", "password")
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/mqtt" || r.URL.Query().Get("route") != "satellite" || r.Header.Get("Sec-WebSocket-Protocol") != "mqtt" {
					http.Error(w, "invalid WebSocket request", http.StatusBadRequest)
					return
				}
				upgrader := websocket.Upgrader{Subprotocols: []string{"mqtt"}}
				ws, err := upgrader.Upgrade(w, r, nil)
				if err != nil {
					return
				}
				defer ws.Close()
				conn, err := net.Dial("tcp", broker.addr)
				if err != nil {
					return
				}
				defer conn.Close()
				wrapped := &websocketConn{Conn: ws}
				go func() {
					_, _ = io.Copy(wrapped, conn)
					ws.Close()
				}()
				_, _ = io.Copy(conn, wrapped)
			})
			var server *httptest.Server
			if secure {
				server = httptest.NewTLSServer(handler)
			} else {
				server = httptest.NewServer(handler)
			}
			defer server.Close()
			cfg := config.MQTTConfig{Broker: name + strings.TrimPrefix(strings.TrimPrefix(server.URL, "https"), "http") + "/mqtt?route=satellite", Username: "user", Password: "password"}
			if secure {
				if err := TestConnection(cfg); err == nil {
					t.Fatal("untrusted certificate accepted")
				}
				cfg.InsecureTLS = true
			}
			if err := TestConnection(cfg); err != nil {
				t.Fatal(err)
			}
			live := config.DefaultConfig()
			live.MQTT = cfg
			client := NewClient()
			client.Start(live)
			defer client.Stop()
			waitFor(t, "WebSocket MQTT connection", func() bool { return client.Status() == StatusConnected })
			cfg.Password = "wrong"
			if err := TestConnection(cfg); err == nil {
				t.Fatal("invalid MQTT credentials accepted")
			}
		})
	}
}

func TestBrokerRejectsUnsupportedScheme(t *testing.T) {
	if _, err := dialBroker(context.Background(), config.MQTTConfig{Broker: "https://localhost"}, time.Second); err == nil {
		t.Fatal("unsupported scheme accepted")
	}
}
