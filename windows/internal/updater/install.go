package updater

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

type ReleaseAsset struct {
	Name   string `json:"name"`
	URL    string `json:"browser_download_url"`
	Size   int64  `json:"size"`
	Digest string `json:"digest"`
}

func (c *Checker) SetAutoUpdate(enabled bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.autoUpdate = enabled
}

func (c *Checker) checkBackground(ctx context.Context) (CheckStatus, error) {
	c.mu.RLock()
	automatic := c.autoUpdate
	c.mu.RUnlock()
	if automatic && runtime.GOOS == "windows" {
		return c.CheckAndInstall(ctx)
	}
	return c.Check(ctx)
}

func (c *Checker) CheckAndInstall(ctx context.Context) (CheckStatus, error) {
	c.installMu.Lock()
	defer c.installMu.Unlock()
	status, err := c.Check(ctx)
	if err != nil || !status.Available {
		return status, err
	}
	if runtime.GOOS != "windows" {
		return c.recordError(fmt.Errorf("automatic installation requires Windows; use %s", status.ReleaseURL))
	}
	if c.installedVersion == status.LatestVersion {
		return status, nil
	}
	c.mu.RLock()
	assets := append([]ReleaseAsset(nil), c.assets...)
	repo := c.repo
	c.mu.RUnlock()
	expected := "Satellite-Setup-" + status.LatestVersion + ".exe"
	for _, asset := range assets {
		if asset.Name != expected {
			continue
		}
		if err := downloadAndLaunch(ctx, asset, repo, status.LatestVersion); err != nil {
			return c.recordError(err)
		}
		c.installedVersion = status.LatestVersion
		return status, nil
	}
	return c.recordError(fmt.Errorf("release has no Windows installer named %s", expected))
}

func downloadAndLaunch(ctx context.Context, asset ReleaseAsset, repo, version string) error {
	parsed, err := url.Parse(asset.URL)
	prefix := "/" + repo + "/releases/download/" + version + "/"
	if err != nil || parsed.Scheme != "https" || parsed.Host != "github.com" || parsed.User != nil || !strings.HasPrefix(parsed.Path, prefix) || filepath.Base(parsed.Path) != asset.Name {
		return fmt.Errorf("invalid installer download URL")
	}
	digest, err := hex.DecodeString(strings.TrimPrefix(asset.Digest, "sha256:"))
	if !strings.HasPrefix(asset.Digest, "sha256:") || err != nil || len(digest) != sha256.Size {
		return fmt.Errorf("release installer has no valid SHA-256 digest")
	}
	const maxSize = 256 << 20
	if asset.Size <= 0 || asset.Size > maxSize {
		return fmt.Errorf("invalid installer size")
	}
	dir, err := os.MkdirTemp("", "satellite-update-")
	if err != nil {
		return err
	}
	launched := false
	defer func() {
		if !launched {
			_ = os.RemoveAll(dir)
		}
	}()
	path := filepath.Join(dir, asset.Name)
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, asset.URL, nil)
	if err != nil {
		return err
	}
	client := &http.Client{CheckRedirect: func(req *http.Request, via []*http.Request) error {
		host := req.URL.Hostname()
		if len(via) >= 10 || req.URL.Scheme != "https" || (host != "github.com" && host != "release-assets.githubusercontent.com" && host != "objects.githubusercontent.com") {
			return fmt.Errorf("unsafe installer redirect")
		}
		return nil
	}}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("installer download returned status %d", resp.StatusCode)
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	hash := sha256.New()
	size, copyErr := io.Copy(io.MultiWriter(file, hash), io.LimitReader(resp.Body, maxSize+1))
	closeErr := file.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	if size != asset.Size || hex.EncodeToString(hash.Sum(nil)) != hex.EncodeToString(digest) {
		return fmt.Errorf("installer integrity verification failed")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	cmd := exec.Command(path)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("open installer: %w", err)
	}
	launched = true
	go func() { _ = cmd.Wait(); _ = os.RemoveAll(dir) }()
	return nil
}
