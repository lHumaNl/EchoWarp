package updater

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestNew(t *testing.T) {
	u, err := New()
	if err != nil {
		t.Fatalf("failed to create updater: %v", err)
	}
	if u == nil {
		t.Fatal("updater should not be nil")
	}
	if u.currentVersion == "" {
		t.Error("current version should not be empty")
	}
	if u.executablePath == "" {
		t.Error("executable path should not be empty")
	}
}

func TestWithRepo(t *testing.T) {
	customRepo := "custom/repo"
	u, err := New(WithRepo(customRepo))
	if err != nil {
		t.Fatalf("failed to create updater: %v", err)
	}
	if u.repo != customRepo {
		t.Errorf("expected repo %q, got %q", customRepo, u.repo)
	}
}

func TestWithHTTPClient(t *testing.T) {
	customClient := &http.Client{}
	u, err := New(WithHTTPClient(customClient))
	if err != nil {
		t.Fatalf("failed to create updater: %v", err)
	}
	if u.httpClient != customClient {
		t.Error("HTTP client should be set")
	}
}

func TestWithAPIBaseURL(t *testing.T) {
	customURL := "https://custom.api.example.com"
	u, err := New(WithAPIBaseURL(customURL))
	if err != nil {
		t.Fatalf("failed to create updater: %v", err)
	}
	if u.apiBaseURL != customURL {
		t.Errorf("expected apiBaseURL %q, got %q", customURL, u.apiBaseURL)
	}
}

func TestCheckForUpdate(t *testing.T) {
	tests := []struct {
		name          string
		targetVersion string
		setupServer   func() *httptest.Server
		wantErr       bool
		errContains   string
	}{
		{
			name:          "get latest release",
			targetVersion: "",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte(`{
						"tag_name": "v1.0.0",
						"name": "Release 1.0.0",
						"html_url": "https://github.com/test/test/releases/v1.0.0",
						"assets": []
					}`))
				}))
			},
			wantErr: false,
		},
		{
			name:          "get specific version",
			targetVersion: "v1.2.0",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte(`{
						"tag_name": "v1.2.0",
						"name": "Release 1.2.0",
						"html_url": "https://github.com/test/test/releases/v1.2.0",
						"assets": []
					}`))
				}))
			},
			wantErr: false,
		},
		{
			name:          "release not found",
			targetVersion: "",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusNotFound)
				}))
			},
			wantErr:     true,
			errContains: "failed to fetch release",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := tt.setupServer()
			defer server.Close()

			u := &Updater{
				repo:       "test/test",
				httpClient: server.Client(),
				apiBaseURL: server.URL,
			}

			release, err := u.CheckForUpdate(context.Background(), tt.targetVersion)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.errContains != "" && !containsSubstring(err.Error(), tt.errContains) {
					t.Errorf("error should contain %q, got %q", tt.errContains, err.Error())
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if release == nil {
					t.Fatal("release should not be nil")
				}
			}
		})
	}
}

func TestGetCurrentVersion(t *testing.T) {
	u, err := New()
	if err != nil {
		t.Fatalf("failed to create updater: %v", err)
	}
	version := u.GetCurrentVersion()
	if version == "" {
		t.Error("version should not be empty")
	}
}

func TestGetExecutablePath(t *testing.T) {
	u, err := New()
	if err != nil {
		t.Fatalf("failed to create updater: %v", err)
	}
	path := u.GetExecutablePath()
	if path == "" {
		t.Error("executable path should not be empty")
	}
}

func TestIsNewer(t *testing.T) {
	tests := []struct {
		name          string
		currentVer    string
		releaseTag    string
		expectedNewer bool
	}{
		{"newer version", "1.0.0", "v1.1.0", true},
		{"same version", "1.0.0", "v1.0.0", false},
		{"older version", "1.1.0", "v1.0.0", true},
		{"version without v prefix", "1.0.0", "1.1.0", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := &Updater{currentVersion: tt.currentVer}
			release := &Release{TagName: tt.releaseTag}
			result := u.IsNewer(release)
			if result != tt.expectedNewer {
				t.Errorf("expected %v, got %v", tt.expectedNewer, result)
			}
		})
	}
}

