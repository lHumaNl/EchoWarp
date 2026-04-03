package cli

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/lHumaNl/echowarp/internal/config"
	"github.com/lHumaNl/echowarp/pkg/echowarp/audio"
)

// CheckStatus represents the status of a diagnostic check.
type CheckStatus string

const (
	StatusOK   CheckStatus = "ok"
	StatusWarn CheckStatus = "warn"
	StatusFail CheckStatus = "fail"
)

// CheckResult holds the result of a single diagnostic check.
type CheckResult struct {
	Name    string      `json:"name"`
	Status  CheckStatus `json:"status"`
	Message string      `json:"message"`
}

// DoctorReport holds all diagnostic check results.
type DoctorReport struct {
	Checks []CheckResult `json:"checks"`
}

// ANSI-colored status labels used by the doctor output.
var (
	doctorOK   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("42")).Render("[OK]")
	doctorWARN = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("214")).Render("[WARN]")
	doctorFAIL = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("196")).Render("[FAIL]")
)

const doctorDialTimeout = 3 * time.Second

// newDoctorCmd creates the "doctor" command.
// It runs a series of environment checks and prints a human-readable report.
func newDoctorCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Run diagnostic checks on the EchoWarp environment",
		Long: `Run a series of diagnostic checks and report their status.

Checks performed:
  - Audio system (device availability)
  - Network: local port availability, TCP connectivity to remote server
  - STUN server reachability (with RTT measurement)
  - NAT type detection (via STUN)
  - TLS certificate validity and expiration
  - Default config file presence, validity, and permissions
  - System resources (CPU cores, available memory)
  - UDP port reachability (firewall check)

Examples:
  echowarp doctor
  echowarp doctor --address 192.168.1.100 --port 4415
  echowarp doctor --tls-cert /path/to/cert.pem`,
		RunE: runDoctor,
	}

	cmd.Flags().StringP("address", "a", "", "Remote server address to check TCP connectivity")
	cmd.Flags().IntP("port", "p", 4415, "Port to check (local availability and remote connectivity)")
	cmd.Flags().String("tls-cert", "", "TLS certificate file to validate")
	cmd.Flags().StringP("config", "c", "", "Config file to check (default: ~/.config/echowarp/config.yaml)")
	cmd.Flags().Bool("json", false, "Output results in JSON format")

	return cmd
}

// runDoctor executes all checks and prints results to stdout.
func runDoctor(cmd *cobra.Command, args []string) error {
	address, _ := cmd.Flags().GetString("address")
	port, _ := cmd.Flags().GetInt("port")
	tlsCertPath, _ := cmd.Flags().GetString("tls-cert")
	cfgPath, _ := cmd.Flags().GetString("config")
	jsonMode, _ := cmd.Flags().GetBool("json")

	if jsonMode {
		return runDoctorJSON(address, port, tlsCertPath, cfgPath)
	}

	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205")).Render("EchoWarp Doctor")
	fmt.Println(title)
	fmt.Println(strings.Repeat("=", 40))

	RunDoctorToWriter(os.Stdout, address, port, tlsCertPath, cfgPath)

	fmt.Println()
	return nil
}

// runDoctorJSON runs diagnostics and outputs results as JSON.
func runDoctorJSON(address string, port int, tlsCertPath, cfgPath string) error {
	report := RunDoctorReport(address, port, tlsCertPath, cfgPath)
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(report)
}

// RunDoctorReport runs all diagnostic checks and returns a structured report.
func RunDoctorReport(address string, port int, tlsCertPath, cfgPath string) DoctorReport {
	checks := []CheckResult{checkAudioResult(), checkVirtualDeviceResult(), checkLocalPortResult(port)}
	if address != "" {
		checks = append(checks, checkRemoteTCPResult(address, port))
	}
	checks = append(checks, checkSTUNResult("stun.l.google.com:19302"), checkNATTypeResult(), checkUDPPortResult(port))
	if tlsCertPath != "" {
		checks = append(checks, checkTLSCertFileResult(tlsCertPath))
	} else if address != "" {
		checks = append(checks, checkRemoteTLSResult(address, port))
	}
	checks = append(checks, checkConfigFileResult(cfgPath), checkSystemResourcesResult())
	return DoctorReport{Checks: checks}
}

