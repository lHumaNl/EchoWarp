package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/lHumaNl/echowarp/internal/version"
	"github.com/lHumaNl/echowarp/pkg/echowarp"
	"github.com/lHumaNl/echowarp/pkg/echowarp/audio"
	"github.com/lHumaNl/echowarp/pkg/echowarp/discovery"
)

// statusResponse is returned by GET /api/v1/status.
type statusResponse struct {
	Status string `json:"status"` // Current node status.
	Uptime int64  `json:"uptime"` // Uptime in seconds.
}

// clientsResponse is returned by GET /api/v1/clients.
type clientsResponse struct {
	Clients []clientInfo `json:"clients"` // List of connected clients.
	Count   int          `json:"count"`   // Current client count.
	Max     int          `json:"max"`     // Maximum allowed clients.
}

// clientInfo represents a connected client.
type clientInfo struct {
	ID      string `json:"id"`      // Unique client identifier.
	Address string `json:"address"` // Client's remote address.
}

// errorResponse is returned on API errors.
type errorResponse struct {
	Error string `json:"error"` // Error message.
}

// configRequest is used for POST/PUT /api/v1/config.
// Only non-nil / non-zero fields are applied to the underlying NodeConfig.
//
// Pointer-typed fields distinguish "user explicitly sent value X" from "field
// absent in the JSON body". Primitive-typed fields (Mode, Address, Port, …)
// remain on the legacy "zero value = don't override" contract for backwards
// compatibility with phase 1 / phase 2 clients.
type configRequest struct {
	Mode       string  `json:"mode,omitempty"`        // "server" or "client".
	Address    string  `json:"address,omitempty"`     // Server address (client mode).
	Port       int     `json:"port,omitempty"`        // TCP signaling port.
	DeviceID   *uint32 `json:"device_id,omitempty"`   // Audio device ID.
	Reverse    *bool   `json:"reverse,omitempty"`     // Reverse audio direction.
	Password   *string `json:"password,omitempty"`    // Authentication password.
	SampleRate uint32  `json:"sample_rate,omitempty"` // Audio sample rate.
	Channels   uint32  `json:"channels,omitempty"`    // Audio channels (1 or 2).
	VirtualMic *bool   `json:"virtual_mic,omitempty"` // Create virtual microphone.
	Loopback   *bool   `json:"loopback,omitempty"`    // Enable system-audio loopback capture.
	AEC        *bool   `json:"aec,omitempty"`         // Enable software AEC (duplex/conference).

	// Phase 3 extended optional fields. All pointer-typed so nil = unset.
	Duplex          *bool     `json:"duplex,omitempty"`
	Conference      *bool     `json:"conference,omitempty"`
	OpusBitrate     *int      `json:"opus_bitrate,omitempty"`
	OpusComplexity  *int      `json:"opus_complexity,omitempty"`
	OpusApplication *string   `json:"opus_application,omitempty"` // "voip" | "audio" | "restricted_lowdelay"
	OpusDTX         *bool     `json:"opus_dtx,omitempty"`
	OpusFEC         *bool     `json:"opus_fec,omitempty"`
	MaxClients      *int      `json:"max_clients,omitempty"`
	Nickname        *string   `json:"nickname,omitempty"`
	HWIDRequired    *bool     `json:"hwid_required,omitempty"`
	STUNServers     *[]string `json:"stun_servers,omitempty"`
	TLSCert         *string   `json:"tls_cert,omitempty"`
	TLSKey          *string   `json:"tls_key,omitempty"`
	TLSInsecure     *bool     `json:"tls_insecure,omitempty"`
}

// validOpusApplications enumerates the accepted values for configRequest.OpusApplication.
// Values are sourced directly from pkg/echowarp/audio constants so the API
// contract always matches what the encoder actually accepts.
var validOpusApplications = map[string]struct{}{
	audio.OpusApplicationVoIP:               {},
	audio.OpusApplicationAudio:              {},
	audio.OpusApplicationRestrictedLowDelay: {},
}

// writeJSON writes a JSON response with the given status code.
// Sets Content-Type header to application/json.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v) //nolint:errcheck
}

