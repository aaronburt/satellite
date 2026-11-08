package desktopui

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"golang.org/x/sys/windows"
)

type Client struct {
	mu      sync.Mutex
	command func() (*exec.Cmd, error)
	cmd     *exec.Cmd
	input   io.WriteCloser
	done    chan struct{}
}

func New() *Client {
	return &Client{command: func() (*exec.Cmd, error) {
		executable, err := os.Executable()
		if err != nil {
			return nil, err
		}
		cmd := exec.Command(executable, "--ui-client")
		cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_NO_WINDOW}
		return cmd, nil
	}}
}

func validateURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "http" || u.User != nil || u.Path != "/" || u.Port() == "" || u.Query().Get("token") == "" {
		return fmt.Errorf("invalid Satellite UI address")
	}
	if u.Hostname() != "localhost" && net.ParseIP(u.Hostname()) == nil {
		return fmt.Errorf("Satellite UI requires a local listener address")
	}
	return nil
}

func (c *Client) Open(rawURL string) error {
	if err := validateURL(rawURL); err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cmd != nil {
		select {
		case <-c.done:
			c.input.Close()
			c.cmd = nil
		default:
			allowForeground.Call(uintptr(c.cmd.Process.Pid))
			if err := json.NewEncoder(c.input).Encode(rawURL); err == nil {
				return nil
			}
			c.stop()
		}
	}
	cmd, err := c.command()
	if err != nil {
		return err
	}
	input, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	output, err := cmd.StdoutPipe()
	if err != nil {
		input.Close()
		return err
	}
	if err := cmd.Start(); err != nil {
		input.Close()
		output.Close()
		return err
	}
	allowForeground.Call(uintptr(cmd.Process.Pid))
	ready := make(chan error, 1)
	go func() {
		line, err := bufio.NewReader(output).ReadString('\n')
		if err == nil && line != "ready\n" {
			err = fmt.Errorf("unexpected UI client response")
		}
		ready <- err
	}()
	err = json.NewEncoder(input).Encode(rawURL)
	if err == nil {
		select {
		case err = <-ready:
		case <-time.After(20 * time.Second):
			err = fmt.Errorf("UI client startup timed out")
		}
	}
	if err != nil {
		input.Close()
		cmd.Process.Kill()
		cmd.Wait()
		return fmt.Errorf("could not start Satellite window; install Microsoft Edge WebView2 Runtime or use Open in Browser: %w", err)
	}
	c.cmd, c.input, c.done = cmd, input, make(chan struct{})
	done := c.done
	go func() {
		cmd.Wait()
		close(done)
	}()
	return nil
}

func (c *Client) stop() {
	if c.cmd == nil {
		return
	}
	c.input.Close()
	select {
	case <-c.done:
	case <-time.After(2 * time.Second):
		c.cmd.Process.Kill()
		<-c.done
	}
	c.cmd = nil
}

func (c *Client) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.stop()
}