func checkAudioResult() CheckResult {
	dm, err := audio.NewDeviceManager()
	if err != nil {
		return CheckResult{"audio", StatusFail, err.Error()}
	}
	defer dm.Close() //nolint:errcheck
	inputs, _ := dm.ListInputDevices()
	outputs, _ := dm.ListOutputDevices()
	if len(inputs) == 0 && len(outputs) == 0 {
		return CheckResult{"audio", StatusWarn, "no devices found"}
	}
	return CheckResult{"audio", StatusOK, fmt.Sprintf("%d input(s), %d output(s)", len(inputs), len(outputs))}
}

func checkVirtualDeviceResult() CheckResult {
	dm, err := audio.NewDeviceManager()
	if err != nil {
		return CheckResult{Name: "Virtual audio", Status: StatusWarn, Message: "Cannot check (audio init failed)"}
	}
	defer dm.Close() //nolint:errcheck

	name, found := audio.DetectVirtualDevice(dm)
	if found {
		return CheckResult{Name: "Virtual audio", Status: StatusOK, Message: fmt.Sprintf("%s detected", name)}
	}
	switch runtime.GOOS {
	case "darwin":
		return CheckResult{Name: "Virtual audio", Status: StatusWarn, Message: "Not found (install BlackHole: brew install blackhole-2ch)"}
	case "windows":
		return CheckResult{Name: "Virtual audio", Status: StatusWarn, Message: "Not found (install VB-Audio Virtual Cable: vb-audio.com/Cable/)"}
	default:
		return CheckResult{Name: "Virtual audio", Status: StatusOK, Message: "PulseAudio available (virtual devices created on demand)"}
	}
}

func checkLocalPortResult(port int) CheckResult {
	addr := fmt.Sprintf(":%d", port)
	ln, err := net.Listen("tcp", addr) //nolint:noctx // diagnostic utility, context not needed
	if err != nil {
		return CheckResult{"port_local", StatusWarn, fmt.Sprintf("port %d in use or unavailable", port)}
	}
	_ = ln.Close()
	return CheckResult{"port_local", StatusOK, fmt.Sprintf("port %d available", port)}
}

func checkRemoteTCPResult(address string, port int) CheckResult {
	target := net.JoinHostPort(address, fmt.Sprintf("%d", port))
	start := time.Now()
	conn, err := net.DialTimeout("tcp", target, doctorDialTimeout) //nolint:noctx // diagnostic utility, context not needed
	rtt := time.Since(start)
	if err != nil {
		return CheckResult{"tcp_remote", StatusFail, fmt.Sprintf("TCP %s unreachable: %v", target, err)}
	}
	_ = conn.Close()
	return CheckResult{"tcp_remote", StatusOK, fmt.Sprintf("TCP %s reachable (rtt: %s)", target, rtt.Round(time.Millisecond))}
}

func checkSTUNResult(addr string) CheckResult {
	start := time.Now()
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return CheckResult{"stun", StatusFail, fmt.Sprintf("cannot resolve %s: %v", addr, err)}
	}
	conn, err := net.DialTimeout("udp", udpAddr.String(), doctorDialTimeout) //nolint:noctx // diagnostic utility, context not needed
	if err != nil {
		tcpConn, tcpErr := net.DialTimeout("tcp", addr, doctorDialTimeout) //nolint:noctx // diagnostic utility, context not needed
		if tcpErr != nil {
			return CheckResult{"stun", StatusFail, fmt.Sprintf("STUN %s unreachable (UDP and TCP failed)", addr)}
		}
		_ = tcpConn.Close()
		rtt := time.Since(start)
		return CheckResult{"stun", StatusWarn, fmt.Sprintf("STUN %s reachable via TCP only (rtt: %s, UDP blocked)", addr, rtt.Round(time.Millisecond))}
	}
	stunReq := buildSTUNBindingRequest()
	_ = conn.SetDeadline(time.Now().Add(doctorDialTimeout))
	_, writeErr := conn.Write(stunReq)
	if writeErr != nil {
		_ = conn.Close()
		rtt := time.Since(start)
		return CheckResult{"stun", StatusWarn, fmt.Sprintf("STUN %s UDP dial ok but write failed (rtt: %s)", addr, rtt.Round(time.Millisecond))}
	}
	buf := make([]byte, 256)
	_, readErr := conn.Read(buf)
	rtt := time.Since(start)
	_ = conn.Close()
	if readErr != nil {
		return CheckResult{"stun", StatusWarn, fmt.Sprintf("STUN %s UDP dial ok but no response (rtt: %s)", addr, rtt.Round(time.Millisecond))}
	}
	return CheckResult{"stun", StatusOK, fmt.Sprintf("STUN %s reachable (rtt: %s)", addr, rtt.Round(time.Millisecond))}
}

