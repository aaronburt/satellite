package webui

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/subtle"
	_ "embed"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"satellite/internal/capabilities"
	"satellite/internal/config"
	"satellite/internal/logger"
	"satellite/internal/mqtt"
	"satellite/internal/output"
	"satellite/internal/telemetry"
	"satellite/internal/toast"
	"satellite/internal/updater"
)

//go:embed static/index.html
var indexHTML []byte

// devIndexPath is read instead of the embedded page in dev mode (--debug),
// so the page can be edited without rebuilding.
const devIndexPath = "internal/webui/static/index.html"

const (
	// maxBodyBytes caps request bodies. Config files are a few KB.
	maxBodyBytes = 1 << 20
	// retireGrace is how long an old server gets to finish in-flight
	// responses after the listen address changes.
	retireGrace = 3 * time.Second
	// logKeepAlive is how often an idle log stream sends a comment line.
	logKeepAlive = 25 * time.Second
	// logBuffer is how many log events a slow stream can fall behind before
	// events are dropped for that stream.
	logBuffer = 256
)

type Server struct {
	mu           sync.Mutex
	server       *http.Server
	cancelBase   context.CancelFunc
	listener     net.Listener
	host         string
	port         int
	token        string
	collector    *telemetry.Collector
	mqttClient   *mqtt.Client
	updater      *updater.Checker
	caps         capabilities.PlatformCapabilities
	running      bool
	webUIEnabled bool
	jsonEnabled  bool
	apiKey       string
	devMode      bool
}

func NewServer(collector *telemetry.Collector, mqttClient *mqtt.Client) *Server {
	return &Server{
		collector:    collector,
		mqttClient:   mqttClient,
		caps:         capabilities.Detect(),
		webUIEnabled: true,
	}
}

func (s *Server) SetCapabilities(caps capabilities.PlatformCapabilities) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.caps = caps
}

func (s *Server) SetUpdater(u *updater.Checker) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.updater = u
}

// SetDevMode makes the page load from disk on every request (used by --debug).
func (s *Server) SetDevMode(enabled bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.devMode = enabled
}

func generateSecureToken() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func generateNonce() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return base64.StdEncoding.EncodeToString(b)
}

// secretMatches compares a provided secret in constant time. An empty
// expected secret never matches.
func secretMatches(provided, expected string) bool {
	if provided == "" || expected == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(provided), []byte(expected)) == 1
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

// URL is the address to open the web UI in a browser, including the token.
func (s *Server) URL() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	host := browseHost(s.host)
	if host == "" {
		host = "127.0.0.1"
	}
	return fmt.Sprintf("http://%s/?token=%s", net.JoinHostPort(host, strconv.Itoa(s.port)), s.token)
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
	return r.Header.Get("X-API-Key")
}

// normalizeBindHost turns a configured bind address into a host for
// net.Listen. Blank means loopback.
func normalizeBindHost(bind string) string {
	bind = strings.TrimSpace(bind)
	bind = strings.TrimSuffix(strings.TrimPrefix(bind, "["), "]")
	if bind == "" {
		return "127.0.0.1"
	}
	return bind
}

// browseHost is the host a browser should use to reach a server bound to
// bind. It is empty for wildcard binds, where any local address works.
func browseHost(bind string) string {
	host := normalizeBindHost(bind)
	if ip := net.ParseIP(host); ip != nil && ip.IsUnspecified() {
		return ""
	}
	return host
}

// validateListen checks a bind address and port before they are saved.
func validateListen(bind string, port int) error {
	host := normalizeBindHost(bind)
	if host != "localhost" && net.ParseIP(host) == nil {
		return fmt.Errorf("listen address %q must be an IP address such as 127.0.0.1 or 0.0.0.0", bind)
	}
	if port < 0 || port > 65535 {
		return fmt.Errorf("port %d must be between 0 and 65535", port)
	}
	return nil
}

func listen(host string, port int) (net.Listener, error) {
	ln, err := net.Listen("tcp", net.JoinHostPort(host, strconv.Itoa(port)))
	if err != nil && port != 0 {
		ln, err = net.Listen("tcp", net.JoinHostPort(host, "0"))
	}
	return ln, err
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]interface{}{"success": false, "error": msg})
}

