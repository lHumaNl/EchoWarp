package cli

import (
	"bytes"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/spf13/cobra"

	"github.com/lHumaNl/echowarp/internal/app"
	"github.com/lHumaNl/echowarp/internal/config"
	"github.com/lHumaNl/echowarp/internal/daemon"
	"github.com/lHumaNl/echowarp/pkg/echowarp"
	"github.com/lHumaNl/echowarp/pkg/echowarp/auth"
)

func TestDaemonCommand_HasSubcommands(t *testing.T) {
	cmd := newDaemonCmd()
	cmds := cmd.Commands()
	if len(cmds) == 0 {
		t.Error("daemon command should have subcommands")
	}

	expectedCmds := []string{"start", "stop", "status"}
	foundCmds := make(map[string]bool)
	for _, c := range cmds {
		foundCmds[c.Name()] = true
	}

	for _, expected := range expectedCmds {
		if !foundCmds[expected] {
			t.Errorf("missing daemon subcommand: %s", expected)
		}
	}
}

func TestDaemonStatusCommand_NoPidFile(t *testing.T) {
	cmd := newDaemonCmd()
	cmd.SetArgs([]string{"status", "--pid-file", "/tmp/echowarp-test-nonexistent.pid"})
	_ = cmd.Execute()
}

func TestDaemonStartCommand_HasFlags(t *testing.T) {
	cmd := newDaemonStartCmd()
	flags := cmd.Flags()
	if flags == nil {
		t.Error("daemon start command should have flags")
	}

	requiredFlags := []string{"port", "device", "password", "log-file", "pid-file"}
	for _, flag := range requiredFlags {
		if flags.Lookup(flag) == nil {
			t.Errorf("daemon start command should have --%s flag", flag)
		}
	}
}

func TestDaemonStartCommand_AllFlags(t *testing.T) {
	cmd := newDaemonStartCmd()
	flags := cmd.Flags()

	expectedFlags := []string{
		"port", "device", "password", "reverse", "sample-rate",
		"channels", "max-clients", "max-auth-failures", "max-reconnect",
		"virtual-mic", "stun-server", "tls-cert", "tls-key", "config",
		"log-level", "log-file", "ban-file", "no-discovery",
		"server-name", "rate-limit", "pid-file", "api-port",
		"api-bind", "api-token", "enable-pprof",
	}

	for _, flag := range expectedFlags {
		if flags.Lookup(flag) == nil {
			t.Errorf("daemon start command should have --%s flag", flag)
		}
	}
}

func TestDaemonStartCommand_DefaultValues(t *testing.T) {
	cmd := newDaemonStartCmd()
	flags := cmd.Flags()

	port, _ := flags.GetInt("port")
	if port != 4415 {
		t.Errorf("expected default port 4415, got %d", port)
	}

	sampleRate, _ := flags.GetInt("sample-rate")
	if sampleRate != 48000 {
		t.Errorf("expected default sample rate 48000, got %d", sampleRate)
	}

	apiPort, _ := flags.GetInt("api-port")
	if apiPort != 8080 {
		t.Errorf("expected default api-port 8080, got %d", apiPort)
	}

	apiBind, _ := flags.GetString("api-bind")
	if apiBind != "127.0.0.1" {
		t.Errorf("expected default api-bind 127.0.0.1, got %s", apiBind)
	}

	enablePprof, _ := flags.GetBool("enable-pprof")
	if enablePprof != false {
		t.Errorf("expected default enable-pprof false, got %v", enablePprof)
	}
}

func TestDaemonStartCommand_Use(t *testing.T) {
	cmd := newDaemonStartCmd()
	if cmd.Use != "start" {
		t.Errorf("expected Use 'start', got %q", cmd.Use)
	}
}

func TestDaemonStartCommand_RunE(t *testing.T) {
	cmd := newDaemonStartCmd()
	if cmd.RunE == nil {
		t.Error("daemon start command should have RunE function")
	}
}

func TestDaemonStopCommand_HasPidFileFlag(t *testing.T) {
	cmd := newDaemonStopCmd()
	flags := cmd.Flags()
	if flags.Lookup("pid-file") == nil {
		t.Error("daemon stop command should have --pid-file flag")
	}
}

func TestDaemonStopCommand_Use(t *testing.T) {
	cmd := newDaemonStopCmd()
	if cmd.Use != "stop" {
		t.Errorf("expected Use 'stop', got %q", cmd.Use)
	}
}

func TestDaemonStopCommand_RunE(t *testing.T) {
	cmd := newDaemonStopCmd()
	if cmd.RunE == nil {
		t.Error("daemon stop command should have RunE function")
	}
}

func TestDaemonStatusCommand_HasPidFileFlag(t *testing.T) {
	cmd := newDaemonStatusCmd()
	flags := cmd.Flags()
	if flags.Lookup("pid-file") == nil {
		t.Error("daemon status command should have --pid-file flag")
	}
}

func TestDaemonStatusCommand_Use(t *testing.T) {
	cmd := newDaemonStatusCmd()
	if cmd.Use != "status" {
		t.Errorf("expected Use 'status', got %q", cmd.Use)
	}
}

func TestDaemonStatusCommand_RunE(t *testing.T) {
	cmd := newDaemonStatusCmd()
	if cmd.RunE == nil {
		t.Error("daemon status command should have RunE function")
	}
}

func TestDaemonCmd_Use(t *testing.T) {
	cmd := newDaemonCmd()
	if cmd.Use != "daemon" {
		t.Errorf("expected Use 'daemon', got %q", cmd.Use)
	}
}

func TestDaemonCmd_Short(t *testing.T) {
	cmd := newDaemonCmd()
	if cmd.Short == "" {
		t.Error("daemon command should have a short description")
	}
}

func TestDaemonCmd_Long(t *testing.T) {
	cmd := newDaemonCmd()
	if cmd.Long == "" {
		t.Error("daemon command should have a long description")
	}
}

func TestNewDaemonCmd(t *testing.T) {
	cmd := newDaemonCmd()
	if cmd == nil {
		t.Error("newDaemonCmd should return non-nil command")
	}
}

func TestNewDaemonStartCmd(t *testing.T) {
	cmd := newDaemonStartCmd()
	if cmd == nil {
		t.Error("newDaemonStartCmd should return non-nil command")
	}
}

func TestNewDaemonStopCmd(t *testing.T) {
	cmd := newDaemonStopCmd()
	if cmd == nil {
		t.Error("newDaemonStopCmd should return non-nil command")
	}
}

func TestNewDaemonStatusCmd(t *testing.T) {
	cmd := newDaemonStatusCmd()
	if cmd == nil {
		t.Error("newDaemonStatusCmd should return non-nil command")
	}
}

func TestDaemonStartCommand_Short(t *testing.T) {
	cmd := newDaemonStartCmd()
	if cmd.Short == "" {
		t.Error("daemon start command should have a short description")
	}
}

func TestDaemonStartCommand_Long(t *testing.T) {
	cmd := newDaemonStartCmd()
	if cmd.Long == "" {
		t.Error("daemon start command should have a long description")
	}
}

