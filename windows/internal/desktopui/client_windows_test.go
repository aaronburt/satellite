package desktopui

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"testing"
	"time"
)

func TestValidateURL(t *testing.T) {
	for _, target := range []string{
		"http://127.0.0.1:1234/?token=secret",
		"http://[::1]:1234/?token=secret",
		"http://192.168.1.2:1234/?token=secret",
	} {
		if err := validateURL(target); err != nil {
			t.Fatalf("rejected valid listener: %v", err)
		}
	}
	for _, target := range []string{
		"https://example.com/?token=secret",
		"file:///C:/Windows/system32/calc.exe",
		"http://127.0.0.1:1234/",
		"http://user@127.0.0.1:1234/?token=secret",
		"http://example.com:1234/?token=secret",
	} {
		if validateURL(target) == nil {
			t.Fatal("accepted invalid UI address")
		}
	}
}

func TestClientProcessLifecycle(t *testing.T) {
	client := &Client{command: func() (*exec.Cmd, error) {
		cmd := exec.Command(os.Args[0], "-test.run=^TestClientHelper$")
		cmd.Env = append(os.Environ(), "SATELLITE_UI_TEST_HELPER=1")
		return cmd, nil
	}}
	t.Cleanup(client.Close)
	url := "http://127.0.0.1:1234/?token=secret"
	if err := client.Open(url); err != nil {
		t.Fatal(err)
	}
	first := client.cmd.Process.Pid
	if err := client.Open("http://127.0.0.1:5678/?token=next"); err != nil {
		t.Fatal(err)
	}
	if client.cmd.Process.Pid != first {
		t.Fatal("reopening started another client process")
	}
	done := client.done
	client.Close()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("client survived agent shutdown")
	}
	if err := client.Open(url); err != nil {
		t.Fatal(err)
	}
	client.cmd.Process.Kill()
	<-client.done
	if err := client.Open(url); err != nil {
		t.Fatalf("could not reopen after client exit: %v", err)
	}
}

func TestClientStartupFailure(t *testing.T) {
	client := &Client{command: func() (*exec.Cmd, error) {
		cmd := exec.Command(os.Args[0], "-test.run=^TestClientHelper$")
		cmd.Env = append(os.Environ(), "SATELLITE_UI_TEST_HELPER=fail")
		return cmd, nil
	}}
	t.Cleanup(client.Close)
	if client.Open("http://127.0.0.1:1234/?token=secret") == nil {
		t.Fatal("client startup failure was ignored")
	}
	if client.cmd != nil {
		t.Fatal("failed client retained")
	}
}

func TestClientHelper(t *testing.T) {
	mode := os.Getenv("SATELLITE_UI_TEST_HELPER")
	if mode == "" {
		return
	}
	if mode == "fail" {
		os.Exit(1)
	}
	decoder := json.NewDecoder(os.Stdin)
	var target string
	if decoder.Decode(&target) != nil || validateURL(target) != nil {
		os.Exit(2)
	}
	fmt.Println("ready")
	for decoder.Decode(&target) == nil {
		if validateURL(target) != nil {
			os.Exit(2)
		}
	}
	os.Exit(0)
}
