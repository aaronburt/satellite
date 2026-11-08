package updater

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"satellite/internal/logger"
)

const (
	DefaultRepo           = "aaronburt/satellite"
	DefaultCheckInterval  = 24 * time.Hour
	DefaultRequestTimeout = 10 * time.Second

	// maxReleaseBody caps how much of the GitHub response is read. A release
	// is a few KB; anything far larger is not a valid answer.
	maxReleaseBody = 8 << 20
)

var desktopReleaseTag = regexp.MustCompile(`^(desktop-)?v[0-9]+\.[0-9]+(\.[0-9]+)?$`)

type GitHubRelease struct {
	TagName     string         `json:"tag_name"`
	Name        string         `json:"name"`
	HTMLURL     string         `json:"html_url"`
	Draft       bool           `json:"draft"`
	Prerelease  bool           `json:"prerelease"`
	PublishedAt string         `json:"published_at"`
	Assets      []ReleaseAsset `json:"assets"`
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
	autoUpdate          bool
	assets              []ReleaseAsset
	installMu           sync.Mutex
	installedVersion    string
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

	for page := 1; page <= 10; page++ {
		endpoint := fmt.Sprintf("%s/repos/%s/releases?per_page=100&page=%d", baseURL, repo, page)
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
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, maxReleaseBody+1))
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			switch resp.StatusCode {
			case http.StatusForbidden, http.StatusTooManyRequests:
				return c.recordError(fmt.Errorf("github api rate limit exceeded"))
			case http.StatusNotFound:
				return c.recordError(fmt.Errorf("github repository or releases not found"))
			default:
				return c.recordError(fmt.Errorf("github api request failed with status %d", resp.StatusCode))
			}
		}
		if readErr != nil {
			return c.recordError(readErr)
		}
		if len(body) > maxReleaseBody {
			return c.recordError(fmt.Errorf("github api response too large"))
		}
		var releases []GitHubRelease
		if err := json.Unmarshal(body, &releases); err != nil {
			return c.recordError(err)
		}
		var selected *GitHubRelease
		for i := range releases {
			release := &releases[i]
			if release.Draft || release.Prerelease || !desktopReleaseTag.MatchString(release.TagName) {
				continue
			}
			hasAsset := false
			for _, asset := range release.Assets {
				if asset.Name == "satellite.exe" || asset.Name == "satellite-linux-amd64" || strings.HasPrefix(asset.Name, "Satellite-Setup-") && strings.HasSuffix(asset.Name, ".exe") {
					hasAsset = true
				}
			}
			if !hasAsset {
				continue
			}
			if selected == nil || IsNewer(selected.TagName, release.TagName) {
				selected = release
			}
		}
		if selected != nil {
			return c.applyRelease(*selected)
		}
		if !strings.Contains(resp.Header.Get("Link"), `rel="next"`) {
			return c.applyRelease(GitHubRelease{})
		}
	}
	return c.recordError(fmt.Errorf("no desktop release found in the first 1000 releases"))
}

func (c *Checker) applyRelease(release GitHubRelease) (CheckStatus, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	isAvailable := false
	if !release.Draft && !release.Prerelease {
		isAvailable = IsNewer(c.currentVersion, release.TagName)
	}

	c.assets = release.Assets
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
			if _, err := c.checkBackground(ctx); err != nil {
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
				if _, err := c.checkBackground(ctx); err != nil {
					logger.Warn("updater", fmt.Sprintf("Background update check error: %v", err))
				}
			}
		}
	}()
}
