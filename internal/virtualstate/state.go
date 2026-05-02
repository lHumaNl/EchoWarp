// Package virtualstate persists host-level virtual audio device ownership.
package virtualstate

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/lHumaNl/echowarp/internal/config"
	"github.com/lHumaNl/echowarp/internal/recent"
)

const (
	FileName = "virtual_devices.yaml"
	Version  = 1
	LockName = FileName + ".lock"

	filePerm = 0o600
	dirPerm  = 0o700

	defaultLockRetryDelay     = 10 * time.Millisecond
	defaultLockAcquireTimeout = 5 * time.Second
	defaultLockStaleTimeout   = 30 * time.Second
	lockMetadataTimeKey       = "created_at="

	BackendPulseAudio = "pulseaudio"
	ModuleNullSink    = "module-null-sink"

	DesiredPresent = "present"
	DesiredAbsent  = "absent"

	ObservedPresent = "present"
	ObservedAbsent  = "absent"
	ObservedUnknown = "unknown"

	RoleServer   = "server"
	RoleClient   = "client"
	RoleUser     = "user"
	RoleImported = "imported"
	RoleUnknown  = "unknown"
)

var (
	stateMu            sync.Mutex
	atomicReplaceFile  = atomicRename
	lockRetryDelay     = defaultLockRetryDelay
	lockAcquireTimeout = defaultLockAcquireTimeout
	lockStaleTimeout   = defaultLockStaleTimeout
)

// State is the on-disk host-level virtual audio device state.
type State struct {
	Version int      `yaml:"version" json:"version"`
	Devices []Device `yaml:"devices" json:"devices"`
}

// Device describes a single managed virtual audio device.
type Device struct {
	ID           string             `yaml:"id" json:"id"`
	BaseName     string             `yaml:"base_name,omitempty" json:"base_name,omitempty"`
	Backend      string             `yaml:"backend" json:"backend"`
	ModuleType   string             `yaml:"module_type" json:"module_type"`
	SinkName     string             `yaml:"sink_name" json:"sink_name"`
	MonitorName  string             `yaml:"monitor_name" json:"monitor_name"`
	PlaybackName string             `yaml:"playback_name,omitempty" json:"playback_name,omitempty"`
	CaptureName  string             `yaml:"capture_name,omitempty" json:"capture_name,omitempty"`
	State        DeviceRuntimeState `yaml:"state" json:"state"`
	Ownership    DeviceOwnership    `yaml:"ownership" json:"ownership"`
	Policy       DevicePolicy       `yaml:"policy" json:"policy"`
}

type DeviceRuntimeState struct {
	Desired  string `yaml:"desired" json:"desired"`
	Observed string `yaml:"observed" json:"observed"`
	ModuleID string `yaml:"module_id,omitempty" json:"module_id,omitempty"`
}

type DeviceOwnership struct {
	CreatedBy string `yaml:"created_by" json:"created_by"`
	SessionID string `yaml:"session_id,omitempty" json:"session_id,omitempty"`
}

type DeviceMetadata struct {
	ID           string
	BaseName     string
	PlaybackName string
	CaptureName  string
	SessionID    string
}

type DevicePolicy struct {
	OnStop  recent.SinkLifecycle `yaml:"on_stop" json:"on_stop"`
	OnStart recent.SinkLifecycle `yaml:"on_start" json:"on_start"`
}

// FilePath returns ~/.config/echowarp/virtual_devices.yaml or test override.
func FilePath() string {
	return filepath.Join(config.EchoWarpDir(), FileName)
}

func Load() (State, error) {
	data, err := os.ReadFile(FilePath())
	if errors.Is(err, os.ErrNotExist) {
		return State{Version: Version}, nil
	}
	if err != nil {
		return State{}, err
	}
	return parseState(data)
}

func Save(state State) error {
	return withStateLock(func() error {
		return saveUnlocked(state)
	})
}

func saveUnlocked(state State) error {
	state.Version = Version
	data, err := yaml.Marshal(state)
	if err != nil {
		return err
	}
	return writeFileAtomic(FilePath(), data)
}

func writeFileAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, dirPerm); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+"-*.tmp")
	if err != nil {
		return err
	}
	return writeTempAndReplace(tmp, path, data)
}

func writeTempAndReplace(tmp *os.File, path string, data []byte) error {
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()
	if err := writeAndSyncTemp(tmp, data); err != nil {
		return err
	}
	if err := atomicReplaceFile(tmpName, path); err != nil {
		return err
	}
	fsyncDirBestEffort(filepath.Dir(path))
	return nil
}

func writeAndSyncTemp(tmp *os.File, data []byte) error {
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	return tmp.Close()
}

func fsyncDirBestEffort(dir string) {
	dirFile, err := os.Open(dir)
	if err != nil {
		return
	}
	defer func() { _ = dirFile.Close() }()
	_ = dirFile.Sync()
}

