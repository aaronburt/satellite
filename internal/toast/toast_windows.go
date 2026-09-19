//go:build windows

package toast

import (
	"bytes"
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"os/exec"
	"syscall"
	"unicode/utf16"
)

func encodeUTF16LE(s string) []byte {
	runes := utf16.Encode([]rune(s))
	encoded := make([]byte, len(runes)*2)
	for i, r := range runes {
		encoded[i*2] = byte(r)
		encoded[i*2+1] = byte(r >> 8)
	}
	return encoded
}

func xmlEscape(s string) string {
	var buf bytes.Buffer
	_ = xml.EscapeText(&buf, []byte(s))
	return buf.String()
}

func Show(n Notification) error {
	escapedTitle := xmlEscape(n.Title)
	escapedMessage := xmlEscape(n.Message)
	escapedURL := xmlEscape(n.URL)

	var launchAttr string
	if escapedURL != "" {
		launchAttr = fmt.Sprintf(` activationType="protocol" launch="%s"`, escapedURL)
	}

	var audioTag string
	if n.Silent {
		audioTag = `<audio silent="true"/>`
	}

	xmlDoc := fmt.Sprintf(`<toast%s><visual><binding template="ToastGeneric"><text>%s</text><text>%s</text></binding></visual>%s</toast>`,
		launchAttr, escapedTitle, escapedMessage, audioTag)

	script := fmt.Sprintf(`[Windows.UI.Notifications.ToastNotificationManager, Windows.UI.Notifications, ContentType = WindowsRuntime] | Out-Null
[Windows.Data.Xml.Dom.XmlDocument, Windows.Data.Xml.Dom.XmlDocument, ContentType = WindowsRuntime] | Out-Null
$xml = New-Object Windows.Data.Xml.Dom.XmlDocument
$xml.LoadXml(@'
%s
'@)
$toast = New-Object Windows.UI.Notifications.ToastNotification $xml
[Windows.UI.Notifications.ToastNotificationManager]::CreateToastNotifier('Satellite').Show($toast)
`, xmlDoc)

	encoded := base64.StdEncoding.EncodeToString(encodeUTF16LE(script))
	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-EncodedCommand", encoded)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("toast notification failed: %w: %s", err, string(output))
	}
	return nil
}
