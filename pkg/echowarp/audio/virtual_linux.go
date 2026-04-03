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

var virtualMicBufPool = sync.Pool{
	New: func() interface{} {
		buf := make([]byte, 0, defaultPCMBufferSize*4)
		return buf
	},
}

type linuxVirtualMic struct {
	name       string
	sampleRate uint32
	channels   uint32
	fifoPath   string
	fifo       *os.File
	moduleIdx  string
}

func newLinuxVirtualMic(name string, sampleRate, channels uint32) (*linuxVirtualMic, error) {
	fifoPath := filepath.Join(os.TempDir(), fmt.Sprintf("echowarp-%s.pcm", name))

	if err := syscall.Mkfifo(fifoPath, 0600); err != nil && !os.IsExist(err) {
		return nil, fmt.Errorf("create FIFO: %w", err)
	}

	format := "float32le"
	cmd := exec.Command("pactl", "load-module", "module-pipe-source",
		fmt.Sprintf("source_name=%s", name),
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

	fifo, err := os.OpenFile(fifoPath, os.O_WRONLY, 0)
	if err != nil {
		exec.Command("pactl", "unload-module", moduleIdx).Run()
		os.Remove(fifoPath)
		return nil, fmt.Errorf("open FIFO: %w", err)
	}

	return &linuxVirtualMic{
		name:       name,
		sampleRate: sampleRate,
		channels:   channels,
		fifoPath:   fifoPath,
		fifo:       fifo,
		moduleIdx:  moduleIdx,
	}, nil
}

func (v *linuxVirtualMic) Write(samples []float32) error {
	needed := len(samples) * 4
	buf := virtualMicBufPool.Get().([]byte)
	if cap(buf) < needed {
		buf = make([]byte, needed)
	} else {
		buf = buf[:needed]
	}
	for i, s := range samples {
		bits := math.Float32bits(s)
		binary.LittleEndian.PutUint32(buf[i*4:], bits)
	}
	_, err := v.fifo.Write(buf)
	virtualMicBufPool.Put(buf[:0])
	return err
}

func (v *linuxVirtualMic) DeviceName() string {
	return v.name
}

func (v *linuxVirtualMic) Close() error {
	if v.fifo != nil {
		v.fifo.Close()
	}
	if v.moduleIdx != "" {
		exec.Command("pactl", "unload-module", v.moduleIdx).Run()
	}
	os.Remove(v.fifoPath)
	return nil
}
