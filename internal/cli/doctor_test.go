package cli

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stdoutMu serializes all tests that capture stdout to avoid interleaving.
var stdoutMu sync.Mutex

// captureOutput redirects stdout during fn execution and returns captured text.
// Must be called under stdoutMu.
func captureOutput(t *testing.T, fn func()) string {
	t.Helper()

	old := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w

	fn()

	_ = w.Close()
	os.Stdout = old

	buf := make([]byte, 64*1024)
	n, _ := r.Read(buf)
	_ = r.Close()
	return string(buf[:n])
}

// --- Pure function tests (safe to run in parallel) ---

func TestIsPrivateIP(t *testing.T) {
	t.Parallel()

	tests := []struct {
		ip      string
		private bool
	}{
		{"10.0.0.1", true},
		{"10.255.255.255", true},
		{"172.16.0.1", true},
		{"172.31.255.255", true},
		{"172.15.0.1", false},
		{"172.32.0.1", false},
		{"192.168.0.1", true},
		{"192.168.255.255", true},
		{"192.169.0.1", false},
		{"8.8.8.8", false},
		{"1.1.1.1", false},
		{"127.0.0.1", false},
		{"fc00::1", true},
		{"fd12::1", true},
		{"2001:db8::1", false},
	}

	for _, tt := range tests {
		t.Run(tt.ip, func(t *testing.T) {
			t.Parallel()
			ip := net.ParseIP(tt.ip)
			require.NotNil(t, ip, "failed to parse IP: %s", tt.ip)
			assert.Equal(t, tt.private, isPrivateIP(ip))
		})
	}
}

func TestBuildSTUNBindingRequest(t *testing.T) {
	t.Parallel()

	req := buildSTUNBindingRequest()

	assert.Equal(t, 20, len(req), "STUN request must be 20 bytes")
	// Type: 0x0001 (Binding Request)
	assert.Equal(t, byte(0x00), req[0])
	assert.Equal(t, byte(0x01), req[1])
	// Length: 0
	assert.Equal(t, byte(0x00), req[2])
	assert.Equal(t, byte(0x00), req[3])
	// Magic cookie: 0x2112A442
	assert.Equal(t, byte(0x21), req[4])
	assert.Equal(t, byte(0x12), req[5])
	assert.Equal(t, byte(0xA4), req[6])
	assert.Equal(t, byte(0x42), req[7])
	// Transaction ID
	for i := 8; i < 20; i++ {
		assert.Equal(t, byte(i), req[i])
	}
}

func TestDecodePEMCert_Valid(t *testing.T) {
	t.Parallel()

	certPEM, _ := generateSelfSignedCert(t, time.Now(), time.Now().Add(365*24*time.Hour))
	der, rest := decodePEMCert(certPEM)
	assert.NotNil(t, der)
	assert.NotNil(t, rest)
	_, err := x509.ParseCertificate(der)
	assert.NoError(t, err)
}

func TestDecodePEMCert_Invalid(t *testing.T) {
	t.Parallel()

	der, rest := decodePEMCert([]byte("not a PEM"))
	assert.Nil(t, der)
	assert.Equal(t, []byte("not a PEM"), rest)
}

func TestDecodePEMCert_Empty(t *testing.T) {
	t.Parallel()

	der, _ := decodePEMCert([]byte{})
	assert.Nil(t, der)
}

// --- Command structure tests (safe to run in parallel) ---

func TestNewDoctorCmd_Flags(t *testing.T) {
	t.Parallel()

	cmd := newDoctorCmd()
	assert.Equal(t, "doctor", cmd.Use)
	assert.NotNil(t, cmd.RunE)

	flags := cmd.Flags()
	assert.NotNil(t, flags.Lookup("address"))
	assert.NotNil(t, flags.Lookup("port"))
	assert.NotNil(t, flags.Lookup("tls-cert"))
	assert.NotNil(t, flags.Lookup("config"))
}

func TestNewDoctorCmd_DefaultPort(t *testing.T) {
	t.Parallel()

	cmd := newDoctorCmd()
	port, err := cmd.Flags().GetInt("port")
	assert.NoError(t, err)
	assert.Equal(t, 4415, port)
}