func TestFindAsset(t *testing.T) {
	release := &Release{
		TagName: "v1.0.0",
		Assets: []Asset{
			{Name: "echowarp-linux-amd64", BrowserDownloadURL: "http://example.com/1"},
			{Name: "echowarp-darwin-amd64", BrowserDownloadURL: "http://example.com/2"},
			{Name: "echowarp-darwin-arm64", BrowserDownloadURL: "http://example.com/3"},
			{Name: "echowarp-windows-amd64.exe", BrowserDownloadURL: "http://example.com/4"},
		},
	}

	u := &Updater{}
	asset := u.findAsset(release)

	if asset == nil {
		t.Fatal("asset should not be nil")
	}

	expectedOS := runtime.GOOS
	if asset.Name != "" && !containsOS(asset.Name, expectedOS) {
		t.Errorf("asset should be for OS %s, got %s", expectedOS, asset.Name)
	}
}

func containsOS(name, os string) bool {
	return len(name) >= len(os) && (name[:len(os)] == os || containsSubstring(name, os))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestGetLatestRelease(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"tag_name": "v1.0.0",
			"name": "Release 1.0.0",
			"html_url": "https://github.com/test/test/releases/v1.0.0",
			"assets": []
		}`))
	}))
	defer server.Close()

	u := &Updater{
		repo:       "test/test",
		httpClient: server.Client(),
		apiBaseURL: server.URL,
	}

	release, err := u.getLatestRelease(context.Background())
	if err != nil {
		t.Fatalf("failed to get latest release: %v", err)
	}

	if release.TagName != "v1.0.0" {
		t.Errorf("expected tag v1.0.0, got %s", release.TagName)
	}
}

func TestGetLatestRelease_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	u := &Updater{
		repo:       "test/test",
		httpClient: server.Client(),
		apiBaseURL: server.URL,
	}

	_, err := u.getLatestRelease(context.Background())
	if err == nil {
		t.Fatal("expected error for 404")
	}
	if !containsSubstring(err.Error(), "not found") {
		t.Errorf("error should mention not found, got: %v", err)
	}
}

func TestGetLatestRelease_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	u := &Updater{
		repo:       "test/test",
		httpClient: server.Client(),
		apiBaseURL: server.URL,
	}

	_, err := u.getLatestRelease(context.Background())
	if err == nil {
		t.Fatal("expected error for 500")
	}
	if !containsSubstring(err.Error(), "GitHub API returned status") {
		t.Errorf("error should mention API status, got: %v", err)
	}
}

func TestGetLatestRelease_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`invalid json`))
	}))
	defer server.Close()

	u := &Updater{
		repo:       "test/test",
		httpClient: server.Client(),
		apiBaseURL: server.URL,
	}

	_, err := u.getLatestRelease(context.Background())
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
	if !containsSubstring(err.Error(), "failed to parse release") {
		t.Errorf("error should mention parse failure, got: %v", err)
	}
}

func TestGetReleaseByTag(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"tag_name": "v1.2.0",
			"name": "Release 1.2.0",
			"html_url": "https://github.com/test/test/releases/v1.2.0",
			"assets": []
		}`))
	}))
	defer server.Close()

	u := &Updater{
		repo:       "test/test",
		httpClient: server.Client(),
		apiBaseURL: server.URL,
	}

	release, err := u.getReleaseByTag(context.Background(), "v1.2.0")
	if err != nil {
		t.Fatalf("failed to get release by tag: %v", err)
	}

	if release.TagName != "v1.2.0" {
		t.Errorf("expected tag v1.2.0, got %s", release.TagName)
	}
}