// sessionInfoResponse is returned by GET /api/v1/session/info.
// This endpoint is unauthenticated — clients use it to discover server capabilities before connecting.
type sessionInfoResponse struct {
	Mode             string `json:"mode"`              // "normal" | "reverse" | "duplex" | "conference"
	TLSRequired      bool   `json:"tls_required"`      // Whether TLS is required.
	TLSSelfSigned    bool   `json:"tls_self_signed"`   // Whether the TLS certificate is self-signed.
	CurrentClients   int    `json:"current_clients"`   // Currently connected clients.
	MaxClients       int    `json:"max_clients"`       // Maximum allowed clients.
	SampleRate       uint32 `json:"sample_rate"`       // Audio sample rate in Hz.
	Channels         uint32 `json:"channels"`          // Audio channels (1 or 2).
	OpusBitrate      int    `json:"opus_bitrate"`      // Opus bitrate in bps.
	ServerVersion    string `json:"server_version"`    // Server software version.
	ServerName       string `json:"server_name"`       // Human-readable server name.
	PasswordRequired bool   `json:"password_required"` // Whether password auth is required.
	HWIDRequired     bool   `json:"hwid_required"`     // Whether server collects HWID for bans.
	ServerID         string `json:"server_id"`         // Stable per-port UUID identifying this server across networks.
}

// writeError writes a JSON error response with the given status code and message.
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, errorResponse{Error: msg})
}

// validateConfigRequest rejects requests whose enumerated fields carry invalid
// values. Returns an error to be written as 400.
func validateConfigRequest(req configRequest) error {
	if req.OpusApplication != nil {
		if _, ok := validOpusApplications[*req.OpusApplication]; !ok {
			return fmt.Errorf("invalid opus_application %q (must be %s, %s, or %s)",
				*req.OpusApplication,
				audio.OpusApplicationVoIP,
				audio.OpusApplicationAudio,
				audio.OpusApplicationRestrictedLowDelay)
		}
	}
	return nil
}

// applyConfigOverrides updates cfg with non-zero values from req.
// Only fields that are set in the request are applied.
func applyConfigOverrides(cfg *echowarp.NodeConfig, req configRequest) {
	if req.Mode != "" {
		cfg.Mode = echowarp.Mode(req.Mode)
	}
	if req.Address != "" {
		cfg.Address = req.Address
	}
	if req.Port != 0 {
		cfg.Port = req.Port
	}
	if req.DeviceID != nil {
		cfg.DeviceID = req.DeviceID
	}
	if req.SampleRate != 0 {
		cfg.SampleRate = req.SampleRate
	}
	if req.Channels != 0 {
		cfg.Channels = req.Channels
	}
	if req.Reverse != nil {
		cfg.Reverse = *req.Reverse
	}
	if req.Password != nil {
		cfg.Password = *req.Password
	}
	if req.VirtualMic != nil {
		cfg.VirtualMic = *req.VirtualMic
	}
	if req.Loopback != nil {
		cfg.Loopback = *req.Loopback
	}
	if req.AEC != nil {
		cfg.AEC = *req.AEC
	}

	// Phase 3 extended fields.
	if req.Duplex != nil {
		cfg.Duplex = *req.Duplex
	}
	if req.Conference != nil {
		cfg.Conference = *req.Conference
	}
	if req.OpusBitrate != nil {
		cfg.OpusBitrate = *req.OpusBitrate
	}
	if req.OpusComplexity != nil {
		cfg.OpusComplexity = *req.OpusComplexity
	}
	if req.OpusApplication != nil {
		cfg.OpusApplication = *req.OpusApplication
	}
	if req.OpusDTX != nil {
		cfg.OpusDTX = *req.OpusDTX
	}
	if req.OpusFEC != nil {
		cfg.OpusFEC = *req.OpusFEC
	}
	if req.MaxClients != nil {
		cfg.MaxClients = *req.MaxClients
	}
	if req.Nickname != nil {
		cfg.Nickname = *req.Nickname
	}
	if req.HWIDRequired != nil {
		cfg.HWIDRequired = *req.HWIDRequired
	}
	if req.STUNServers != nil {
		cfg.STUNServers = append([]string(nil), (*req.STUNServers)...)
	}
	if req.TLSCert != nil {
		cfg.TLSCert = *req.TLSCert
	}
	if req.TLSKey != nil {
		cfg.TLSKey = *req.TLSKey
	}
	if req.TLSInsecure != nil {
		cfg.TLSInsecure = *req.TLSInsecure
	}
}