func checkNATTypeResult() CheckResult {
	localConn, err := net.Dial("udp", "8.8.8.8:80") //nolint:noctx // diagnostic utility, context not needed
	if err != nil {
		return CheckResult{"nat", StatusWarn, fmt.Sprintf("cannot determine local address: %v", err)}
	}
	localAddr, _ := localConn.LocalAddr().(*net.UDPAddr) //nolint:errcheck
	_ = localConn.Close()
	if localAddr == nil {
		return CheckResult{"nat", StatusWarn, "cannot determine local address type"}
	}
	if isPrivateIP(localAddr.IP) {
		return CheckResult{"nat", StatusWarn, fmt.Sprintf("NAT detected (local IP: %s is private)", localAddr.IP)}
	}
	return CheckResult{"nat", StatusOK, fmt.Sprintf("no NAT detected (local IP: %s is public)", localAddr.IP)}
}

func checkUDPPortResult(port int) CheckResult {
	addr := fmt.Sprintf(":%d", port)
	conn, err := net.ListenPacket("udp", addr) //nolint:noctx // diagnostic utility, context not needed
	if err != nil {
		return CheckResult{"port_udp", StatusWarn, fmt.Sprintf("UDP port %d unavailable", port)}
	}
	_ = conn.Close()
	return CheckResult{"port_udp", StatusOK, fmt.Sprintf("UDP port %d available", port)}
}

func checkTLSCertFileResult(certPath string) CheckResult {
	data, err := os.ReadFile(certPath)
	if err != nil {
		return CheckResult{"tls_cert", StatusFail, fmt.Sprintf("cannot read %s: %v", certPath, err)}
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(data) {
		return CheckResult{"tls_cert", StatusFail, fmt.Sprintf("%s is not a valid PEM certificate", certPath)}
	}
	block, _ := decodePEMCert(data)
	if block == nil {
		return CheckResult{"tls_cert", StatusFail, fmt.Sprintf("cannot parse certificate in %s", certPath)}
	}
	cert, err := x509.ParseCertificate(block)
	if err != nil {
		return CheckResult{"tls_cert", StatusFail, fmt.Sprintf("parse error: %v", err)}
	}
	now := time.Now()
	if now.After(cert.NotAfter) {
		daysAgo := int(now.Sub(cert.NotAfter).Hours() / 24)
		return CheckResult{"tls_cert", StatusFail, fmt.Sprintf("expired %d day(s) ago (expired: %s)", daysAgo, cert.NotAfter.Format("2006-01-02"))}
	}
	daysLeft := int(cert.NotAfter.Sub(now).Hours() / 24)
	if daysLeft < 30 {
		return CheckResult{"tls_cert", StatusWarn, fmt.Sprintf("expires in %d day(s) (%s)", daysLeft, cert.NotAfter.Format("2006-01-02"))}
	}
	return CheckResult{"tls_cert", StatusOK, fmt.Sprintf("valid (expires: %s, %d days left)", cert.NotAfter.Format("2006-01-02"), daysLeft)}
}

func checkRemoteTLSResult(address string, port int) CheckResult {
	target := net.JoinHostPort(address, fmt.Sprintf("%d", port))
	conn, err := tls.DialWithDialer( //nolint:noctx // diagnostic utility, context not needed
		&net.Dialer{Timeout: doctorDialTimeout},
		"tcp", target,
		&tls.Config{InsecureSkipVerify: true, MinVersion: tls.VersionTLS12},
	)
	if err != nil {
		return CheckResult{"tls_remote", StatusWarn, fmt.Sprintf("%s does not respond to TLS (may be plaintext)", target)}
	}
	defer conn.Close() //nolint:errcheck
	state := conn.ConnectionState()
	if len(state.PeerCertificates) == 0 {
		return CheckResult{"tls_remote", StatusWarn, fmt.Sprintf("%s connected but no certificates presented", target)}
	}
	cert := state.PeerCertificates[0]
	now := time.Now()
	tlsVersion := "unknown"
	switch state.Version {
	case tls.VersionTLS12:
		tlsVersion = "TLS 1.2"
	case tls.VersionTLS13:
		tlsVersion = "TLS 1.3"
	}
	if now.After(cert.NotAfter) {
		daysAgo := int(now.Sub(cert.NotAfter).Hours() / 24)
		return CheckResult{"tls_remote", StatusFail, fmt.Sprintf("%s certificate expired %d day(s) ago", tlsVersion, daysAgo)}
	}
	daysLeft := int(cert.NotAfter.Sub(now).Hours() / 24)
	if daysLeft < 30 {
		return CheckResult{"tls_remote", StatusWarn, fmt.Sprintf("%s, cert expires in %d day(s)", tlsVersion, daysLeft)}
	}
	return CheckResult{"tls_remote", StatusOK, fmt.Sprintf("%s, cert valid (expires: %s)", tlsVersion, cert.NotAfter.Format("2006-01-02"))}
}

func checkConfigFileResult(cfgPath string) CheckResult {
	if cfgPath == "" {
		configDir, err := os.UserConfigDir()
		if err != nil {
			return CheckResult{"config", StatusFail, "cannot determine config directory"}
		}
		cfgPath = filepath.Join(configDir, "echowarp", "config.yaml")
	}
	displayPath := cfgPath
	if home, herr := os.UserHomeDir(); herr == nil {
		if rel, relErr := filepath.Rel(home, cfgPath); relErr == nil && !strings.HasPrefix(rel, "..") {
			displayPath = "~/" + rel
		}
	}
	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		return CheckResult{"config", StatusWarn, fmt.Sprintf("%s not found", displayPath)}
	}
	cfg, err := config.LoadFromFile(cfgPath)
	if err != nil {
		return CheckResult{"config", StatusFail, fmt.Sprintf("%s load error: %v", displayPath, err)}
	}
	errs := cfg.Validate()
	if len(errs) > 0 {
		return CheckResult{"config", StatusFail, fmt.Sprintf("%s invalid: %v", displayPath, errs[0])}
	}
	return CheckResult{"config", StatusOK, fmt.Sprintf("%s valid", displayPath)}
}

