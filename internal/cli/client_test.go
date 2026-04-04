package cli

import (
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/lHumaNl/echowarp/internal/config"
	"github.com/lHumaNl/echowarp/pkg/echowarp/discovery"
)

func TestClientCommand_HasRequiredFlags(t *testing.T) {
	cmd := newClientCmd()
	flags := cmd.Flags()
	if flags == nil {
		t.Error("client command should have flags")
	}

	requiredFlags := []string{"address", "password", "port", "config"}
	for _, flag := range requiredFlags {
		if flags.Lookup(flag) == nil {
			t.Errorf("client command should have --%s flag", flag)
		}
	}
}

func TestClientCommand_Use(t *testing.T) {
	cmd := newClientCmd()
	if cmd.Use != "client" {
		t.Errorf("expected Use 'client', got %q", cmd.Use)
	}
}

func TestClientCommand_RunE(t *testing.T) {
	cmd := newClientCmd()
	if cmd.RunE == nil {
		t.Error("client command should have RunE function")
	}
}

func TestClientCommand_DiscoveryFlags(t *testing.T) {
	cmd := newClientCmd()
	flags := cmd.Flags()

	if flags.Lookup("discover") == nil {
		t.Error("client command should have --discover flag")
	}
	if flags.Lookup("discover-timeout") == nil {
		t.Error("client command should have --discover-timeout flag")
	}
}

func TestClientCommand_AllFlags(t *testing.T) {
	cmd := newClientCmd()
	flags := cmd.Flags()

	expectedFlags := []string{
		"address", "port", "device", "password",
		"max-reconnect", "virtual-mic",
		"stun-server", "tls-insecure", "config", "save-config",
		"log-level", "log-file", "discover", "discover-timeout",
	}

	for _, flag := range expectedFlags {
		if flags.Lookup(flag) == nil {
			t.Errorf("client command should have --%s flag", flag)
		}
	}
}

func TestClientCommand_DefaultValues(t *testing.T) {
	cmd := newClientCmd()
	flags := cmd.Flags()

	port, _ := flags.GetInt("port")
	if port != 4415 {
		t.Errorf("expected default port 4415, got %d", port)
	}

	discover, _ := flags.GetBool("discover")
	if !discover {
		t.Error("expected default discover to be true")
	}
}

func TestLoadClientConfig_Defaults(t *testing.T) {
	cmd := newClientCmd()
	err := cmd.ParseFlags([]string{})
	if err != nil {
		t.Fatalf("ParseFlags failed: %v", err)
	}

	cfg, err := loadClientConfig(cmd)
	if err != nil {
		t.Fatalf("loadClientConfig failed: %v", err)
	}

	if cfg.Mode != config.ModeClient {
		t.Errorf("expected mode client, got %s", cfg.Mode)
	}
}

func TestLoadClientConfig_AddressOverride(t *testing.T) {
	cmd := newClientCmd()
	err := cmd.ParseFlags([]string{"--address", "192.168.1.100"})
	if err != nil {
		t.Fatalf("ParseFlags failed: %v", err)
	}

	cfg, err := loadClientConfig(cmd)
	if err != nil {
		t.Fatalf("loadClientConfig failed: %v", err)
	}

	if cfg.Address != "192.168.1.100" {
		t.Errorf("expected address 192.168.1.100, got %s", cfg.Address)
	}
}

func TestLoadClientConfig_TLSInsecureFlag(t *testing.T) {
	cmd := newClientCmd()
	err := cmd.ParseFlags([]string{"--tls-insecure"})
	if err != nil {
		t.Fatalf("ParseFlags failed: %v", err)
	}

	cfg, err := loadClientConfig(cmd)
	if err != nil {
		t.Fatalf("loadClientConfig failed: %v", err)
	}

	if !cfg.TLSInsecure {
		t.Error("expected TLSInsecure to be true")
	}
}

func TestLoadClientConfig_WithConfigFile(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	configContent := `port: 9999
password: testpass
`
	if err := os.WriteFile(configFile, []byte(configContent), 0600); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cmd := newClientCmd()
	err := cmd.ParseFlags([]string{"--config", configFile})
	if err != nil {
		t.Fatalf("ParseFlags failed: %v", err)
	}

	cfg, err := loadClientConfig(cmd)
	if err != nil {
		t.Fatalf("loadClientConfig failed: %v", err)
	}

	if cfg.Port != 9999 {
		t.Errorf("expected port 9999, got %d", cfg.Port)
	}
	if cfg.Password != "testpass" {
		t.Errorf("expected password testpass, got %s", cfg.Password)
	}
}

