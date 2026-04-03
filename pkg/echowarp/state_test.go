package echowarp

import (
	"testing"
)

func TestStateIdle_Name(t *testing.T) {
	s := &idleState{}
	if s.Name() != StatusIdle {
		t.Errorf("Expected %s, got %s", StatusIdle, s.Name())
	}
}

func TestStateIdle_CanStart(t *testing.T) {
	s := &idleState{}
	if !s.CanStart() {
		t.Error("Expected CanStart to be true")
	}
	if s.CanStop() {
		t.Error("Expected CanStop to be false")
	}
	if s.CanPause() {
		t.Error("Expected CanPause to be false")
	}
	if s.CanResume() {
		t.Error("Expected CanResume to be false")
	}
	if !s.CanReconfigure() {
		t.Error("Expected CanReconfigure to be true")
	}
}

func TestStateConnecting_Name(t *testing.T) {
	s := &connectingState{}
	if s.Name() != StatusConnecting {
		t.Errorf("Expected %s, got %s", StatusConnecting, s.Name())
	}
}

func TestStateConnecting_Transitions(t *testing.T) {
	s := &connectingState{}
	if s.CanStart() {
		t.Error("Expected CanStart to be false")
	}
	if !s.CanStop() {
		t.Error("Expected CanStop to be true")
	}
	if s.CanPause() {
		t.Error("Expected CanPause to be false")
	}
	if s.CanResume() {
		t.Error("Expected CanResume to be false")
	}
	if s.CanReconfigure() {
		t.Error("Expected CanReconfigure to be false")
	}
}

func TestStateStreaming_Name(t *testing.T) {
	s := &streamingState{}
	if s.Name() != StatusStreaming {
		t.Errorf("Expected %s, got %s", StatusStreaming, s.Name())
	}
}

func TestStateStreaming_Transitions(t *testing.T) {
	s := &streamingState{}
	if s.CanStart() {
		t.Error("Expected CanStart to be false")
	}
	if !s.CanStop() {
		t.Error("Expected CanStop to be true")
	}
	if !s.CanPause() {
		t.Error("Expected CanPause to be true")
	}
	if s.CanResume() {
		t.Error("Expected CanResume to be false")
	}
	if s.CanReconfigure() {
		t.Error("Expected CanReconfigure to be false")
	}
}

func TestStatePaused_Name(t *testing.T) {
	s := &pausedState{}
	if s.Name() != StatusPaused {
		t.Errorf("Expected %s, got %s", StatusPaused, s.Name())
	}
}

func TestStatePaused_Transitions(t *testing.T) {
	s := &pausedState{}
	if s.CanStart() {
		t.Error("Expected CanStart to be false")
	}
	if !s.CanStop() {
		t.Error("Expected CanStop to be true")
	}
	if s.CanPause() {
		t.Error("Expected CanPause to be false")
	}
	if !s.CanResume() {
		t.Error("Expected CanResume to be true")
	}
	if s.CanReconfigure() {
		t.Error("Expected CanReconfigure to be false")
	}
}

func TestStateStopped_Name(t *testing.T) {
	s := &stoppedState{}
	if s.Name() != StatusStopped {
		t.Errorf("Expected %s, got %s", StatusStopped, s.Name())
	}
}

func TestStateStopped_Transitions(t *testing.T) {
	s := &stoppedState{}
	if !s.CanStart() {
		t.Error("Expected CanStart to be true")
	}
	if s.CanStop() {
		t.Error("Expected CanStop to be false")
	}
	if s.CanPause() {
		t.Error("Expected CanPause to be false")
	}
	if s.CanResume() {
		t.Error("Expected CanResume to be false")
	}
	if !s.CanReconfigure() {
		t.Error("Expected CanReconfigure to be true")
	}
}

func TestStateReconnecting_Name(t *testing.T) {
	s := &reconnectingState{}
	if s.Name() != StatusReconnecting {
		t.Errorf("Expected %s, got %s", StatusReconnecting, s.Name())
	}
}

func TestStateReconnecting_Transitions(t *testing.T) {
	s := &reconnectingState{}
	if s.CanStart() {
		t.Error("Expected CanStart to be false")
	}
	if !s.CanStop() {
		t.Error("Expected CanStop to be true")
	}
	if s.CanPause() {
		t.Error("Expected CanPause to be false")
	}
	if s.CanResume() {
		t.Error("Expected CanResume to be false")
	}
	if s.CanReconfigure() {
		t.Error("Expected CanReconfigure to be false")
	}
}

func TestStateFromStatus_AllStates(t *testing.T) {
	tests := []struct {
		status   NodeStatus
		expected NodeState
	}{
		{StatusIdle, stateIdle},
		{StatusConnecting, stateConnecting},
		{StatusStreaming, stateStreaming},
		{StatusPaused, statePaused},
		{StatusStopped, stateStopped},
		{StatusReconnecting, stateReconnecting},
	}

	for _, tt := range tests {
		result := stateFromStatus(tt.status)
		if result != tt.expected {
			t.Errorf("stateFromStatus(%s) = %v, want %v", tt.status, result, tt.expected)
		}
	}
}

func TestStateFromStatus_Unknown(t *testing.T) {
	result := stateFromStatus(NodeStatus("unknown"))
	if result != stateIdle {
		t.Errorf("Expected stateIdle for unknown status, got %v", result)
	}
}