func (s *APIServer) handleStatus(w http.ResponseWriter, r *http.Request) {
	status := s.node.Status()
	uptime := s.node.Uptime()
	writeJSON(w, http.StatusOK, statusResponse{
		Status: string(status),
		Uptime: int64(uptime.Seconds()),
	})
}

func (s *APIServer) handleVersion(w http.ResponseWriter, r *http.Request) {
	versionInfo := map[string]string{
		"version":   version.Version,
		"commit":    version.Commit,
		"buildDate": version.BuildDate,
		"goVersion": runtime.Version(),
	}
	writeJSON(w, http.StatusOK, versionInfo)
}

func (s *APIServer) handleStats(w http.ResponseWriter, r *http.Request) {
	stats := s.node.Stats()
	writeJSON(w, http.StatusOK, stats)
}

func (s *APIServer) handleDevices(w http.ResponseWriter, r *http.Request) {
	devices, err := echowarp.Devices()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, devices)
}

func (s *APIServer) handleClients(w http.ResponseWriter, r *http.Request) {
	clients, maxClients := s.node.Clients()
	infos := make([]clientInfo, len(clients))
	for i, c := range clients {
		infos[i] = clientInfo{ID: c.ID, Address: c.Address}
	}
	writeJSON(w, http.StatusOK, clientsResponse{
		Clients: infos,
		Count:   len(infos),
		Max:     maxClients,
	})
}

func (s *APIServer) handleConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		cfg := s.node.SafeConfig()
		writeJSON(w, http.StatusOK, cfg)
	case http.MethodPut, http.MethodPost:
		var req configRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
			return
		}
		if err := validateConfigRequest(req); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		cfg := s.node.Config()
		applyConfigOverrides(&cfg, req)
		if err := s.node.Reconfigure(cfg); err != nil {
			writeError(w, http.StatusConflict, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "configured"})
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *APIServer) handleStart(w http.ResponseWriter, r *http.Request) {
	if r.ContentLength > 0 {
		var req configRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
			return
		}
		if err := validateConfigRequest(req); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		cfg := s.node.Config()
		applyConfigOverrides(&cfg, req)
		if err := s.node.Reconfigure(cfg); err != nil {
			writeError(w, http.StatusConflict, err.Error())
			return
		}
	}

	if s.node.Status() != echowarp.StatusIdle && s.node.Status() != echowarp.StatusStopped {
		writeError(w, http.StatusConflict, "node already running")
		return
	}

	// Start the node in a goroutine and wait up to 2s for a synchronous error.
	// If the node fails fast (e.g., missing runner factory, invalid config) the
	// caller gets a 500 with the error. If the node is still starting after
	// the grace period we return 202 Accepted and keep it running in the
	// background — further status can be polled via GET /api/v1/status.
	errCh := make(chan error, 1)
	go func() {
		errCh <- s.node.Start(context.Background())
	}()

	select {
	case err := <-errCh:
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "started"})
	case <-time.After(2 * time.Second):
		writeJSON(w, http.StatusAccepted, map[string]string{"status": "starting"})
	case <-r.Context().Done():
		writeError(w, http.StatusRequestTimeout, "request canceled")
	}
}

// connectRequest is the body for POST /api/v1/connect.
// It configures the node in client mode and starts it. Either Address or
// Discover must be set. Discover triggers mDNS auto-discovery (single server).
type connectRequest struct {
	Address  string  `json:"address,omitempty"`   // Target server address.
	Port     int     `json:"port,omitempty"`      // Target TCP port (default 4415 from current cfg).
	Password string  `json:"password,omitempty"`  // Authentication password.
	DeviceID *uint32 `json:"device_id,omitempty"` // Audio device ID.
	Nickname string  `json:"nickname,omitempty"`  // Chat display name.
	Discover bool    `json:"discover,omitempty"`  // Auto-discover server via mDNS.
}