func (s *Server) Start(preferredPort int) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return s.port, nil
	}

	cfg := config.Get()
	if preferredPort <= 0 {
		preferredPort = cfg.GetPort()
	}
	host := normalizeBindHost(cfg.BindAddress)

	ln, err := listen(host, preferredPort)
	if err != nil {
		return 0, err
	}
	if s.token == "" {
		s.token = generateSecureToken()
	}
	s.serveLocked(host, ln)
	logger.Info("api", fmt.Sprintf("HTTP server listening on %s", ln.Addr()))
	return s.port, nil
}

// serveLocked starts a new http.Server on ln and makes it current.
// Callers hold s.mu.
func (s *Server) serveLocked(host string, ln net.Listener) {
	baseCtx, cancel := context.WithCancel(context.Background())
	httpSrv := &http.Server{
		Handler:           s.routes(),
		ReadHeaderTimeout: 10 * time.Second,
		BaseContext:       func(net.Listener) context.Context { return baseCtx },
	}
	s.server = httpSrv
	s.cancelBase = cancel
	s.listener = ln
	s.host = host
	s.port = ln.Addr().(*net.TCPAddr).Port
	s.running = true

	go func() {
		_ = httpSrv.Serve(ln)
		cancel()
		s.mu.Lock()
		// Only clear state if this is still the current server. After a
		// restart the old server's Serve returns late and must not mark the
		// new one as stopped.
		if s.server == httpSrv {
			s.server = nil
			s.cancelBase = nil
			s.listener = nil
			s.running = false
		}
		s.mu.Unlock()
	}()
}

// relisten moves the running server to a new address without dropping
// in-flight requests or changing the token. The old server finishes its
// current responses and is then closed. If the new address can't be bound,
// the server stays on the old address and the error is returned.
func (s *Server) relisten(host string, port int) (int, error) {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return 0, errors.New("server is not running")
	}
	oldSrv, oldCancel, oldLn := s.server, s.cancelBase, s.listener
	oldHost, oldPort := s.host, s.port
	if port == 0 {
		port = oldPort
	}

	// Free the old address first, so the same port can be reused when only
	// the bind address changes. Open connections are not affected.
	_ = oldLn.Close()
	ln, err := listen(host, port)
	if err != nil {
		listenErr := err
		ln, err = listen(oldHost, oldPort)
		if err != nil {
			s.mu.Unlock()
			return 0, fmt.Errorf("%v (and could not reopen %s: %v)", listenErr, net.JoinHostPort(oldHost, strconv.Itoa(oldPort)), err)
		}
		s.serveLocked(oldHost, ln)
		newPort := s.port
		s.mu.Unlock()
		go retire(oldSrv, oldCancel)
		return newPort, listenErr
	}
	s.serveLocked(host, ln)
	newPort := s.port
	s.mu.Unlock()

	go retire(oldSrv, oldCancel)
	logger.Info("api", fmt.Sprintf("HTTP server moved to %s", ln.Addr()))
	return newPort, nil
}

// retire ends an old server's log streams, lets in-flight responses finish,
// then closes it.
func retire(srv *http.Server, cancel context.CancelFunc) {
	if cancel != nil {
		cancel()
	}
	if srv == nil {
		return
	}
	ctx, done := context.WithTimeout(context.Background(), retireGrace)
	defer done()
	_ = srv.Shutdown(ctx)
	_ = srv.Close()
}

func (s *Server) routes() http.Handler {
	mux := http.NewServeMux()

	api := func(next http.HandlerFunc) http.HandlerFunc {
		return s.webUIMiddleware(s.authMiddleware(func(w http.ResponseWriter, r *http.Request) {
			r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
			next(w, r)
		}))
	}

	mux.HandleFunc("/", s.webUIMiddleware(s.handleIndex))
	mux.HandleFunc("/api/status", api(s.handleStatus))
	mux.HandleFunc("/api/capabilities", api(s.handleCapabilities))
	mux.HandleFunc("/api/config", api(s.handleConfig))
	mux.HandleFunc("/api/config/export", api(s.handleConfigExport))
	mux.HandleFunc("/api/config/import", api(s.handleConfigImport))
	mux.HandleFunc("/api/action", api(s.handleAction))
	mux.HandleFunc("/api/test-connection", api(s.handleTestConnection))
	mux.HandleFunc("/api/test-webhook", api(s.handleTestWebhook))
	mux.HandleFunc("/api/test-telemetry-webhook", api(s.handleTestTelemetryWebhook))
	mux.HandleFunc("/api/update", api(s.handleUpdateStatus))
	mux.HandleFunc("/api/update/check", api(s.handleUpdateCheck))
	mux.HandleFunc("/api/logs/stream", api(s.handleLogsStream))
	mux.HandleFunc("/json", s.handleJSON)
	mux.HandleFunc("/json/", s.handleJSON)

	return securityHeaders(mux)
}

