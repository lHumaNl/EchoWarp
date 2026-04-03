package transport

import (
	"bufio"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"sync"
	"time"

	ewerrors "github.com/lHumaNl/echowarp/pkg/echowarp/errors"
)

const (
	defaultMaxMsgSize  = 64 * 1024
	defaultIdleTimeout = 30 * time.Second
)

// TCPSignaler implements the Signaler interface using TCP with optional TLS.
// It uses a newline-delimited JSON protocol for message framing.
//
// Security features:
//   - TLS 1.2+ support with certificate pinning
//   - Idle timeout to prevent resource exhaustion
//   - Maximum message size limits
//
// Thread-safe: Internal state protected by sync.Mutex and sync.RWMutex.
type TCPSignaler struct {
	role       SignalingRole
	address    string
	tlsConfig  *tls.Config
	maxMsgSize int

	idleTimeout        time.Duration
	pinnedFingerprints []string

	mu            sync.Mutex
	listener      net.Listener
	conn          net.Conn
	remoteAddr    string
	listenAddr    string
	recvCh        chan SignalingMessage
	writeMu       sync.Mutex
	readCtx       context.Context
	readCancel    context.CancelFunc
	ready         chan struct{}
	readyOnce     sync.Once
	listenerReady chan struct{}
}

// SignalerOption configures a TCPSignaler during construction.
type SignalerOption func(*TCPSignaler)

// WithTLS enables TLS encryption for the signaling connection.
// If config.MinVersion is not set, it defaults to TLS 1.2.
func WithTLS(config *tls.Config) SignalerOption {
	return func(s *TCPSignaler) {
		if config != nil && config.MinVersion == 0 {
			config.MinVersion = tls.VersionTLS12
		}
		s.tlsConfig = config
	}
}

// WithMaxMessageSize sets the maximum message size in bytes.
// Messages larger than this will be rejected. Default is 64KB.
func WithMaxMessageSize(bytes int) SignalerOption {
	return func(s *TCPSignaler) {
		s.maxMsgSize = bytes
	}
}

// WithIdleTimeout sets the idle connection timeout.
// If no message is received within this duration, the connection is closed.
// A value of 0 disables idle timeout. Default is 30 seconds.
func WithIdleTimeout(d time.Duration) SignalerOption {
	return func(s *TCPSignaler) {
		s.idleTimeout = d
	}
}

// WithCertificatePinning enables TLS certificate pinning.
// Only connections with certificates matching one of the provided SHA-256 fingerprints are accepted.
// Fingerprints should be lowercase hex strings.
func WithCertificatePinning(fingerprints []string) SignalerOption {
	return func(s *TCPSignaler) {
		s.pinnedFingerprints = fingerprints
	}
}

// NewTCPSignaler creates a new TCPSignaler for the given role and address.
// For RoleServer, address is the listen address (e.g., ":8080").
// For RoleClient, address is the server address (e.g., "192.168.1.1:8080").
func NewTCPSignaler(role SignalingRole, address string, opts ...SignalerOption) *TCPSignaler {
	s := &TCPSignaler{
		role:          role,
		address:       address,
		maxMsgSize:    defaultMaxMsgSize,
		recvCh:        make(chan SignalingMessage, 64),
		ready:         make(chan struct{}),
		listenerReady: make(chan struct{}),
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// NewTCPSignalerFromConn creates a TCPSignaler from an existing connection.
// This is used by the server to handle accepted connections.
func NewTCPSignalerFromConn(conn net.Conn, opts ...SignalerOption) *TCPSignaler {
	s := &TCPSignaler{
		role:          RoleServer,
		maxMsgSize:    defaultMaxMsgSize,
		recvCh:        make(chan SignalingMessage, 64),
		ready:         make(chan struct{}),
		listenerReady: make(chan struct{}),
		conn:          conn,
		remoteAddr:    conn.RemoteAddr().String(),
		listenAddr:    conn.LocalAddr().String(),
	}
	for _, opt := range opts {
		opt(s)
	}
	s.markReady()
	close(s.listenerReady)
	return s
}

// StartWithConn starts the read loop for an existing connection.
// Used when the connection was already established (e.g., accepted by server).
func (s *TCPSignaler) StartWithConn(ctx context.Context) {
	s.mu.Lock()
	s.readCtx, s.readCancel = context.WithCancel(ctx)
	s.mu.Unlock()
	go s.readLoop()
}

// Start establishes the signaling connection.
// For servers, this starts listening and waits for a client connection.
// For clients, this connects to the server.
// Blocks until the connection is established or context is canceled.
func (s *TCPSignaler) Start(ctx context.Context) error {
	s.mu.Lock()
	s.readCtx, s.readCancel = context.WithCancel(ctx)
	s.mu.Unlock()

	if s.role == RoleServer {
		return s.startServer(ctx)
	}
	return s.startClient(ctx)
}

func (s *TCPSignaler) startServer(ctx context.Context) error {
	var listener net.Listener
	var err error
	if s.tlsConfig != nil {
		listener, err = tls.Listen("tcp", s.address, s.tlsConfig)
	} else {
		slog.Debug("Signaling over TCP with ECDH+AES-256-GCM encryption (no TLS)")
		listener, err = net.Listen("tcp", s.address) //nolint:noctx
	}
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", s.address, err)
	}

	s.mu.Lock()
	s.listener = listener
	s.listenAddr = listener.Addr().String()
	s.mu.Unlock()
	close(s.listenerReady)

	acceptCh := make(chan net.Conn, 1)
	errCh := make(chan error, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			errCh <- err
			return
		}
		acceptCh <- conn
	}()

	select {
	case conn := <-acceptCh:
		if err := s.verifyCertificate(conn); err != nil {
			_ = conn.Close() //nolint:errcheck
			return fmt.Errorf("certificate verification failed: %w", err)
		}
		s.mu.Lock()
		s.conn = conn
		s.remoteAddr = conn.RemoteAddr().String()
		s.mu.Unlock()
		s.markReady()
		go s.readLoop()
		return nil
	case err := <-errCh:
		return fmt.Errorf("failed to accept connection: %w", err)
	case <-ctx.Done():
		_ = listener.Close() //nolint:errcheck
		return ctx.Err()
	}
}

