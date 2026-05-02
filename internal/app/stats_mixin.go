package app

import "github.com/lHumaNl/echowarp/pkg/echowarp/transport"

// StatsMixin embeds stats publishing fields and methods shared by ServerApp
// and ClientApp. Both structs embed this type so the duplicated
// publishStats / sendDisconnected helpers live in one place.
type StatsMixin struct {
	statsCh   chan<- transport.ConnectionStats
	statsHook func(transport.ConnectionStats)
}

// publishStats forwards a ConnectionStats snapshot to both the TUI statsCh
// (if configured) and the API statsHook (if configured). Non-blocking for
// the channel sink — the fallback drops the value when the TUI is not
// draining, matching the original reportStats behavior.
func (st *StatsMixin) publishStats(stats transport.ConnectionStats) {
	if st.statsCh != nil {
		select {
		case st.statsCh <- stats:
		default:
		}
	}
	if st.statsHook != nil {
		st.statsHook(stats)
	}
}

// sendDisconnected sends a final "disconnected" stats update to all sinks.
func (st *StatsMixin) sendDisconnected() {
	if st.statsCh == nil && st.statsHook == nil {
		return
	}
	st.publishStats(transport.ConnectionStats{State: "disconnected"})
}
