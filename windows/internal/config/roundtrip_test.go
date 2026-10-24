package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"satellite/internal/testutil"
)

// fillAllFields sets every exported field of v (recursively) to a distinct
// non-zero value, so round-trip tests notice any field that gets dropped.
func fillAllFields(t *testing.T, v reflect.Value, prefix string, counter *int) {
	t.Helper()
	for i := 0; i < v.NumField(); i++ {
		f := v.Field(i)
		name := prefix + v.Type().Field(i).Name
		*counter++
		switch f.Kind() {
		case reflect.String:
			f.SetString(fmt.Sprintf("value-%s-%d", strings.ToLower(name), *counter))
		case reflect.Bool:
			f.SetBool(true)
		case reflect.Int, reflect.Int64, reflect.Int32:
			f.SetInt(int64(1000 + *counter))
		case reflect.Struct:
			fillAllFields(t, f, name+".", counter)
		default:
			t.Fatalf("fillAllFields: unsupported kind %s for %s; extend the helper", f.Kind(), name)
		}
	}
}

func fullyPopulatedConfig(t *testing.T) Config {
	var cfg Config
	n := 0
	fillAllFields(t, reflect.ValueOf(&cfg).Elem(), "", &n)
	return cfg
}

// Every setting must survive export -> import, otherwise moving a config to
// a new machine silently resets it. This test fails when a new Config field
// is added without being carried through ExportData.
func TestExportImportPreservesEveryField(t *testing.T) {
	for _, pass := range []string{"", "correct horse battery staple"} {
		name := "plaintext"
		if pass != "" {
			name = "encrypted"
		}
		t.Run(name, func(t *testing.T) {
			orig := fullyPopulatedConfig(t)
			data, err := ExportConfig(orig, pass)
			if err != nil {
				t.Fatal(err)
			}
			res, err := ImportConfig(data, ImportOptions{Passphrase: pass})
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(res.Config, orig) {
				gotJSON, _ := json.MarshalIndent(res.Config, "", " ")
				wantJSON, _ := json.MarshalIndent(orig, "", " ")
				t.Fatalf("round trip changed config\n got: %s\nwant: %s", gotJSON, wantJSON)
			}
			if res.SourceAppVersion != Version || res.IsNewerVersion {
				t.Fatalf("version info = %q newer=%v", res.SourceAppVersion, res.IsNewerVersion)
			}
		})
	}
}

func TestSaveLoadPreservesEveryField(t *testing.T) {
	testutil.IsolateConfigDir(t, t.TempDir())
	orig := fullyPopulatedConfig(t)
	// Load normalises Port/WebUIPort to match each other only when one is 0.
	if err := Save(orig); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(loaded, orig) {
		t.Fatalf("save/load changed config:\n got %+v\nwant %+v", loaded, orig)
	}
}

func TestEncryptedExportDoesNotLeakSecrets(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MQTT.Password = "hunter2-mqtt-password"
	cfg.APIKey = "api-key-should-be-hidden"
	cfg.Webhook.Secret = "webhook-secret-value"

	data, err := ExportConfig(cfg, "pass")
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{cfg.MQTT.Password, cfg.APIKey, cfg.Webhook.Secret} {
		if strings.Contains(string(data), secret) {
			t.Fatalf("encrypted export contains %q in clear text", secret)
		}
	}
	var env ExportEnvelope
	if err := json.Unmarshal(data, &env); err != nil {
		t.Fatal(err)
	}
	if !env.Encrypted || env.Data != nil || env.KDF != "pbkdf2-sha256" || env.Iterations < 100000 {
		t.Fatalf("unexpected envelope: %+v", env)
	}
}

func TestEncryptedExportsAreSaltedAndNonced(t *testing.T) {
	cfg := DefaultConfig()
	a, _ := ExportConfig(cfg, "pass")
	b, _ := ExportConfig(cfg, "pass")
	var ea, eb ExportEnvelope
	_ = json.Unmarshal(a, &ea)
	_ = json.Unmarshal(b, &eb)
	if ea.Salt == eb.Salt || ea.Nonce == eb.Nonce || ea.Payload == eb.Payload {
		t.Fatal("two exports with the same passphrase must use fresh salt and nonce")
	}
}

