package diagnostics

import (
	"context"
	"crypto/x509"
	"errors"
	"net"
	"net/url"
	"strings"
	"sync"
	"time"
)

type Problem struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Hint    string `json:"hint"`
}

type Event struct {
	Problem
	At        time.Time `json:"at"`
	Operation string    `json:"operation"`
}

type Snapshot struct {
	Status           string     `json:"status"`
	Since            time.Time  `json:"since"`
	Attempts         uint64     `json:"attempts"`
	Reconnects       uint64     `json:"reconnects"`
	Publishes        uint64     `json:"publishes"`
	Failures         uint64     `json:"failures"`
	LastAttempt      *time.Time `json:"last_attempt,omitempty"`
	LastConnected    *time.Time `json:"last_connected,omitempty"`
	LastDisconnected *time.Time `json:"last_disconnected,omitempty"`
	LastPublished    *time.Time `json:"last_published,omitempty"`
	NextRetry        *time.Time `json:"next_retry,omitempty"`
	CurrentError     *Event     `json:"current_error,omitempty"`
	RecentErrors     []Event    `json:"recent_errors"`
}

type Tracker struct {
	mu sync.Mutex
	s  Snapshot
}

func New() *Tracker {
	return &Tracker{s: Snapshot{Since: time.Now(), RecentErrors: []Event{}}}
}

func (t *Tracker) Attempt() {
	t.mu.Lock()
	defer t.mu.Unlock()
	now := time.Now()
	t.s.Attempts++
	t.s.LastAttempt = &now
	t.s.NextRetry = nil
}

func (t *Tracker) Connected() {
	t.mu.Lock()
	defer t.mu.Unlock()
	now := time.Now()
	if t.s.LastConnected != nil {
		t.s.Reconnects++
	}
	t.s.LastConnected = &now
	t.s.NextRetry = nil
	if t.s.CurrentError != nil && t.s.CurrentError.Operation == "connect" {
		t.s.CurrentError = nil
	}
}

func (t *Tracker) Disconnected() {
	t.mu.Lock()
	defer t.mu.Unlock()
	now := time.Now()
	t.s.LastDisconnected = &now
	t.s.NextRetry = nil
}

func (t *Tracker) Published() {
	t.mu.Lock()
	defer t.mu.Unlock()
	now := time.Now()
	t.s.Publishes++
	t.s.LastPublished = &now
	t.s.NextRetry = nil
	if t.s.CurrentError != nil && t.s.CurrentError.Operation == "publish" {
		t.s.CurrentError = nil
	}
}

func (t *Tracker) Failed(operation string, problem Problem) {
	t.mu.Lock()
	defer t.mu.Unlock()
	ev := Event{Problem: problem, At: time.Now(), Operation: operation}
	t.s.Failures++
	t.s.CurrentError = &ev
	t.s.RecentErrors = append(t.s.RecentErrors, ev)
	if len(t.s.RecentErrors) > 5 {
		t.s.RecentErrors = t.s.RecentErrors[len(t.s.RecentErrors)-5:]
	}
}

func (t *Tracker) RetryAt(at time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if at.IsZero() {
		t.s.NextRetry = nil
	} else {
		t.s.NextRetry = &at
	}
}

func (t *Tracker) Snapshot(status string) Snapshot {
	t.mu.Lock()
	defer t.mu.Unlock()
	s := t.s
	s.Status = status
	s.RecentErrors = append([]Event{}, t.s.RecentErrors...)
	return s
}

func Explain(err error) Problem {
	var dns *net.DNSError
	var unknownCA x509.UnknownAuthorityError
	var hostname x509.HostnameError
	var cert x509.CertificateInvalidError
	var netErr net.Error
	var urlErr *url.Error
	switch {
	case errors.As(err, &unknownCA), errors.As(err, &hostname), errors.As(err, &cert):
		return Problem{"tls", "The server certificate could not be verified.", "Check the server hostname, certificate expiry and trusted certificate authority."}
	case errors.As(err, &dns):
		return Problem{"dns", "The server name could not be resolved.", "Check the hostname and this device's DNS and network connection."}
	case errors.Is(err, context.DeadlineExceeded), errors.As(err, &netErr) && netErr.Timeout():
		return Problem{"timeout", "The server did not respond in time.", "Check the address, port, firewall and whether the server is running."}
	case errors.As(err, &urlErr) && urlErr.Op == "parse":
		return Problem{"address", "The server address is invalid.", "Check the URL format, hostname and port in Settings."}
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "certificate"), strings.Contains(msg, "tls"), strings.Contains(msg, "first record does not look"):
		return Problem{"tls", "A secure connection could not be established.", "Check the TLS setting, server port and certificate configuration."}
	case strings.Contains(msg, "refused"), strings.Contains(msg, "actively refused"):
		return Problem{"refused", "The server refused the connection.", "Check the port and that the broker or webhook server is listening."}
	default:
		return Problem{"connection", "The connection or delivery failed.", "Check the network, server settings and server logs. Satellite will retry according to its reporting policy."}
	}
}

func HTTPProblem(code int) Problem {
	switch code {
	case 401, 403:
		return Problem{"authorization", "The server rejected the request credentials or permissions.", "Check the bearer token and the endpoint's access rules."}
	case 404:
		return Problem{"not_found", "The webhook endpoint was not found.", "Check the webhook URL and that the receiver is enabled."}
	case 429:
		return Problem{"rate_limit", "The server is limiting requests.", "Increase the minimum reporting interval or check the receiver's rate limits."}
	default:
		if code >= 500 {
			return Problem{"server", "The webhook server returned an error.", "Check the receiver's health and logs. Temporary failures are retried."}
		}
		return Problem{"http", "The webhook server did not accept the request.", "Check the endpoint and its expected payload format and access rules."}
	}
}