// securityHeaders applies headers every response should carry. The token is
// in the page URL, so the referrer must never be sent.
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}

func (s *Server) authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		expected := s.Token()
		if !secretMatches(r.Header.Get("X-API-Token"), expected) && !secretMatches(r.URL.Query().Get("token"), expected) {
			logger.Warn("api", fmt.Sprintf("Unauthorized access attempt to %s from %s", r.URL.Path, r.RemoteAddr))
			writeJSON(w, http.StatusUnauthorized, map[string]interface{}{
				"error": "Unauthorized: invalid or missing ephemeral token",
			})
			return
		}
		next(w, r)
	}
}

func (s *Server) webUIMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.IsWebUIEnabled() {
			http.NotFound(w, r)
			return
		}
		next(w, r)
	}
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	if !secretMatches(r.URL.Query().Get("token"), s.Token()) {
		logger.Warn("api", fmt.Sprintf("Unauthorized webui access attempt from %s", r.RemoteAddr))
		http.Error(w, "This link has expired or is invalid. Reopen the web UI from the Satellite tray icon.", http.StatusUnauthorized)
		return
	}

	s.mu.Lock()
	devMode := s.devMode
	s.mu.Unlock()

	rawHTML := indexHTML
	if devMode {
		if diskHTML, err := os.ReadFile(devIndexPath); err == nil {
			rawHTML = diskHTML
		}
	}

	nonce := generateNonce()
	rendered := bytes.ReplaceAll(rawHTML, []byte("{{VERSION}}"), []byte(config.Version))
	rendered = bytes.ReplaceAll(rendered, []byte("{{CSP_NONCE}}"), []byte(nonce))

	w.Header().Set("Content-Security-Policy", strings.Join([]string{
		"default-src 'none'",
		"script-src 'nonce-" + nonce + "'",
		"style-src 'unsafe-inline'",
		"img-src 'self' data:",
		"connect-src 'self'",
		"base-uri 'none'",
		"form-action 'none'",
		"frame-ancestors 'none'",
	}, "; "))
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(rendered)
}

func (s *Server) handleJSON(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if !s.IsJSONEnabled() {
		writeJSON(w, http.StatusNotFound, map[string]interface{}{
			"error": "Not Found: JSON endpoint is disabled",
		})
		return
	}

	if !secretMatches(getAPIKeyFromRequest(r), s.APIKey()) {
		logger.Warn("api", fmt.Sprintf("Unauthorized /json access attempt from %s", r.RemoteAddr))
		writeJSON(w, http.StatusUnauthorized, map[string]interface{}{
			"error": "Unauthorized: invalid or missing api_key",
		})
		return
	}

	writeJSON(w, http.StatusOK, s.collector.Last())
}

func (s *Server) Port() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.port
}

// Stop shuts the server down and forgets the token, so old links stop
// working.
func (s *Server) Stop() {
	s.mu.Lock()
	srv, cancel := s.server, s.cancelBase
	s.server = nil
	s.cancelBase = nil
	s.listener = nil
	s.token = ""
	s.running = false
	s.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	if srv != nil {
		_ = srv.Close()
	}
}

// requireGET rejects anything but GET with 405 and an Allow header. HEAD is
// rejected too: these endpoints are read by the page and scripts with GET,
// and a HEAD on the log stream would otherwise hold a connection open.
func requireGET(w http.ResponseWriter, r *http.Request) bool {
	if r.Method == http.MethodGet {
		return true
	}
	w.Header().Set("Allow", http.MethodGet)
	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	return false
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	if !requireGET(w, r) {
		return
	}
	s.mu.Lock()
	u := s.updater
	s.mu.Unlock()

	var updateStatus interface{}
	if u != nil {
		updateStatus = u.Status()
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"mqtt_status":   string(s.mqttClient.Status()),
		"telemetry":     s.collector.Last(),
		"update_status": updateStatus,
	})
}

