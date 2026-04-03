//go:build linux

package audio

import (
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
)

var virtualSpeakerBufPool = sync.Pool{
	New: func() interface{} {
		buf := make([]float32, 0, defaultPCMBufferSize)
		return buf
	},
}

type linuxVirtualSpeaker struct {
	name       string
	sampleRate uint32
	channels   uint32
	fifoPath   string
	fifo       *os.File
	moduleIdx  string
	mu         sync.Mutex
}

func newLinuxVirtualSpeaker(name string, sampleRate, channels uint32) (*linuxVirtualSpeaker, error) {
	fifoPath := filepath.Join(os.TempDir(), fmt.Sprintf("echowarp-%s-sink.pcm", name))

	if err := syscall.Mkfifo(fifoPath, 0600); err != nil && !os.IsExist(err) {
		return nil, fmt.Errorf("create FIFO: %w", err)
	}

	format := "float32le"
	// Create a null sink that apps can output to, with a monitor we can read from
	cmd := exec.Command("pactl", "load-module", "module-pipe-sink",
		fmt.Sprintf("sink_name=%s", name),
		fmt.Sprintf("file=%s", fifoPath),
		fmt.Sprintf("format=%s", format),
		fmt.Sprintf("rate=%d", sampleRate),
		fmt.Sprintf("channels=%d", channels),
	)
	output, err := cmd.Output()
	if err != nil {
		os.Remove(fifoPath)
		return nil, fmt.Errorf("load PulseAudio module: %w", err)
	}
	moduleIdx := string(output)
	for i := 0; i < len(moduleIdx); i++ {
		if moduleIdx[i] == '\n' {
			moduleIdx = moduleIdx[:i]
			break
		}
	}

	fifo, err := os.OpenFile(fifoPath, os.O_RDONLY, 0)
	if err != nil {
		exec.Command("pactl", "unload-module", moduleIdx).Run()
		os.Remove(fifoPath)
		return nil, fmt.Errorf("open FIFO: %w", err)
	}

	return &linuxVirtualSpeaker{
		name:       name,
		sampleRate: sampleRate,
		channels:   channels,
		fifoPath:   fifoPath,
		fifo:       fifo,
		moduleIdx:  moduleIdx,
	}, nil
}

func (v *linuxVirtualSpeaker) Read() ([]float32, error) {
	// Read one frame of audio (20ms at configured sample rate)
	samplesPerFrame := int(v.sampleRate) * int(v.channels) * 20 / 1000
	buf := make([]byte, samplesPerFrame*4)

	n, err := v.fifo.Read(buf)
	if err != nil {
		return nil, err
	}

	numSamples := n / 4
	samples := virtualSpeakerBufPool.Get().([]float32)
	if cap(samples) < numSamples {
		samples = make([]float32, numSamples)
	} else {
		samples = samples[:numSamples]
	}

	for i := 0; i < numSamples; i++ {
		bits := binary.LittleEndian.Uint32(buf[i*4:])
		samples[i] = math.Float32frombits(bits)
	}

	return samples, nil
}

func (v *linuxVirtualSpeaker) DeviceName() string {
	return v.name
}

func (v *linuxVirtualSpeaker) Close() error {
	if v.fifo != nil {
		v.fifo.Close()
	}
	if v.moduleIdx != "" {
		exec.Command("pactl", "unload-module", v.moduleIdx).Run()
	}
	os.Remove(v.fifoPath)
	return nil
}
