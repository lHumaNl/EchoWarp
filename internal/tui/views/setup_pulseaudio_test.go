package views

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreatePulseAudioSinkIgnoresMonitorUpdateFailure(t *testing.T) {
	stub := stubPulseAudioCommands(t)
	stub.runErr = errors.New("source not ready")
	vs := customVirtualSinkPreset("custom_ads", "Ads")

	moduleID, err := createPulseAudioSink(vs)

	require.NoError(t, err)
	assert.Equal(t, "77", moduleID)
	assert.Empty(t, stub.unloadedModules)
	assert.Equal(t, pulseAudioMonitorUpdateAttempts, stub.updateAttempts)
}

func TestCreatePulseAudioSinkLoadModuleFailureIsFatal(t *testing.T) {
	stub := stubPulseAudioCommands(t)
	stub.outputErr = errors.New("load failed")
	vs := customVirtualSinkPreset("custom_load_fail", "Load Fail")

	moduleID, err := createPulseAudioSink(vs)

	require.Error(t, err)
	assert.Empty(t, moduleID)
	assert.Contains(t, err.Error(), "pactl failed")
	assert.Empty(t, stub.unloadedModules)
	assert.Zero(t, stub.updateAttempts)
}

func TestCreatePulseAudioSinkEmptyModuleIDIsFatal(t *testing.T) {
	stub := stubPulseAudioCommands(t)
	stub.outputValue = "\n"
	vs := customVirtualSinkPreset("custom_empty", "Empty")

	moduleID, err := createPulseAudioSink(vs)

	require.Error(t, err)
	assert.Empty(t, moduleID)
	assert.Contains(t, err.Error(), "empty module ID")
	assert.Empty(t, stub.unloadedModules)
	assert.Zero(t, stub.updateAttempts)
}

func TestMonitorDescriptionUpdateRetrySucceedsAfterInitialFailure(t *testing.T) {
	stub := stubPulseAudioCommands(t)
	stub.failUpdateAttempts = 1
	vs := customVirtualSinkPreset("custom_retry", "Retry")

	moduleID, err := createPulseAudioSink(vs)

	require.NoError(t, err)
	assert.Equal(t, "77", moduleID)
	assert.Empty(t, stub.unloadedModules)
	assert.Equal(t, 2, stub.updateAttempts)
}

type pulseAudioCommandStub struct {
	outputErr          error
	outputValue        string
	runErr             error
	failUpdateAttempts int
	updateAttempts     int
	unloadedModules    []string
}

func stubPulseAudioCommands(t *testing.T) *pulseAudioCommandStub {
	t.Helper()
	oldOutput := pulseAudioCommandOutputFn
	oldRun := pulseAudioCommandRunFn
	oldDelay := pulseAudioMonitorUpdateDelay
	stub := &pulseAudioCommandStub{}
	pulseAudioMonitorUpdateDelay = 0
	pulseAudioCommandOutputFn = stub.output
	pulseAudioCommandRunFn = stub.run
	t.Cleanup(func() {
		pulseAudioCommandOutputFn = oldOutput
		pulseAudioCommandRunFn = oldRun
		pulseAudioMonitorUpdateDelay = oldDelay
	})
	return stub
}

func (s *pulseAudioCommandStub) output(_ context.Context, args []string) ([]byte, error) {
	if len(args) > 0 && args[0] == "load-module" {
		if s.outputErr != nil {
			return nil, s.outputErr
		}
		if s.outputValue != "" {
			return []byte(s.outputValue), nil
		}
		return []byte("77\n"), nil
	}
	return nil, errors.New("unexpected output command")
}

func (s *pulseAudioCommandStub) run(_ context.Context, args []string) error {
	if len(args) > 0 && args[0] == "unload-module" {
		s.unloadedModules = append(s.unloadedModules, args[1])
		return nil
	}
	return s.updateMonitor(args)
}

func (s *pulseAudioCommandStub) updateMonitor(args []string) error {
	if len(args) == 0 || args[0] != "update-source-proplist" {
		return errors.New("unexpected run command")
	}
	s.updateAttempts++
	if s.updateAttempts <= s.failUpdateAttempts || s.runErr != nil {
		return firstError(s.runErr, errors.New("transient source error"))
	}
	return nil
}

func firstError(primary error, fallback error) error {
	if primary != nil {
		return primary
	}
	return fallback
}