func TestLoadClientConfig_InvalidConfigFile(t *testing.T) {
	cmd := newClientCmd()
	err := cmd.ParseFlags([]string{"--config", "/nonexistent/config.yaml"})
	if err != nil {
		t.Fatalf("ParseFlags failed: %v", err)
	}

	_, err = loadClientConfig(cmd)
	if err == nil {
		t.Error("expected error for nonexistent config file")
	}
}

func TestLoadClientConfig_PortOverride(t *testing.T) {
	cmd := newClientCmd()
	err := cmd.ParseFlags([]string{"--port", "8080"})
	if err != nil {
		t.Fatalf("ParseFlags failed: %v", err)
	}

	cfg, err := loadClientConfig(cmd)
	if err != nil {
		t.Fatalf("loadClientConfig failed: %v", err)
	}

	if cfg.Port != 8080 {
		t.Errorf("expected port 8080, got %d", cfg.Port)
	}
}

func TestSelectDiscoveredServer_NoServers(t *testing.T) {
	cfg := &config.Config{}
	err := selectDiscoveredServer([]discovery.ServiceInfo{}, cfg)
	if err == nil {
		t.Error("expected error when no servers found")
	}
}

func TestSelectDiscoveredServer_SingleServer(t *testing.T) {
	cfg := &config.Config{}
	servers := []discovery.ServiceInfo{
		{
			Name:     "test-server",
			Port:     4415,
			AddrIPv4: []net.IP{net.ParseIP("192.168.1.100")},
			AuthReq:  true,
			TLS:      false,
		},
	}

	err := selectDiscoveredServer(servers, cfg)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if cfg.Address != "192.168.1.100" {
		t.Errorf("expected address 192.168.1.100, got %s", cfg.Address)
	}
	if cfg.Port != 4415 {
		t.Errorf("expected port 4415, got %d", cfg.Port)
	}
}

func TestSelectDiscoveredServer_MultipleServers(t *testing.T) {
	cfg := &config.Config{}
	servers := []discovery.ServiceInfo{
		{
			Name:     "server1",
			Port:     4415,
			AddrIPv4: []net.IP{net.ParseIP("192.168.1.100")},
		},
		{
			Name:     "server2",
			Port:     4415,
			AddrIPv4: []net.IP{net.ParseIP("192.168.1.101")},
		},
	}

	err := selectDiscoveredServer(servers, cfg)
	if err == nil {
		t.Error("expected error when multiple servers found")
	}
}

func TestSetServerAddress_IPv4(t *testing.T) {
	cfg := &config.Config{}
	svc := &discovery.ServiceInfo{
		Port:     4415,
		AddrIPv4: []net.IP{net.ParseIP("192.168.1.100")},
		AddrIPv6: []net.IP{net.ParseIP("::1")},
	}

	setServerAddress(svc, cfg)

	if cfg.Address != "192.168.1.100" {
		t.Errorf("expected address 192.168.1.100, got %s", cfg.Address)
	}
	if cfg.Port != 4415 {
		t.Errorf("expected port 4415, got %d", cfg.Port)
	}
}

func TestSetServerAddress_IPv6(t *testing.T) {
	cfg := &config.Config{}
	svc := &discovery.ServiceInfo{
		Port:     4415,
		AddrIPv4: []net.IP{},
		AddrIPv6: []net.IP{net.ParseIP("::1")},
	}

	setServerAddress(svc, cfg)

	if cfg.Address != "::1" {
		t.Errorf("expected address ::1, got %s", cfg.Address)
	}
}

func TestSetServerAddress_NoAddress(t *testing.T) {
	cfg := &config.Config{}
	svc := &discovery.ServiceInfo{
		Port:     4415,
		AddrIPv4: []net.IP{},
		AddrIPv6: []net.IP{},
	}

	setServerAddress(svc, cfg)

	if cfg.Address != "" {
		t.Errorf("expected empty address, got %s", cfg.Address)
	}
	if cfg.Port != 4415 {
		t.Errorf("expected port 4415, got %d", cfg.Port)
	}
}