func TestDaemonStopCommand_Short(t *testing.T) {
	cmd := newDaemonStopCmd()
	if cmd.Short == "" {
		t.Error("daemon stop command should have a short description")
	}
}

func TestDaemonStopCommand_Long(t *testing.T) {
	cmd := newDaemonStopCmd()
	if cmd.Long == "" {
		t.Error("daemon stop command should have a long description")
	}
}

func TestDaemonStatusCommand_Short(t *testing.T) {
	cmd := newDaemonStatusCmd()
	if cmd.Short == "" {
		t.Error("daemon status command should have a short description")
	}
}

func TestDaemonStatusCommand_Long(t *testing.T) {
	cmd := newDaemonStatusCmd()
	if cmd.Long == "" {
		t.Error("daemon status command should have a long description")
	}
}

func TestGetFlagBool(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		flag     string
		expected bool
	}{
		{"flag set true", []string{"--no-discovery"}, "no-discovery", true},
		{"flag not set", []string{}, "no-discovery", false},
		{"enable-pprof set", []string{"--enable-pprof"}, "enable-pprof", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := newDaemonStartCmd()
			_ = cmd.ParseFlags(tt.args)
			result := getFlagBool(cmd, tt.flag)
			if result != tt.expected {
				t.Errorf("getFlagBool(%s) = %v, want %v", tt.flag, result, tt.expected)
			}
		})
	}
}

func TestGetFlagString(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		flag     string
		expected string
	}{
		{"server-name set", []string{"--server-name", "myserver"}, "server-name", "myserver"},
		{"server-name not set", []string{}, "server-name", ""},
		{"api-bind set", []string{"--api-bind", "0.0.0.0"}, "api-bind", "0.0.0.0"},
		{"api-token set", []string{"--api-token", "secret"}, "api-token", "secret"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := newDaemonStartCmd()
			_ = cmd.ParseFlags(tt.args)
			result := getFlagString(cmd, tt.flag)
			if result != tt.expected {
				t.Errorf("getFlagString(%s) = %v, want %v", tt.flag, result, tt.expected)
			}
		})
	}
}

func TestGetFlagInt(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		flag     string
		expected int
	}{
		{"rate-limit set", []string{"--rate-limit", "10"}, "rate-limit", 10},
		{"rate-limit default", []string{}, "rate-limit", 5},
		{"api-port set", []string{"--api-port", "9090"}, "api-port", 9090},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := newDaemonStartCmd()
			_ = cmd.ParseFlags(tt.args)
			result := getFlagInt(cmd, tt.flag)
			if result != tt.expected {
				t.Errorf("getFlagInt(%s) = %v, want %v", tt.flag, result, tt.expected)
			}
		})
	}
}

func TestGetDaemonFlags(t *testing.T) {
	tests := []struct {
		name                string
		args                []string
		expectedNoDiscovery bool
		expectedServerName  string
		expectedRateLimit   int
		expectedAPIPort     int
		expectedAPIBind     string
		expectedAPIToken    string
		expectedEnablePprof bool
	}{
		{
			name:                "default values",
			args:                []string{},
			expectedNoDiscovery: false,
			expectedServerName:  "",
			expectedRateLimit:   5,
			expectedAPIPort:     8080,
			expectedAPIBind:     "127.0.0.1",
			expectedAPIToken:    "",
			expectedEnablePprof: false,
		},
		{
			name:                "all flags set",
			args:                []string{"--no-discovery", "--server-name", "test", "--rate-limit", "20", "--api-port", "9090", "--api-bind", "0.0.0.0", "--api-token", "token123", "--enable-pprof"},
			expectedNoDiscovery: true,
			expectedServerName:  "test",
			expectedRateLimit:   20,
			expectedAPIPort:     9090,
			expectedAPIBind:     "0.0.0.0",
			expectedAPIToken:    "token123",
			expectedEnablePprof: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := newDaemonStartCmd()
			_ = cmd.ParseFlags(tt.args)
			flags := getDaemonFlags(cmd)
			if flags.noDiscovery != tt.expectedNoDiscovery {
				t.Errorf("noDiscovery = %v, want %v", flags.noDiscovery, tt.expectedNoDiscovery)
			}
			if flags.serverName != tt.expectedServerName {
				t.Errorf("serverName = %v, want %v", flags.serverName, tt.expectedServerName)
			}
			if flags.rateLimit != tt.expectedRateLimit {
				t.Errorf("rateLimit = %v, want %v", flags.rateLimit, tt.expectedRateLimit)
			}
			if flags.apiPort != tt.expectedAPIPort {
				t.Errorf("apiPort = %v, want %v", flags.apiPort, tt.expectedAPIPort)
			}
			if flags.apiBind != tt.expectedAPIBind {
				t.Errorf("apiBind = %v, want %v", flags.apiBind, tt.expectedAPIBind)
			}
			if flags.apiToken != tt.expectedAPIToken {
				t.Errorf("apiToken = %v, want %v", flags.apiToken, tt.expectedAPIToken)
			}
			if flags.enablePprof != tt.expectedEnablePprof {
				t.Errorf("enablePprof = %v, want %v", flags.enablePprof, tt.expectedEnablePprof)
			}
		})
	}
}

func TestCreateDaemonRateLimiter(t *testing.T) {
	tests := []struct {
		name      string
		rateLimit int
		expectNil bool
	}{
		{"disabled (0)", 0, true},
		{"enabled (5)", 5, false},
		{"enabled (100)", 100, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			limiter := createDaemonRateLimiter(tt.rateLimit)
			if (limiter == nil) != tt.expectNil {
				t.Errorf("createDaemonRateLimiter(%d) nil = %v, want %v", tt.rateLimit, limiter == nil, tt.expectNil)
			}
		})
	}
}

func TestSetupDaemonDiscovery(t *testing.T) {
	tests := []struct {
		name        string
		noDiscovery bool
		serverName  string
	}{
		{"discovery enabled, no name", false, ""},
		{"discovery enabled, with name", false, "test-server"},
		{"discovery disabled", true, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			logger := slog.New(slog.NewTextHandler(&buf, nil))
			setupDaemonDiscovery(tt.noDiscovery, tt.serverName, logger)
		})
	}
}

