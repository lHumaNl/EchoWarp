package virtualstate

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lHumaNl/echowarp/internal/recent"
)

func TestFilePathUsesEchoWarpConfigDir(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ECHOWARP_CONFIG_DIR", dir)

	assert.Equal(t, filepath.Join(dir, FileName), FilePath())
}

func TestUpsertPresentWritesMinimumSchema(t *testing.T) {
	t.Setenv("ECHOWARP_CONFIG_DIR", t.TempDir())
	policy := DevicePolicy{OnStop: recent.SinkKeep, OnStart: recent.SinkRecreate}

	require.NoError(t, UpsertPresent("EchoWarp", "Monitor of EchoWarp", "42", RoleServer, policy))
	device, ok, err := LoadDevice("EchoWarp")

	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, Version, mustLoadState(t).Version)
	assert.Equal(t, BackendPulseAudio, device.Backend)
	assert.Equal(t, ModuleNullSink, device.ModuleType)
	assert.Equal(t, DesiredPresent, device.State.Desired)
	assert.Equal(t, ObservedPresent, device.State.Observed)
	assert.Equal(t, RoleServer, device.Ownership.CreatedBy)
	assert.Equal(t, recent.SinkRecreate, device.Policy.OnStart)
}

func TestMarkAbsentSuppressesFutureRecreate(t *testing.T) {
	t.Setenv("ECHOWARP_CONFIG_DIR", t.TempDir())

	require.NoError(t, MarkAbsent("EchoWarp", RoleUser))
	device, ok, err := LoadDevice("EchoWarp")

	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, DesiredAbsent, device.State.Desired)
	assert.Equal(t, ObservedAbsent, device.State.Observed)
	assert.Empty(t, device.State.ModuleID)
}

func TestSaveAtomicFailureKeepsExistingYAML(t *testing.T) {
	t.Setenv("ECHOWARP_CONFIG_DIR", t.TempDir())
	initial := State{Devices: []Device{presentDevice("existing", "monitor", "1", RoleServer, safePolicy(), DeviceMetadata{})}}
	require.NoError(t, Save(initial))

	replaceErr := errors.New("replace failed")
	replace := atomicReplaceFile
	atomicReplaceFile = func(_, _ string) error { return replaceErr }
	t.Cleanup(func() { atomicReplaceFile = replace })

	err := Save(State{Devices: []Device{presentDevice("new", "monitor", "2", RoleClient, safePolicy(), DeviceMetadata{})}})

	require.ErrorIs(t, err, replaceErr)
	state := mustLoadState(t)
	assert.Len(t, state.Devices, 1)
	assert.Equal(t, "existing", state.Devices[0].SinkName)
}

func TestConcurrentUpsertsPreserveAllDevices(t *testing.T) {
	t.Setenv("ECHOWARP_CONFIG_DIR", t.TempDir())
	const writerCount = 16
	start := make(chan struct{})
	errs := make(chan error, writerCount)
	var wg sync.WaitGroup

	for i := range writerCount {
		wg.Add(1)
		go concurrentUpsert(&wg, start, errs, i)
	}
	close(start)
	wg.Wait()
	close(errs)

	for err := range errs {
		require.NoError(t, err)
	}
	assert.Len(t, mustLoadState(t).Devices, writerCount)
}

func TestStaleLockFileIsRecovered(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ECHOWARP_CONFIG_DIR", dir)
	lockPath := filepath.Join(dir, LockName)
	require.NoError(t, os.WriteFile(lockPath, []byte("orphaned lock"), filePerm))
	staleTime := time.Now().Add(-lockStaleTimeout - time.Second)
	require.NoError(t, os.Chtimes(lockPath, staleTime, staleTime))

	err := UpsertPresent("EchoWarp", "Monitor of EchoWarp", "42", RoleServer, safePolicy())

	require.NoError(t, err)
	device, ok, err := LoadDevice("EchoWarp")
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, RoleServer, device.Ownership.CreatedBy)
}

func TestFreshLockFileBlocksAcquire(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ECHOWARP_CONFIG_DIR", dir)
	restoreLockTimings := setTestLockTimings(20*time.Millisecond, time.Millisecond, time.Hour)
	t.Cleanup(restoreLockTimings)
	lockPath := filepath.Join(dir, LockName)
	require.NoError(t, os.WriteFile(lockPath, []byte(lockMetadata()), filePerm))

	err := UpsertPresent("EchoWarp", "Monitor of EchoWarp", "42", RoleServer, safePolicy())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "acquire virtual state lock")
	assert.FileExists(t, lockPath)
}

func TestLoadLegacyAndMultiDeviceStateMigration(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ECHOWARP_CONFIG_DIR", dir)
	yamlData := strings.TrimSpace(`
devices:
  - sink_name: EchoWarp
    monitor_name: Monitor of EchoWarp
    state:
      desired: present
      observed: present
      module_id: "42"
    ownership:
      created_by: server
    policy:
      on_stop: delete
      on_start: recreate
  - id: virtual-beefcafe
    base_name: Studio
    backend: pulseaudio
    module_type: module-null-sink
    sink_name: echowarp_studio_beefcafe
    monitor_name: echowarp_studio_beefcafe.monitor
    playback_name: Playback Studio
    capture_name: Capture Studio
    state:
      desired: present
      observed: present
      module_id: "77"
    ownership:
      created_by: client
      session_id: session-123
    policy:
      on_stop: keep
      on_start: recreate
`) + "\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, FileName), []byte(yamlData), 0o600))

	state, err := Load()
	require.NoError(t, err)
	assert.Equal(t, Version, state.Version)
	legacy, ok, err := LoadDevice("EchoWarp")
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, DeviceID("EchoWarp"), legacy.ID)
	assert.Equal(t, BackendPulseAudio, legacy.Backend)
	assert.Equal(t, ModuleNullSink, legacy.ModuleType)
	studio, ok, err := LoadDevice("echowarp_studio_beefcafe")
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, "virtual-beefcafe", studio.ID)
	assert.Equal(t, "Studio", studio.BaseName)
	assert.Equal(t, "Playback Studio", studio.PlaybackName)
	assert.Equal(t, "Capture Studio", studio.CaptureName)
	assert.Equal(t, "session-123", studio.Ownership.SessionID)
}

func concurrentUpsert(wg *sync.WaitGroup, start <-chan struct{}, errs chan<- error, index int) {
	defer wg.Done()
	<-start
	sinkName := fmt.Sprintf("EchoWarp-%02d", index)
	err := UpsertPresent(sinkName, "Monitor of "+sinkName, fmt.Sprint(index), RoleServer, safePolicy())
	errs <- err
}

func setTestLockTimings(acquire, retry, stale time.Duration) func() {
	oldAcquire := lockAcquireTimeout
	oldRetry := lockRetryDelay
	oldStale := lockStaleTimeout
	lockAcquireTimeout = acquire
	lockRetryDelay = retry
	lockStaleTimeout = stale
	return func() {
		lockAcquireTimeout = oldAcquire
		lockRetryDelay = oldRetry
		lockStaleTimeout = oldStale
	}
}

func mustLoadState(t *testing.T) State {
	t.Helper()
	state, err := Load()
	require.NoError(t, err)
	return state
}