// handleConnect configures the node in client mode and starts it. This is the
// client-side counterpart of handleStart: useful when the daemon was started
// idle (no explicit mode) and the caller wants to switch to client mode on
// demand — e.g., the Decky plugin connecting to a LAN server.
//
// Returns:
//   - 400 if neither address nor discover is set.
//   - 409 if the node is already running.
//   - 500 if Node.Start() fails synchronously within 2s.
//   - 202 if the node is still starting after 2s (ongoing in background).
//   - 200 on fast successful start.
func (s *APIServer) handleConnect(w http.ResponseWriter, r *http.Request) {
	var req connectRequest
	if r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
			return
		}
	}

	if req.Address == "" && !req.Discover {
		writeError(w, http.StatusBadRequest, "address or discover must be set")
		return
	}

	if s.node.Status() != echowarp.StatusIdle && s.node.Status() != echowarp.StatusStopped {
		writeError(w, http.StatusConflict, "already running")
		return
	}

	cfg := s.node.Config()
	cfg.Mode = echowarp.ModeClient
	if req.Address != "" {
		cfg.Address = req.Address
	}
	if req.Port != 0 {
		cfg.Port = req.Port
	}
	if req.Password != "" {
		cfg.Password = req.Password
	}
	if req.DeviceID != nil {
		cfg.DeviceID = req.DeviceID
	}
	if req.Nickname != "" {
		cfg.Nickname = req.Nickname
	}

	if err := s.node.Reconfigure(cfg); err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- s.node.Start(context.Background())
	}()

	select {
	case err := <-errCh:
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "connected"})
	case <-time.After(2 * time.Second):
		writeJSON(w, http.StatusAccepted, map[string]string{"status": "connecting"})
	case <-r.Context().Done():
		writeError(w, http.StatusRequestTimeout, "request canceled")
	}
}

// handleDisconnect is a semantic alias for handleStop used by client-side
// callers. It delegates to the same stop logic — the distinction is purely
// for API clarity (Decky plugin disconnects from a server vs. stopping a
// running server).
func (s *APIServer) handleDisconnect(w http.ResponseWriter, r *http.Request) {
	s.handleStop(w, r)
}

func (s *APIServer) handleStop(w http.ResponseWriter, r *http.Request) {
	if s.node.Status() == echowarp.StatusStopped || s.node.Status() == echowarp.StatusIdle {
		writeError(w, http.StatusConflict, "node not running")
		return
	}

	if err := s.node.Stop(); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "stopped"})
}

func (s *APIServer) handlePause(w http.ResponseWriter, r *http.Request) {
	if err := s.node.Pause(); err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "paused"})
}

func (s *APIServer) handleResume(w http.ResponseWriter, r *http.Request) {
	if err := s.node.Resume(); err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "streaming"})
}

func (s *APIServer) handleShutdown(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "shutting down"})
	go func() {
		time.Sleep(100 * time.Millisecond)
		_ = s.node.Stop() //nolint:errcheck
		_ = s.Stop()      //nolint:errcheck
	}()
}

// handleSessionInfo returns public server capabilities for client auto-configuration.
// This endpoint is unauthenticated — clients probe it before connecting.
func (s *APIServer) handleSessionInfo(w http.ResponseWriter, r *http.Request) {
	cfg := s.node.Config()

	mode := "normal"
	if cfg.Conference {
		mode = "conference"
	} else if cfg.Duplex {
		mode = "duplex"
	} else if cfg.Reverse {
		mode = "reverse"
	}

	hostname, _ := os.Hostname() //nolint:errcheck
	clients, _ := s.node.Clients()

	resp := sessionInfoResponse{
		Mode:             mode,
		TLSRequired:      cfg.TLS,
		TLSSelfSigned:    cfg.TLSSelfSigned,
		CurrentClients:   len(clients),
		MaxClients:       cfg.MaxClients,
		SampleRate:       cfg.SampleRate,
		Channels:         cfg.Channels,
		OpusBitrate:      cfg.OpusBitrate,
		ServerVersion:    version.Version,
		ServerName:       hostname,
		PasswordRequired: cfg.Password != "",
		HWIDRequired:     cfg.HWIDRequired,
		ServerID:         s.serverID(),
	}
	writeJSON(w, http.StatusOK, resp)
}

// ── Discovery ────────────────────────────────────────────────────────────────