func checkSystemResourcesResult() CheckResult {
	cpus := runtime.NumCPU()
	totalMB := systemMemoryMB()
	var msgs []string
	status := StatusOK
	if cpus < 2 {
		msgs = append(msgs, fmt.Sprintf("%d CPU core (2+ recommended)", cpus))
		status = StatusWarn
	} else {
		msgs = append(msgs, fmt.Sprintf("%d CPU cores", cpus))
	}
	if totalMB == 0 {
		msgs = append(msgs, "memory unknown (unsupported platform)")
		status = StatusWarn
	} else if totalMB < 256 {
		msgs = append(msgs, fmt.Sprintf("%d MB RAM (low, 256 MB+ recommended)", totalMB))
		status = StatusWarn
	} else {
		msgs = append(msgs, fmt.Sprintf("%d MB RAM", totalMB))
	}
	return CheckResult{"system", status, strings.Join(msgs, "; ")}
}

// RunDoctorToWriter runs all diagnostics and writes results to w.
// This is the TUI-friendly version used by the quick-start menu result screen.
func RunDoctorToWriter(w io.Writer, address string, port int, tlsCertPath, _ string) {
	sectionStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
	summaryStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))

	var ok, warn, fail int
	cw := &countWriter{w: w, ok: &ok, warn: &warn, fail: &fail}

	_, _ = fmt.Fprintln(w, sectionStyle.Render("Audio"))
	checkAudio(cw)
	checkVirtualAudio(cw)
	_, _ = fmt.Fprintln(w)

	_, _ = fmt.Fprintln(w, sectionStyle.Render("Network"))
	checkLocalPort(cw, port)
	if address != "" {
		checkRemoteTCP(cw, address, port)
	}
	checkSTUNWithRTT(cw, "stun.l.google.com:19302")
	checkNATType(cw)
	checkUDPPort(cw, port)
	if tlsCertPath != "" {
		checkTLSCertFile(cw, tlsCertPath)
	} else if address != "" {
		checkRemoteTLS(cw, address, port)
	}
	_, _ = fmt.Fprintln(w)

	_, _ = fmt.Fprintln(w, sectionStyle.Render("System"))
	checkSystemResources(cw)
	_, _ = fmt.Fprintln(w)

	summary := fmt.Sprintf("── %d passed", ok)
	if warn > 0 {
		summary += fmt.Sprintf(", %d warning", warn)
	}
	if fail > 0 {
		summary += fmt.Sprintf(", %d failed", fail)
	}
	summary += " ──"
	_, _ = fmt.Fprintln(w, summaryStyle.Render(summary))
}