func TestConvertToNodeConfig(t *testing.T) {
	deviceID := uint32(5)
	cfg := config.Config{
		Mode:            config.ModeServer,
		Reverse:         true,
		Port:            8080,
		Address:         "192.168.1.100",
		DeviceID:        &deviceID,
		SampleRate:      44100,
		Channels:        2,
		VirtualMic:      true,
		OpusBitrate:     64000,
		OpusComplexity:  8,
		OpusApplication: "voip",
		OpusDTX:         true,
		OpusFEC:         true,
		Password:        "testpass",
		TLSCert:         "/path/cert.pem",
		TLSKey:          "/path/key.pem",
		TLSInsecure:     true,
		MaxClients:      10,
		STUNServers:     []string{"stun:example.com:3478"},
		TURNServers:     []config.TURNServer{{URL: "turn:example.com:3478", Username: "user", Credential: "cred"}},
	}

	nodeCfg := convertToNodeConfig(cfg)

	if nodeCfg.Mode != echowarp.Mode(config.ModeServer) {
		t.Errorf("mode = %v, want %v", nodeCfg.Mode, config.ModeServer)
	}
	if !nodeCfg.Reverse {
		t.Error("reverse should be true")
	}
	if nodeCfg.Port != 8080 {
		t.Errorf("port = %v, want 8080", nodeCfg.Port)
	}
	if nodeCfg.Address != "192.168.1.100" {
		t.Errorf("address = %v, want 192.168.1.100", nodeCfg.Address)
	}
	if nodeCfg.DeviceID == nil || *nodeCfg.DeviceID != 5 {
		t.Errorf("deviceID = %v, want 5", nodeCfg.DeviceID)
	}
	if nodeCfg.SampleRate != 44100 {
		t.Errorf("sampleRate = %v, want 44100", nodeCfg.SampleRate)
	}
	if nodeCfg.Channels != 2 {
		t.Errorf("channels = %v, want 2", nodeCfg.Channels)
	}
	if !nodeCfg.VirtualMic {
		t.Error("virtualMic should be true")
	}
	if nodeCfg.Password != "testpass" {
		t.Errorf("password = %v, want testpass", nodeCfg.Password)
	}
	if len(nodeCfg.STUNServers) != 1 {
		t.Errorf("STUNServers count = %v, want 1", len(nodeCfg.STUNServers))
	}
	if len(nodeCfg.TURNServers) != 1 {
		t.Errorf("TURNServers count = %v, want 1", len(nodeCfg.TURNServers))
	}
}

func TestConvertToNodeConfig_EmptySTUNTURN(t *testing.T) {
	cfg := config.Config{
		Mode:        config.ModeClient,
		STUNServers: []string{},
		TURNServers: []config.TURNServer{},
	}

	nodeCfg := convertToNodeConfig(cfg)

	if len(nodeCfg.STUNServers) != 0 {
		t.Errorf("STUNServers count = %v, want 0", len(nodeCfg.STUNServers))
	}
	if len(nodeCfg.TURNServers) != 0 {
		t.Errorf("TURNServers count = %v, want 0", len(nodeCfg.TURNServers))
	}
}

func TestSetupLogger_DefaultPath(t *testing.T) {
	cfg := config.Config{
		LogLevel: "info",
		LogFile:  "",
	}

	logger, err := setupLogger(cfg)
	if err != nil {
		t.Errorf("setupLogger failed: %v", err)
	}
	if logger == nil {
		t.Error("logger should not be nil")
	}
}

func TestSetupLogger_CustomPath(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping on Windows - log file handle prevents TempDir cleanup")
	}
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "daemon.log")

	cfg := config.Config{
		LogLevel: "debug",
		LogFile:  logFile,
	}

	logger, err := setupLogger(cfg)
	if err != nil {
		t.Errorf("setupLogger failed: %v", err)
	}
	if logger == nil {
		t.Error("logger should not be nil")
	}
}

func TestSetupLogger_InvalidPath(t *testing.T) {
	// Create a file that blocks directory creation at that path.
	blocker := filepath.Join(t.TempDir(), "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}

	cfg := config.Config{
		LogLevel: "info",
		LogFile:  filepath.Join(blocker, "sub", "daemon.log"),
	}

	_, err := setupLogger(cfg)
	if err == nil {
		t.Error("expected error for invalid log path")
	}
}

func TestRunDaemonStop_NotRunning(t *testing.T) {
	cmd := newDaemonCmd()
	cmd.SetArgs([]string{"stop", "--pid-file", "/tmp/echowarp-test-nonexistent-stop.pid"})
	_ = cmd.Execute()
}

func TestRunDaemonStart_InvalidConfig(t *testing.T) {
	cmd := newDaemonStartCmd()
	_ = cmd.ParseFlags([]string{"--port", "-1", "--pid-file", "/tmp/test-daemon.pid"})

	err := runDaemonStart(cmd, []string{})
	if err == nil {
		t.Error("expected error for invalid config")
	}
}

func TestRunDaemonStart_MissingConfigFile(t *testing.T) {
	cmd := newDaemonStartCmd()
	_ = cmd.ParseFlags([]string{"--config", "/nonexistent/config.yaml", "--pid-file", "/tmp/test-daemon.pid"})

	err := runDaemonStart(cmd, []string{})
	if err == nil {
		t.Error("expected error for missing config file")
	}
}

func TestCheckIfRunning_NoPIDFile(t *testing.T) {
	d := daemon.New("/tmp/echowarp-test-check-notrunning.pid")
	err := checkIfRunning(d)
	if err != nil {
		t.Errorf("checkIfRunning should return nil when no daemon running: %v", err)
	}
}

func TestDaemonFlags_BanFilePath(t *testing.T) {
	tmpDir := t.TempDir()
	banFile := filepath.Join(tmpDir, "ban.json")

	cmd := newDaemonStartCmd()
	_ = cmd.ParseFlags([]string{"--ban-file", banFile})
	flags := getDaemonFlags(cmd)

	if flags.banFilePath != banFile {
		t.Errorf("banFilePath = %v, want %v", flags.banFilePath, banFile)
	}
}

func TestDaemonFlags_DefaultBanFilePath(t *testing.T) {
	cmd := newDaemonStartCmd()
	_ = cmd.ParseFlags([]string{})
	flags := getDaemonFlags(cmd)

	if flags.banFilePath == "" {
		t.Error("banFilePath should have default value")
	}
}

func TestCreateRunnerFactory(t *testing.T) {
	cfg := config.Config{
		Mode:     config.ModeServer,
		Port:     4415,
		Password: "test",
	}

	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))
	rateLimiter := auth.NewIPRateLimiter(5)

	factory := createRunnerFactory(cfg, logger, rateLimiter, nil)
	if factory == nil {
		t.Error("createRunnerFactory should return non-nil factory")
	}
}

func TestCreateNode(t *testing.T) {
	deviceID := uint32(0)
	cfg := config.Config{
		Mode:       config.ModeServer,
		Port:       4415,
		Password:   "test",
		DeviceID:   &deviceID,
		SampleRate: 48000,
		Channels:   1,
	}

	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))
	rateLimiter := auth.NewIPRateLimiter(5)

	node, err := createNode(cfg, logger, rateLimiter, nil)
	if err != nil {
		t.Errorf("createNode failed: %v", err)
	}
	if node == nil {
		t.Error("createNode should return non-nil node")
	}
}

func TestRunDaemonStatus_WithStalePIDFile(t *testing.T) {
	tmpDir := t.TempDir()
	pidFile := filepath.Join(tmpDir, "daemon.pid")

	_ = os.WriteFile(pidFile, []byte("99999999"), 0600)

	cmd := newDaemonCmd()
	cmd.SetArgs([]string{"status", "--pid-file", pidFile})
	_ = cmd.Execute()
}