func (s *Server) handleCapabilities(w http.ResponseWriter, r *http.Request) {
	if !requireGET(w, r) {
		return
	}
	s.mu.Lock()
	caps := s.caps
	s.mu.Unlock()
	writeJSON(w, http.StatusOK, caps)
}

func (s *Server) handleUpdateStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	s.mu.Lock()
	u := s.updater
	s.mu.Unlock()

	if u == nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"available":       false,
			"current_version": config.Version,
		})
		return
	}
	writeJSON(w, http.StatusOK, u.Status())
}

func (s *Server) handleUpdateCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	s.mu.Lock()
	u := s.updater
	s.mu.Unlock()

	if u == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]interface{}{
			"error": "Update checker is not configured",
		})
		return
	}

	status, err := u.Check(r.Context())
	code := http.StatusOK
	if err != nil {
		code = http.StatusInternalServerError
	}
	writeJSON(w, code, status)
}

// redactConfig blanks every secret before a config is sent to the browser.
func redactConfig(cfg config.Config) config.Config {
	cfg.MQTT.Password = ""
	cfg.TelemetryWebhook.BearerToken = ""
	cfg.Webhook.Secret = ""
	return cfg
}

// mergeIncomingConfig fills in what a save request left blank. The request
// was decoded on top of prev, so fields it didn't mention keep their saved
// values; blank secrets mean "keep the saved one".
func mergeIncomingConfig(prev, next config.Config) config.Config {
	if next.MQTT.Password == "" {
		next.MQTT.Password = prev.MQTT.Password
	}
	if next.TelemetryWebhook.BearerToken == "" {
		next.TelemetryWebhook.BearerToken = prev.TelemetryWebhook.BearerToken
	}
	if next.Webhook.Secret == "" {
		next.Webhook.Secret = prev.Webhook.Secret
	}
	next.TelemetryWebhook.URL = strings.TrimSpace(next.TelemetryWebhook.URL)
	next.Webhook.URL = strings.TrimSpace(next.Webhook.URL)
	next.Webhook.MinLevel = strings.ToLower(strings.TrimSpace(next.Webhook.MinLevel))
	if next.Webhook.MinLevel == "" {
		next.Webhook.MinLevel = "info"
	}
	if next.TelemetryWebhook.MinIntervalSec < 0 {
		next.TelemetryWebhook.MinIntervalSec = 0
	}
	if next.IntervalSec <= 0 {
		next.IntervalSec = 5
	}
	next.NodeID = strings.TrimSpace(next.NodeID)
	if next.NodeID == "" {
		next.NodeID = prev.NodeID
	}
	next.BindAddress = strings.TrimSpace(next.BindAddress)
	if next.BindAddress == "" {
		next.BindAddress = prev.BindAddress
	}
	if next.BindAddress == "" {
		next.BindAddress = "127.0.0.1"
	}
	if next.JSONEnabled && next.APIKey == "" {
		if prev.APIKey != "" {
			next.APIKey = prev.APIKey
		} else {
			next.APIKey = config.GenerateAPIKey()
		}
	}
	if next.Port > 0 {
		next.WebUIPort = next.Port
	} else if next.WebUIPort > 0 {
		next.Port = next.WebUIPort
	} else {
		next.Port = 0
		next.WebUIPort = 0
	}
	if next.UpdateRepo == "" {
		next.UpdateRepo = prev.UpdateRepo
	}
	if next.HeartbeatSec <= 0 {
		next.HeartbeatSec = prev.HeartbeatSec
	}
	return next
}

// listenResult tells the page where the web UI lives after a save.
type listenResult struct {
	Moved bool   `json:"moved"`
	Host  string `json:"host,omitempty"`
	Port  int    `json:"port"`
}

// prepareListen validates next's address and, if it changed while the
// server is running, moves the server there. It runs before the config is
// saved, so an address that can't be used is never saved.
func (s *Server) prepareListen(prev, next config.Config) (*listenResult, error) {
	if err := validateListen(next.BindAddress, next.GetPort()); err != nil {
		return nil, err
	}
	newHost := normalizeBindHost(next.BindAddress)
	changed := next.GetPort() != prev.GetPort() || newHost != normalizeBindHost(prev.BindAddress)
	if !changed || !s.IsRunning() {
		return nil, nil
	}
	port, err := s.relisten(newHost, next.GetPort())
	if err != nil {
		return nil, fmt.Errorf("could not listen on %s: %v", net.JoinHostPort(newHost, strconv.Itoa(next.GetPort())), err)
	}
	return &listenResult{Moved: true, Host: browseHost(newHost), Port: port}, nil
}

