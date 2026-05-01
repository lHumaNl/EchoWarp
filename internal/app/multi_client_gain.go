package app

func (mc *multiClient) setClientGain(gain *DeviceGainControl) {
	mc.clientGainMu.Lock()
	defer mc.clientGainMu.Unlock()
	mc.clientGain = gain
}

func (mc *multiClient) getClientGain() *DeviceGainControl {
	mc.clientGainMu.RLock()
	defer mc.clientGainMu.RUnlock()
	return mc.clientGain
}

func (mc *multiClient) setIncomingGain(gain *DeviceGainControl) {
	mc.incomingGainMu.Lock()
	defer mc.incomingGainMu.Unlock()
	mc.incomingGain = gain
}

func (mc *multiClient) getIncomingGain() *DeviceGainControl {
	mc.incomingGainMu.RLock()
	defer mc.incomingGainMu.RUnlock()
	return mc.incomingGain
}