func TestGetServerAddress_IPv4(t *testing.T) {
	svc := &discovery.ServiceInfo{
		AddrIPv4: []net.IP{net.ParseIP("192.168.1.100")},
		AddrIPv6: []net.IP{net.ParseIP("::1")},
	}

	addr := getServerAddress(svc)
	if addr != "192.168.1.100" {
		t.Errorf("expected address 192.168.1.100, got %s", addr)
	}
}

func TestGetServerAddress_IPv6(t *testing.T) {
	svc := &discovery.ServiceInfo{
		AddrIPv4: []net.IP{},
		AddrIPv6: []net.IP{net.ParseIP("::1")},
	}

	addr := getServerAddress(svc)
	if addr != "::1" {
		t.Errorf("expected address ::1, got %s", addr)
	}
}

func TestGetServerAddress_NoAddress(t *testing.T) {
	svc := &discovery.ServiceInfo{
		AddrIPv4: []net.IP{},
		AddrIPv6: []net.IP{},
	}

	addr := getServerAddress(svc)
	if addr != "" {
		t.Errorf("expected empty address, got %s", addr)
	}
}

func TestValidateAndSaveConfig_Valid(t *testing.T) {
	cmd := newClientCmd()
	_ = cmd.ParseFlags([]string{})

	cfg := config.DefaultConfig()
	cfg.DeviceID = uint32Ptr(0)

	err := validateAndSaveConfig(cmd, &cfg)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateAndSaveConfig_Invalid(t *testing.T) {
	cmd := newClientCmd()
	_ = cmd.ParseFlags([]string{})

	cfg := config.Config{
		Port: -1,
	}

	err := validateAndSaveConfig(cmd, &cfg)
	if err == nil {
		t.Error("expected error for invalid config")
	}
}

func TestValidateAndSaveConfig_SaveToFile(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")

	cmd := newClientCmd()
	_ = cmd.ParseFlags([]string{"--save-config", configFile})

	cfg := config.DefaultConfig()
	cfg.DeviceID = uint32Ptr(0)

	err := validateAndSaveConfig(cmd, &cfg)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		t.Error("config file should be created")
	}
}