// discoverRequest is the request body for POST /api/v1/discover.
type discoverRequest struct {
	TimeoutMs int `json:"timeout_ms"` // Discovery timeout in milliseconds (default 5000).
}

// discoveredServer is a single discovered EchoWarp server.
type discoveredServer struct {
	Name string `json:"name"` // mDNS instance name.
	Addr string `json:"addr"` // First IPv4 address (or host name if none).
	Port int    `json:"port"` // TCP port.
}

// handleDiscover triggers an mDNS discovery and returns the results.
func (s *APIServer) handleDiscover(w http.ResponseWriter, r *http.Request) {
	var req discoverRequest
	if r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
			return
		}
	}

	timeoutMs := req.TimeoutMs
	if timeoutMs <= 0 {
		timeoutMs = 5000
	}

	d := discovery.NewDiscoverer()
	ch, err := d.Discover(r.Context(), time.Duration(timeoutMs)*time.Millisecond)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "discovery error: "+err.Error())
		return
	}

	var servers []discoveredServer
	for info := range ch {
		addr := info.Host
		if len(info.AddrIPv4) > 0 {
			addr = info.AddrIPv4[0].String()
		}
		servers = append(servers, discoveredServer{
			Name: info.Name,
			Addr: addr,
			Port: info.Port,
		})
	}

	if servers == nil {
		servers = []discoveredServer{}
	}
	writeJSON(w, http.StatusOK, servers)
}

// ── Device Control ───────────────────────────────────────────────────────────

// muteRequest is the request body for POST /api/v1/devices/{id}/mute.
type muteRequest struct {
	Muted bool `json:"muted"`
}

// volumeRequest is the request body for POST /api/v1/devices/{id}/volume and
// POST /api/v1/conference/participants/{id}/volume.
type volumeRequest struct {
	Volume float64 `json:"volume"`
}

// parseDeviceID extracts and validates the {id} path segment from the URL.
// URL pattern: /api/v1/devices/{id}/...
func parseDeviceID(r *http.Request, prefix string) (int, error) {
	// r.PathValue works with Go 1.22 ServeMux patterns; fall back to manual parse.
	idStr := r.PathValue("id")
	if idStr == "" {
		// Manual extraction: strip prefix, take the next segment.
		rest := strings.TrimPrefix(r.URL.Path, prefix)
		rest = strings.TrimPrefix(rest, "/")
		parts := strings.SplitN(rest, "/", 2)
		idStr = parts[0]
	}
	id, err := strconv.Atoi(idStr)
	if err != nil || id < 0 {
		return 0, fmt.Errorf("invalid device id %q", idStr)
	}
	return id, nil
}

// handleDeviceMute sets the absolute mute state of a device.
//
// Response codes:
//   - 200 on success (command successfully enqueued to the runner).
//   - 400 on invalid device id or malformed JSON body.
//   - 409 when the node is not running (no runner to receive the command).
//   - 500 on any other error propagated from the runner (e.g. queue full).
//
// Note on task 013: the command is delivered to ServerApp/ClientApp's internal
// device command channel. The consumer that actually applies it to the mixer
// is wired up by task 013 — until that is complete, API calls reach the
// channel but may be drained by a log-only consumer. The API itself returns
// 200 only when the command is successfully queued.
func (s *APIServer) handleDeviceMute(w http.ResponseWriter, r *http.Request) {
	id, err := parseDeviceID(r, "/api/v1/devices")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	var req muteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	if err := s.node.SetDeviceMute(id, req.Muted); err != nil {
		writeError(w, deviceCommandStatus(s.node.Status()), err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":    "ok",
		"device_id": id,
		"muted":     req.Muted,
	})
}

// handleDeviceVolume sets the absolute volume multiplier of a device. Valid
// range 0.0–1.5 (matching task 013 — the mixer caps volume at 1.5 to avoid
// clipping).
//
// Response codes: see handleDeviceMute.
func (s *APIServer) handleDeviceVolume(w http.ResponseWriter, r *http.Request) {
	id, err := parseDeviceID(r, "/api/v1/devices")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	var req volumeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	if req.Volume < 0.0 || req.Volume > 1.5 {
		writeError(w, http.StatusBadRequest, "volume must be between 0.0 and 1.5")
		return
	}

	if err := s.node.SetDeviceVolume(id, req.Volume); err != nil {
		writeError(w, deviceCommandStatus(s.node.Status()), err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":    "ok",
		"device_id": id,
		"volume":    req.Volume,
	})
}