func TestRunDaemonStop_WithStalePIDFile(t *testing.T) {
	tmpDir := t.TempDir()
	pidFile := filepath.Join(tmpDir, "daemon.pid")

	_ = os.WriteFile(pidFile, []byte("99999999"), 0600)

	cmd := newDaemonCmd()
	cmd.SetArgs([]string{"stop", "--pid-file", pidFile})
	_ = cmd.Execute()
}

func TestSetupDaemon(t *testing.T) {
	tmpDir := t.TempDir()
	pidFile := filepath.Join(tmpDir, "daemon.pid")
	configFile := filepath.Join(tmpDir, "config.yaml")
	configContent := `port: 4415
password: test
sample_rate: 48000
channels: 1
log_level: info
`
	_ = os.WriteFile(configFile, []byte(configContent), 0600)

	cmd := newDaemonStartCmd()
	_ = cmd.ParseFlags([]string{"--pid-file", pidFile, "--config", configFile})

	d, cfg, logger, err := setupDaemon(cmd)
	if err != nil {
		t.Errorf("setupDaemon failed: %v", err)
	}
	if d == nil {
		t.Error("daemon should not be nil")
	}
	if cfg.Port != 4415 {
		t.Errorf("expected port 4415, got %d", cfg.Port)
	}
	if logger == nil {
		t.Error("logger should not be nil")
	}
}

func TestSetupDaemon_InvalidConfig(t *testing.T) {
	cmd := newDaemonStartCmd()
	_ = cmd.ParseFlags([]string{"--pid-file", "/tmp/test.pid", "--port", "-1"})

	_, _, _, err := setupDaemon(cmd)
	if err == nil {
		t.Error("expected error for invalid config")
	}
}

func TestSetupDaemon_AlreadyRunning(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping on Windows - Signal(0) not supported for process detection")
	}
	tmpDir := t.TempDir()
	pidFile := filepath.Join(tmpDir, "daemon.pid")

	_ = os.WriteFile(pidFile, []byte(fmt.Sprintf("%d", os.Getpid())), 0600)

	cmd := newDaemonStartCmd()
	_ = cmd.ParseFlags([]string{"--pid-file", pidFile})

	_, _, _, err := setupDaemon(cmd)
	if err == nil {
		t.Error("expected error when daemon is already running")
	}
	_ = os.Remove(pidFile)
}

func TestCheckIfRunning_AlreadyRunning(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping on Windows - Signal(0) not supported for process detection")
	}
	tmpDir := t.TempDir()
	pidFile := filepath.Join(tmpDir, "daemon.pid")

	_ = os.WriteFile(pidFile, []byte(fmt.Sprintf("%d", os.Getpid())), 0600)

	d := daemon.New(pidFile)
	err := checkIfRunning(d)
	if err == nil {
		t.Error("expected error when daemon is already running")
	}
	_ = os.Remove(pidFile)
}

func TestRunDaemonStart_WritePIDError(t *testing.T) {
	blocker := filepath.Join(t.TempDir(), "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	invalidPID := filepath.Join(blocker, "sub", "daemon.pid")

	cmd := newDaemonStartCmd()
	_ = cmd.ParseFlags([]string{"--pid-file", invalidPID, "--device", "0"})

	err := runDaemonStart(cmd, []string{})
	if err == nil {
		t.Error("expected error when cannot write PID file")
	}
}

func TestRunDaemonStart_LoggerError(t *testing.T) {
	tmpDir := t.TempDir()
	pidFile := filepath.Join(tmpDir, "daemon.pid")
	// Create a file that blocks directory creation for the log path.
	blocker := filepath.Join(tmpDir, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	invalidLogFile := filepath.Join(blocker, "sub", "daemon.log")
	configFile := filepath.Join(tmpDir, "config.yaml")
	configContent := fmt.Sprintf(`port: 4415
password: test
sample_rate: 48000
channels: 1
log_level: info
log_file: %s
`, invalidLogFile)
	_ = os.WriteFile(configFile, []byte(configContent), 0600)

	cmd := newDaemonStartCmd()
	_ = cmd.ParseFlags([]string{"--pid-file", pidFile, "--config", configFile})

	_, _, _, err := setupDaemon(cmd)
	if err == nil {
		t.Error("expected error when cannot create logger")
	}
}

func TestSetupDaemon_LoadConfigError(t *testing.T) {
	cmd := newDaemonStartCmd()
	_ = cmd.ParseFlags([]string{"--pid-file", "/tmp/test.pid", "--config", "/nonexistent/config.yaml"})

	_, _, _, err := setupDaemon(cmd)
	if err == nil {
		t.Error("expected error when config file not found")
	}
}

func TestCreateDaemonRateLimiter_Nil(t *testing.T) {
	limiter := createDaemonRateLimiter(0)
	if limiter != nil {
		t.Error("expected nil rate limiter when rate limit is 0")
	}
}

func TestCreateDaemonRateLimiter_NonNil(t *testing.T) {
	limiter := createDaemonRateLimiter(10)
	if limiter == nil {
		t.Error("expected non-nil rate limiter when rate limit > 0")
	}
}

func TestRunDaemonStatus_Running(t *testing.T) {
	tmpDir := t.TempDir()
	pidFile := filepath.Join(tmpDir, "daemon.pid")

	_ = os.WriteFile(pidFile, []byte(fmt.Sprintf("%d", os.Getpid())), 0600)

	cmd := newDaemonCmd()
	cmd.SetArgs([]string{"status", "--pid-file", pidFile})
	_ = cmd.Execute()

	_ = os.Remove(pidFile)
}

func TestRunDaemonStatus_StalePIDFile(t *testing.T) {
	tmpDir := t.TempDir()
	pidFile := filepath.Join(tmpDir, "daemon.pid")

	_ = os.WriteFile(pidFile, []byte("999999999"), 0600)

	cmd := newDaemonCmd()
	cmd.SetArgs([]string{"status", "--pid-file", pidFile})
	_ = cmd.Execute()

	_ = os.Remove(pidFile)
}

func TestRunDaemonStop_NotRunningWithStalePID(t *testing.T) {
	tmpDir := t.TempDir()
	pidFile := filepath.Join(tmpDir, "daemon.pid")

	_ = os.WriteFile(pidFile, []byte("999999999"), 0600)

	cmd := newDaemonCmd()
	cmd.SetArgs([]string{"stop", "--pid-file", pidFile})
	_ = cmd.Execute()

	_ = os.Remove(pidFile)
}

func TestCreateRunnerFactory_InvokesFactory(t *testing.T) {
	cfg := config.Config{
		Mode:       config.ModeServer,
		Port:       4415,
		Password:   "test",
		SampleRate: 48000,
		Channels:   1,
	}

	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))
	rateLimiter := auth.NewIPRateLimiter(5)

	factory := createRunnerFactory(cfg, logger, rateLimiter, nil)
	if factory == nil {
		t.Fatal("createRunnerFactory should return non-nil factory")
	}

	runner, err := factory(echowarp.NodeConfig{}, logger, nil, nil, rateLimiter)
	if err != nil {
		t.Errorf("factory should not error: %v", err)
	}
	if runner == nil {
		t.Error("factory should return non-nil runner")
	}
}

