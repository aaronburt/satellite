package output

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Webhook delivery recovery policy (desktop). See docs/mqtt-and-webhooks.md.
//
// Only the newest snapshot is kept, in memory. A failed POST is retried with
// the original payload (and timestamp) using capped exponential backoff with
// jitter, until it succeeds, a permanent error comes back, the attempt or age
// limit is hit, or a newer snapshot replaces it.
const (
	retryMaxAttempts   = 5                // total attempts per snapshot, including the first
	retryBaseDelay     = 2 * time.Second  // delay before the second attempt
	retryMaxDelay      = 60 * time.Second // cap on computed backoff
	retryMaxRetryAfter = 5 * time.Minute  // cap on a server's Retry-After
	retryMaxAge        = 5 * time.Minute  // snapshots older than this are dropped
)

// StatusError is returned by Send when the endpoint answers outside 2xx.
type StatusError struct {
	Code       int
	RetryAfter time.Duration // parsed Retry-After header, 0 if absent
}

func (e *StatusError) Error() string {
	return "webhook returned status " + strconv.Itoa(e.Code)
}

// IsRetryable reports whether a failed delivery is worth retrying. Network
// failures and timeouts, 408, 425, 429 and 5xx are retryable; any other
// status (bad URL, auth failure, 4xx) is permanent.
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}
	var se *StatusError
	if errors.As(err, &se) {
		switch {
		case se.Code == http.StatusRequestTimeout, se.Code == http.StatusTooEarly,
			se.Code == http.StatusTooManyRequests, se.Code >= 500:
			return true
		default:
			return false
		}
	}
	var pe *permanentError
	return !errors.As(err, &pe)
}

// permanentError marks failures that happen before a request is sent
// (empty or malformed URL) so they are never retried.
type permanentError struct{ err error }

func (e *permanentError) Error() string { return e.err.Error() }
func (e *permanentError) Unwrap() error { return e.err }

// RetryDelay returns how long to wait before attempt number next (2 for the
// first retry). jitter is a value in [0,1); the delay lands in [d/2, d].
// A server's Retry-After wins when it is longer, capped at retryMaxRetryAfter.
func RetryDelay(next int, err error, jitter float64) time.Duration {
	d := retryBaseDelay
	for i := 2; i < next && d < retryMaxDelay; i++ {
		d *= 2
	}
	if d > retryMaxDelay {
		d = retryMaxDelay
	}
	d = d/2 + time.Duration(jitter*float64(d/2))

	var se *StatusError
	if errors.As(err, &se) && se.RetryAfter > d {
		d = se.RetryAfter
		if d > retryMaxRetryAfter {
			d = retryMaxRetryAfter
		}
	}
	return d
}

// parseRetryAfter accepts delta-seconds or an HTTP date.
func parseRetryAfter(v string, now time.Time) time.Duration {
	v = strings.TrimSpace(v)
	if v == "" {
		return 0
	}
	if secs, err := strconv.Atoi(v); err == nil {
		if secs < 0 {
			return 0
		}
		return time.Duration(secs) * time.Second
	}
	if t, err := http.ParseTime(v); err == nil && t.After(now) {
		return t.Sub(now)
	}
	return 0
}