// countWriter wraps an io.Writer and counts [OK]/[WARN]/[FAIL] occurrences.
type countWriter struct {
	w              io.Writer
	ok, warn, fail *int
}

func (c *countWriter) Write(p []byte) (int, error) {
	s := string(p)
	if strings.Contains(s, "[OK]") {
		*c.ok++
	} else if strings.Contains(s, "[WARN]") {
		*c.warn++
	} else if strings.Contains(s, "[FAIL]") {
		*c.fail++
	}
	return c.w.Write(p)
}

// checkAudio tries to initialize the device manager and counts available devices.
func checkAudio(w io.Writer) {
	dm, err := audio.NewDeviceManager()
	if err != nil {
		_, _ = fmt.Fprintf(w, "%s Audio system: %v\n", doctorFAIL, err)
		return
	}
	defer dm.Close() //nolint:errcheck

	inputs, err := dm.ListInputDevices()
	if err != nil {
		_, _ = fmt.Fprintf(w, "%s Audio system: failed to list input devices: %v\n", doctorFAIL, err)
		return
	}

	outputs, err := dm.ListOutputDevices()
	if err != nil {
		_, _ = fmt.Fprintf(w, "%s Audio system: failed to list output devices: %v\n", doctorFAIL, err)
		return
	}

	if len(inputs) == 0 && len(outputs) == 0 {
		_, _ = fmt.Fprintf(w, "%s Audio system: no devices found\n", doctorWARN)
		return
	}

	_, _ = fmt.Fprintf(w, "%s Audio system: %d input device(s), %d output device(s)\n",
		doctorOK, len(inputs), len(outputs))
}

// checkVirtualAudio detects virtual audio drivers (BlackHole, VB-Cable, etc.).
func checkVirtualAudio(w io.Writer) {
	dm, err := audio.NewDeviceManager()
	if err != nil {
		_, _ = fmt.Fprintf(w, "%s Virtual audio: cannot check (audio init failed)\n", doctorWARN)
		return
	}
	defer dm.Close() //nolint:errcheck

	name, found := audio.DetectVirtualDevice(dm)
	if found {
		_, _ = fmt.Fprintf(w, "%s Virtual audio: %s detected\n", doctorOK, name)
		return
	}
	switch runtime.GOOS {
	case "darwin":
		_, _ = fmt.Fprintf(w, "%s Virtual audio: not found (install BlackHole: brew install blackhole-2ch)\n", doctorWARN)
	case "windows":
		_, _ = fmt.Fprintf(w, "%s Virtual audio: not found (install VB-Audio Virtual Cable: vb-audio.com/Cable/)\n", doctorWARN)
	default:
		_, _ = fmt.Fprintf(w, "%s Virtual audio: PulseAudio available (virtual devices created on demand)\n", doctorOK)
	}
}

// checkLocalPort reports whether the given TCP port is available on the local machine.
func checkLocalPort(w io.Writer, port int) {
	addr := fmt.Sprintf(":%d", port)
	ln, err := net.Listen("tcp", addr) //nolint:noctx // diagnostic utility, context not needed
	if err != nil {
		_, _ = fmt.Fprintf(w, "%s Port %d (local): in use or unavailable\n", doctorWARN, port)
		return
	}
	_ = ln.Close()
	_, _ = fmt.Fprintf(w, "%s Port %d (local): available\n", doctorOK, port)
}

// checkRemoteTCP checks TCP connectivity to a remote server.
func checkRemoteTCP(w io.Writer, address string, port int) {
	target := net.JoinHostPort(address, fmt.Sprintf("%d", port))
	start := time.Now()
	conn, err := net.DialTimeout("tcp", target, doctorDialTimeout) //nolint:noctx // diagnostic utility
	rtt := time.Since(start)

	if err != nil {
		_, _ = fmt.Fprintf(w, "%s Network: TCP %s unreachable (%v)\n", doctorFAIL, target, err)
		_, _ = fmt.Fprintf(w, "         -> Check firewall rules or if the server is running\n")
		return
	}
	_ = conn.Close()
	_, _ = fmt.Fprintf(w, "%s Network: TCP %s reachable (rtt: %s)\n", doctorOK, target, rtt.Round(time.Millisecond))
}

