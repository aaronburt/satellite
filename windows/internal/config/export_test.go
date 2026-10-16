package config

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"testing"
)

func TestExportImportPlaintext(t *testing.T) {
	orig := DefaultConfig()
	orig.NodeID = "test-node-1"
	orig.MQTT.Broker = "tcp://192.168.1.100:1883"
	orig.MQTT.Username = "mqtt_user"
	orig.MQTT.Password = "secret_password"
	orig.Expose.CPU = true
	orig.Expose.GPU = true
	orig.Expose.Storage = true

	data, err := ExportConfig(orig, "")
	if err != nil {
		t.Fatalf("unexpected export error: %v", err)
	}

	res, err := ImportConfig(data, ImportOptions{})
	if err != nil {
		t.Fatalf("unexpected import error: %v", err)
	}

	if res.Config.NodeID != orig.NodeID {
		t.Errorf("expected node ID %q, got %q", orig.NodeID, res.Config.NodeID)
	}
	if res.Config.MQTT.Broker != orig.MQTT.Broker {
		t.Errorf("expected broker %q, got %q", orig.MQTT.Broker, res.Config.MQTT.Broker)
	}
	if res.Config.MQTT.Password != orig.MQTT.Password {
		t.Errorf("expected password %q, got %q", orig.MQTT.Password, res.Config.MQTT.Password)
	}
	if !res.Config.Expose.GPU {
		t.Errorf("expected GPU exposure to be true")
	}
	if res.SourceAppVersion != Version {
		t.Errorf("expected source app version %q, got %q", Version, res.SourceAppVersion)
	}
	if res.IsNewerVersion {
		t.Errorf("expected isNewerVersion to be false")
	}
}

func TestExportImportEncrypted(t *testing.T) {
	orig := DefaultConfig()
	orig.MQTT.Broker = "tcp://secure.broker:8883"
	orig.MQTT.Password = "very-secret-token"

	passphrase := "myStrongPass123!"
	encryptedData, err := ExportConfig(orig, passphrase)
	if err != nil {
		t.Fatalf("unexpected export error: %v", err)
	}

	var env ExportEnvelope
	if err := json.Unmarshal(encryptedData, &env); err != nil {
		t.Fatalf("unexpected json unmarshal error: %v", err)
	}
	if !env.Encrypted {
		t.Fatalf("expected envelope to be marked encrypted")
	}
	if env.Data != nil {
		t.Fatalf("expected plaintext data to be nil in encrypted envelope")
	}

	res, err := ImportConfig(encryptedData, ImportOptions{Passphrase: passphrase})
	if err != nil {
		t.Fatalf("unexpected import error: %v", err)
	}
	if res.Config.MQTT.Password != orig.MQTT.Password {
		t.Errorf("expected password %q, got %q", orig.MQTT.Password, res.Config.MQTT.Password)
	}

	_, err = ImportConfig(encryptedData, ImportOptions{})
	if !errors.Is(err, ErrPassphraseRequired) {
		t.Errorf("expected ErrPassphraseRequired, got %v", err)
	}

	_, err = ImportConfig(encryptedData, ImportOptions{Passphrase: "wrong-pass"})
	if !errors.Is(err, ErrInvalidPassphrase) {
		t.Errorf("expected ErrInvalidPassphrase, got %v", err)
	}
}

func TestImportTamperedPayload(t *testing.T) {
	orig := DefaultConfig()
	passphrase := "secret123"
	data, err := ExportConfig(orig, passphrase)
	if err != nil {
		t.Fatalf("unexpected export error: %v", err)
	}

	var env ExportEnvelope
	if err := json.Unmarshal(data, &env); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	rawCipher, err := base64.StdEncoding.DecodeString(env.Payload)
	if err != nil {
		t.Fatalf("decode payload error: %v", err)
	}
	rawCipher[len(rawCipher)-1] ^= 0xFF
	env.Payload = base64.StdEncoding.EncodeToString(rawCipher)

	tamperedData, _ := json.Marshal(env)
	_, err = ImportConfig(tamperedData, ImportOptions{Passphrase: passphrase})
	if !errors.Is(err, ErrInvalidPassphrase) {
		t.Errorf("expected ErrInvalidPassphrase for tampered payload, got %v", err)
	}
}

func TestImportResetNodeID(t *testing.T) {
	orig := DefaultConfig()
	orig.NodeID = "old-machine-desktop"
	orig.MQTT.ClientID = "satellite-old-machine-desktop"

	data, err := ExportConfig(orig, "")
	if err != nil {
		t.Fatalf("export error: %v", err)
	}

	res, err := ImportConfig(data, ImportOptions{ResetNodeID: true})
	if err != nil {
		t.Fatalf("import error: %v", err)
	}

	def := DefaultConfig()
	if res.Config.NodeID != def.NodeID {
		t.Errorf("expected adapted node ID %q, got %q", def.NodeID, res.Config.NodeID)
	}
	if res.Config.MQTT.ClientID != "satellite-"+def.NodeID {
		t.Errorf("expected adapted client ID, got %q", res.Config.MQTT.ClientID)
	}
}

func TestImportVersionComparison(t *testing.T) {
	orig := DefaultConfig()
	data, err := ExportConfig(orig, "")
	if err != nil {
		t.Fatalf("export error: %v", err)
	}

	var env ExportEnvelope
	if err := json.Unmarshal(data, &env); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	env.AppVersion = "99.0.0"
	futureData, _ := json.Marshal(env)
	resFuture, err := ImportConfig(futureData, ImportOptions{})
	if err != nil {
		t.Fatalf("import error: %v", err)
	}
	if !resFuture.IsNewerVersion {
		t.Errorf("expected IsNewerVersion to be true for 99.0.0")
	}

	env.AppVersion = "0.1.0"
	oldData, _ := json.Marshal(env)
	resOld, err := ImportConfig(oldData, ImportOptions{})
	if err != nil {
		t.Fatalf("import error: %v", err)
	}
	if resOld.IsNewerVersion {
		t.Errorf("expected IsNewerVersion to be false for 0.1.0")
	}
}

func TestImportInvalidEnvelope(t *testing.T) {
	_, err := ImportConfig([]byte("invalid json"), ImportOptions{})
	if !errors.Is(err, ErrInvalidEnvelope) {
		t.Errorf("expected ErrInvalidEnvelope for malformed json, got %v", err)
	}

	badFormat := []byte(`{"format":"unknown","app_version":"0.18.0"}`)
	_, err = ImportConfig(badFormat, ImportOptions{})
	if !errors.Is(err, ErrInvalidEnvelope) {
		t.Errorf("expected ErrInvalidEnvelope for bad format, got %v", err)
	}

	emptyEncrypted := []byte(`{"format":"satellite-config","app_version":"0.18.0","encrypted":true}`)
	_, err = ImportConfig(emptyEncrypted, ImportOptions{Passphrase: "test"})
	if !errors.Is(err, ErrInvalidEnvelope) {
		t.Errorf("expected ErrInvalidEnvelope for incomplete encrypted envelope, got %v", err)
	}
}