func (s *TCPSignaler) startClient(_ context.Context) error {
	var conn net.Conn
	var err error
	if s.tlsConfig != nil {
		conn, err = tls.Dial("tcp", s.address, s.tlsConfig) //nolint:noctx
	} else {
		slog.Debug("Signaling over TCP with ECDH+AES-256-GCM encryption (no TLS)")
		conn, err = net.Dial("tcp", s.address) //nolint:noctx
	}
	if err != nil {
		return ewerrors.Wrap(err, ewerrors.ErrConnectionFailed, fmt.Sprintf("failed to connect to %s", s.address)).
			WithSuggestion("Check that the server is running, the address/port are correct, and no firewall is blocking the connection")
	}

	if err := s.verifyCertificate(conn); err != nil {
		_ = conn.Close() //nolint:errcheck
		return fmt.Errorf("certificate verification failed: %w", err)
	}

	s.mu.Lock()
	s.conn = conn
	s.remoteAddr = conn.RemoteAddr().String()
	s.listenAddr = conn.LocalAddr().String()
	s.mu.Unlock()
	s.markReady()
	go s.readLoop()
	return nil
}

func (s *TCPSignaler) markReady() {
	s.readyOnce.Do(func() {
		close(s.ready)
	})
}

func (s *TCPSignaler) verifyCertificate(conn net.Conn) error {
	if len(s.pinnedFingerprints) == 0 {
		return nil
	}

	tlsConn, ok := conn.(*tls.Conn)
	if !ok {
		return errors.New("certificate pinning requires TLS connection")
	}

	state := tlsConn.ConnectionState()
	if len(state.PeerCertificates) == 0 {
		return errors.New("no peer certificate")
	}

	cert := state.PeerCertificates[0]
	fingerprint := sha256Fingerprint(cert.Raw)

	for _, pinned := range s.pinnedFingerprints {
		if fingerprint == pinned {
			return nil
		}
	}

	return fmt.Errorf("certificate fingerprint %s not in pinned list", fingerprint)
}

func sha256Fingerprint(cert []byte) string {
	h := sha256.Sum256(cert)
	return hex.EncodeToString(h[:])
}

func (s *TCPSignaler) readLoop() {
	s.mu.Lock()
	conn := s.conn
	readCtx := s.readCtx
	idleTimeout := s.idleTimeout
	s.mu.Unlock()
	if conn == nil || readCtx == nil {
		return
	}

	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, s.maxMsgSize), s.maxMsgSize)

	for {
		if idleTimeout > 0 {
			if err := conn.SetReadDeadline(time.Now().Add(idleTimeout)); err != nil {
				slog.Warn("Failed to set read deadline", "error", err)
			}
		}

		if !scanner.Scan() {
			if err := scanner.Err(); err != nil {
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					slog.Warn("Connection idle timeout", "remote", conn.RemoteAddr())
				}
			}
			return
		}

		if readCtx.Err() != nil {
			return
		}

		var msg SignalingMessage
		if err := json.Unmarshal(scanner.Bytes(), &msg); err != nil {
			continue
		}
		select {
		case s.recvCh <- msg:
		case <-readCtx.Done():
			return
		}
	}
}

// Send transmits a signaling message over the connection.
// Blocks until the connection is ready. Thread-safe for concurrent sends.
func (s *TCPSignaler) Send(msg SignalingMessage) error {
	<-s.ready

	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	if s.conn == nil {
		return fmt.Errorf("connection not established")
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	data = append(data, '\n')
	_, err = s.conn.Write(data)
	if err != nil {
		return fmt.Errorf("failed to write message: %w", err)
	}
	return nil
}

// Receive returns the channel for incoming signaling messages.
func (s *TCPSignaler) Receive() <-chan SignalingMessage {
	return s.recvCh
}

// RemoteAddr returns the remote peer's address string.
func (s *TCPSignaler) RemoteAddr() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.remoteAddr
}

// ListenAddr returns the local listen address.
func (s *TCPSignaler) ListenAddr() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.listenAddr
}

// Ready returns a channel that closes when the connection is established.
func (s *TCPSignaler) Ready() <-chan struct{} {
	return s.ready
}

// ListenerReady returns a channel that closes when the server is listening.
// Only relevant for RoleServer.
func (s *TCPSignaler) ListenerReady() <-chan struct{} {
	return s.listenerReady
}

// Close terminates the signaling connection and releases resources.
func (s *TCPSignaler) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.readCancel != nil {
		s.readCancel()
	}

	var errs []error

	if s.conn != nil {
		if err := s.conn.Close(); err != nil {
			errs = append(errs, err)
		}
		s.conn = nil
	}

	if s.listener != nil {
		if err := s.listener.Close(); err != nil {
			errs = append(errs, err)
		}
		s.listener = nil
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors during close: %v", errs)
	}
	return nil
}