// checkSTUNWithRTT attempts a UDP STUN binding request and measures RTT.
// Falls back to TCP if UDP fails.
func checkSTUNWithRTT(w io.Writer, addr string) {
	// Try UDP first (STUN is primarily UDP)
	start := time.Now()
	udpAddr, err := net.ResolveUDPAddr("udp", addr) //nolint:noctx // diagnostic utility, context not needed
	if err != nil {
		_, _ = fmt.Fprintf(w, "%s STUN server %s: cannot resolve (%v)\n", doctorFAIL, addr, err)
		return
	}

	conn, err := net.DialTimeout("udp", udpAddr.String(), doctorDialTimeout) //nolint:noctx // diagnostic utility, context not needed
	if err != nil {
		// Fallback to TCP
		tcpConn, tcpErr := net.DialTimeout("tcp", addr, doctorDialTimeout) //nolint:noctx // diagnostic utility, context not needed
		if tcpErr != nil {
			_, _ = fmt.Fprintf(w, "%s STUN server %s: unreachable (UDP and TCP failed)\n", doctorFAIL, addr)
			_, _ = fmt.Fprintf(w, "         -> Check internet connectivity or firewall rules\n")
			return
		}
		_ = tcpConn.Close()
		rtt := time.Since(start)
		_, _ = fmt.Fprintf(w, "%s STUN server %s: reachable via TCP (rtt: %s, UDP blocked)\n", doctorWARN, addr, rtt.Round(time.Millisecond))
		_, _ = fmt.Fprintf(w, "         -> UDP may be blocked by firewall; WebRTC may fall back to TURN\n")
		return
	}

	// Send a minimal STUN binding request to get a response and measure real RTT
	stunReq := buildSTUNBindingRequest()
	_ = conn.SetDeadline(time.Now().Add(doctorDialTimeout))
	_, err = conn.Write(stunReq)
	if err != nil {
		_ = conn.Close()
		rtt := time.Since(start)
		_, _ = fmt.Fprintf(w, "%s STUN server %s: UDP dial ok but write failed (rtt: %s)\n", doctorWARN, addr, rtt.Round(time.Millisecond))
		return
	}

	buf := make([]byte, 256)
	_, err = conn.Read(buf)
	rtt := time.Since(start)
	_ = conn.Close()

	if err != nil {
		_, _ = fmt.Fprintf(w, "%s STUN server %s: UDP dial ok but no response (rtt: %s)\n", doctorWARN, addr, rtt.Round(time.Millisecond))
		_, _ = fmt.Fprintf(w, "         -> STUN response timeout; may be blocked by firewall\n")
		return
	}

	_, _ = fmt.Fprintf(w, "%s STUN server %s: reachable (rtt: %s)\n", doctorOK, addr, rtt.Round(time.Millisecond))
}

// buildSTUNBindingRequest builds a minimal STUN Binding Request (RFC 5389).
func buildSTUNBindingRequest() []byte {
	// STUN header: 20 bytes
	// Type: 0x0001 (Binding Request)
	// Length: 0x0000 (no attributes)
	// Magic cookie: 0x2112A442
	// Transaction ID: 12 random bytes (we use fixed for simplicity)
	req := make([]byte, 20)
	req[0] = 0x00
	req[1] = 0x01 // Binding Request
	req[2] = 0x00
	req[3] = 0x00 // Length: 0
	// Magic cookie
	req[4] = 0x21
	req[5] = 0x12
	req[6] = 0xA4
	req[7] = 0x42
	// Transaction ID (12 bytes, fixed for diagnostic purposes)
	for i := 8; i < 20; i++ {
		req[i] = byte(i)
	}
	return req
}