func TestImportPassphraseHandling(t *testing.T) {
	data, err := ExportConfig(DefaultConfig(), "  secret  ")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ImportConfig(data, ImportOptions{}); !errors.Is(err, ErrPassphraseRequired) {
		t.Fatalf("no passphrase: %v", err)
	}
	if _, err := ImportConfig(data, ImportOptions{Passphrase: "   "}); !errors.Is(err, ErrPassphraseRequired) {
		t.Fatalf("blank passphrase: %v", err)
	}
	// Surrounding whitespace is trimmed on both sides.
	if _, err := ImportConfig(data, ImportOptions{Passphrase: "secret"}); err != nil {
		t.Fatalf("trimmed passphrase should work: %v", err)
	}
	if _, err := ImportConfig(data, ImportOptions{Passphrase: "Secret"}); !errors.Is(err, ErrInvalidPassphrase) {
		t.Fatalf("wrong case: %v", err)
	}
}

// An exported file may come from anywhere, so the KDF cost it asks for must
// be bounded or importing it could hang the agent.
func TestImportRejectsExcessiveKDFIterations(t *testing.T) {
	data, _ := ExportConfig(DefaultConfig(), "pass")
	var env ExportEnvelope
	_ = json.Unmarshal(data, &env)
	env.Iterations = 2_000_000_000
	hostile, _ := json.Marshal(env)

	done := make(chan error, 1)
	go func() {
		_, err := ImportConfig(hostile, ImportOptions{Passphrase: "pass"})
		done <- err
	}()
	select {
	case err := <-done:
		if !errors.Is(err, ErrInvalidEnvelope) {
			t.Fatalf("err = %v, want ErrInvalidEnvelope", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("import with huge iteration count did not return promptly")
	}
}

func TestImportRejectsMalformedEncryptedFields(t *testing.T) {
	data, _ := ExportConfig(DefaultConfig(), "pass")
	var good ExportEnvelope
	_ = json.Unmarshal(data, &good)

	mutations := map[string]func(*ExportEnvelope){
		"bad salt hex":    func(e *ExportEnvelope) { e.Salt = "zz" },
		"empty salt":      func(e *ExportEnvelope) { e.Salt = "" },
		"bad nonce hex":   func(e *ExportEnvelope) { e.Nonce = "not-hex" },
		"short nonce":     func(e *ExportEnvelope) { e.Nonce = "0011" },
		"bad payload b64": func(e *ExportEnvelope) { e.Payload = "%%%" },
		"empty payload":   func(e *ExportEnvelope) { e.Payload = "" },
		"negative iters":  func(e *ExportEnvelope) { e.Iterations = -5 },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			env := good
			mutate(&env)
			raw, _ := json.Marshal(env)
			_, err := ImportConfig(raw, ImportOptions{Passphrase: "pass"})
			if err == nil {
				t.Fatal("expected an error")
			}
			if !errors.Is(err, ErrInvalidEnvelope) {
				t.Fatalf("err = %v, want ErrInvalidEnvelope", err)
			}
		})
	}
}

func TestImportPlaintextWithoutDataIsRejected(t *testing.T) {
	raw := []byte(`{"format":"satellite-config","app_version":"0.25.0","encrypted":false}`)
	if _, err := ImportConfig(raw, ImportOptions{}); !errors.Is(err, ErrInvalidEnvelope) {
		t.Fatalf("err = %v", err)
	}
}

func TestImportFillsDefaultsForMissingValues(t *testing.T) {
	raw := []byte(`{"format":"satellite-config","app_version":"0.20.0","encrypted":false,
		"data":{"node_id":"desk","mqtt":{"broker":"tcp://b:1883"},"expose":{"cpu":true},
		"json_enabled":true,"webhook":{}}}`)
	res, err := ImportConfig(raw, ImportOptions{})
	if err != nil {
		t.Fatal(err)
	}
	def := DefaultConfig()
	c := res.Config
	if c.NodeID != "desk" || c.MQTT.Broker != "tcp://b:1883" {
		t.Fatalf("explicit values lost: %+v", c)
	}
	if c.IntervalSec != def.IntervalSec || c.BindAddress != "127.0.0.1" ||
		c.UpdateRepo != def.UpdateRepo || c.MQTT.TopicPrefix != "satellite" || c.Webhook.MinLevel != "info" {
		t.Fatalf("defaults not applied: %+v", c)
	}
	if c.APIKey == "" || len(c.APIKey) != 32 {
		t.Fatalf("JSON API enabled without key must get a generated key, got %q", c.APIKey)
	}
	if res.IsNewerVersion {
		t.Fatal("0.20.0 is older than the running version")
	}
}

func TestImportResetAPIKey(t *testing.T) {
	cfg := DefaultConfig()
	cfg.APIKey = "original"
	data, _ := ExportConfig(cfg, "")
	res, err := ImportConfig(data, ImportOptions{ResetAPIKey: true})
	if err != nil {
		t.Fatal(err)
	}
	if res.Config.APIKey == "original" || len(res.Config.APIKey) != 32 {
		t.Fatalf("API key not regenerated: %q", res.Config.APIKey)
	}
}