// applyConfig pushes a saved config to the running services.
func (s *Server) applyConfig(next config.Config) {
	s.mu.Lock()
	u := s.updater
	s.mu.Unlock()
	if u != nil {
		u.SetEnabled(next.CheckUpdates)
	}

	s.SetJSONEnabled(next.JSONEnabled)
	s.SetAPIKey(next.APIKey)
	logger.UpdateConfig(next.Webhook, next.NodeID)

	if next.JSONEnabled && !s.IsRunning() {
		_, _ = s.Start(next.GetPort())
	} else if !next.JSONEnabled && !s.IsWebUIEnabled() && s.IsRunning() {
		s.Stop()
	}

	if s.mqttClient != nil {
		s.mqttClient.UpdateConfigAndRestart(next)
	}
}

func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, redactConfig(config.Get()))
	case http.MethodPost:
		prev := config.Get()
		next := prev
		if err := json.NewDecoder(r.Body).Decode(&next); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		next = mergeIncomingConfig(prev, next)

		moved, err := s.prepareListen(prev, next)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := config.Save(next); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		s.applyConfig(next)
		logger.Info("api", "Configuration updated via WebUI")

		resp := map[string]interface{}{"success": true}
		if moved != nil {
			resp["listen"] = moved
		}
		writeJSON(w, http.StatusOK, resp)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleConfigExport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Passphrase string `json:"passphrase"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	cfg := config.Get()
	data, err := config.ExportConfig(cfg, req.Passphrase)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, exportFilename(cfg.NodeID)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

// exportFilename builds a download name that is safe in a header.
func exportFilename(nodeID string) string {
	safe := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_', r == '.':
			return r
		default:
			return '_'
		}
	}, nodeID)
	if safe == "" {
		safe = "node"
	}
	return "satellite-config-" + safe + ".satconfig"
}

func (s *Server) handleConfigImport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var rawData []byte
	var passphrase string
	// Default to adapting the node ID to this machine, matching the UI
	// checkbox and the --reset-node-id flag.
	resetNodeID := true

	if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
		if err := r.ParseMultipartForm(maxBodyBytes); err != nil {
			writeError(w, http.StatusBadRequest, "failed to parse multipart form")
			return
		}
		file, _, err := r.FormFile("file")
		if err != nil {
			writeError(w, http.StatusBadRequest, "missing file in upload")
			return
		}
		defer file.Close()
		buf := new(bytes.Buffer)
		if _, err := buf.ReadFrom(file); err != nil {
			writeError(w, http.StatusBadRequest, "failed to read uploaded file")
			return
		}
		rawData = buf.Bytes()
		passphrase = r.FormValue("passphrase")
		if r.FormValue("reset_node_id") == "false" {
			resetNodeID = false
		}
	} else {
		var req struct {
			Content     string `json:"content"`
			Passphrase  string `json:"passphrase"`
			ResetNodeID *bool  `json:"reset_node_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		rawData = []byte(req.Content)
		passphrase = req.Passphrase
		if req.ResetNodeID != nil {
			resetNodeID = *req.ResetNodeID
		}
	}

	res, err := config.ImportConfig(rawData, config.ImportOptions{
		Passphrase:  passphrase,
		ResetNodeID: resetNodeID,
	})
	if err != nil {
		// A wrong passphrase is 422, not 401: 401 means the page's token is
		// no longer valid.
		status := http.StatusBadRequest
		if errors.Is(err, config.ErrInvalidPassphrase) || errors.Is(err, config.ErrPassphraseRequired) {
			status = http.StatusUnprocessableEntity
		}
		writeError(w, status, err.Error())
		return
	}

	prev := config.Get()
	next := res.Config
	if next.HeartbeatSec <= 0 {
		next.HeartbeatSec = prev.HeartbeatSec
	}

	moved, err := s.prepareListen(prev, next)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := config.Save(next); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.applyConfig(next)
	logger.Info("config", fmt.Sprintf("Imported configuration from Satellite v%s (ResetNodeID=%v)", res.SourceAppVersion, resetNodeID))

	resp := map[string]interface{}{
		"success":            true,
		"source_app_version": res.SourceAppVersion,
		"is_newer_version":   res.IsNewerVersion,
		"node_id":            next.NodeID,
	}
	if moved != nil {
		resp["listen"] = moved
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleTestConnection(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var mCfg config.MQTTConfig
	if err := json.NewDecoder(r.Body).Decode(&mCfg); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	mCfg = withSavedMQTTPassword(mCfg, config.Get().MQTT)

	if err := mqtt.TestConnection(mCfg); err != nil {
		writeError(w, http.StatusOK, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"success": true})
}

// withSavedMQTTPassword fills a blank password from the saved config, but
// only for the saved broker and user, so a test can't send it elsewhere.
func withSavedMQTTPassword(req, saved config.MQTTConfig) config.MQTTConfig {
	if req.Password == "" &&
		strings.TrimSpace(req.Broker) == strings.TrimSpace(saved.Broker) &&
		req.Username == saved.Username {
		req.Password = saved.Password
	}
	return req
}

// savedSecretFor returns the saved secret only when url is blank or is the
// saved URL, so a test can't send a stored secret to a new address. A blank
// url becomes the saved URL.
func savedSecretFor(url, secret, savedURL, savedSecret string) (string, string) {
	url = strings.TrimSpace(url)
	if url == "" {
		url = strings.TrimSpace(savedURL)
	}
	if secret == "" && url == strings.TrimSpace(savedURL) {
		secret = savedSecret
	}
	return url, secret
}

func (s *Server) handleTestWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var wCfg config.WebhookConfig
	if err := json.NewDecoder(r.Body).Decode(&wCfg); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	current := config.Get()
	wCfg.URL, wCfg.Secret = savedSecretFor(wCfg.URL, wCfg.Secret, current.Webhook.URL, current.Webhook.Secret)

	if err := logger.SendTestWebhook(wCfg, current.NodeID); err != nil {
		writeError(w, http.StatusOK, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"success": true})
}