func withStateLock(fn func() error) error {
	unlock, err := lockStateFile()
	if err != nil {
		return err
	}
	defer unlock()
	return fn()
}

func lockStateFile() (func(), error) {
	stateMu.Lock()
	lockPath := filepath.Join(config.EchoWarpDir(), LockName)
	if err := os.MkdirAll(filepath.Dir(lockPath), dirPerm); err != nil {
		stateMu.Unlock()
		return nil, err
	}
	return acquireLockFile(lockPath)
}

func acquireLockFile(lockPath string) (func(), error) {
	deadline := time.Now().Add(lockAcquireTimeout)
	for {
		file, err := createLockFile(lockPath)
		if err == nil {
			return lockFileRelease(lockPath, file), nil
		}
		if errors.Is(err, os.ErrExist) {
			if recovered, staleErr := recoverStaleLock(lockPath); staleErr != nil {
				stateMu.Unlock()
				return nil, staleErr
			} else if recovered {
				continue
			}
		}
		if !errors.Is(err, os.ErrExist) || time.Now().After(deadline) {
			stateMu.Unlock()
			return nil, fmt.Errorf("acquire virtual state lock: %w", err)
		}
		time.Sleep(lockRetryDelay)
	}
}

func createLockFile(lockPath string) (*os.File, error) {
	file, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, filePerm)
	if err != nil {
		return nil, err
	}
	if _, err := file.WriteString(lockMetadata()); err != nil {
		_ = file.Close()
		_ = os.Remove(lockPath)
		return nil, err
	}
	return file, nil
}

func lockMetadata() string {
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}
	return fmt.Sprintf("pid=%d\nhostname=%s\n%s%s\n", os.Getpid(), hostname, lockMetadataTimeKey, time.Now().Format(time.RFC3339Nano))
}

func recoverStaleLock(lockPath string) (bool, error) {
	stale, err := isStaleLock(lockPath, time.Now())
	if errors.Is(err, os.ErrNotExist) {
		return true, nil
	}
	if err != nil || !stale {
		return false, err
	}
	if err := os.Remove(lockPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return false, fmt.Errorf("remove stale virtual state lock: %w", err)
	}
	return true, nil
}

func isStaleLock(lockPath string, now time.Time) (bool, error) {
	stat, err := os.Stat(lockPath)
	if err != nil {
		return false, err
	}
	createdAt, ok := readLockCreatedAt(lockPath)
	if ok {
		return now.Sub(createdAt) > lockStaleTimeout, nil
	}
	return now.Sub(stat.ModTime()) > lockStaleTimeout, nil
}

func readLockCreatedAt(lockPath string) (time.Time, bool) {
	data, err := os.ReadFile(lockPath)
	if err != nil {
		return time.Time{}, false
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, lockMetadataTimeKey) {
			return parseLockCreatedAt(line)
		}
	}
	return time.Time{}, false
}

func parseLockCreatedAt(line string) (time.Time, bool) {
	value := strings.TrimPrefix(line, lockMetadataTimeKey)
	createdAt, err := time.Parse(time.RFC3339Nano, value)
	return createdAt, err == nil
}

func lockFileRelease(lockPath string, file *os.File) func() {
	return func() {
		_ = file.Close()
		_ = os.Remove(lockPath)
		stateMu.Unlock()
	}
}

func LoadDevice(sinkName string) (Device, bool, error) {
	state, err := Load()
	if err != nil {
		return Device{}, false, err
	}
	device, _, ok := Find(state, sinkName)
	return device, ok, nil
}

func Find(state State, sinkName string) (Device, int, bool) {
	for i, device := range state.Devices {
		if device.SinkName == sinkName && isNullSinkModule(device.ModuleType) {
			return normalizeDevice(device), i, true
		}
	}
	return Device{}, -1, false
}

func isNullSinkModule(moduleType string) bool {
	return moduleType == "" || moduleType == ModuleNullSink
}

func UpsertPresent(sinkName, monitorName, moduleID, owner string, policy DevicePolicy) error {
	return UpsertPresentWithMetadata(sinkName, monitorName, moduleID, owner, policy, DeviceMetadata{})
}

func UpsertPresentWithMetadata(
	sinkName, monitorName, moduleID, owner string,
	policy DevicePolicy,
	metadata DeviceMetadata,
) error {
	return withStateLock(func() error {
		return upsertPresentUnlocked(sinkName, monitorName, moduleID, owner, policy, metadata)
	})
}

func upsertPresentUnlocked(
	sinkName, monitorName, moduleID, owner string,
	policy DevicePolicy,
	metadata DeviceMetadata,
) error {
	state, err := Load()
	if err != nil {
		return err
	}
	upsertDevice(&state, presentDevice(sinkName, monitorName, moduleID, owner, policy, metadata))
	return saveUnlocked(state)
}

func ImportObservedPresent(sinkName, monitorName, moduleID string) error {
	return withStateLock(func() error {
		return importObservedPresentUnlocked(sinkName, monitorName, moduleID)
	})
}

