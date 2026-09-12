// Package update checks GitHub Releases for a newer Freedom To Parrots
// build and, only with explicit user consent (see Tracker.Apply - never
// called on its own), replaces the running executable with it and
// restarts. Checking happens on its own; applying never does.
package update

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	releaseAPI    = "https://api.github.com/repos/isamo09/FreedomToParrots/releases/latest"
	checkTimeout  = 8 * time.Second
	downloadTime  = 5 * time.Minute
	checkInterval = 12 * time.Hour
	sumsAssetName = "FreedomToParrots-SHA256SUMS.txt" // distinct from olcBOX's own same-named asset in the same release
)

// Info describes the result of the latest version check.
type Info struct {
	Current   string `json:"current"`
	Latest    string `json:"latest,omitempty"`
	Available bool   `json:"available"`
	URL       string `json:"url,omitempty"` // release page, for humans
	Err       string `json:"error,omitempty"`

	assetURL string // download URL for this platform's asset
	sha256   string // expected hex digest from the release's SHA256SUMS.txt
}

// ErrNoAsset means the latest release has no build for this OS/architecture.
var ErrNoAsset = errors.New("update: latest release has no build for this platform")

// ErrNotVerifiable means the release has no checksum to verify the
// download against - Apply refuses to install an unverifiable binary.
var ErrNotVerifiable = errors.New("update: release has no checksum to verify the download against")

// ErrNoUpdate means Apply was called with nothing newer available.
var ErrNoUpdate = errors.New("update: no newer version available")

type ghAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

type ghRelease struct {
	TagName string    `json:"tag_name"`
	HTMLURL string    `json:"html_url"`
	Assets  []ghAsset `json:"assets"`
}

// check queries the latest GitHub release and compares it to current. A
// "dev" (or empty) current version - a build straight from `go build`, not
// a release artifact - never reports an update available; there's nothing
// meaningful to compare against.
func check(ctx context.Context, current string) (Info, error) {
	info := Info{Current: current}
	if current == "" || current == "dev" {
		return info, nil
	}

	ctx, cancel := context.WithTimeout(ctx, checkTimeout)
	defer cancel()

	rel, err := fetchJSON(ctx, releaseAPI)
	if err != nil {
		return info, err
	}

	info.Latest, info.URL = rel.TagName, rel.HTMLURL
	info.Available = newer(rel.TagName, current)

	want := assetName()

	var sumsURL string

	for _, a := range rel.Assets {
		switch a.Name {
		case want:
			info.assetURL = a.BrowserDownloadURL
		case sumsAssetName:
			sumsURL = a.BrowserDownloadURL
		}
	}

	if info.assetURL == "" {
		return info, ErrNoAsset
	}

	if sumsURL != "" {
		if sum, err := fetchExpectedSum(ctx, sumsURL, want); err == nil {
			info.sha256 = sum
		}
	}

	return info, nil
}

func fetchJSON(ctx context.Context, url string) (ghRelease, error) {
	var rel ghRelease

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return rel, err
	}

	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return rel, err
	}
	defer resp.Body.Close() //nolint:errcheck // response body close, error not actionable

	if resp.StatusCode != http.StatusOK {
		return rel, fmt.Errorf("github: %s", resp.Status)
	}

	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return rel, fmt.Errorf("parse release info: %w", err)
	}

	return rel, nil
}

// fetchExpectedSum downloads a `sha256sum`-format checksums file and
// returns the hex digest for the named file.
func fetchExpectedSum(ctx context.Context, url, name string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close() //nolint:errcheck // response body close, error not actionable

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("fetch checksums: %s", resp.Status)
	}

	sc := bufio.NewScanner(io.LimitReader(resp.Body, 1<<20))
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) != 2 {
			continue
		}

		if strings.TrimPrefix(fields[1], "*") == name {
			return strings.ToLower(fields[0]), nil
		}
	}

	return "", fmt.Errorf("%s: no checksum entry", name)
}

func assetName() string {
	ext := ""
	if runtime.GOOS == "windows" {
		ext = ".exe"
	}

	return fmt.Sprintf("FreedomToParrots-%s-%s%s", runtime.GOOS, runtime.GOARCH, ext)
}

// newer reports whether latest > current as simple vMAJOR.MINOR.PATCH
// version tags (no pre-release suffixes - that's all this project uses).
func newer(latest, current string) bool {
	lv, ok := parseVersion(latest)
	if !ok {
		return false
	}

	cv, ok := parseVersion(current)
	if !ok {
		return false
	}

	for i := range lv {
		if lv[i] != cv[i] {
			return lv[i] > cv[i]
		}
	}

	return false
}