func TestIsVersionNewer(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"1.0.0", "0.9.9", true},
		{"0.10.0", "0.9.0", true}, // numeric, not lexical
		{"0.25.1", "0.25.0", true},
		{"v0.26.0", "0.25.0", true},
		{"V1.0.0", "v0.99.99", true},
		{"0.25.0", "0.25.0", false},
		{"0.25.0-beta+meta", "0.25.0", false},
		{"0.24.9", "0.25.0", false},
		{"", "0.0.1", false},
		{"garbage", "0.0.0", false},
	}
	for _, tc := range cases {
		if got := isVersionNewer(tc.a, tc.b); got != tc.want {
			t.Errorf("isVersionNewer(%q, %q) = %v, want %v", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestSaveRestrictsPermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permissions do not apply on Windows")
	}
	testutil.IsolateConfigDir(t, t.TempDir())
	if err := Save(DefaultConfig()); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(ConfigFilePath())
	if err != nil {
		t.Fatal(err)
	}
	// The file holds the MQTT password and API key.
	if perm := fi.Mode().Perm(); perm != 0o600 {
		t.Fatalf("config file mode = %o, want 600", perm)
	}
	di, _ := os.Stat(ConfigDir())
	if perm := di.Mode().Perm(); perm != 0o700 {
		t.Fatalf("config dir mode = %o, want 700", perm)
	}
	if _, err := os.Stat(ConfigFilePath() + ".tmp"); !os.IsNotExist(err) {
		t.Fatal("temporary file left behind after save")
	}
}

func TestSaveFailsWhenDirectoryCannotBeCreated(t *testing.T) {
	dir := t.TempDir()
	blocker := filepath.Join(dir, "file")
	if err := os.WriteFile(blocker, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	// The config dir would have to live under a regular file.
	testutil.IsolateConfigDir(t, blocker)
	before := Get()
	if err := Save(DefaultConfig()); err == nil {
		t.Fatal("expected Save to fail")
	}
	if !reflect.DeepEqual(Get(), before) {
		t.Fatal("failed Save must not change the in-memory config")
	}
}

func TestLoadKeepsDefaultsForFieldsMissingFromFile(t *testing.T) {
	testutil.IsolateConfigDir(t, t.TempDir())
	if err := os.MkdirAll(ConfigDir(), 0o700); err != nil {
		t.Fatal(err)
	}
	// An old config file written before most settings existed.
	if err := os.WriteFile(ConfigFilePath(), []byte(`{"node_id":"legacy","unknown_field":42}`), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	def := DefaultConfig()
	if cfg.NodeID != "legacy" || cfg.IntervalSec != def.IntervalSec || cfg.Expose != def.Expose ||
		cfg.UpdateRepo != def.UpdateRepo || cfg.HeartbeatSec != def.HeartbeatSec {
		t.Fatalf("missing fields should keep defaults: %+v", cfg)
	}
	if Get().NodeID != "legacy" {
		t.Fatal("Load should update the in-memory config")
	}
}

func TestLoadReturnsErrorForUnreadableFile(t *testing.T) {
	testutil.IsolateConfigDir(t, t.TempDir())
	// A directory where the file should be cannot be read as a file.
	if err := os.MkdirAll(ConfigFilePath(), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(); err == nil {
		t.Fatal("expected error")
	}
}

func TestDefaultConfigNormalisesHostname(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.NodeID == "" || cfg.NodeID != strings.ToLower(cfg.NodeID) || strings.Contains(cfg.NodeID, " ") {
		t.Fatalf("NodeID %q should be lower-case without spaces", cfg.NodeID)
	}
	if cfg.MQTT.ClientID != "satellite-"+cfg.NodeID {
		t.Fatalf("ClientID = %q", cfg.MQTT.ClientID)
	}
	if cfg.BindAddress != "127.0.0.1" {
		t.Fatal("web UI must bind to loopback by default")
	}
}

func TestGetAndSaveAreSafeConcurrently(t *testing.T) {
	testutil.IsolateConfigDir(t, t.TempDir())
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(2)
		go func(i int) {
			defer wg.Done()
			cfg := DefaultConfig()
			cfg.IntervalSec = i + 1
			_ = Save(cfg)
		}(i)
		go func() { defer wg.Done(); _ = Get() }()
	}
	wg.Wait()
	loaded, err := Load()
	if err != nil {
		t.Fatalf("config file corrupted by concurrent saves: %v", err)
	}
	if loaded.IntervalSec < 1 || loaded.IntervalSec > 20 {
		t.Fatalf("unexpected interval %d", loaded.IntervalSec)
	}
}