func TestRunDaemonStart_CreateNodeError(t *testing.T) {
	tmpDir := t.TempDir()
	pidFile := filepath.Join(tmpDir, "daemon.pid")
	configFile := filepath.Join(tmpDir, "config.yaml")

	configContent := `port: 4415
password: test
sample_rate: 99999
channels: 1
log_level: info
`
	_ = os.WriteFile(configFile, []byte(configContent), 0600)

	cmd := newDaemonStartCmd()
	_ = cmd.ParseFlags([]string{"--pid-file", pidFile, "--config", configFile})

	err := runDaemonStart(cmd, []string{})
	if err == nil {
		t.Error("expected error when node creation fails")
	}
}

// TestStartAPIServer tests the startAPIServer function.
func TestStartAPIServer(t *testing.T) {
	t.Skip("Skipping - requires actual node and may start server")
}

// TestStartAPIServer_WithPprof tests startAPIServer with pprof enabled.
func TestStartAPIServer_WithPprof(t *testing.T) {
	t.Skip("Skipping - requires actual node and may start server")
}

// TestRunDaemonStart_WithAllFlags tests runDaemonStart with all flags set.
// Note: This test skips actual daemon start as it would block.
func TestRunDaemonStart_WithAllFlags(t *testing.T) {
	t.Skip("Skipping - daemon start blocks waiting for connections")
}

// TestRunDaemonStart_InvalidLogLevel tests runDaemonStart with invalid log level.
func TestRunDaemonStart_InvalidLogLevel(t *testing.T) {
	tmpDir := t.TempDir()
	pidFile := filepath.Join(tmpDir, "daemon.pid")

	cmd := newDaemonStartCmd()
	_ = cmd.ParseFlags([]string{
		"--pid-file", pidFile,
		"--log-level", "invalid",
	})

	_, _, _, err := setupDaemon(cmd)
	if err == nil {
		t.Error("expected error for invalid log level")
	}
}

// TestRunDaemonStart_PortOutOfRange tests runDaemonStart with invalid port.
func TestRunDaemonStart_PortOutOfRange(t *testing.T) {
	tmpDir := t.TempDir()
	pidFile := filepath.Join(tmpDir, "daemon.pid")

	cmd := newDaemonStartCmd()
	_ = cmd.ParseFlags([]string{
		"--pid-file", pidFile,
		"--port", "70000",
	})

	_, _, _, err := setupDaemon(cmd)
	if err == nil {
		t.Error("expected error for invalid port")
	}
}

// TestRunDaemonStart_ZeroPort tests runDaemonStart with zero port.
func TestRunDaemonStart_ZeroPort(t *testing.T) {
	tmpDir := t.TempDir()
	pidFile := filepath.Join(tmpDir, "daemon.pid")

	cmd := newDaemonStartCmd()
	_ = cmd.ParseFlags([]string{
		"--pid-file", pidFile,
		"--port", "0",
	})

	_, _, _, err := setupDaemon(cmd)
	if err == nil {
		t.Error("expected error for zero port")
	}
}

// TestRunDaemonStart_InvalidSampleRate tests runDaemonStart with invalid sample rate.
func TestRunDaemonStart_InvalidSampleRate(t *testing.T) {
	tmpDir := t.TempDir()
	pidFile := filepath.Join(tmpDir, "daemon.pid")

	cmd := newDaemonStartCmd()
	_ = cmd.ParseFlags([]string{
		"--pid-file", pidFile,
		"--sample-rate", "12345",
	})

	_, _, _, err := setupDaemon(cmd)
	if err == nil {
		t.Error("expected error for invalid sample rate")
	}
}

// TestRunDaemonStart_InvalidChannels tests runDaemonStart with invalid channels.
func TestRunDaemonStart_InvalidChannels(t *testing.T) {
	tmpDir := t.TempDir()
	pidFile := filepath.Join(tmpDir, "daemon.pid")

	cmd := newDaemonStartCmd()
	_ = cmd.ParseFlags([]string{
		"--pid-file", pidFile,
		"--channels", "3",
	})

	_, _, _, err := setupDaemon(cmd)
	if err == nil {
		t.Error("expected error for invalid channels")
	}
}

// TestCreateNode_InvalidDevice tests createNode with invalid device.
func TestCreateNode_InvalidDevice(t *testing.T) {
	cfg := config.Config{
		Mode:       config.ModeServer,
		Port:       4415,
		Password:   "test",
		DeviceID:   uint32Ptr(999),
		SampleRate: 48000,
		Channels:   1,
	}

	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))

	// createNode should succeed even with invalid device ID
	// (device validation happens at runtime)
	node, err := createNode(cfg, logger, nil, nil)
	if err != nil {
		t.Logf("createNode returned error (expected on some systems): %v", err)
	}
	if node != nil {
		t.Log("createNode succeeded")
	}
}

// TestRunDaemonStop_WithRunningDaemon tests runDaemonStop when daemon is running.
// Note: This test is skipped as it would send SIGTERM to the test process.
func TestRunDaemonStop_WithRunningDaemon(t *testing.T) {
	t.Skip("Skipping - would send SIGTERM to test process")
}

// TestSetupDaemon_EnvVarOverride tests that environment variables override config file values in daemon.
func TestSetupDaemon_EnvVarOverride(t *testing.T) {
	tmpDir := t.TempDir()
	pidFile := filepath.Join(tmpDir, "daemon.pid")
	configFile := filepath.Join(tmpDir, "config.yaml")
	configContent := `port: 8080
sample_rate: 24000
password: filepass
log_level: info
`
	_ = os.WriteFile(configFile, []byte(configContent), 0600)

	// Set environment variables
	os.Setenv("ECHOWARP_PORT", "9999")
	os.Setenv("ECHOWARP_PASSWORD", "envpass")
	os.Setenv("ECHOWARP_LOG_LEVEL", "debug")
	defer func() {
		os.Unsetenv("ECHOWARP_PORT")
		os.Unsetenv("ECHOWARP_PASSWORD")
		os.Unsetenv("ECHOWARP_LOG_LEVEL")
	}()

	cmd := newDaemonStartCmd()
	_ = cmd.ParseFlags([]string{"--pid-file", pidFile, "--config", configFile})

	_, cfg, _, err := setupDaemon(cmd)
	if err != nil {
		t.Fatalf("setupDaemon failed: %v", err)
	}

	// Env vars should override config file values
	if cfg.Port != 9999 {
		t.Errorf("expected port 9999 from env var, got %d", cfg.Port)
	}
	if cfg.Password != "envpass" {
		t.Errorf("expected password 'envpass' from env var, got %s", cfg.Password)
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("expected log level 'debug' from env var, got %s", cfg.LogLevel)
	}
	// Sample rate should still be from config file (not overridden by env)
	if cfg.SampleRate != 24000 {
		t.Errorf("expected sample rate 24000 from config file, got %d", cfg.SampleRate)
	}
}

