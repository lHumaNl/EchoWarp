package app

import "context"

func (m *ServerMonitorMixer) setMonitorContext(ctx context.Context) {
	m.lifecycleMu.Lock()
	m.monitorCtx = ctx
	m.lifecycleMu.Unlock()
}

func (m *ServerMonitorMixer) sourceContext(clientCtx context.Context) context.Context {
	monitorCtx := m.currentMonitorContext()
	if monitorCtx == nil {
		return clientCtx
	}
	return mergeMonitorSourceContext(clientCtx, monitorCtx)
}

func (m *ServerMonitorMixer) currentMonitorContext() context.Context {
	m.lifecycleMu.RLock()
	defer m.lifecycleMu.RUnlock()
	return m.monitorCtx
}

func mergeMonitorSourceContext(clientCtx, monitorCtx context.Context) context.Context {
	sourceCtx, cancel := context.WithCancel(clientCtx)
	go cancelMonitorSourceOnDone(sourceCtx, monitorCtx, cancel)
	return sourceCtx
}

func cancelMonitorSourceOnDone(sourceCtx, monitorCtx context.Context, cancel context.CancelFunc) {
	select {
	case <-sourceCtx.Done():
	case <-monitorCtx.Done():
		cancel()
	}
}
