package mqtt

import (
	"errors"
	"strings"

	"satellite/internal/diagnostics"
)

type connectionError struct {
	code byte
	err  error
}

func (e *connectionError) Error() string { return e.err.Error() }
func (e *connectionError) Unwrap() error { return e.err }

func ConnectionProblem(err error) diagnostics.Problem {
	var rejection *connectionError
	if errors.As(err, &rejection) {
		switch rejection.code {
		case 0x86, 0x87, 0x8a, 0x8c:
			return diagnostics.Problem{Code: "authorization", Message: "The broker rejected the credentials or permissions.", Hint: "Check the MQTT username, password and broker access rules."}
		case 0x85:
			return diagnostics.Problem{Code: "client_id", Message: "The broker rejected the client ID.", Hint: "Use a unique client ID accepted by the broker."}
		case 0x8e:
			return diagnostics.Problem{Code: "client_id", Message: "Another MQTT connection took over this client ID.", Hint: "Give every device and MQTT client its own unique client ID."}
		default:
			return diagnostics.Problem{Code: "broker", Message: "The broker did not accept the MQTT connection.", Hint: "Check the broker's health, connection limits and MQTT 5 support."}
		}
	}
	return diagnostics.Explain(err)
}

func (c *Client) Diagnostics() diagnostics.Snapshot {
	c.mu.RLock()
	defer c.mu.RUnlock()
	status := strings.ToLower(string(c.status))
	if strings.TrimSpace(c.cfg.MQTT.Broker) == "" {
		status = "disabled"
	}
	return c.diagnostics.Snapshot(status)
}
