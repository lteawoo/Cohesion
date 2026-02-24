package system

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	defaultRepoOwner = "lteawoo"
	defaultRepoName  = "Cohesion"
)

type UpdateCheckerConfig struct {
	RepoOwner      string
	RepoName       string
	CacheTTL       time.Duration
	RequestTimeout time.Duration
	APIBaseURL     string
}

type UpdateCheckResult struct {
	CurrentVersion  string `json:"currentVersion"`
	LatestVersion   string `json:"latestVersion,omitempty"`
	UpdateAvailable bool   `json:"updateAvailable"`
	ReleaseURL      string `json:"releaseUrl"`
	CheckedAt       string `json:"checkedAt"`
	Error           string `json:"error,omitempty"`
}

type latestReleaseResponse struct {
	TagName string `json:"tag_name"`
	HTMLURL string `json:"html_url"`
}

type cachedLatestRelease struct {
	expiresAt time.Time
	tagName   string
	htmlURL   string
}

type UpdateChecker struct {
	repoOwner      string
	repoName       string
	cacheTTL       time.Duration
	requestTimeout time.Duration
	apiBaseURL     string
	client         *http.Client

	mu     sync.Mutex
	cached *cachedLatestRelease
}

func NewUpdateChecker(cfg UpdateCheckerConfig) *UpdateChecker {
	repoOwner := strings.TrimSpace(cfg.RepoOwner)
	if repoOwner == "" {
		repoOwner = defaultRepoOwner
	}

	repoName := strings.TrimSpace(cfg.RepoName)
	if repoName == "" {
		repoName = defaultRepoName
	}

	cacheTTL := cfg.CacheTTL
	if cacheTTL <= 0 {
		cacheTTL = 10 * time.Minute
	}

	requestTimeout := cfg.RequestTimeout
	if requestTimeout <= 0 {
		requestTimeout = 3 * time.Second
	}

	return &UpdateChecker{
		repoOwner:      repoOwner,
		repoName:       repoName,
		cacheTTL:       cacheTTL,
		requestTimeout: requestTimeout,
		apiBaseURL:     strings.TrimSpace(cfg.APIBaseURL),
		client: &http.Client{
			Timeout: requestTimeout,
		},
	}
}

func (c *UpdateChecker) Check(ctx context.Context, currentVersion string) (UpdateCheckResult, error) {
	now := time.Now().UTC()
	currentTag := normalizeVersionTag(currentVersion)
	defaultReleaseURL := fmt.Sprintf("https://github.com/%s/%s/releases", c.repoOwner, c.repoName)

	result := UpdateCheckResult{
		CurrentVersion:  currentTag,
		UpdateAvailable: false,
		ReleaseURL:      defaultReleaseURL,
		CheckedAt:       now.Format(time.RFC3339),
	}

	latestTag, latestURL, err := c.getLatestRelease(ctx, now)
	if err != nil {
		return result, err
	}

	result.LatestVersion = latestTag
	if latestURL != "" {
		result.ReleaseURL = latestURL
	}
	if isNewerVersion(currentTag, latestTag) {
		result.UpdateAvailable = true
	}

	return result, nil
}

func (c *UpdateChecker) getLatestRelease(ctx context.Context, now time.Time) (string, string, error) {
	if tag, url, ok := c.getCached(now); ok {
		return tag, url, nil
	}

	requestURL := c.latestReleaseAPIURL()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "cohesion-update-checker")

	resp, err := c.client.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("latest release request failed with status %d", resp.StatusCode)
	}

	var payload latestReleaseResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", "", err
	}

	tag := normalizeVersionTag(payload.TagName)
	if tag == "dev" {
		return "", "", fmt.Errorf("latest release tag is empty")
	}

	c.setCached(now, tag, strings.TrimSpace(payload.HTMLURL))
	return tag, strings.TrimSpace(payload.HTMLURL), nil
}

func (c *UpdateChecker) latestReleaseAPIURL() string {
	if c.apiBaseURL != "" {
		base := strings.TrimRight(c.apiBaseURL, "/")
		return base + "/releases/latest"
	}
	return fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", c.repoOwner, c.repoName)
}

func (c *UpdateChecker) getCached(now time.Time) (string, string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.cached == nil || !now.Before(c.cached.expiresAt) {
		return "", "", false
	}

	return c.cached.tagName, c.cached.htmlURL, true
}

func (c *UpdateChecker) setCached(now time.Time, tagName, htmlURL string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.cached = &cachedLatestRelease{
		expiresAt: now.Add(c.cacheTTL),
		tagName:   tagName,
		htmlURL:   htmlURL,
	}
}

func normalizeVersionTag(input string) string {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return "dev"
	}

	if strings.HasPrefix(trimmed, "v") || strings.HasPrefix(trimmed, "V") {
		suffix := strings.TrimSpace(trimmed[1:])
		if suffix == "" {
			return "dev"
		}
		return "v" + suffix
	}

	first := trimmed[0]
	if first >= '0' && first <= '9' {
		return "v" + trimmed
	}

	return trimmed
}

func isNewerVersion(currentTag, latestTag string) bool {
	currentCore, okCurrent := parseSemverCore(currentTag)
	latestCore, okLatest := parseSemverCore(latestTag)
	if !okCurrent || !okLatest {
		return false
	}

	for i := 0; i < 3; i++ {
		if latestCore[i] > currentCore[i] {
			return true
		}
		if latestCore[i] < currentCore[i] {
			return false
		}
	}
	return false
}

func parseSemverCore(tag string) ([3]int, bool) {
	var zero [3]int
	trimmed := strings.TrimSpace(tag)
	if len(trimmed) < 2 || (trimmed[0] != 'v' && trimmed[0] != 'V') {
		return zero, false
	}

	withoutPrefix := trimmed[1:]
	core := withoutPrefix
	if plusIndex := strings.IndexByte(core, '+'); plusIndex >= 0 {
		core = core[:plusIndex]
	}
	if hyphenIndex := strings.IndexByte(core, '-'); hyphenIndex >= 0 {
		core = core[:hyphenIndex]
	}
	if core == "" {
		return zero, false
	}

	parts := strings.Split(core, ".")
	if len(parts) > 3 {
		return zero, false
	}

	var parsed [3]int
	for i := 0; i < 3; i++ {
		if i >= len(parts) {
			parsed[i] = 0
			continue
		}

		part := strings.TrimSpace(parts[i])
		if part == "" {
			return zero, false
		}
		number, err := strconv.Atoi(part)
		if err != nil || number < 0 {
			return zero, false
		}
		parsed[i] = number
	}

	return parsed, true
}
