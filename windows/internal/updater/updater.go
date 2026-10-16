package updater

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"satellite/internal/logger"
)

const (
	DefaultRepo           = "aaronburt/satellite"
	DefaultCheckInterval  = 24 * time.Hour
	DefaultRequestTimeout = 10 * time.Second
)

type GitHubRelease struct {
	TagName     string `json:"tag_name"`
	Name        string `json:"name"`
	HTMLURL     string `json:"html_url"`
	Draft       bool   `json:"draft"`
	Prerelease  bool   `json:"prerelease"`
	PublishedAt string `json:"published_at"`
}

type CheckStatus struct {
	CheckedAt      time.Time `json:"checked_at"`
	Available      bool      `json:"available"`
	CurrentVersion string    `json:"current_version"`
	LatestVersion  string    `json:"latest_version"`
	ReleaseURL     string    `json:"release_url"`
	Error          string    `json:"error,omitempty"`
}

type OnUpdateFoundFunc func(status CheckStatus)

type Checker struct {
	repo                string
	currentVersion      string
	apiBaseURL          string
	client              *http.Client
	mu                  sync.RWMutex
	status              CheckStatus
	lastNotifiedVersion string
	onUpdateFound       OnUpdateFoundFunc
	enabled             bool
}

func NewChecker(repo, currentVersion string, onUpdateFound OnUpdateFoundFunc) *Checker {
	cleanRepo := strings.TrimSpace(repo)
	if cleanRepo == "" {
		cleanRepo = DefaultRepo
	}

	return &Checker{
		repo:           cleanRepo,
		currentVersion: currentVersion,
		apiBaseURL:     "https://api.github.com",
		client: &http.Client{
			Timeout: DefaultRequestTimeout,
		},
		onUpdateFound: onUpdateFound,
		enabled:       true,
		status: CheckStatus{
			CurrentVersion: currentVersion,
		},
	}
}

func (c *Checker) SetEnabled(enabled bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.enabled = enabled
}

func (c *Checker) IsEnabled() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.enabled
}

func (c *Checker) SetAPIBaseURL(baseURL string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.apiBaseURL = strings.TrimRight(baseURL, "/")
}

func (c *Checker) Status() CheckStatus {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.status
}

func (c *Checker) Check(ctx context.Context) (CheckStatus, error) {
	c.mu.RLock()
	baseURL := c.apiBaseURL
	repo := c.repo
	currentVer := c.currentVersion
	c.mu.RUnlock()

	endpoint := fmt.Sprintf("%s/repos/%s/releases/latest", baseURL, repo)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return c.recordError(err)
	}

	req.Header.Set("User-Agent", "Satellite-Agent/"+currentVer)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := c.client.Do(req)
	if err != nil {
		return c.recordError(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var statusErr error
		switch resp.StatusCode {
		case http.StatusForbidden, http.StatusTooManyRequests:
			statusErr = fmt.Errorf("github api rate limit exceeded")
		case http.StatusNotFound:
			statusErr = fmt.Errorf("github repository or releases not found")
		default:
			statusErr = fmt.Errorf("github api request failed with status %d", resp.StatusCode)
		}
		return c.recordError(statusErr)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return c.recordError(err)
	}

	var release GitHubRelease
	if err := json.Unmarshal(body, &release); err != nil {
		return c.recordError(err)
	}

	return c.applyRelease(release)
}

func (c *Checker) applyRelease(release GitHubRelease) (CheckStatus, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	isAvailable := false
	if !release.Draft && !release.Prerelease {
		isAvailable = IsNewer(c.currentVersion, release.TagName)
	}

	c.status = CheckStatus{
		CheckedAt:      time.Now(),
		Available:      isAvailable,
		CurrentVersion: c.currentVersion,
		LatestVersion:  release.TagName,
		ReleaseURL:     release.HTMLURL,
		Error:          "",
	}

	shouldNotify := isAvailable && release.TagName != c.lastNotifiedVersion
	if shouldNotify {
		c.lastNotifiedVersion = release.TagName
		if c.onUpdateFound != nil {
			go c.onUpdateFound(c.status)
		}
	}

	return c.status, nil
}

func (c *Checker) recordError(err error) (CheckStatus, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.status.CheckedAt = time.Now()
	c.status.Error = err.Error()

	return c.status, err
}

func (c *Checker) StartBackground(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = DefaultCheckInterval
	}

	go func() {
		if c.IsEnabled() {
			if _, err := c.Check(ctx); err != nil {
				logger.Warn("updater", fmt.Sprintf("Initial update check error: %v", err))
			}
		}

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if !c.IsEnabled() {
					continue
				}
				if _, err := c.Check(ctx); err != nil {
					logger.Warn("updater", fmt.Sprintf("Background update check error: %v", err))
				}
			}
		}
	}()
}