func TestNewDoctorCmd_ShortFlags(t *testing.T) {
	t.Parallel()

	cmd := newDoctorCmd()

	flag := cmd.Flags().ShorthandLookup("a")
	assert.NotNil(t, flag)
	assert.Equal(t, "address", flag.Name)

	flag = cmd.Flags().ShorthandLookup("p")
	assert.NotNil(t, flag)
	assert.Equal(t, "port", flag.Name)

	flag = cmd.Flags().ShorthandLookup("c")
	assert.NotNil(t, flag)
	assert.Equal(t, "config", flag.Name)
}

// --- Stdout-capturing tests (serialized via stdoutMu) ---

func TestCheckLocalPort_Available(t *testing.T) {
	stdoutMu.Lock()
	defer stdoutMu.Unlock()

	port := getFreePort(t)
	out := captureOutput(t, func() {
		checkLocalPort(os.Stdout, port)
	})

	assert.Contains(t, out, "available")
	assert.Contains(t, out, fmt.Sprintf("Port %d", port))
}

func TestCheckLocalPort_InUse(t *testing.T) {
	stdoutMu.Lock()
	defer stdoutMu.Unlock()

	ln, err := net.Listen("tcp", ":0")
	require.NoError(t, err)
	defer ln.Close()

	port := ln.Addr().(*net.TCPAddr).Port

	out := captureOutput(t, func() {
		checkLocalPort(os.Stdout, port)
	})

	assert.Contains(t, out, "in use")
}

func TestCheckUDPPort_Available(t *testing.T) {
	stdoutMu.Lock()
	defer stdoutMu.Unlock()

	port := getFreeUDPPort(t)
	out := captureOutput(t, func() {
		checkUDPPort(os.Stdout, port)
	})

	assert.Contains(t, out, "available")
	assert.Contains(t, out, fmt.Sprintf("UDP port %d", port))
}

func TestCheckUDPPort_InUse(t *testing.T) {
	stdoutMu.Lock()
	defer stdoutMu.Unlock()

	conn, err := net.ListenPacket("udp", ":0")
	require.NoError(t, err)
	defer conn.Close()

	port := conn.LocalAddr().(*net.UDPAddr).Port

	out := captureOutput(t, func() {
		checkUDPPort(os.Stdout, port)
	})

	assert.Contains(t, out, "unavailable")
}

func TestCheckRemoteTCP_Reachable(t *testing.T) {
	stdoutMu.Lock()
	defer stdoutMu.Unlock()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer ln.Close()

	port := ln.Addr().(*net.TCPAddr).Port

	out := captureOutput(t, func() {
		checkRemoteTCP(os.Stdout, "127.0.0.1", port)
	})

	assert.Contains(t, out, "reachable")
	assert.Contains(t, out, "rtt:")
}

func TestCheckRemoteTCP_Unreachable(t *testing.T) {
	stdoutMu.Lock()
	defer stdoutMu.Unlock()

	out := captureOutput(t, func() {
		checkRemoteTCP(os.Stdout, "127.0.0.1", 1)
	})

	assert.Contains(t, out, "unreachable")
}

func TestCheckTLSCertFile_ValidCert(t *testing.T) {
	stdoutMu.Lock()
	defer stdoutMu.Unlock()

	certPEM, _ := generateSelfSignedCert(t, time.Now(), time.Now().Add(365*24*time.Hour))
	tmpFile := filepath.Join(t.TempDir(), "valid.pem")
	require.NoError(t, os.WriteFile(tmpFile, certPEM, 0600))

	out := captureOutput(t, func() {
		checkTLSCertFile(os.Stdout, tmpFile)
	})

	assert.Contains(t, out, "valid")
	assert.Contains(t, out, "days left")
}

func TestCheckTLSCertFile_ExpiredCert(t *testing.T) {
	stdoutMu.Lock()
	defer stdoutMu.Unlock()

	certPEM, _ := generateSelfSignedCert(t,
		time.Now().Add(-365*24*time.Hour),
		time.Now().Add(-24*time.Hour),
	)
	tmpFile := filepath.Join(t.TempDir(), "expired.pem")
	require.NoError(t, os.WriteFile(tmpFile, certPEM, 0600))

	out := captureOutput(t, func() {
		checkTLSCertFile(os.Stdout, tmpFile)
	})

	assert.Contains(t, out, "expired")
}

