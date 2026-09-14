package webui

import (
	"bytes"
	"crypto/rand"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"sync"

	"satellite/internal/config"
	"satellite/internal/mqtt"
	"satellite/internal/telemetry"
)

//go:embed static/index.html
var indexHTML []byte

type Server struct {
	mu         sync.Mutex
	server     *http.Server
	listener   net.Listener
	port       int
	token      string
	collector  *telemetry.Collector
	mqttClient *mqtt.Client
	running    bool
}

func NewServer(collector *telemetry.Collector, mqttClient *mqtt.Client) *Server {
	return &Server{
		collector:  collector,
		mqttClient: mqttClient,
	}
}

func generateSecureToken() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func (s *Server) IsRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}

func (s *Server) Token() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.token
}

func (s *Server) Start(preferredPort int) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return s.port, nil
	}

	addr := fmt.Sprintf("127.0.0.1:%d", preferredPort)
	ln, err := net.Listen("tcp", addr)
	if err != nil && preferredPort != 0 {
		ln, err = net.Listen("tcp", "127.0.0.1:0")
	}
	if err != nil {
		return 0, err
	}

	s.listener = ln
	s.port = ln.Addr().(*net.TCPAddr).Port
	s.token = generateSecureToken()

	mux := http.NewServeMux()

	authMiddleware := func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			tokenParam := r.URL.Query().Get("token")
			tokenHeader := r.Header.Get("X-API-Token")

			s.mu.Lock()
			expected := s.token
			s.mu.Unlock()

			if tokenParam != expected && tokenHeader != expected {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"error": "Unauthorized: invalid or missing ephemeral token",
				})
				return
			}
			next(w, r)
		}
	}

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		tokenParam := r.URL.Query().Get("token")
		s.mu.Lock()
		expected := s.token
		s.mu.Unlock()

		if tokenParam != expected {
			http.Error(w, "Unauthorized: valid ephemeral token required in URL", http.StatusUnauthorized)
			return
		}

		rendered := bytes.ReplaceAll(indexHTML, []byte("{{VERSION}}"), []byte(config.Version))
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(rendered)
	})

	mux.HandleFunc("/api/status", authMiddleware(s.handleStatus))
	mux.HandleFunc("/api/config", authMiddleware(s.handleConfig))
	mux.HandleFunc("/api/test-connection", authMiddleware(s.handleTestConnection))

	httpSrv := &http.Server{Handler: mux}
	s.server = httpSrv
	s.running = true

	go func() {
		_ = httpSrv.Serve(s.listener)
		s.mu.Lock()
		s.running = false
		s.mu.Unlock()
	}()

	return s.port, nil
}

func (s *Server) Port() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.port
}

func (s *Server) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.server != nil {
		_ = s.server.Close()
		s.server = nil
	}
	if s.listener != nil {
		_ = s.listener.Close()
		s.listener = nil
	}
	s.token = ""
	s.running = false
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	snap := s.collector.Last()
	mqttStatus := string(s.mqttClient.Status())

	resp := map[string]interface{}{
		"mqtt_status": mqttStatus,
		"telemetry":   snap,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method == http.MethodGet {
		cfg := config.Get()
		cfg.MQTT.Password = ""
		_ = json.NewEncoder(w).Encode(cfg)
		return
	}

	if r.Method == http.MethodPost {
		var incoming config.Config
		if err := json.NewDecoder(r.Body).Decode(&incoming); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": err.Error()})
			return
		}

		current := config.Get()
		if incoming.MQTT.Password == "" {
			incoming.MQTT.Password = current.MQTT.Password
		}
		if incoming.IntervalSec <= 0 {
			incoming.IntervalSec = 5
		}
		if incoming.NodeID == "" {
			incoming.NodeID = current.NodeID
		}
		incoming.WebUIPort = current.WebUIPort

		if err := config.Save(incoming); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": err.Error()})
			return
		}

		s.mqttClient.UpdateConfigAndRestart(incoming)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": true})
		return
	}

	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

func (s *Server) handleTestConnection(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var mCfg config.MQTTConfig
	if err := json.NewDecoder(r.Body).Decode(&mCfg); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": err.Error()})
		return
	}

	if mCfg.Password == "" {
		mCfg.Password = config.Get().MQTT.Password
	}

	err := mqtt.TestConnection(mCfg)
	w.Header().Set("Content-Type", "application/json")
	if err != nil {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": err.Error()})
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": true})
}