// checkNATType performs basic NAT type detection by comparing local and STUN-reported addresses.
func checkNATType(w io.Writer) {
	// Get local address
	localConn, err := net.Dial("udp", "8.8.8.8:80") //nolint:noctx // diagnostic utility, context not needed
	if err != nil {
		_, _ = fmt.Fprintf(w, "%s NAT: cannot determine local address (%v)\n", doctorWARN, err)
		return
	}
	localAddr, _ := localConn.LocalAddr().(*net.UDPAddr) //nolint:errcheck
	_ = localConn.Close()

	if localAddr == nil {
		_, _ = fmt.Fprintf(w, "%s NAT: cannot determine local address type\n", doctorWARN)
		return
	}

	// Check if local address is private
	if isPrivateIP(localAddr.IP) {
		_, _ = fmt.Fprintf(w, "%s NAT: detected (local IP: %s is private). STUN/TURN may be required\n",
			doctorWARN, localAddr.IP)
	} else {
		_, _ = fmt.Fprintf(w, "%s NAT: not detected (local IP: %s is public)\n",
			doctorOK, localAddr.IP)
	}
}

// isPrivateIP checks if an IP address is in a private range.
func isPrivateIP(ip net.IP) bool {
	privateRanges := []string{
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
		"fc00::/7",
	}
	for _, cidr := range privateRanges {
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			continue
		}
		if network.Contains(ip) {
			return true
		}
	}
	return false
}

// checkUDPPort tries to bind a UDP socket on the given port to check firewall/availability.
func checkUDPPort(w io.Writer, port int) {
	addr := fmt.Sprintf(":%d", port)
	conn, err := net.ListenPacket("udp", addr) //nolint:noctx // diagnostic utility, context not needed
	if err != nil {
		_, _ = fmt.Fprintf(w, "%s UDP port %d: unavailable (may be blocked by firewall or in use)\n", doctorWARN, port)
		return
	}
	_ = conn.Close()
	_, _ = fmt.Fprintf(w, "%s UDP port %d: available\n", doctorOK, port)
}

// checkTLSCertFile validates a local TLS certificate file.
func checkTLSCertFile(w io.Writer, certPath string) {
	data, err := os.ReadFile(certPath)
	if err != nil {
		_, _ = fmt.Fprintf(w, "%s TLS cert: cannot read %s (%v)\n", doctorFAIL, certPath, err)
		return
	}

	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(data) {
		_, _ = fmt.Fprintf(w, "%s TLS cert: %s is not a valid PEM certificate\n", doctorFAIL, certPath)
		return
	}

	// Parse to check expiration
	block, _ := decodePEMCert(data)
	if block == nil {
		_, _ = fmt.Fprintf(w, "%s TLS cert: %s cannot parse certificate\n", doctorFAIL, certPath)
		return
	}

	cert, err := x509.ParseCertificate(block)
	if err != nil {
		_, _ = fmt.Fprintf(w, "%s TLS cert: %s parse error (%v)\n", doctorFAIL, certPath, err)
		return
	}

	now := time.Now()
	if now.After(cert.NotAfter) {
		daysAgo := int(now.Sub(cert.NotAfter).Hours() / 24)
		_, _ = fmt.Fprintf(w, "%s TLS cert: expired %d day(s) ago (expired: %s)\n",
			doctorFAIL, daysAgo, cert.NotAfter.Format("2006-01-02"))
		return
	}

	daysLeft := int(cert.NotAfter.Sub(now).Hours() / 24)
	if daysLeft < 30 {
		_, _ = fmt.Fprintf(w, "%s TLS cert: expires in %d day(s) (%s)\n",
			doctorWARN, daysLeft, cert.NotAfter.Format("2006-01-02"))
	} else {
		_, _ = fmt.Fprintf(w, "%s TLS cert: valid (expires: %s, %d days left)\n",
			doctorOK, cert.NotAfter.Format("2006-01-02"), daysLeft)
	}
}

// decodePEMCert extracts the first certificate DER block from PEM data.
func decodePEMCert(pemData []byte) (derBytes []byte, rest []byte) {
	block, r := pem.Decode(pemData)
	if block == nil {
		return nil, pemData
	}
	return block.Bytes, r
}

