package echowarp

// NodeState defines the interface for the State pattern implementation.
// Each state encapsulates the allowed transitions and operations for that status.
type NodeState interface {
	// Name returns the NodeStatus corresponding to this state.
	Name() NodeStatus
	// CanStart returns true if the node can be started from this state.
	CanStart() bool
	// CanStop returns true if the node can be stopped from this state.
	CanStop() bool
	// CanPause returns true if streaming can be paused from this state.
	CanPause() bool
	// CanResume returns true if streaming can be resumed from this state.
	CanResume() bool
	// CanReconfigure returns true if configuration can be changed in this state.
	CanReconfigure() bool
}

// State implementations - each represents a valid NodeStatus.

type idleState struct{}         // Node is ready to start.
type connectingState struct{}   // Node is establishing connection.
type streamingState struct{}    // Node is actively streaming audio.
type pausedState struct{}       // Streaming is temporarily suspended.
type stoppedState struct{}      // Node has been stopped.
type reconnectingState struct{} // Node is attempting to reconnect.

func (s *idleState) Name() NodeStatus         { return StatusIdle }
func (s *connectingState) Name() NodeStatus   { return StatusConnecting }
func (s *streamingState) Name() NodeStatus    { return StatusStreaming }
func (s *pausedState) Name() NodeStatus       { return StatusPaused }
func (s *stoppedState) Name() NodeStatus      { return StatusStopped }
func (s *reconnectingState) Name() NodeStatus { return StatusReconnecting }

func (s *idleState) CanStart() bool       { return true }
func (s *idleState) CanStop() bool        { return false }
func (s *idleState) CanPause() bool       { return false }
func (s *idleState) CanResume() bool      { return false }
func (s *idleState) CanReconfigure() bool { return true }

func (s *connectingState) CanStart() bool       { return false }
func (s *connectingState) CanStop() bool        { return true }
func (s *connectingState) CanPause() bool       { return false }
func (s *connectingState) CanResume() bool      { return false }
func (s *connectingState) CanReconfigure() bool { return false }

func (s *streamingState) CanStart() bool       { return false }
func (s *streamingState) CanStop() bool        { return true }
func (s *streamingState) CanPause() bool       { return true }
func (s *streamingState) CanResume() bool      { return false }
func (s *streamingState) CanReconfigure() bool { return false }

func (s *pausedState) CanStart() bool       { return false }
func (s *pausedState) CanStop() bool        { return true }
func (s *pausedState) CanPause() bool       { return false }
func (s *pausedState) CanResume() bool      { return true }
func (s *pausedState) CanReconfigure() bool { return false }

func (s *stoppedState) CanStart() bool       { return true }
func (s *stoppedState) CanStop() bool        { return false }
func (s *stoppedState) CanPause() bool       { return false }
func (s *stoppedState) CanResume() bool      { return false }
func (s *stoppedState) CanReconfigure() bool { return true }

func (s *reconnectingState) CanStart() bool       { return false }
func (s *reconnectingState) CanStop() bool        { return true }
func (s *reconnectingState) CanPause() bool       { return false }
func (s *reconnectingState) CanResume() bool      { return false }
func (s *reconnectingState) CanReconfigure() bool { return false }

var (
	stateIdle         = &idleState{}
	stateConnecting   = &connectingState{}
	stateStreaming    = &streamingState{}
	statePaused       = &pausedState{}
	stateStopped      = &stoppedState{}
	stateReconnecting = &reconnectingState{}
)

func stateFromStatus(status NodeStatus) NodeState {
	switch status {
	case StatusIdle:
		return stateIdle
	case StatusConnecting:
		return stateConnecting
	case StatusStreaming:
		return stateStreaming
	case StatusPaused:
		return statePaused
	case StatusStopped:
		return stateStopped
	case StatusReconnecting:
		return stateReconnecting
	default:
		return stateIdle
	}
}