// deviceCommandStatus maps a Node.SetDevice* error into an HTTP status code.
// Errors originating from an idle/stopped node map to 409 Conflict (the
// caller must Start the node first); every other error maps to 500.
func deviceCommandStatus(status echowarp.NodeStatus) int {
	switch status {
	case echowarp.StatusIdle, echowarp.StatusStopped:
		return http.StatusConflict
	}
	return http.StatusInternalServerError
}

// ── Conference ───────────────────────────────────────────────────────────────

// participantMuteRequest is the request body for muting a participant.
type participantMuteRequest struct {
	Muted bool `json:"muted"`
}

// parseParticipantID extracts the {id} path segment for conference routes.
// URL pattern: /api/v1/conference/participants/{id}/...
func parseParticipantID(r *http.Request) string {
	id := r.PathValue("id")
	if id != "" {
		return id
	}
	// Manual extraction for older patterns.
	const prefix = "/api/v1/conference/participants/"
	rest := strings.TrimPrefix(r.URL.Path, prefix)
	parts := strings.SplitN(rest, "/", 2)
	return parts[0]
}

// handleConferenceParticipants lists current conference participants.
// TODO: implement when Node exposes conference participant enumeration.
func (s *APIServer) handleConferenceParticipants(w http.ResponseWriter, r *http.Request) {
	// TODO: call s.node.Participants() once the Node exposes it.
	writeJSON(w, http.StatusNotImplemented, map[string]string{
		"error": "conference participant listing not yet implemented",
	})
}

// handleConferenceParticipantMute mutes or unmutes a conference participant.
// TODO: implement when Node exposes per-participant mute control.
func (s *APIServer) handleConferenceParticipantMute(w http.ResponseWriter, r *http.Request) {
	pid := parseParticipantID(r)
	if pid == "" {
		writeError(w, http.StatusBadRequest, "missing participant id")
		return
	}

	var req participantMuteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	// TODO: call s.node.MuteParticipant(pid, req.Muted) once the Node exposes it.
	writeJSON(w, http.StatusNotImplemented, map[string]string{
		"error": fmt.Sprintf("participant mute control not yet implemented (id=%s)", pid),
	})
}

// handleConferenceParticipantKick kicks a participant from the conference.
// TODO: implement when Node exposes participant kick functionality.
func (s *APIServer) handleConferenceParticipantKick(w http.ResponseWriter, r *http.Request) {
	pid := parseParticipantID(r)
	if pid == "" {
		writeError(w, http.StatusBadRequest, "missing participant id")
		return
	}

	// TODO: call s.node.KickParticipant(pid) once the Node exposes it.
	writeJSON(w, http.StatusNotImplemented, map[string]string{
		"error": fmt.Sprintf("participant kick not yet implemented (id=%s)", pid),
	})
}

// handleConferenceParticipantVolume sets a conference participant's volume.
// TODO: implement when Node exposes per-participant volume control.
func (s *APIServer) handleConferenceParticipantVolume(w http.ResponseWriter, r *http.Request) {
	pid := parseParticipantID(r)
	if pid == "" {
		writeError(w, http.StatusBadRequest, "missing participant id")
		return
	}

	var req volumeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	if req.Volume < 0.0 || req.Volume > 2.0 {
		writeError(w, http.StatusBadRequest, "volume must be between 0.0 and 2.0")
		return
	}

	// TODO: call s.node.SetParticipantVolume(pid, req.Volume) once the Node exposes it.
	writeJSON(w, http.StatusNotImplemented, map[string]string{
		"error": fmt.Sprintf("participant volume control not yet implemented (id=%s)", pid),
	})
}

// ─────────────────────────────────────────────────────────────────────────────

func (s *APIServer) handleHealthz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"version": version.Version,
	})
}

func (s *APIServer) handleReadyz(w http.ResponseWriter, r *http.Request) {
	if s.node == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"status": "not ready",
		})
		return
	}

	status := s.node.Status()
	if status == echowarp.StatusStopped {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"status": "not ready",
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"status": "ready",
	})
}
