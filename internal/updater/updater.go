// Package updater provides functionality to check for and install updates from GitHub releases.
package updater

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/lHumaNl/echowarp/internal/version"
)

const (
	defaultGitHubRepo   = "lHumaNl/echowarp"
	defaultTimeout      = 30 * time.Second
	githubAPIBase       = "https://api.github.com/repos"
	backupSuffix        = ".backup"
	maxDownloadAttempts = 3
)

// Release represents a GitHub release.
type Release struct {
	TagName string  `json:"tag_name"`
	Name    string  `json:"name"`
	Assets  []Asset `json:"assets"`
	HTMLURL string  `json:"html_url"`
}

// Asset represents a release asset (binary file).
type Asset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
}

// Updater handles checking and installing updates.
type Updater struct {
	currentVersion string
	repo           string
	httpClient     *http.Client
	executablePath string
	apiBaseURL     string // For testing purposes
}

// New creates a new Updater instance.
func New(opts ...Option) (*Updater, error) {
	execPath, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("failed to get executable path: %w", err)
	}

	u := &Updater{
		currentVersion: version.Version,
		repo:           defaultGitHubRepo,
		httpClient: &http.Client{
			Timeout: defaultTimeout,
		},
		executablePath: execPath,
		apiBaseURL:     githubAPIBase,
	}

	for _, opt := range opts {
		opt(u)
	}

	return u, nil
}

// Option configures the Updater.
type Option func(*Updater)

// WithRepo sets a custom GitHub repository.
func WithRepo(repo string) Option {
	return func(u *Updater) {
		u.repo = repo
	}
}

// WithHTTPClient sets a custom HTTP client.
func WithHTTPClient(client *http.Client) Option {
	return func(u *Updater) {
		u.httpClient = client
	}
}

// WithAPIBaseURL sets a custom API base URL (for testing).
func WithAPIBaseURL(url string) Option {
	return func(u *Updater) {
		u.apiBaseURL = url
	}
}

// CheckForUpdate checks GitHub releases for a newer version.
func (u *Updater) CheckForUpdate(ctx context.Context, targetVersion string) (*Release, error) {
	var release *Release
	var err error

	if targetVersion != "" {
		release, err = u.getReleaseByTag(ctx, targetVersion)
	} else {
		release, err = u.getLatestRelease(ctx)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to fetch release: %w", err)
	}

	if release == nil {
		return nil, fmt.Errorf("no release found")
	}

	return release, nil
}

// IsNewer checks if the release version is newer than current version.
func (u *Updater) IsNewer(release *Release) bool {
	releaseVer := strings.TrimPrefix(release.TagName, "v")
	currentVer := strings.TrimPrefix(u.currentVersion, "v")
	return releaseVer != currentVer
}

// DownloadRelease downloads the appropriate binary for the current platform.
func (u *Updater) DownloadRelease(ctx context.Context, release *Release, destPath string, progress func(int)) error {
	asset := u.findAsset(release)
	if asset == nil {
		return fmt.Errorf("no suitable asset found for %s/%s", runtime.GOOS, runtime.GOARCH)
	}

	resp, err := u.fetchReleaseAsset(ctx, asset.BrowserDownloadURL)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }() //nolint:errcheck

	tmpFile, cleanup, err := u.downloadToTempFile(resp.Body, asset.Size, progress)
	if err != nil {
		return err
	}
	defer cleanup()

	return u.finalizeDownload(tmpFile, destPath)
}

// fetchReleaseAsset creates and executes an HTTP request for the release asset.
func (u *Updater) fetchReleaseAsset(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := u.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to download release: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close() //nolint:errcheck
		return nil, fmt.Errorf("download failed with status: %s", resp.Status)
	}

	return resp, nil
}

// downloadToTempFile downloads content to a temporary file with optional progress reporting.
// Returns the temp file, a cleanup function, and any error.
func (u *Updater) downloadToTempFile(body io.Reader, size int64, progress func(int)) (*os.File, func(), error) {
	tmpFile, err := os.CreateTemp("", "echowarp-download-*")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create temp file: %w", err)
	}

	cleanup := func() {
		_ = os.Remove(tmpFile.Name()) //nolint:errcheck
		_ = tmpFile.Close()           //nolint:errcheck
	}

	writer := io.Writer(tmpFile)
	if progress != nil {
		writer = &progressWriter{
			writer:   tmpFile,
			total:    size,
			progress: progress,
		}
	}

	if _, err := io.Copy(writer, body); err != nil {
		cleanup()
		return nil, nil, fmt.Errorf("failed to write download: %w", err)
	}

	return tmpFile, cleanup, nil
}