func importObservedPresentUnlocked(sinkName, monitorName, moduleID string) error {
	state, err := Load()
	if err != nil {
		return err
	}
	markObservedPresent(&state, sinkName, monitorName, moduleID)
	return saveUnlocked(state)
}

func MarkAbsent(sinkName, owner string) error {
	return withStateLock(func() error {
		return markAbsentUnlocked(sinkName, owner)
	})
}

func DeleteDevice(sinkName string) error {
	return withStateLock(func() error {
		return deleteDeviceUnlocked(sinkName)
	})
}

func deleteDeviceUnlocked(sinkName string) error {
	state, err := Load()
	if err != nil {
		return err
	}
	deleteDevice(&state, sinkName)
	return saveUnlocked(state)
}

func markAbsentUnlocked(sinkName, owner string) error {
	state, err := Load()
	if err != nil {
		return err
	}
	markAbsent(&state, sinkName, owner)
	return saveUnlocked(state)
}

func IsCurrentRoleOwner(device Device, role string) bool {
	return device.Ownership.CreatedBy == role
}

func IsOtherRoleOwner(device Device, role string) bool {
	owner := device.Ownership.CreatedBy
	return (owner == RoleServer || owner == RoleClient) && owner != role
}

func parseState(data []byte) (State, error) {
	var state State
	if err := yaml.Unmarshal(data, &state); err != nil {
		return State{}, err
	}
	if state.Version == 0 {
		state.Version = Version
	}
	return state, nil
}

func upsertDevice(state *State, device Device) {
	_, idx, ok := Find(*state, device.SinkName)
	if ok {
		state.Devices[idx] = device
		return
	}
	state.Devices = append(state.Devices, device)
}

func markObservedPresent(state *State, sinkName, monitorName, moduleID string) {
	device, idx, ok := Find(*state, sinkName)
	if !ok {
		device = presentDevice(sinkName, monitorName, moduleID, RoleImported, safePolicy(), DeviceMetadata{})
		state.Devices = append(state.Devices, device)
		return
	}
	device.State.Observed = ObservedPresent
	device.State.ModuleID = moduleID
	state.Devices[idx] = device
}

func markAbsent(state *State, sinkName, owner string) {
	device, idx, ok := Find(*state, sinkName)
	if !ok {
		device = presentDevice(sinkName, "", "", owner, safePolicy(), DeviceMetadata{})
		idx = len(state.Devices)
		state.Devices = append(state.Devices, device)
	}
	device.State = DeviceRuntimeState{Desired: DesiredAbsent, Observed: ObservedAbsent}
	device.Ownership.CreatedBy = owner
	state.Devices[idx] = device
}

func deleteDevice(state *State, sinkName string) {
	_, idx, ok := Find(*state, sinkName)
	if !ok {
		return
	}
	state.Devices = append(state.Devices[:idx], state.Devices[idx+1:]...)
}

func presentDevice(
	sinkName, monitorName, moduleID, owner string,
	policy DevicePolicy,
	metadata DeviceMetadata,
) Device {
	id := metadata.ID
	if id == "" {
		id = DeviceID(sinkName)
	}
	return Device{
		ID:           id,
		BaseName:     metadata.BaseName,
		Backend:      BackendPulseAudio,
		ModuleType:   ModuleNullSink,
		SinkName:     sinkName,
		MonitorName:  monitorName,
		PlaybackName: metadata.PlaybackName,
		CaptureName:  metadata.CaptureName,
		State:        DeviceRuntimeState{Desired: DesiredPresent, Observed: ObservedPresent, ModuleID: moduleID},
		Ownership:    DeviceOwnership{CreatedBy: owner, SessionID: metadata.SessionID},
		Policy:       normalizePolicy(policy),
	}
}

func normalizeDevice(device Device) Device {
	if device.ID == "" {
		device.ID = DeviceID(device.SinkName)
	}
	if device.Backend == "" {
		device.Backend = BackendPulseAudio
	}
	if device.ModuleType == "" {
		device.ModuleType = ModuleNullSink
	}
	device.Policy = normalizePolicy(device.Policy)
	return device
}

func normalizePolicy(policy DevicePolicy) DevicePolicy {
	if policy.OnStop == "" {
		policy.OnStop = recent.SinkKeep
	}
	if policy.OnStart == "" {
		policy.OnStart = recent.SinkKeep
	}
	return policy
}

func safePolicy() DevicePolicy {
	return DevicePolicy{OnStop: recent.SinkKeep, OnStart: recent.SinkKeep}
}

func DeviceID(sinkName string) string {
	return BackendPulseAudio + ":" + ModuleNullSink + ":" + sinkName
}

func NewSessionID() string {
	return randomHexID("session")
}

func NewVirtualDeviceID() string {
	return randomHexID("virtual")
}

func randomHexID(prefix string) string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
	}
	return prefix + "-" + hex.EncodeToString(b[:])
}
