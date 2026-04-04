//go:build !amd64 || !gc || noasm

package audio

func mixAccumulateAVX(dst, src []float32) {
	mixAccumulatePure(dst, src)
}

func mixGainAVX(dst []float32, gain float32) {
	mixGainPure(dst, gain)
}

func mixTanhAVX(dst []float32) {
	mixTanhPure(dst)
}

func mixAccumulateSSE(dst, src []float32) {
	mixAccumulatePure(dst, src)
}

func mixGainSSE(dst []float32, gain float32) {
	mixGainPure(dst, gain)
}

func mixTanhSSE(dst []float32) {
	mixTanhPure(dst)
}