// finalizeDownload sets executable permissions and moves the file to destination.
func (u *Updater) finalizeDownload(tmpFile *os.File, destPath string) error {
	if err := tmpFile.Chmod(0755); err != nil {
		return fmt.Errorf("failed to set executable permissions: %w", err)
	}

	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("failed to close temp file: %w", err)
	}

	if err := os.Rename(tmpFile.Name(), destPath); err != nil {
		return fmt.Errorf("failed to move downloaded file: %w", err)
	}

	return nil
}

// Install replaces the current binary with the downloaded one.
func (u *Updater) Install(downloadedPath string) error {
	if _, err := os.Stat(downloadedPath); os.IsNotExist(err) {
		return fmt.Errorf("downloaded file not found: %s", downloadedPath)
	}

	backupPath := u.executablePath + backupSuffix
	if err := copyFile(u.executablePath, backupPath); err != nil {
		return fmt.Errorf("failed to create backup: %w", err)
	}

	if err := os.Rename(downloadedPath, u.executablePath); err != nil {
		// Attempt to restore backup
		if restoreErr := os.Rename(backupPath, u.executablePath); restoreErr != nil {
			return fmt.Errorf("failed to install update and restore backup: %w (restore error: %v)", err, restoreErr)
		}
		return fmt.Errorf("failed to install update: %w", err)
	}

	// Clean up backup after successful install
	_ = os.Remove(backupPath) //nolint:errcheck

	return nil
}

// VerifyChecksum verifies the SHA256 checksum of the downloaded file.
func (u *Updater) VerifyChecksum(filePath, expectedChecksum string) error {
	if expectedChecksum == "" {
		return nil // No checksum provided, skip verification
	}

	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file for checksum: %w", err)
	}
	defer func() { _ = file.Close() }() //nolint:errcheck

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return fmt.Errorf("failed to compute checksum: %w", err)
	}

	actualChecksum := hex.EncodeToString(hash.Sum(nil))
	if actualChecksum != expectedChecksum {
		return fmt.Errorf("checksum mismatch: expected %s, got %s", expectedChecksum, actualChecksum)
	}

	return nil
}

// getLatestRelease fetches the latest release from GitHub.
func (u *Updater) getLatestRelease(ctx context.Context) (*Release, error) {
	url := fmt.Sprintf("%s/%s/releases/latest", u.apiBaseURL, u.repo)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := u.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }() //nolint:errcheck

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("repository or releases not found")
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var release Release
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("failed to parse release: %w", err)
	}

	return &release, nil
}

// getReleaseByTag fetches a specific release by tag.
func (u *Updater) getReleaseByTag(ctx context.Context, tag string) (*Release, error) {
	if !strings.HasPrefix(tag, "v") {
		tag = "v" + tag
	}

	url := fmt.Sprintf("%s/%s/releases/tags/%s", u.apiBaseURL, u.repo, tag)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := u.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }() //nolint:errcheck

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("release %s not found", tag)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var release Release
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("failed to parse release: %w", err)
	}

	return &release, nil
}

// findAsset finds the appropriate asset for the current platform.
func (u *Updater) findAsset(release *Release) *Asset {
	osName := runtime.GOOS
	arch := runtime.GOARCH

	// Map architecture names
	archMap := map[string]string{
		"amd64": "amd64",
		"arm64": "arm64",
		"arm":   "arm",
	}

	assetArch := archMap[arch]

	for i := range release.Assets {
		asset := &release.Assets[i]
		name := strings.ToLower(asset.Name)

		// Check if asset matches OS and architecture
		if strings.Contains(name, osName) && strings.Contains(name, assetArch) {
			return asset
		}
	}

	return nil
}

// copyFile copies a file from src to dst.
func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = srcFile.Close() }() //nolint:errcheck

	srcInfo, err := srcFile.Stat()
	if err != nil {
		return err
	}

	dstFile, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, srcInfo.Mode())
	if err != nil {
		return err
	}
	defer func() { _ = dstFile.Close() }() //nolint:errcheck

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return err
	}

	return nil
}

// progressWriter wraps an io.Writer to report download progress.
type progressWriter struct {
	writer   io.Writer
	total    int64
	written  int64
	progress func(int)
}

func (pw *progressWriter) Write(p []byte) (int, error) {
	n, err := pw.writer.Write(p)
	if err != nil {
		return n, err
	}

	pw.written += int64(n)
	if pw.progress != nil && pw.total > 0 {
		percentage := int((pw.written * 100) / pw.total)
		pw.progress(percentage)
	}

	return n, nil
}

// GetCurrentVersion returns the current version.
func (u *Updater) GetCurrentVersion() string {
	return u.currentVersion
}

// GetExecutablePath returns the current executable path.
func (u *Updater) GetExecutablePath() string {
	return u.executablePath
}
