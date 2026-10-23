package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"satellite/internal/config"
	"satellite/internal/testutil"
)

func TestNoConfigFlagsIsNotHandled(t *testing.T) {
	var out, errOut bytes.Buffer
	if handled, _ := runConfigCommand(config.DefaultConfig(), "", "", "", true, &out, &errOut); handled {
		t.Fatal("no -export/-import should fall through to normal start-up")
	}
}

func TestExportThenImportFromCommandLine(t *testing.T) {
	testutil.IsolateConfigDir(t, t.TempDir())
	dir := t.TempDir()
	file := filepath.Join(dir, "backup.satconfig")

	src := config.DefaultConfig()
	src.NodeID = "old-pc"
	src.MQTT.Broker = "tcp://broker:1883"
	src.MQTT.Password = "pw"

	var out, errOut bytes.Buffer
	handled, code := runConfigCommand(src, file, "", "secret", true, &out, &errOut)
	if !handled || code != 0 || !strings.Contains(out.String(), "exported successfully") {
		t.Fatalf("export: handled=%v code=%d out=%q err=%q", handled, code, out.String(), errOut.String())
	}
	raw, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(raw, []byte(`"pw"`)) {
		t.Fatal("encrypted export file contains the password")
	}
	if runtime.GOOS != "windows" {
		if fi, _ := os.Stat(file); fi.Mode().Perm() != 0o600 {
			t.Errorf("export file mode = %o, want 600", fi.Mode().Perm())
		}
	}

	out.Reset()
	handled, code = runConfigCommand(config.DefaultConfig(), "", file, "secret", false, &out, &errOut)
	if !handled || code != 0 {
		t.Fatalf("import: code=%d err=%q", code, errOut.String())
	}
	saved, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if saved.NodeID != "old-pc" || saved.MQTT.Broker != "tcp://broker:1883" || saved.MQTT.Password != "pw" {
		t.Fatalf("imported config = %+v", saved)
	}
	if !strings.Contains(out.String(), "Active Node ID: old-pc") || strings.Contains(out.String(), "newer version") {
		t.Errorf("import output = %q", out.String())
	}
}

func TestImportFromNewerVersionPrintsNotice(t *testing.T) {
	testutil.IsolateConfigDir(t, t.TempDir())
	data, _ := config.ExportConfig(config.DefaultConfig(), "")
	var env map[string]any
	_ = json.Unmarshal(data, &env)
	env["app_version"] = "99.0.0"
	data, _ = json.Marshal(env)
	file := filepath.Join(t.TempDir(), "new.satconfig")
	_ = os.WriteFile(file, data, 0o600)

	var out, errOut bytes.Buffer
	if _, code := runConfigCommand(config.DefaultConfig(), "", file, "", true, &out, &errOut); code != 0 {
		t.Fatalf("code %d: %s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "newer version of Satellite (v99.0.0)") {
		t.Fatalf("output = %q", out.String())
	}
}

func TestConfigCommandFailures(t *testing.T) {
	testutil.IsolateConfigDir(t, t.TempDir())
	dir := t.TempDir()
	encrypted := filepath.Join(dir, "enc.satconfig")
	var out, errOut bytes.Buffer
	_, _ = runConfigCommand(config.DefaultConfig(), encrypted, "", "right", true, &out, &errOut)

	cases := []struct {
		name              string
		export, imp, pass string
		wantErr           string
	}{
		{"export to missing directory", filepath.Join(dir, "nope", "x"), "", "", "Error writing exported file"},
		{"import missing file", "", filepath.Join(dir, "missing"), "", "Error reading import file"},
		{"import wrong passphrase", "", encrypted, "wrong", "Error importing configuration"},
		{"import without passphrase", "", encrypted, "", "passphrase required"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var out, errOut bytes.Buffer
			handled, code := runConfigCommand(config.DefaultConfig(), tc.export, tc.imp, tc.pass, true, &out, &errOut)
			if !handled || code != 1 || !strings.Contains(errOut.String(), tc.wantErr) {
				t.Fatalf("handled=%v code=%d stderr=%q", handled, code, errOut.String())
			}
		})
	}

	// A failed import must leave the saved config untouched.
	before, _ := config.Load()
	if before.NodeID != config.DefaultConfig().NodeID {
		t.Fatalf("config changed by failed imports: %+v", before)
	}
}