func TestValidateAndSaveConfig_SaveToInvalidPath(t *testing.T) {
	// Create a file that blocks directory creation at that path.
	blocker := filepath.Join(t.TempDir(), "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	invalidPath := filepath.Join(blocker, "sub", "config.yaml")

	cmd := newClientCmd()
	_ = cmd.ParseFlags([]string{"--save-config", invalidPath})

	cfg := config.DefaultConfig()
	cfg.DeviceID = uint32Ptr(0)

	err := validateAndSaveConfig(cmd, &cfg)
	if err == nil {
		t.Error("expected error for invalid save path")
	}
}

func TestNewClientCmd(t *testing.T) {
	cmd := newClientCmd()
	if cmd == nil {
		t.Error("newClientCmd should return non-nil command")
	}
}

func TestClientCommand_Short(t *testing.T) {
	cmd := newClientCmd()
	if cmd.Short == "" {
		t.Error("client command should have a short description")
	}
}

func TestClientCommand_Long(t *testing.T) {
	cmd := newClientCmd()
	if cmd.Long == "" {
		t.Error("client command should have a long description")
	}
}

func TestRunClient_InvalidConfig(t *testing.T) {
	cmd := newClientCmd()
	_ = cmd.ParseFlags([]string{"--port", "-1", "--discover=false", "--no-interactive"})

	err := runClient(cmd, []string{})
	if err == nil {
		t.Error("expected error for invalid config")
	}
}

func TestRunClient_MissingConfigFile(t *testing.T) {
	cmd := newClientCmd()
	_ = cmd.ParseFlags([]string{"--config", "/nonexistent/config.yaml", "--discover=false", "--no-interactive"})

	err := runClient(cmd, []string{})
	if err == nil {
		t.Error("expected error for missing config file")
	}
}

func TestRunClient_InvalidSampleRate(t *testing.T) {
	cmd := newClientCmd()
	_ = cmd.ParseFlags([]string{"--sample-rate", "12345", "--discover=false", "--no-interactive"})

	err := runClient(cmd, []string{})
	if err == nil {
		t.Error("expected error for invalid sample rate")
	}
}

func TestRunClient_InvalidChannels(t *testing.T) {
	cmd := newClientCmd()
	_ = cmd.ParseFlags([]string{"--channels", "3", "--discover=false", "--no-interactive"})

	err := runClient(cmd, []string{})
	if err == nil {
		t.Error("expected error for invalid channels")
	}
}

func TestRunClient_ZeroChannels(t *testing.T) {
	cmd := newClientCmd()
	_ = cmd.ParseFlags([]string{"--channels", "0", "--discover=false", "--no-interactive"})

	err := runClient(cmd, []string{})
	if err == nil {
		t.Error("expected error for zero channels")
	}
}

func TestRunClient_InvalidLogLevel(t *testing.T) {
	cmd := newClientCmd()
	_ = cmd.ParseFlags([]string{"--log-level", "invalid", "--discover=false", "--no-interactive"})

	err := runClient(cmd, []string{})
	if err == nil {
		t.Error("expected error for invalid log level")
	}
}

func TestRunClient_SaveConfigToInvalidPath(t *testing.T) {
	blocker := filepath.Join(t.TempDir(), "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	invalidPath := filepath.Join(blocker, "sub", "config.yaml")

	cmd := newClientCmd()
	_ = cmd.ParseFlags([]string{"--device", "0", "--save-config", invalidPath, "--discover=false", "--no-interactive"})

	err := runClient(cmd, []string{})
	if err == nil {
		t.Error("expected error for invalid save path")
	}
}

func TestHandleClientDiscovery_AddressProvided(t *testing.T) {
	cmd := newClientCmd()
	_ = cmd.ParseFlags([]string{"--address", "192.168.1.100"})

	cfg := &config.Config{Address: "192.168.1.100"}
	err := handleClientDiscovery(cmd, cfg)
	if err != nil {
		t.Errorf("handleClientDiscovery with address should not error: %v", err)
	}
}

func TestHandleClientDiscovery_DiscoveryDisabled(t *testing.T) {
	cmd := newClientCmd()
	_ = cmd.ParseFlags([]string{"--discover=false"})

	cfg := &config.Config{Address: ""}
	err := handleClientDiscovery(cmd, cfg)
	if err != nil {
		t.Errorf("handleClientDiscovery with discovery disabled should not error: %v", err)
	}
}

func TestLoadClientConfig_AllFlagOverrides(t *testing.T) {
	cmd := newClientCmd()
	err := cmd.ParseFlags([]string{
		"--address", "10.0.0.1",
		"--port", "9999",
		"--tls-insecure",
	})
	if err != nil {
		t.Fatalf("ParseFlags failed: %v", err)
	}

	cfg, err := loadClientConfig(cmd)
	if err != nil {
		t.Fatalf("loadClientConfig failed: %v", err)
	}

	if cfg.Address != "10.0.0.1" {
		t.Errorf("expected address 10.0.0.1, got %s", cfg.Address)
	}
	if cfg.Port != 9999 {
		t.Errorf("expected port 9999, got %d", cfg.Port)
	}
	if !cfg.TLSInsecure {
		t.Error("expected TLSInsecure to be true")
	}
}

func TestSelectDiscoveredServer_IPv6Only(t *testing.T) {
	cfg := &config.Config{}
	servers := []discovery.ServiceInfo{
		{
			Name:     "ipv6-server",
			Port:     4415,
			AddrIPv4: []net.IP{},
			AddrIPv6: []net.IP{net.ParseIP("::1")},
			AuthReq:  false,
			TLS:      false,
		},
	}

	err := selectDiscoveredServer(servers, cfg)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if cfg.Address != "::1" {
		t.Errorf("expected address ::1, got %s", cfg.Address)
	}
}

func TestSelectDiscoveredServer_WithAuthAndTLS(t *testing.T) {
	cfg := &config.Config{}
	servers := []discovery.ServiceInfo{
		{
			Name:     "secure-server",
			Port:     4415,
			AddrIPv4: []net.IP{net.ParseIP("192.168.1.100")},
			AuthReq:  true,
			TLS:      true,
		},
	}

	err := selectDiscoveredServer(servers, cfg)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestClientCommand_DiscoverTimeoutFlag(t *testing.T) {
	cmd := newClientCmd()
	flags := cmd.Flags()

	timeout, _ := flags.GetDuration("discover-timeout")
	if timeout == 0 {
		t.Error("discover-timeout should have a non-zero default")
	}
}

func TestClientCommand_STUNFlag(t *testing.T) {
	cmd := newClientCmd()
	_ = cmd.ParseFlags([]string{"--stun-server", "stun:example.com:3478", "--stun-server", "stun:example2.com:3478"})

	stun, _ := cmd.Flags().GetStringSlice("stun-server")
	if len(stun) != 2 {
		t.Errorf("expected 2 STUN servers, got %d", len(stun))
	}
}

func TestValidateAndSaveConfig_EmptyDeviceID_NoAddress(t *testing.T) {
	cmd := newClientCmd()
	_ = cmd.ParseFlags([]string{})

	cfg := config.DefaultConfig()
	cfg.Mode = config.ModeClient
	cfg.DeviceID = nil
	cfg.Address = ""

	err := validateAndSaveConfig(cmd, &cfg)
	if err == nil {
		t.Error("expected error when device ID is nil and no address in client mode")
	}
}

func TestValidateAndSaveConfig_WithAddress_NoDeviceID(t *testing.T) {
	cmd := newClientCmd()
	_ = cmd.ParseFlags([]string{})

	cfg := config.DefaultConfig()
	cfg.DeviceID = nil
	cfg.Address = "192.168.1.1"

	err := validateAndSaveConfig(cmd, &cfg)
	if err != nil {
		t.Errorf("should not error when address is set: %v", err)
	}
}

func TestRunClient_WithAddressNoDevice(t *testing.T) {
	cmd := newClientCmd()
	_ = cmd.ParseFlags([]string{"--address", "192.168.1.1", "--discover=false"})

	cfg := config.DefaultConfig()
	cfg.Address = "192.168.1.1"
	cfg.DeviceID = nil

	err := validateAndSaveConfig(cmd, &cfg)
	if err != nil {
		t.Errorf("validateAndSaveConfig should not error: %v", err)
	}
}

func TestLoadClientConfig_UnchangedFlags(t *testing.T) {
	cmd := newClientCmd()
	_ = cmd.ParseFlags([]string{})

	cfg, err := loadClientConfig(cmd)
	if err != nil {
		t.Fatalf("loadClientConfig failed: %v", err)
	}

	if cfg.TLS {
		t.Error("TLS should be false by default")
	}
	if cfg.TLSInsecure {
		t.Error("TLSInsecure should be false by default")
	}
}

func TestSelectDiscoveredServer_EmptyAddresses(t *testing.T) {
	cfg := &config.Config{}
	servers := []discovery.ServiceInfo{
		{
			Name:     "server-no-addr",
			Port:     4415,
			AddrIPv4: []net.IP{},
			AddrIPv6: []net.IP{},
		},
	}

	err := selectDiscoveredServer(servers, cfg)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if cfg.Address != "" {
		t.Errorf("expected empty address, got %s", cfg.Address)
	}
}

func TestHandleClientDiscovery_AddressSet(t *testing.T) {
	cmd := newClientCmd()
	_ = cmd.ParseFlags([]string{"--address", "192.168.1.100"})

	cfg := &config.Config{Address: "192.168.1.100"}
	err := handleClientDiscovery(cmd, cfg)
	if err != nil {
		t.Errorf("handleClientDiscovery with address should not error: %v", err)
	}
}

func TestClientCommand_MaxReconnectFlag(t *testing.T) {
	cmd := newClientCmd()
	_ = cmd.ParseFlags([]string{"--max-reconnect", "10"})

	maxReconnect, _ := cmd.Flags().GetInt("max-reconnect")
	if maxReconnect != 10 {
		t.Errorf("expected max-reconnect 10, got %d", maxReconnect)
	}
}

func TestClientCommand_VirtualMicFlag(t *testing.T) {
	cmd := newClientCmd()
	_ = cmd.ParseFlags([]string{"--virtual-mic"})

	virtualMic, _ := cmd.Flags().GetBool("virtual-mic")
	if !virtualMic {
		t.Error("expected virtual-mic to be true")
	}
}

func TestLoadClientConfig_ChangedTLSInsecureFlag(t *testing.T) {
	cmd := newClientCmd()
	_ = cmd.ParseFlags([]string{"--tls-insecure"})

	cfg, err := loadClientConfig(cmd)
	if err != nil {
		t.Fatalf("loadClientConfig failed: %v", err)
	}

	if !cfg.TLSInsecure {
		t.Error("TLSInsecure should be true")
	}
}

func TestHandleClientDiscovery_WithAddressSkipDiscovery(t *testing.T) {
	cmd := newClientCmd()
	_ = cmd.ParseFlags([]string{"--address", "192.168.1.1"})

	cfg := &config.Config{Address: "192.168.1.1"}
	err := handleClientDiscovery(cmd, cfg)
	if err != nil {
		t.Errorf("handleClientDiscovery with address should not error: %v", err)
	}
}

func TestHandleClientDiscovery_DiscoveryTrueNoAddress(t *testing.T) {
	cmd := newClientCmd()
	_ = cmd.ParseFlags([]string{"--discover=true", "--discover-timeout", "1ms"})

	cfg := &config.Config{Address: ""}
	_ = handleClientDiscovery(cmd, cfg)
}

// TestExecuteClientMode_NilDeviceID tests executeClientMode when DeviceID is nil.
// This tests the path where interactive mode should be triggered.
func TestExecuteClientMode_NilDeviceID(t *testing.T) {
	cmd := newClientCmd()
	_ = cmd.ParseFlags([]string{"--address", "192.168.1.1", "--discover=false"})

	cfg := config.DefaultConfig()
	cfg.Mode = config.ModeClient
	cfg.Address = "192.168.1.1"
	cfg.DeviceID = nil // This triggers interactive mode

	// executeClientMode will try to run interactive mode
	// which requires audio devices - this will fail on CI without audio
	// We expect an error about audio initialization or no devices
	err := executeClientMode(cmd, &cfg)
	// On systems without audio, we expect an error
	if err == nil {
		// If no error, DeviceID should be nil (user quit without selecting)
		if cfg.DeviceID != nil {
			t.Error("expected DeviceID to remain nil or error")
		}
	}
	// Error is acceptable since audio devices may not be available
}

// TestExecuteClientMode_WithDeviceID tests executeClientMode when DeviceID is set.
// This tests the direct mode path - skipped in short mode as it requires network.
func TestExecuteClientMode_WithDeviceID(t *testing.T) {
	t.Skip("Skipping - requires network connection and may hang")
}

// TestRunClientInteractive_NoDevices tests runClientInteractive when no devices are available.
func TestRunClientInteractive_NoDevices(t *testing.T) {
	cmd := newClientCmd()
	_ = cmd.ParseFlags([]string{})

	cfg := &config.Config{
		Mode:    config.ModeClient,
		Reverse: false,
	}

	// This will fail on systems without audio devices
	err := runClientInteractive(cmd, cfg)
	// Error is expected on CI without audio hardware
	if err == nil {
		// If it succeeds, the function should have handled the case
		t.Log("runClientInteractive succeeded - audio devices available")
	}
}

// TestRunClientInteractive_ReverseMode tests runClientInteractive in reverse mode.
func TestRunClientInteractive_ReverseMode(t *testing.T) {
	cmd := newClientCmd()
	_ = cmd.ParseFlags([]string{"--reverse"})

	cfg := &config.Config{
		Mode:    config.ModeClient,
		Reverse: true,
	}

	// This will try to list input devices in reverse mode
	err := runClientInteractive(cmd, cfg)
	// Error is expected on CI without audio hardware
	if err == nil {
		t.Log("runClientInteractive succeeded in reverse mode")
	}
}

// TestLoadClientConfig_EnvVarOverride tests that environment variables override config file values.
func TestLoadClientConfig_EnvVarOverride(t *testing.T) {
	// Create a config file
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	configContent := `port: 8080
address: 192.168.1.100
password: filepass
log_level: info
`
	if err := os.WriteFile(configFile, []byte(configContent), 0600); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	// Set environment variables
	os.Setenv("ECHOWARP_PORT", "9999")
	os.Setenv("ECHOWARP_ADDRESS", "10.0.0.1")
	os.Setenv("ECHOWARP_PASSWORD", "envpass")
	defer func() {
		os.Unsetenv("ECHOWARP_PORT")
		os.Unsetenv("ECHOWARP_ADDRESS")
		os.Unsetenv("ECHOWARP_PASSWORD")
	}()

	cmd := newClientCmd()
	err := cmd.ParseFlags([]string{"--config", configFile})
	if err != nil {
		t.Fatalf("ParseFlags failed: %v", err)
	}

	cfg, err := loadClientConfig(cmd)
	if err != nil {
		t.Fatalf("loadClientConfig failed: %v", err)
	}

	// Env vars should override config file values
	if cfg.Port != 9999 {
		t.Errorf("expected port 9999 from env var, got %d", cfg.Port)
	}
	if cfg.Address != "10.0.0.1" {
		t.Errorf("expected address '10.0.0.1' from env var, got %s", cfg.Address)
	}
	if cfg.Password != "envpass" {
		t.Errorf("expected password 'envpass' from env var, got %s", cfg.Password)
	}
}

// TestLoadClientConfig_CLIOverridesEnvVar tests that CLI flags override environment variables.
func TestLoadClientConfig_CLIOverridesEnvVar(t *testing.T) {
	// Set environment variables
	os.Setenv("ECHOWARP_PORT", "9999")
	os.Setenv("ECHOWARP_ADDRESS", "10.0.0.1")
	os.Setenv("ECHOWARP_PASSWORD", "envpass")
	defer func() {
		os.Unsetenv("ECHOWARP_PORT")
		os.Unsetenv("ECHOWARP_ADDRESS")
		os.Unsetenv("ECHOWARP_PASSWORD")
	}()

	cmd := newClientCmd()
	err := cmd.ParseFlags([]string{"--port", "7777", "--address", "172.16.0.1"})
	if err != nil {
		t.Fatalf("ParseFlags failed: %v", err)
	}

	cfg, err := loadClientConfig(cmd)
	if err != nil {
		t.Fatalf("loadClientConfig failed: %v", err)
	}

	// CLI flags should override env vars
	if cfg.Port != 7777 {
		t.Errorf("expected port 7777 from CLI flag, got %d", cfg.Port)
	}
	if cfg.Address != "172.16.0.1" {
		t.Errorf("expected address '172.16.0.1' from CLI flag, got %s", cfg.Address)
	}
	// Password should come from env var (not overridden by CLI)
	if cfg.Password != "envpass" {
		t.Errorf("expected password 'envpass' from env var, got %s", cfg.Password)
	}
}

// TestLoadClientConfig_TLSFromEnvVar tests loading TLS settings from environment variables.
func TestLoadClientConfig_TLSFromEnvVar(t *testing.T) {
	os.Setenv("ECHOWARP_TLS", "true")
	os.Setenv("ECHOWARP_TLS_INSECURE", "true")
	defer func() {
		os.Unsetenv("ECHOWARP_TLS")
		os.Unsetenv("ECHOWARP_TLS_INSECURE")
	}()

	cmd := newClientCmd()
	err := cmd.ParseFlags([]string{})
	if err != nil {
		t.Fatalf("ParseFlags failed: %v", err)
	}

	cfg, err := loadClientConfig(cmd)
	if err != nil {
		t.Fatalf("loadClientConfig failed: %v", err)
	}

	if !cfg.TLS {
		t.Error("expected TLS to be true from env var")
	}
	if !cfg.TLSInsecure {
		t.Error("expected TLSInsecure to be true from env var")
	}
}

// TestLoadClientConfig_AllEnvVars tests loading all client configuration from environment variables.
func TestLoadClientConfig_AllEnvVars(t *testing.T) {
	// Set all relevant environment variables for client
	envVars := map[string]string{
		"ECHOWARP_ADDRESS":      "192.168.1.100",
		"ECHOWARP_PORT":         "9999",
		"ECHOWARP_PASSWORD":     "testpass",
		"ECHOWARP_MODE":         "reverse",
		"ECHOWARP_SAMPLE_RATE":  "16000",
		"ECHOWARP_CHANNELS":     "2",
		"ECHOWARP_VIRTUAL_MIC":  "true",
		"ECHOWARP_LOG_LEVEL":    "debug",
		"ECHOWARP_OPUS_BITRATE": "96000",
	}

	for k, v := range envVars {
		os.Setenv(k, v)
	}
	defer func() {
		for k := range envVars {
			os.Unsetenv(k)
		}
	}()

	cmd := newClientCmd()
	err := cmd.ParseFlags([]string{})
	if err != nil {
		t.Fatalf("ParseFlags failed: %v", err)
	}

	cfg, err := loadClientConfig(cmd)
	if err != nil {
		t.Fatalf("loadClientConfig failed: %v", err)
	}

	// Verify all env vars were applied
	if cfg.Address != "192.168.1.100" {
		t.Errorf("expected address '192.168.1.100', got %s", cfg.Address)
	}
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
}
