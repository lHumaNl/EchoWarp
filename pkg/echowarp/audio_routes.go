package echowarp

import (
	"context"
	"regexp"

	ewerrors "github.com/lHumaNl/echowarp/pkg/echowarp/errors"
)

const (
	audioRouteReceive = "receive"
	audioRouteSend    = "send"
	audioRouteAdmin   = "admin"
	audioRouteSelf    = "self"
	audioRouteMaxID   = 128
)

var audioRouteIDPattern = regexp.MustCompile(`^(\*|[A-Za-z0-9_.:-]+)$`)

// AudioRouteRule identifies one directed restriction in an independent scope.
// Muted=false removes only this exact (scope, source, recipient) restriction.
type AudioRouteRule struct {
	Scope     string `json:"scope"`
	Source    string `json:"source"`
	Recipient string `json:"recipient"`
	Muted     bool   `json:"muted"`
}

// AudioRouteState is a room-scoped snapshot, with separate reasons for denial.
type AudioRouteState struct {
	SelfID string           `json:"self_id"`
	Rules  []AudioRouteRule `json:"rules"`
}

// AudioRouteController is an optional, concurrency-safe Runner capability.
// Implementations bind self to the authenticated connection, validate room IDs
// and permissions, and honor cancellation. SetAudioRoute must wait for server
// application/acknowledgment, not merely enqueue a command. Denial in any scope
// wins; removing one rule never removes wildcard rules or another scope's rules.
// AudioRoutes returns a caller-owned snapshot; rules do not persist across rooms.
type AudioRouteController interface {
	SetAudioRoute(context.Context, AudioRouteRule) error
	AudioRoutes(context.Context) (AudioRouteState, error)
}

// SetAudioRoute validates and synchronously applies a directed audio rule.
// Receive recipients and send sources must use self; the runtime resolves it.
// A client cannot issue admin rules. Runtime authorization remains mandatory.
func (n *Node) SetAudioRoute(ctx context.Context, rule AudioRouteRule) error {
	if err := validateAudioRoute(rule); err != nil {
		return err
	}
	ctrl, mode, err := n.audioRouteController()
	if err != nil {
		return err
	}
	if mode == ModeClient && rule.Scope == audioRouteAdmin {
		return ewerrors.NewError(ewerrors.ErrAuthFailed, "Clients cannot change admin audio routes")
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		return ctxErr
	}
	return ctrl.SetAudioRoute(ctx, rule)
}

// AudioRoutes reads routing restrictions from the running controller.
// Neither this method nor SetAudioRoute holds the Node lock during callbacks.
func (n *Node) AudioRoutes(ctx context.Context) (AudioRouteState, error) {
	ctrl, _, err := n.audioRouteController()
	if err != nil {
		return AudioRouteState{}, err
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		return AudioRouteState{}, ctxErr
	}
	state, err := ctrl.AudioRoutes(ctx)
	if err != nil {
		return AudioRouteState{}, err
	}
	state.Rules = append([]AudioRouteRule{}, state.Rules...)
	return state, nil
}

func (n *Node) audioRouteController() (AudioRouteController, Mode, error) {
	n.mu.RLock()
	runner, mode, status := n.runner, n.config.Mode, n.status
	running := n.state.CanStop()
	n.mu.RUnlock()
	if runner == nil || !running {
		return nil, mode, ewerrors.NewError(ewerrors.ErrNotRunning, "Node is not running").
			WithContext("status", string(status)).
			WithSuggestion("Start the node before managing audio routes")
	}
	ctrl, ok := runner.(AudioRouteController)
	if !ok {
		return nil, mode, ewerrors.NewError(ewerrors.ErrInternalState, "Runner does not support audio routes").
			WithSuggestion("Use a runner implementing AudioRouteController")
	}
	return ctrl, mode, nil
}

func validateAudioRoute(rule AudioRouteRule) error {
	switch rule.Scope {
	case audioRouteReceive, audioRouteSend, audioRouteAdmin:
	default:
		return ewerrors.NewError(ewerrors.ErrConfigValidation, "Invalid audio route scope")
	}
	if err := validateAudioRouteID(rule.Source); err != nil {
		return err
	}
	if err := validateAudioRouteID(rule.Recipient); err != nil {
		return err
	}
	if (rule.Scope == audioRouteReceive && rule.Recipient != audioRouteSelf) ||
		(rule.Scope == audioRouteSend && rule.Source != audioRouteSelf) {
		return ewerrors.NewError(ewerrors.ErrAuthFailed, "Receive recipient and send source must be self")
	}
	return nil
}

func validateAudioRouteID(id string) error {
	if id == "" || len(id) > audioRouteMaxID || !audioRouteIDPattern.MatchString(id) {
		return ewerrors.NewError(ewerrors.ErrConfigValidation, "Invalid audio route identity").
			WithSuggestion("Use *, self, or a room ID of 1-128 ASCII letters, digits, underscores, dots, colons, or hyphens")
	}
	return nil
}