// TestSetupDaemon_CLIOverridesEnvVar tests that CLI flags override environment variables in daemon.
func TestSetupDaemon_CLIOverridesEnvVar(t *testing.T) {
	tmpDir := t.TempDir()
	pidFile := filepath.Join(tmpDir, "daemon.pid")

	// Set environment variables
	os.Setenv("ECHOWARP_PORT", "9999")
	os.Setenv("ECHOWARP_PASSWORD", "envpass")
	os.Setenv("ECHOWARP_LOG_LEVEL", "debug")
	defer func() {
		os.Unsetenv("ECHOWARP_PORT")
		os.Unsetenv("ECHOWARP_PASSWORD")
		os.Unsetenv("ECHOWARP_LOG_LEVEL")
	}()

	cmd := newDaemonStartCmd()
	_ = cmd.ParseFlags([]string{"--pid-file", pidFile, "--port", "7777"})

	_, cfg, _, err := setupDaemon(cmd)
	if err != nil {
		t.Fatalf("setupDaemon failed: %v", err)
	}

	// CLI flags should override env vars
	if cfg.Port != 7777 {
		t.Errorf("expected port 7777 from CLI flag, got %d", cfg.Port)
	}
	// Password should come from env var (not overridden by CLI)
	if cfg.Password != "envpass" {
		t.Errorf("expected password 'envpass' from env var, got %s", cfg.Password)
	}
	// Log level should come from env var
	if cfg.LogLevel != "debug" {
		t.Errorf("expected log level 'debug' from env var, got %s", cfg.LogLevel)
	}
}

// TestSetupDaemon_AllEnvVars tests loading all daemon configuration from environment variables.
func TestSetupDaemon_AllEnvVars(t *testing.T) {
	tmpDir := t.TempDir()
	pidFile := filepath.Join(tmpDir, "daemon.pid")

	// Set all relevant environment variables for daemon
	envVars := map[string]string{
		"ECHOWARP_PORT":         "9999",
		"ECHOWARP_PASSWORD":     "testpass",
		"ECHOWARP_MODE":         "reverse",
		"ECHOWARP_SAMPLE_RATE":  "16000",
		"ECHOWARP_CHANNELS":     "2",
		"ECHOWARP_VIRTUAL_MIC":  "true",
		"ECHOWARP_LOG_LEVEL":    "debug",
		"ECHOWARP_OPUS_BITRATE": "96000",
		"ECHOWARP_MAX_CLIENTS":  "10",
	}

	for k, v := range envVars {
		os.Setenv(k, v)
	}
	defer func() {
		for k := range envVars {
			os.Unsetenv(k)
		}
	}()

	cmd := newDaemonStartCmd()
	_ = cmd.ParseFlags([]string{"--pid-file", pidFile})

	_, cfg, _, err := setupDaemon(cmd)
	if err != nil {
		t.Fatalf("setupDaemon failed: %v", err)
	}

	// Verify all env vars were applied
	if cfg.Port != 9999 {
		t.Errorf("expected port 9999, got %d", cfg.Port)
	}
	if cfg.Password != "testpass" {
		t.Errorf("expected password 'testpass', got %s", cfg.Password)
	}
	if !cfg.Reverse {
		t.Error("expected reverse to be true")
	}
	if cfg.SampleRate != 16000 {
		t.Errorf("expected sample rate 16000, got %d", cfg.SampleRate)
	}
	if cfg.Channels != 2 {
		t.Errorf("expected channels 2, got %d", cfg.Channels)
	}
	if !cfg.VirtualMic {
		t.Error("expected virtual_mic to be true")
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("expected log level 'debug', got %s", cfg.LogLevel)
	}
	if cfg.OpusBitrate != 96000 {
		t.Errorf("expected opus bitrate 96000, got %d", cfg.OpusBitrate)
	}
	if cfg.MaxClients != 10 {
		t.Errorf("expected max_clients 10, got %d", cfg.MaxClients)
	}
}

// TestSetupDaemon_STUNFromEnvVar tests loading STUN servers from environment variable in daemon.
func TestSetupDaemon_STUNFromEnvVar(t *testing.T) {
	tmpDir := t.TempDir()
	pidFile := filepath.Join(tmpDir, "daemon.pid")

	os.Setenv("ECHOWARP_STUN_SERVERS", "stun:server1.com:3478,stun:server2.com:3478,stun:server3.com:3478")
	defer os.Unsetenv("ECHOWARP_STUN_SERVERS")

	cmd := newDaemonStartCmd()
	_ = cmd.ParseFlags([]string{"--pid-file", pidFile})

	_, cfg, _, err := setupDaemon(cmd)
	if err != nil {
		t.Fatalf("setupDaemon failed: %v", err)
	}

	if len(cfg.STUNServers) != 3 {
		t.Errorf("expected 3 STUN servers, got %d", len(cfg.STUNServers))
	}
	if len(cfg.STUNServers) > 0 {
		if cfg.STUNServers[0] != "stun:server1.com:3478" {
			t.Errorf("expected first STUN server 'stun:server1.com:3478', got %s", cfg.STUNServers[0])
		}
	}
}

// TestSetupDaemon_PriorityChain tests the full priority chain in daemon:
// CLI flags > env vars > config file > defaults
func TestSetupDaemon_PriorityChain(t *testing.T) {
	tmpDir := t.TempDir()
	pidFile := filepath.Join(tmpDir, "daemon.pid")
	configFile := filepath.Join(tmpDir, "config.yaml")
	configContent := `port: 8080
sample_rate: 24000
channels: 2
max_clients: 5
log_level: warn
`
	_ = os.WriteFile(configFile, []byte(configContent), 0600)

	// Set environment variables (lower priority than CLI flags)
	os.Setenv("ECHOWARP_PORT", "9999")
	os.Setenv("ECHOWARP_SAMPLE_RATE", "16000")
	defer func() {
		os.Unsetenv("ECHOWARP_PORT")
		os.Unsetenv("ECHOWARP_SAMPLE_RATE")
	}()

	// CLI flags override everything
	cmd := newDaemonStartCmd()
	_ = cmd.ParseFlags([]string{
		"--pid-file", pidFile,
		"--config", configFile,
		"--port", "7777",
	})

	_, cfg, _, err := setupDaemon(cmd)
	if err != nil {
		t.Fatalf("setupDaemon failed: %v", err)
	}

	// Port: CLI flag (7777) should override env var (9999) and config file (8080)
	if cfg.Port != 7777 {
		t.Errorf("expected port 7777 from CLI flag, got %d", cfg.Port)
	}

	// Sample rate: env var (16000) should override config file (24000)
	if cfg.SampleRate != 16000 {
		t.Errorf("expected sample rate 16000 from env var, got %d", cfg.SampleRate)
	}

	// Channels: config file (2) should be used (no env var or CLI override)
	if cfg.Channels != 2 {
		t.Errorf("expected channels 2 from config file, got %d", cfg.Channels)
	}

	// Max clients: config file (5) should be used (no env var or CLI override)
	if cfg.MaxClients != 5 {
		t.Errorf("expected max_clients 5 from config file, got %d", cfg.MaxClients)
	}

	// Log level: config file (warn) should be used (no env var or CLI override)
	if cfg.LogLevel != "warn" {
		t.Errorf("expected log level 'warn' from config file, got %s", cfg.LogLevel)
	}
}