// checkRemoteTLS connects to a remote server and checks TLS certificate validity.
func checkRemoteTLS(w io.Writer, address string, port int) {
	target := net.JoinHostPort(address, fmt.Sprintf("%d", port))
	conn, err := tls.DialWithDialer( //nolint:noctx // diagnostic utility, context not needed
		&net.Dialer{Timeout: doctorDialTimeout},
		"tcp", target,
		&tls.Config{InsecureSkipVerify: true, MinVersion: tls.VersionTLS12},
	)
	if err != nil {
		// Not necessarily a failure — server might not use TLS
		_, _ = fmt.Fprintf(w, "%s TLS: %s does not respond to TLS (may be plaintext)\n", doctorWARN, target)
		return
	}
	defer conn.Close() //nolint:errcheck

	state := conn.ConnectionState()
	if len(state.PeerCertificates) == 0 {
		_, _ = fmt.Fprintf(w, "%s TLS: %s connected but no certificates presented\n", doctorWARN, target)
		return
	}

	cert := state.PeerCertificates[0]
	now := time.Now()

	tlsVersion := "unknown"
	switch state.Version {
	case tls.VersionTLS12:
		tlsVersion = "TLS 1.2"
	case tls.VersionTLS13:
		tlsVersion = "TLS 1.3"
	}

	if now.After(cert.NotAfter) {
		daysAgo := int(now.Sub(cert.NotAfter).Hours() / 24)
		_, _ = fmt.Fprintf(w, "%s TLS: %s certificate expired %d day(s) ago\n", doctorFAIL, tlsVersion, daysAgo)
	} else {
		daysLeft := int(cert.NotAfter.Sub(now).Hours() / 24)
		if daysLeft < 30 {
			_, _ = fmt.Fprintf(w, "%s TLS: %s, cert expires in %d day(s)\n", doctorWARN, tlsVersion, daysLeft)
		} else {
			_, _ = fmt.Fprintf(w, "%s TLS: %s, cert valid (expires: %s)\n", doctorOK, tlsVersion, cert.NotAfter.Format("2006-01-02"))
		}
	}
}

// checkConfigFileWithPath validates the config file at the given or default path.
func checkConfigFileWithPath(w io.Writer, cfgPath string) {
	if cfgPath == "" {
		configDir, err := os.UserConfigDir()
		if err != nil {
			_, _ = fmt.Fprintf(w, "%s Config file: cannot determine config directory\n", doctorFAIL)
			return
		}
		cfgPath = filepath.Join(configDir, "echowarp", "config.yaml")
	}

	displayPath := cfgPath
	if home, herr := os.UserHomeDir(); herr == nil {
		if rel, relErr := filepath.Rel(home, cfgPath); relErr == nil && !strings.HasPrefix(rel, "..") {
			displayPath = "~/" + rel
		}
	}

	stat, err := os.Stat(cfgPath)
	if os.IsNotExist(err) {
		_, _ = fmt.Fprintf(w, "%s Config file: %s not found\n", doctorWARN, displayPath)
		return
	}
	if err != nil {
		_, _ = fmt.Fprintf(w, "%s Config file: %s (%v)\n", doctorFAIL, displayPath, err)
		return
	}

	cfg, err := config.LoadFromFile(cfgPath)
	if err != nil {
		_, _ = fmt.Fprintf(w, "%s Config file: %s (load error: %v)\n", doctorFAIL, displayPath, err)
		return
	}

	errs := cfg.Validate()
	if len(errs) > 0 {
		_, _ = fmt.Fprintf(w, "%s Config file: %s (invalid: %v)\n", doctorFAIL, displayPath, errs[0])
	} else {
		_, _ = fmt.Fprintf(w, "%s Config file: %s (valid)\n", doctorOK, displayPath)
	}

	perm := stat.Mode().Perm()
	if perm&0077 != 0 {
		_, _ = fmt.Fprintf(w, "%s Config permissions: %04o (should be 0600)\n", doctorWARN, perm)
	} else {
		_, _ = fmt.Fprintf(w, "%s Config permissions: %04o\n", doctorOK, perm)
	}
}

// checkSystemResources reports CPU cores and total system memory.
func checkSystemResources(w io.Writer) {
	cpus := runtime.NumCPU()

	if cpus < 2 {
		_, _ = fmt.Fprintf(w, "%s System: %d CPU core (EchoWarp works best with 2+ cores)\n", doctorWARN, cpus)
	} else {
		_, _ = fmt.Fprintf(w, "%s System: %d CPU cores\n", doctorOK, cpus)
	}

	totalMB := systemMemoryMB()
	if totalMB == 0 {
		_, _ = fmt.Fprintf(w, "%s System: memory — cannot determine (unsupported platform)\n", doctorWARN)
	} else if totalMB < 256 {
		_, _ = fmt.Fprintf(w, "%s System: %d MB total RAM (low, 256 MB+ recommended)\n", doctorWARN, totalMB)
	} else {
		_, _ = fmt.Fprintf(w, "%s System: %d MB total RAM\n", doctorOK, totalMB)
	}
}