// handleTestTelemetryWebhook sends one live snapshot to the telemetry webhook.
// A blank URL falls back to the saved one; a blank token falls back to the
// saved token only for the saved URL.
func (s *Server) handleTestTelemetryWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var wh config.TelemetryWebhookConfig
	if err := json.NewDecoder(r.Body).Decode(&wh); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	current := config.Get()
	wh.URL, wh.BearerToken = savedSecretFor(wh.URL, wh.BearerToken, current.TelemetryWebhook.URL, current.TelemetryWebhook.BearerToken)

	snap := s.collector.Collect(current.Expose)
	if err := output.SendTest(wh, current.NodeID, snap); err != nil {
		writeError(w, http.StatusOK, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"success": true})
}

func (s *Server) handleAction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Action string `json:"action"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	action := strings.ToLower(strings.TrimSpace(req.Action))

	if action == "test_toast" {
		go func() {
			_ = toast.Show(toast.Notification{
				Title:   "Satellite Test",
				Message: "Quick action test notification from Web UI.",
			})
		}()
		writeJSON(w, http.StatusOK, map[string]interface{}{"success": true})
		return
	}

	reg := s.mqttClient.Registry()
	if reg == nil {
		writeError(w, http.StatusInternalServerError, "action registry unavailable")
		return
	}

	if err := reg.Execute(action); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"success": true})
}

// handleLogsStream sends log events as server-sent events. Only this
// goroutine writes to w: the logger listener just queues events, so nothing
// can write after the handler has returned.
func (s *Server) handleLogsStream(w http.ResponseWriter, r *http.Request) {
	if !requireGET(w, r) {
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	events := make(chan logger.Event, logBuffer)
	unsub := logger.AddListener(func(ev logger.Event) {
		select {
		case events <- ev:
		default: // client too slow: drop rather than block the logger
		}
	})
	defer unsub()

	fmt.Fprintf(w, ": connected\n\n")
	flusher.Flush()

	keepAlive := time.NewTicker(logKeepAlive)
	defer keepAlive.Stop()

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case ev := <-events:
			data, err := json.Marshal(ev)
			if err != nil {
				continue
			}
			if _, err := fmt.Fprintf(w, "data: %s\n\n", data); err != nil {
				return
			}
			flusher.Flush()
		case <-keepAlive.C:
			if _, err := fmt.Fprintf(w, ": ping\n\n"); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}