func TestGetReleaseByTag_AddVPrefix(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"tag_name": "v1.2.0",
			"name": "Release 1.2.0",
			"html_url": "https://github.com/test/test/releases/v1.2.0",
			"assets": []
		}`))
	}))
	defer server.Close()

	u := &Updater{
		repo:       "test/test",
		httpClient: server.Client(),
		apiBaseURL: server.URL,
	}

	// Test without 'v' prefix - should be added automatically
	release, err := u.getReleaseByTag(context.Background(), "1.2.0")
	if err != nil {
		t.Fatalf("failed to get release by tag: %v", err)
	}

	if release.TagName != "v1.2.0" {
		t.Errorf("expected tag v1.2.0, got %s", release.TagName)
	}
}

func TestGetReleaseByTag_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	u := &Updater{
		repo:       "test/test",
		httpClient: server.Client(),
		apiBaseURL: server.URL,
	}

	_, err := u.getReleaseByTag(context.Background(), "v9.9.9")
	if err == nil {
		t.Fatal("expected error for 404")
	}
	if !containsSubstring(err.Error(), "not found") {
		t.Errorf("error should mention not found, got: %v", err)
	}
}

func TestGetReleaseByTag_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()

	u := &Updater{
		repo:       "test/test",
		httpClient: server.Client(),
		apiBaseURL: server.URL,
	}

	_, err := u.getReleaseByTag(context.Background(), "v1.0.0")
	if err == nil {
		t.Fatal("expected error for 403")
	}
	if !containsSubstring(err.Error(), "GitHub API returned status") {
		t.Errorf("error should mention API status, got: %v", err)
	}
}

func TestGetReleaseByTag_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`not valid json`))
	}))
	defer server.Close()

	u := &Updater{
		repo:       "test/test",
		httpClient: server.Client(),
		apiBaseURL: server.URL,
	}

	_, err := u.getReleaseByTag(context.Background(), "v1.0.0")
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
	if !containsSubstring(err.Error(), "failed to parse release") {
		t.Errorf("error should mention parse failure, got: %v", err)
	}
}

func TestDownloadRelease(t *testing.T) {
	content := []byte("fake binary content")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(content)
	}))
	defer server.Close()

	release := &Release{
		TagName: "v1.0.0",
		Assets: []Asset{
			{
				Name:               "echowarp-" + runtime.GOOS + "-" + runtime.GOARCH,
				BrowserDownloadURL: server.URL,
				Size:               int64(len(content)),
			},
		},
	}

	tmpDir := t.TempDir()
	destPath := filepath.Join(tmpDir, "echowarp-new")

	u := &Updater{
		httpClient: server.Client(),
	}

	err := u.DownloadRelease(context.Background(), release, destPath, nil)
	if err != nil {
		t.Fatalf("failed to download release: %v", err)
	}

	data, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("failed to read downloaded file: %v", err)
	}

	if !bytes.Equal(data, content) {
		t.Error("downloaded content does not match")
	}

	info, err := os.Stat(destPath)
	if err != nil {
		t.Fatalf("failed to stat file: %v", err)
	}

	if runtime.GOOS != "windows" && info.Mode()&0111 == 0 {
		t.Error("file should be executable")
	}
}

func TestDownloadRelease_NoAsset(t *testing.T) {
	release := &Release{
		TagName: "v1.0.0",
		Assets:  []Asset{}, // No assets
	}

	u := &Updater{}
	err := u.DownloadRelease(context.Background(), release, "/tmp/test", nil)
	if err == nil {
		t.Fatal("expected error when no suitable asset found")
	}
	if !containsSubstring(err.Error(), "no suitable asset found") {
		t.Errorf("error should mention no suitable asset, got: %v", err)
	}
}

func TestDownloadRelease_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	release := &Release{
		TagName: "v1.0.0",
		Assets: []Asset{
			{
				Name:               "echowarp-" + runtime.GOOS + "-" + runtime.GOARCH,
				BrowserDownloadURL: server.URL,
			},
		},
	}

	u := &Updater{
		httpClient: server.Client(),
	}

	err := u.DownloadRelease(context.Background(), release, filepath.Join(t.TempDir(), "test"), nil)
	if err == nil {
		t.Fatal("expected error for HTTP 500")
	}
	if !containsSubstring(err.Error(), "download failed") {
		t.Errorf("error should mention download failed, got: %v", err)
	}
}

func TestDownloadRelease_WithProgress(t *testing.T) {
	content := []byte("fake binary content for progress test")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(content)
	}))
	defer server.Close()

	release := &Release{
		TagName: "v1.0.0",
		Assets: []Asset{
			{
				Name:               "echowarp-" + runtime.GOOS + "-" + runtime.GOARCH,
				BrowserDownloadURL: server.URL,
				Size:               int64(len(content)),
			},
		},
	}

	tmpDir := t.TempDir()
	destPath := filepath.Join(tmpDir, "echowarp-new")

	var progressCalls []int
	progressFunc := func(p int) {
		progressCalls = append(progressCalls, p)
	}

	u := &Updater{
		httpClient: server.Client(),
	}

	err := u.DownloadRelease(context.Background(), release, destPath, progressFunc)
	if err != nil {
		t.Fatalf("failed to download: %v", err)
	}

	if len(progressCalls) == 0 {
		t.Error("progress function should have been called")
	}

	// Last progress should be 100%
	if progressCalls[len(progressCalls)-1] != 100 {
		t.Errorf("final progress should be 100%%, got %d%%", progressCalls[len(progressCalls)-1])
	}
}

func TestVerifyChecksum(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "test")
	content := []byte("test content")
	if err := os.WriteFile(tmpFile, content, 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	u := &Updater{}

	// Test with no checksum (should pass)
	if err := u.VerifyChecksum(tmpFile, ""); err != nil {
		t.Errorf("should pass with empty checksum: %v", err)
	}

	// Test with correct checksum
	hash := sha256.Sum256(content)
	correctChecksum := hex.EncodeToString(hash[:])
	if err := u.VerifyChecksum(tmpFile, correctChecksum); err != nil {
		t.Errorf("should pass with correct checksum: %v", err)
	}

	// Test with incorrect checksum
	incorrectChecksum := "0000000000000000000000000000000000000000000000000000000000000000"
	err := u.VerifyChecksum(tmpFile, incorrectChecksum)
	if err == nil {
		t.Fatal("expected error for incorrect checksum")
	}
	if !containsSubstring(err.Error(), "checksum mismatch") {
		t.Errorf("error should mention checksum mismatch, got: %v", err)
	}

	// Test with nonexistent file
	err = u.VerifyChecksum("/nonexistent/file", "abc123")
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
	if !containsSubstring(err.Error(), "failed to open file") {
		t.Errorf("error should mention failed to open file, got: %v", err)
	}
}

func TestProgressWriter(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "progress-test-*")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	progress := 0
	pw := &progressWriter{
		writer:  tmpFile,
		total:   100,
		written: 0,
		progress: func(p int) {
			progress = p
		},
	}

	data := make([]byte, 50)
	n, err := pw.Write(data)
	if err != nil {
		t.Fatalf("write failed: %v", err)
	}
	if n != 50 {
		t.Errorf("expected 50 bytes written, got %d", n)
	}
	if progress != 50 {
		t.Errorf("expected progress 50%%, got %d%%", progress)
	}
}

func TestCopyFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Create source file
	srcPath := filepath.Join(tmpDir, "source.txt")
	content := []byte("test content for copy")
	if err := os.WriteFile(srcPath, content, 0644); err != nil {
		t.Fatalf("failed to create source file: %v", err)
	}

	// Make source executable
	if err := os.Chmod(srcPath, 0755); err != nil {
		t.Fatalf("failed to chmod source: %v", err)
	}

	// Copy file
	dstPath := filepath.Join(tmpDir, "destination.txt")
	if err := copyFile(srcPath, dstPath); err != nil {
		t.Fatalf("copyFile failed: %v", err)
	}

	// Verify content
	dstContent, err := os.ReadFile(dstPath)
	if err != nil {
		t.Fatalf("failed to read destination: %v", err)
	}
	if !bytes.Equal(dstContent, content) {
		t.Error("content mismatch")
	}

	// Verify permissions were preserved
	dstInfo, err := os.Stat(dstPath)
	if err != nil {
		t.Fatalf("failed to stat destination: %v", err)
	}
	if runtime.GOOS != "windows" && dstInfo.Mode()&0111 == 0 {
		t.Error("destination should be executable")
	}
}

func TestCopyFile_SourceNotExist(t *testing.T) {
	err := copyFile("/nonexistent/file.txt", "/tmp/destination.txt")
	if err == nil {
		t.Fatal("expected error for nonexistent source")
	}
}

func TestInstall(t *testing.T) {
	tmpDir := t.TempDir()

	// Create fake current executable
	execPath := filepath.Join(tmpDir, "current")
	execContent := []byte("current version")
	if err := os.WriteFile(execPath, execContent, 0755); err != nil {
		t.Fatalf("failed to create exec: %v", err)
	}

	// Create downloaded file
	downloadedPath := filepath.Join(tmpDir, "downloaded")
	downloadedContent := []byte("new version")
	if err := os.WriteFile(downloadedPath, downloadedContent, 0755); err != nil {
		t.Fatalf("failed to create downloaded: %v", err)
	}

	u := &Updater{executablePath: execPath}

	// Install
	if err := u.Install(downloadedPath); err != nil {
		t.Fatalf("install failed: %v", err)
	}

	// Verify new content
	installedContent, err := os.ReadFile(execPath)
	if err != nil {
		t.Fatalf("failed to read installed: %v", err)
	}
	if !bytes.Equal(installedContent, downloadedContent) {
		t.Error("installed content mismatch")
	}

	// Verify backup was cleaned up
	backupPath := execPath + backupSuffix
	if _, err := os.Stat(backupPath); !os.IsNotExist(err) {
		t.Error("backup should be cleaned up")
	}
}

func TestInstall_DownloadedNotExist(t *testing.T) {
	u := &Updater{executablePath: "/tmp/fake"}
	err := u.Install("/nonexistent/file")
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
	if !containsSubstring(err.Error(), "not found") {
		t.Errorf("error should mention 'not found', got %v", err)
	}
}
