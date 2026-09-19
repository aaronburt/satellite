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
	"strings"
	"sync"

	"satellite/internal/config"
	"satellite/internal/mqtt"
	"satellite/internal/telemetry"
)

//go:embed static/index.html
var indexHTML []byte

type Server struct {
	mu           sync.Mutex
	server       *http.Server
	listener     net.Listener
	port         int
	token        string
	collector    *telemetry.Collector
	mqttClient   *mqtt.Client
	running      bool
	webUIEnabled bool
	jsonEnabled  bool
	apiKey       string
}

func NewServer(collector *telemetry.Collector, mqttClient *mqtt.Client) *Server {
	return &Server{
		collector:    collector,
		mqttClient:   mqttClient,
		webUIEnabled: true,
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

func (s *Server) SetWebUIEnabled(enabled bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.webUIEnabled = enabled
}

func (s *Server) IsWebUIEnabled() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.webUIEnabled
}

func (s *Server) SetJSONEnabled(enabled bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.jsonEnabled = enabled
}

func (s *Server) IsJSONEnabled() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.jsonEnabled
}

func (s *Server) SetAPIKey(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.apiKey = key
}

func (s *Server) APIKey() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.apiKey
}

func getAPIKeyFromRequest(r *http.Request) string {
	if key := r.URL.Query().Get("api_key"); key != "" {
		return key
	}
	if key := r.Header.Get("X-API-Key"); key != "" {
		return key
	}
	for name, values := range r.Header {
		if strings.EqualFold(name, "x-api-key") && len(values) > 0 {
			return values[0]
		}
	}
	return ""
}

func (s *Server) Start(preferredPort int) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return s.port, nil
	}

	cfg := config.Get()
	bindHost := cfg.BindAddress
	if bindHost == "" {
		bindHost = "127.0.0.1"
	}

	addr := fmt.Sprintf("%s:%d", bindHost, preferredPort)
	ln, err := net.Listen("tcp", addr)
	if err != nil && preferredPort != 0 {
		ln, err = net.Listen("tcp", fmt.Sprintf("%s:0", bindHost))
	}
	if err != nil {
		return 0, err
	}

	s.listener = ln
	s.port = ln.Addr().(*net.TCPAddr).Port
	if s.token == "" {
		s.token = generateSecureToken()
	}

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

	webUIMiddleware := func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if !s.IsWebUIEnabled() {
				http.NotFound(w, r)
				return
			}
			next(w, r)
		}
	}

	mux.HandleFunc("/", webUIMiddleware(func(w http.ResponseWriter, r *http.Request) {
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
	}))

	mux.HandleFunc("/api/status", webUIMiddleware(authMiddleware(s.handleStatus)))
	mux.HandleFunc("/api/config", webUIMiddleware(authMiddleware(s.handleConfig)))
	mux.HandleFunc("/api/test-connection", webUIMiddleware(authMiddleware(s.handleTestConnection)))
	mux.HandleFunc("/json", s.handleJSON)
	mux.HandleFunc("/json/", s.handleJSON)

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

func (s *Server) handleJSON(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if !s.IsJSONEnabled() {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "Not Found: JSON endpoint is disabled",
		})
		return
	}

	providedKey := getAPIKeyFromRequest(r)
	s.mu.Lock()
	expectedKey := s.apiKey
	s.mu.Unlock()

	if expectedKey == "" || providedKey != expectedKey {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "Unauthorized: invalid or missing api_key",
		})
		return
	}

	snap := s.collector.Last()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(snap)
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
		if incoming.BindAddress == "" {
			incoming.BindAddress = current.BindAddress
		}
		if incoming.JSONEnabled && incoming.APIKey == "" {
			if current.APIKey != "" {
				incoming.APIKey = current.APIKey
			} else {
				incoming.APIKey = config.GenerateAPIKey()
			}
		}
		incoming.WebUIPort = current.WebUIPort

		if err := config.Save(incoming); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": err.Error()})
			return
		}

		s.SetJSONEnabled(incoming.JSONEnabled)
		s.SetAPIKey(incoming.APIKey)
		if incoming.JSONEnabled && !s.IsRunning() {
			_, _ = s.Start(incoming.WebUIPort)
		} else if !incoming.JSONEnabled && !s.IsWebUIEnabled() && s.IsRunning() {
			s.Stop()
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