func parseVersion(v string) ([3]int, bool) {
	var out [3]int

	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	parts := strings.SplitN(v, ".", 3)

	for i := 0; i < len(parts) && i < 3; i++ {
		n, err := strconv.Atoi(parts[i])
		if err != nil {
			return out, false
		}

		out[i] = n
	}

	return out, true
}

// Tracker holds the last known update-check result, refreshed once at
// startup and periodically afterward, and safe to read concurrently from
// the console dashboard and the web panel.
type Tracker struct {
	mu   sync.Mutex
	info Info
}

// NewTracker builds a tracker for the given running version.
func NewTracker(version string) *Tracker {
	return &Tracker{info: Info{Current: version}}
}

// Run checks once immediately, then every checkInterval, until ctx is done.
func (t *Tracker) Run(ctx context.Context) {
	t.refresh(ctx)

	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			t.refresh(ctx)
		}
	}
}

func (t *Tracker) refresh(ctx context.Context) {
	current := t.Snapshot().Current

	info, err := check(ctx, current)
	if err != nil && !errors.Is(err, ErrNoAsset) {
		info.Err = err.Error()
	}

	info.Current = current

	t.mu.Lock()
	t.info = info
	t.mu.Unlock()
}

// Snapshot returns the last known check result.
func (t *Tracker) Snapshot() Info {
	t.mu.Lock()
	defer t.mu.Unlock()

	return t.info
}

// Apply downloads and verifies the available update, replaces the running
// executable with it, and starts the new version as a fresh process. It
// does not stop this process - the caller (which just got explicit user
// consent to update) is responsible for shutting this one down afterward.
// Never call this without that consent: it always changes what's on disk.
func (t *Tracker) Apply(ctx context.Context) error {
	info := t.Snapshot()
	if !info.Available {
		return ErrNoUpdate
	}

	if info.sha256 == "" {
		return ErrNotVerifiable
	}

	ctx, cancel := context.WithTimeout(ctx, downloadTime)
	defer cancel()

	if err := applyInfo(ctx, info); err != nil {
		return err
	}

	return relaunch()
}

// CleanupOld removes a leftover "<exe>.old" from a previous Apply that
// couldn't delete it immediately (the old executable can still briefly be
// "in use" right as the process exits). Safe to call on every startup -
// it's a no-op when there's nothing to clean up.
func CleanupOld() {
	exe, err := runningExecutable()
	if err != nil {
		return
	}

	_ = os.Remove(exe + ".old")
}

func applyInfo(ctx context.Context, info Info) error {
	exe, err := runningExecutable()
	if err != nil {
		return err
	}

	dir := filepath.Dir(exe)
	tmp := filepath.Join(dir, ".update-"+strings.TrimPrefix(info.Latest, "v")+filepath.Ext(exe))

	if err := downloadVerified(ctx, info.assetURL, info.sha256, tmp); err != nil {
		return err
	}

	if err := os.Chmod(tmp, 0o755); err != nil { //nolint:gosec // executable needs to stay executable
		_ = os.Remove(tmp)

		return fmt.Errorf("chmod: %w", err)
	}

	old := exe + ".old"
	_ = os.Remove(old) // leftover from a previous update, if any; harmless if absent

	if err := os.Rename(exe, old); err != nil {
		_ = os.Remove(tmp)

		return fmt.Errorf("move current executable aside: %w", err)
	}

	if err := os.Rename(tmp, exe); err != nil {
		_ = os.Rename(old, exe) // put the original back so the install isn't left broken

		return fmt.Errorf("install new executable: %w", err)
	}

	_ = os.Remove(old) // best-effort; fine if still "in use" and this fails

	return nil
}

func downloadVerified(ctx context.Context, url, expectedSum, dest string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close() //nolint:errcheck // response body close, error not actionable

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download: %s", resp.Status)
	}

	f, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755) //nolint:gosec // destined to be the executable
	if err != nil {
		return err
	}

	h := sha256.New()

	_, err = io.Copy(io.MultiWriter(f, h), resp.Body)
	closeErr := f.Close()

	if err != nil {
		_ = os.Remove(dest)

		return fmt.Errorf("download: %w", err)
	}

	if closeErr != nil {
		_ = os.Remove(dest)

		return fmt.Errorf("save download: %w", closeErr)
	}

	if got := hex.EncodeToString(h.Sum(nil)); got != expectedSum {
		_ = os.Remove(dest)

		return fmt.Errorf("checksum mismatch: got %s, expected %s", got, expectedSum)
	}

	return nil
}

func runningExecutable() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("locate running executable: %w", err)
	}

	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return "", fmt.Errorf("resolve running executable: %w", err)
	}

	return exe, nil
}