func TestCheckTLSCertFile_ExpiringSoon(t *testing.T) {
	stdoutMu.Lock()
	defer stdoutMu.Unlock()

	certPEM, _ := generateSelfSignedCert(t,
		time.Now().Add(-24*time.Hour),
		time.Now().Add(15*24*time.Hour),
	)
	tmpFile := filepath.Join(t.TempDir(), "expiring.pem")
	require.NoError(t, os.WriteFile(tmpFile, certPEM, 0600))

	out := captureOutput(t, func() {
		checkTLSCertFile(os.Stdout, tmpFile)
	})

	assert.Contains(t, out, "expires in")
}

func TestCheckTLSCertFile_NotFound(t *testing.T) {
	stdoutMu.Lock()
	defer stdoutMu.Unlock()

	out := captureOutput(t, func() {
		checkTLSCertFile(os.Stdout, "/nonexistent/cert.pem")
	})

	assert.Contains(t, out, "cannot read")
}

func TestCheckTLSCertFile_InvalidPEM(t *testing.T) {
	stdoutMu.Lock()
	defer stdoutMu.Unlock()

	tmpFile := filepath.Join(t.TempDir(), "invalid.pem")
	require.NoError(t, os.WriteFile(tmpFile, []byte("not a cert"), 0600))

	out := captureOutput(t, func() {
		checkTLSCertFile(os.Stdout, tmpFile)
	})

	assert.Contains(t, out, "not a valid PEM")
}

func TestCheckRemoteTLS_WithTLSServer(t *testing.T) {
	stdoutMu.Lock()
	defer stdoutMu.Unlock()

	certPEM, keyPEM := generateSelfSignedCert(t, time.Now(), time.Now().Add(365*24*time.Hour))
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	require.NoError(t, err)

	// Use a raw TCP listener with manual TLS wrapping so we control the handshake
	tcpLn, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer tcpLn.Close()

	tlsCfg := &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	}

	go func() {
		for {
			tcpConn, err := tcpLn.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				tlsConn := tls.Server(c, tlsCfg)
				// Complete the handshake
				_ = tlsConn.Handshake()
				// Hold the connection open
				buf := make([]byte, 1)
				_, _ = tlsConn.Read(buf)
				_ = tlsConn.Close()
			}(tcpConn)
		}
	}()

	port := tcpLn.Addr().(*net.TCPAddr).Port

	out := captureOutput(t, func() {
		checkRemoteTLS(os.Stdout, "127.0.0.1", port)
	})

	assert.Contains(t, out, "TLS")
	assert.True(t,
		strings.Contains(out, "cert valid") || strings.Contains(out, "TLS 1."),
		"expected TLS info in output: %s", out)
}

func TestCheckRemoteTLS_PlaintextServer(t *testing.T) {
	stdoutMu.Lock()
	defer stdoutMu.Unlock()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer ln.Close()

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			// Write something non-TLS and close
			_, _ = conn.Write([]byte("hello"))
			_ = conn.Close()
		}
	}()

	port := ln.Addr().(*net.TCPAddr).Port

	out := captureOutput(t, func() {
		checkRemoteTLS(os.Stdout, "127.0.0.1", port)
	})

	assert.Contains(t, out, "does not respond to TLS")
}

func TestCheckConfigFileWithPath_ValidConfig(t *testing.T) {
	stdoutMu.Lock()
	defer stdoutMu.Unlock()

	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.yaml")

	yamlContent := `mode: server
port: 4415
sample_rate: 48000
channels: 1
log_level: info
audio_buffer_frames: 5
`
	require.NoError(t, os.WriteFile(cfgPath, []byte(yamlContent), 0600))

	out := captureOutput(t, func() {
		checkConfigFileWithPath(os.Stdout, cfgPath)
	})

	assert.Contains(t, out, "valid")
	assert.Contains(t, out, "0600")
}

