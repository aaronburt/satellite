package webui

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"satellite/internal/config"
	"satellite/internal/diagnostics"
	"satellite/internal/mqtt"
	"satellite/internal/output"
)

func (s *Server) connectionDiagnostics() map[string]diagnostics.Snapshot {
	s.mu.Lock()
	webhook := s.telemetryWebhook
	s.mu.Unlock()
	wh := diagnostics.Snapshot{Status: "unavailable", RecentErrors: []diagnostics.Event{}}
	if webhook != nil {
		wh = webhook.Diagnostics()
	}
	return map[string]diagnostics.Snapshot{
		"mqtt":              s.mqttClient.Diagnostics(),
		"telemetry_webhook": wh,
	}
}

func (s *Server) handleDiagnostics(w http.ResponseWriter, r *http.Request) {
	if !requireGET(w, r) {
		return
	}
	writeJSON(w, http.StatusOK, s.connectionDiagnostics())
}

func (s *Server) handleDiagnosticTest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Transport string `json:"transport"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Choose MQTT or telemetry_webhook for the connection test.")
		return
	}
	cfg := config.Get()
	started := time.Now()
	var err error
	var problem diagnostics.Problem
	var message string
	switch req.Transport {
	case "mqtt":
		if strings.TrimSpace(cfg.MQTT.Broker) == "" {
			writeError(w, http.StatusBadRequest, "Save an MQTT broker address in Settings first.")
			return
		}
		err = mqtt.TestConnection(cfg.MQTT)
		if err != nil {
			problem = mqtt.ConnectionProblem(err)
		}
		message = "Broker connection and authentication succeeded. This test does not verify topic permissions or Home Assistant discovery."
	case "telemetry_webhook":
		if !cfg.TelemetryWebhook.Enabled || strings.TrimSpace(cfg.TelemetryWebhook.URL) == "" {
			writeError(w, http.StatusBadRequest, "Enable and save a telemetry webhook in Settings first.")
			return
		}
		err = output.SendTest(cfg.TelemetryWebhook, cfg.NodeID, s.collector.Last())
		if err != nil {
			problem = output.WebhookProblem(err)
		}
		message = "The webhook accepted the test snapshot (HTTP 2xx). This does not verify downstream processing."
	default:
		writeError(w, http.StatusBadRequest, "Choose MQTT or telemetry_webhook for the connection test.")
		return
	}
	result := map[string]interface{}{
		"success":     err == nil,
		"duration_ms": time.Since(started).Milliseconds(),
		"message":     message,
	}
	if err != nil {
		result["message"] = problem.Message
		result["hint"] = problem.Hint
		result["code"] = problem.Code
	}
	writeJSON(w, http.StatusOK, result)
}