// TestDaemonStartCommand_ClientModeFlags asserts that phase-2 client-mode
// flags are registered on `daemon start` and carry the spec defaults.
func TestDaemonStartCommand_ClientModeFlags(t *testing.T) {
	cmd := newDaemonStartCmd()
	flags := cmd.Flags()

	for _, name := range []string{"mode", "address", "discover", "nickname",
		"auto-reconnect", "auto-reconnect-attempts", "tls-insecure"} {
		if flags.Lookup(name) == nil {
			t.Errorf("daemon start command should have --%s flag", name)
		}
	}

	mode, _ := flags.GetString("mode")
	if mode != "server" {
		t.Errorf("expected default mode 'server', got %q", mode)
	}

	autoReconnect, _ := flags.GetBool("auto-reconnect")
	if !autoReconnect {
		t.Error("expected default auto-reconnect=true")
	}

	attempts, _ := flags.GetInt("auto-reconnect-attempts")
	if attempts != 5 {
		t.Errorf("expected default auto-reconnect-attempts=5, got %d", attempts)
	}
}

// TestCreateRunnerFactory_ClientMode asserts that createRunnerFactory routes
// client-mode configs to NewClientApp rather than NewServerApp. The returned
// runner's concrete type is the witness for the branch being taken — ensures
// phase 2's ClientApp wiring is live and not shadowed by the server branch.
func TestCreateRunnerFactory_ClientMode(t *testing.T) {
	cfg := config.Config{
		Mode:       config.ModeClient,
		Port:       4415,
		Address:    "127.0.0.1",
		Password:   "test",
		SampleRate: 48000,
		Channels:   1,
	}

	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))

	factory := createRunnerFactory(cfg, logger, nil, nil)
	if factory == nil {
		t.Fatal("createRunnerFactory should return non-nil factory")
	}

	runner, err := factory(echowarp.NodeConfig{}, logger, nil, nil, nil)
	if err != nil {
		t.Fatalf("factory should not error: %v", err)
	}
	if _, ok := runner.(*app.ClientApp); !ok {
		t.Errorf("expected *app.ClientApp for client mode, got %T", runner)
	}
}

// TestCreateRunnerFactory_ServerModeRoute asserts the default branch still
// produces a ServerApp — regression guard so that the new switch doesn't
// silently break the existing server path.
func TestCreateRunnerFactory_ServerModeRoute(t *testing.T) {
	cfg := config.Config{
		Mode:       config.ModeServer,
		Port:       4415,
		Password:   "test",
		SampleRate: 48000,
		Channels:   1,
	}

	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))
	rateLimiter := auth.NewIPRateLimiter(5)

	factory := createRunnerFactory(cfg, logger, rateLimiter, nil)
	runner, err := factory(echowarp.NodeConfig{}, logger, nil, nil, rateLimiter)
	if err != nil {
		t.Fatalf("factory should not error: %v", err)
	}
	if _, ok := runner.(*app.ServerApp); !ok {
		t.Errorf("expected *app.ServerApp for server mode, got %T", runner)
	}
}

// TestApplyDaemonClientOverrides asserts that applyDaemonClientOverrides
// correctly copies client-mode CLI flags into the config. Client-mode defaults
// (auto-reconnect=true, attempts=5) are seeded by LoadWithViper and are NOT
// reapplied here — see TestLoadWithViperClientModeDefaults below.
func TestApplyDaemonClientOverrides(t *testing.T) {
	cmd := newDaemonStartCmd()
	_ = cmd.Flags().Set("address", "10.0.0.5")
	_ = cmd.Flags().Set("nickname", "alice")
	_ = cmd.Flags().Set("tls-insecure", "true")

	// Simulate the post-LoadWithViper(ModeClient) state: defaults already seeded.
	cfg := config.Config{
		Mode:                  config.ModeServer,
		AutoReconnect:         true,
		AutoReconnectAttempts: 5,
	}
	applyDaemonClientOverrides(cmd, &cfg)

	if cfg.Mode != config.ModeClient {
		t.Errorf("expected mode=client, got %q", cfg.Mode)
	}
	if cfg.Address != "10.0.0.5" {
		t.Errorf("expected address=10.0.0.5, got %q", cfg.Address)
	}
	if cfg.Nickname != "alice" {
		t.Errorf("expected nickname=alice, got %q", cfg.Nickname)
	}
	if !cfg.TLSInsecure {
		t.Error("expected tls-insecure=true")
	}
	// Defaults preserved because no flag was passed.
	if !cfg.AutoReconnect {
		t.Error("expected auto-reconnect default=true to be preserved")
	}
	if cfg.AutoReconnectAttempts != 5 {
		t.Errorf("expected auto-reconnect-attempts default=5, got %d", cfg.AutoReconnectAttempts)
	}
}

// TestDaemonFlagsExtended verifies each phase 3 CLI flag is registered on the
// daemon start command and parses into the expected type/value.
func TestDaemonFlagsExtended(t *testing.T) {
	cases := []struct {
		name     string
		arg      string
		lookup   func(cmd *cobra.Command) (any, error)
		expected any
	}{
		{"duplex", "--duplex=true", func(c *cobra.Command) (any, error) { return c.Flags().GetBool("duplex") }, true},
		{"conference", "--conference=true", func(c *cobra.Command) (any, error) { return c.Flags().GetBool("conference") }, true},
		{"loopback", "--loopback=true", func(c *cobra.Command) (any, error) { return c.Flags().GetBool("loopback") }, true},
		{"aec", "--aec=true", func(c *cobra.Command) (any, error) { return c.Flags().GetBool("aec") }, true},
		{"server-muted", "--server-muted=true", func(c *cobra.Command) (any, error) { return c.Flags().GetBool("server-muted") }, true},
		{"hwid-required", "--hwid-required=true", func(c *cobra.Command) (any, error) { return c.Flags().GetBool("hwid-required") }, true},
		{"record", "--record=mix", func(c *cobra.Command) (any, error) { return c.Flags().GetString("record") }, "mix"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := newDaemonStartCmd()
			if cmd.Flags().Lookup(tc.name) == nil {
				t.Fatalf("daemon start command missing --%s flag", tc.name)
			}
			if err := cmd.ParseFlags([]string{tc.arg}); err != nil {
				t.Fatalf("ParseFlags(%q) failed: %v", tc.arg, err)
			}
			if !cmd.Flags().Changed(tc.name) {
				t.Errorf("flag --%s should be marked Changed after ParseFlags", tc.name)
			}
			got, err := tc.lookup(cmd)
			if err != nil {
				t.Fatalf("GetFlag error: %v", err)
			}
			if got != tc.expected {
				t.Errorf("flag --%s: got %v, want %v", tc.name, got, tc.expected)
			}
		})
	}
}