func TestCheckConfigFileWithPath_InsecurePermissions(t *testing.T) {
	stdoutMu.Lock()
	defer stdoutMu.Unlock()

	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.yaml")

	yamlContent := `mode: server
port: 4415
sample_rate: 48000
channels: 1
log_level: info
audio_buffer_frames: 5
`
	require.NoError(t, os.WriteFile(cfgPath, []byte(yamlContent), 0644))

	out := captureOutput(t, func() {
		checkConfigFileWithPath(os.Stdout, cfgPath)
	})

	assert.Contains(t, out, "should be 0600")
}

func TestCheckConfigFileWithPath_NotFound(t *testing.T) {
	stdoutMu.Lock()
	defer stdoutMu.Unlock()

	out := captureOutput(t, func() {
		checkConfigFileWithPath(os.Stdout, filepath.Join(t.TempDir(), "nonexistent.yaml"))
	})

	assert.Contains(t, out, "not found")
}

func TestCheckConfigFileWithPath_InvalidYAML(t *testing.T) {
	stdoutMu.Lock()
	defer stdoutMu.Unlock()

	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "bad.yaml")
	// Use YAML that produces invalid config values (port out of range)
	require.NoError(t, os.WriteFile(cfgPath, []byte("port: -999\nchannels: 99\nlog_level: nope\naudio_buffer_frames: 0\n"), 0600))

	out := captureOutput(t, func() {
		checkConfigFileWithPath(os.Stdout, cfgPath)
	})

	assert.Contains(t, out, "invalid")
}

func TestCheckConfigFileWithPath_InvalidConfig(t *testing.T) {
	stdoutMu.Lock()
	defer stdoutMu.Unlock()

	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.yaml")

	yamlContent := `mode: server
port: -1
sample_rate: 48000
channels: 1
log_level: info
audio_buffer_frames: 5
`
	require.NoError(t, os.WriteFile(cfgPath, []byte(yamlContent), 0600))

	out := captureOutput(t, func() {
		checkConfigFileWithPath(os.Stdout, cfgPath)
	})

	assert.Contains(t, out, "invalid")
}

func TestCheckSystemResources(t *testing.T) {
	stdoutMu.Lock()
	defer stdoutMu.Unlock()

	out := captureOutput(t, func() {
		checkSystemResources(os.Stdout)
	})

	assert.Contains(t, out, "CPU")
	assert.Contains(t, out, "MB")
}

func TestCheckAudio_DoesNotPanic(t *testing.T) {
	stdoutMu.Lock()
	defer stdoutMu.Unlock()

	out := captureOutput(t, func() {
		checkAudio(os.Stdout)
	})

	assert.Contains(t, out, "Audio system")
}

func TestCheckNATType_DoesNotPanic(t *testing.T) {
	stdoutMu.Lock()
	defer stdoutMu.Unlock()

	out := captureOutput(t, func() {
		checkNATType(os.Stdout)
	})

	assert.Contains(t, out, "NAT")
}

func TestCheckSTUNWithRTT_UnresolvableAddress(t *testing.T) {
	stdoutMu.Lock()
	defer stdoutMu.Unlock()

	out := captureOutput(t, func() {
		checkSTUNWithRTT(os.Stdout, "this.host.does.not.exist.invalid:3478")
	})

	assert.True(t,
		strings.Contains(out, "unreachable") ||
			strings.Contains(out, "cannot resolve") ||
			strings.Contains(out, "no response"),
		"expected failure message, got: %s", out)
}

// --- Helpers ---

func getFreePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", ":0")
	require.NoError(t, err)
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()
	return port
}

func getFreeUDPPort(t *testing.T) int {
	t.Helper()
	conn, err := net.ListenPacket("udp", ":0")
	require.NoError(t, err)
	port := conn.LocalAddr().(*net.UDPAddr).Port
	_ = conn.Close()
	return port
}

func generateSelfSignedCert(t *testing.T, notBefore, notAfter time.Time) (certPEM, keyPEM []byte) {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"EchoWarp Test"},
		},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	require.NoError(t, err)

	certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})

	keyBytes, err := x509.MarshalECPrivateKey(key)
	require.NoError(t, err)
	keyPEM = pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyBytes})

	return certPEM, keyPEM
}