// TestDaemonAutoReconnectRespectsFile is the phase 3 regression guard for the
// phase 2 follow-up warning. It simulates a file-loaded config with
// auto_reconnect=false and verifies that applyDaemonClientOverrides no longer
// forces the default back to true when the CLI flag is not explicitly passed.
func TestDaemonAutoReconnectRespectsFile(t *testing.T) {
	cmd := newDaemonStartCmd()
	_ = cmd.Flags().Set("address", "10.0.0.5")
	// Note: --auto-reconnect is NOT Set here, so Changed() returns false.

	// File-loaded config: user explicitly disabled auto-reconnect.
	cfg := config.Config{
		Mode:                  config.ModeClient,
		AutoReconnect:         false,
		AutoReconnectAttempts: 0,
	}
	applyDaemonClientOverrides(cmd, &cfg)

	if cfg.AutoReconnect {
		t.Error("auto-reconnect must remain false when file disabled it and no flag passed")
	}
	if cfg.AutoReconnectAttempts != 0 {
		t.Errorf("auto-reconnect-attempts must remain 0 when no flag passed, got %d", cfg.AutoReconnectAttempts)
	}
}

// TestLoadWithViperClientModeDefaults verifies that LoadWithViper seeds
// client-mode defaults (attempts=5) before any file or env parsing.
// AutoReconnect defaults to false (opt-in via --auto-reconnect or config).
func TestLoadWithViperClientModeDefaults(t *testing.T) {
	// Isolate from any ambient env vars.
	t.Setenv("ECHOWARP_AUTO_RECONNECT", "")
	t.Setenv("ECHOWARP_AUTO_RECONNECT_ATTEMPTS", "")

	cfg, err := config.LoadWithViper("", config.ModeClient)
	if err != nil {
		t.Fatalf("LoadWithViper error: %v", err)
	}
	if cfg.AutoReconnect {
		t.Error("expected client-mode default auto-reconnect=false (opt-in)")
	}
	if cfg.AutoReconnectAttempts != 5 {
		t.Errorf("expected client-mode default attempts=5, got %d", cfg.AutoReconnectAttempts)
	}
}

// TestRunDaemonStart_ClientModeMissingAddress asserts that `daemon start
// --mode client` without --address or --discover fails validation up-front
// instead of booting a half-configured client.
func TestRunDaemonStart_ClientModeMissingAddress(t *testing.T) {
	tmpDir := t.TempDir()
	pidFile := filepath.Join(tmpDir, "daemon.pid")
	logFile := filepath.Join(tmpDir, "daemon.log")

	cmd := newDaemonStartCmd()
	_ = cmd.Flags().Set("mode", "client")
	_ = cmd.Flags().Set("password", "test")
	_ = cmd.Flags().Set("pid-file", pidFile)
	_ = cmd.Flags().Set("log-file", logFile)

	err := runDaemonStart(cmd, nil)
	if err == nil {
		t.Fatal("expected error for client mode without --address/--discover")
	}
	// The specific error phrase must mention --address/--discover so CLI users
	// can self-serve without reading the source.
	if !bytes.Contains([]byte(err.Error()), []byte("address")) &&
		!bytes.Contains([]byte(err.Error()), []byte("discover")) {
		t.Errorf("error should mention --address or --discover: %v", err)
	}
}

// TestLoadWithViperFileOverridesClientDefaults is the end-to-end regression
// guard for the phase 2 follow-up warning: a file-backed client config that
// explicitly disables auto-reconnect must survive LoadWithViper's default
// seeding AND applyDaemonClientOverrides when no --auto-reconnect flag is set.
func TestLoadWithViperFileOverridesClientDefaults(t *testing.T) {
	// Isolate from any ambient env vars that could clobber the file values.
	t.Setenv("ECHOWARP_AUTO_RECONNECT", "")
	t.Setenv("ECHOWARP_AUTO_RECONNECT_ATTEMPTS", "")

	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "daemon.yaml")
	const body = "auto_reconnect: false\nauto_reconnect_attempts: 99\n"
	if err := os.WriteFile(cfgPath, []byte(body), 0o600); err != nil {
		t.Fatalf("write temp config: %v", err)
	}

	cfg, err := config.LoadWithViper(cfgPath, config.ModeClient)
	if err != nil {
		t.Fatalf("LoadWithViper: %v", err)
	}
	if cfg.AutoReconnect {
		t.Error("file value auto_reconnect=false must win over client-mode default")
	}
	if cfg.AutoReconnectAttempts != 99 {
		t.Errorf("file value auto_reconnect_attempts=99 must win, got %d", cfg.AutoReconnectAttempts)
	}

	// Build a fresh command where no flags are Changed: applyDaemonClientOverrides
	// must NOT clobber the file-provided values.
	cmd := newDaemonStartCmd()
	applyDaemonClientOverrides(cmd, &cfg)

	if cfg.AutoReconnect {
		t.Error("applyDaemonClientOverrides must preserve file auto_reconnect=false when no flag is set")
	}
	if cfg.AutoReconnectAttempts != 99 {
		t.Errorf("applyDaemonClientOverrides must preserve file auto_reconnect_attempts=99, got %d", cfg.AutoReconnectAttempts)
	}
}

// TestValidateDaemonRecordMode_Invalid asserts that unknown --record values are
// rejected with a clear error listing the valid options.
func TestValidateDaemonRecordMode_Invalid(t *testing.T) {
	err := validateDaemonRecordMode("bogus")
	if err == nil {
		t.Fatal("expected error for bogus record mode")
	}
	msg := err.Error()
	for _, want := range []string{"record", "mix", "tracks", "both"} {
		if !bytes.Contains([]byte(msg), []byte(want)) {
			t.Errorf("error should mention %q; got %q", want, msg)
		}
	}
}

// TestValidateDaemonRecordMode_Valid verifies that all accepted --record values
// pass validation without error.
func TestValidateDaemonRecordMode_Valid(t *testing.T) {
	for _, mode := range []string{"", "mix", "tracks", "both"} {
		if err := validateDaemonRecordMode(mode); err != nil {
			t.Errorf("validateDaemonRecordMode(%q): unexpected error %v", mode, err)
		}
	}
}

// TestDaemonRecordInvalidValue exercises the full runDaemonStart path with
// --record=bogus and asserts the error surfaces before any side effect (no PID
// file written).
func TestDaemonRecordInvalidValue(t *testing.T) {
	tmpDir := t.TempDir()
	pidFile := filepath.Join(tmpDir, "daemon.pid")
	logFile := filepath.Join(tmpDir, "daemon.log")

	cmd := newDaemonStartCmd()
	_ = cmd.Flags().Set("password", "test")
	_ = cmd.Flags().Set("pid-file", pidFile)
	_ = cmd.Flags().Set("log-file", logFile)
	_ = cmd.Flags().Set("record", "bogus")

	err := runDaemonStart(cmd, nil)
	if err == nil {
		t.Fatal("expected error for --record=bogus")
	}
	if !bytes.Contains([]byte(err.Error()), []byte("record")) {
		t.Errorf("error should mention 'record': %v", err)
	}
	if _, statErr := os.Stat(pidFile); statErr == nil {
		t.Error("PID file must not be written when --record validation fails")
	}
}
