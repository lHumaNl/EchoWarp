package audio

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// RecordingMode specifies what to record.
type RecordingMode int

const (
	// RecordMix records the total mix into a single file.
	RecordMix RecordingMode = iota
	// RecordTracks records each participant into a separate file.
	RecordTracks
	// RecordBoth records both the mix and separate tracks.
	RecordBoth
)

const (
	// maxWAVDataSize is the threshold at which a WAV file is rotated (~3.5 GB).
	maxWAVDataSize = 3_500_000_000
	// bufioSize is the buffer size for the buffered writer.
	bufioSize = 32 * 1024
)

// convBufPool is a sync.Pool for byte buffers used in WriteSamples conversion.
var convBufPool = sync.Pool{
	New: func() any {
		b := make([]byte, 0, 8192)
		return &b
	},
}

// invalidFilenameChars contains characters that are invalid in filenames.
const invalidFilenameChars = `/\:*?"<>|`

// SanitizeFilename replaces characters invalid for filesystems with '_',
// trims spaces, and limits the result to 64 characters.
func SanitizeFilename(name string) string {
	var b strings.Builder
	b.Grow(len(name))
	for _, r := range name {
		if r == 0 || strings.ContainsRune(invalidFilenameChars, r) {
			b.WriteByte('_')
		} else {
			b.WriteRune(r)
		}
	}
	result := strings.TrimSpace(b.String())
	if len(result) > 64 {
		result = result[:64]
	}
	return result
}

// WAVWriter writes PCM audio samples to a WAV file.
// It writes the header on Close when the final size is known.
// Supports file rotation at ~3.5 GB and periodic header flushing for crash safety.
type WAVWriter struct {
	mu            sync.Mutex
	f             *os.File
	bw            *bufio.Writer
	basePath      string
	sampleRate    uint32
	channels      uint16
	dataSize      uint32 // current part data size
	totalDataSize uint64 // cumulative across all parts
	partNum       int
	closed        bool
}

// NewWAVWriter creates a new WAV file at path.
func NewWAVWriter(path string, sampleRate uint32, channels uint16) (*WAVWriter, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return nil, fmt.Errorf("create recording dir: %w", err)
	}

	f, err := os.Create(path)
	if err != nil {
		return nil, fmt.Errorf("create WAV file: %w", err)
	}

	bw := bufio.NewWriterSize(f, bufioSize)

	w := &WAVWriter{
		f:          f,
		bw:         bw,
		basePath:   path,
		sampleRate: sampleRate,
		channels:   channels,
		partNum:    0,
	}

	// Write placeholder header (44 bytes), will be overwritten on Close.
	header := make([]byte, 44)
	if _, err := bw.Write(header); err != nil {
		_ = f.Close() //nolint:errcheck
		return nil, fmt.Errorf("write WAV header placeholder: %w", err)
	}

	return w, nil
}

// partPath returns the file path for the given part number.
// Part 0 uses the original path; subsequent parts get _partN suffix.
func (w *WAVWriter) partPath(partNum int) string {
	if partNum == 0 {
		return w.basePath
	}
	ext := filepath.Ext(w.basePath)
	base := strings.TrimSuffix(w.basePath, ext)
	return fmt.Sprintf("%s_part%d%s", base, partNum, ext)
}

// rotate closes the current file part and opens a new one.
// Must be called with w.mu held.
func (w *WAVWriter) rotate() error {
	// Flush bufio
	if err := w.bw.Flush(); err != nil {
		return fmt.Errorf("flush before rotation: %w", err)
	}

	// Write final header for current part
	if _, err := w.f.Seek(0, io.SeekStart); err != nil {
		_ = w.f.Close() //nolint:errcheck
		return fmt.Errorf("seek WAV header for rotation: %w", err)
	}
	header := buildWAVHeader(w.sampleRate, w.channels, w.dataSize)
	if _, err := w.f.Write(header); err != nil {
		_ = w.f.Close() //nolint:errcheck
		return fmt.Errorf("write WAV header for rotation: %w", err)
	}
	if err := w.f.Close(); err != nil {
		return fmt.Errorf("close WAV part for rotation: %w", err)
	}

	// Accumulate and reset
	w.totalDataSize += uint64(w.dataSize)
	w.dataSize = 0
	w.partNum++

	// Open new part
	newPath := w.partPath(w.partNum)
	f, err := os.Create(newPath)
	if err != nil {
		return fmt.Errorf("create WAV part %d: %w", w.partNum, err)
	}
	w.f = f
	w.bw = bufio.NewWriterSize(f, bufioSize)

	// Write placeholder header
	placeholder := make([]byte, 44)
	if _, err := w.bw.Write(placeholder); err != nil {
		_ = f.Close() //nolint:errcheck
		w.f = nil
		w.bw = nil
		return fmt.Errorf("write WAV header placeholder for part %d: %w", w.partNum, err)
	}

	return nil
}

// WriteSamples writes float32 PCM samples (range [-1.0, 1.0]) as 16-bit PCM.
func (w *WAVWriter) WriteSamples(samples []float32) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return fmt.Errorf("WAV writer is closed")
	}
	if w.f == nil {
		return fmt.Errorf("WAV writer file is nil (previous rotation failed)")
	}

	needed := len(samples) * 2

	// Check if writing would exceed the max WAV data size, rotate if needed.
	if uint64(w.dataSize)+uint64(needed) > maxWAVDataSize {
		if err := w.rotate(); err != nil {
			return err
		}
	}

	// Get buffer from pool
	bp := convBufPool.Get().(*[]byte) //nolint:errcheck
	buf := *bp
	if cap(buf) < needed {
		buf = make([]byte, needed)
	} else {
		buf = buf[:needed]
	}

	for i, s := range samples {
		// Clamp to [-1.0, 1.0]
		if s > 1.0 {
			s = 1.0
		} else if s < -1.0 {
			s = -1.0
		}
		val := int16(s * 32767)
		binary.LittleEndian.PutUint16(buf[i*2:], uint16(val))
	}

	n, err := w.bw.Write(buf)
	if err != nil {
		*bp = buf
		convBufPool.Put(bp)
		return fmt.Errorf("write WAV data: %w", err)
	}
	w.dataSize += uint32(n)

	*bp = buf
	convBufPool.Put(bp)
	return nil
}

// FlushHeader seeks to the beginning, writes the current WAV header with
// the current dataSize, then seeks back to the end and flushes the bufio writer.
// This makes the file readable even after a crash. Should be called periodically
// (e.g. every ~5 seconds) from external code.
func (w *WAVWriter) FlushHeader() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return nil
	}

	// Flush buffered data first so file position is accurate
	if err := w.bw.Flush(); err != nil {
		return fmt.Errorf("flush bufio before header write: %w", err)
	}

	// Seek to beginning and write header
	if _, err := w.f.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("seek to header: %w", err)
	}
	header := buildWAVHeader(w.sampleRate, w.channels, w.dataSize)
	if _, err := w.f.Write(header); err != nil {
		return fmt.Errorf("write WAV header: %w", err)
	}

	// Seek back to end
	if _, err := w.f.Seek(0, io.SeekEnd); err != nil {
		return fmt.Errorf("seek to end: %w", err)
	}

	// Re-wrap bufio around the file at the new position
	w.bw.Reset(w.f)

	return nil
}

// Close finalizes the WAV header and closes the file.
func (w *WAVWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return nil
	}
	w.closed = true

	// Flush buffered data
	if err := w.bw.Flush(); err != nil {
		_ = w.f.Close() //nolint:errcheck
		return fmt.Errorf("flush bufio on close: %w", err)
	}

	// Seek to beginning and write proper header
	if _, err := w.f.Seek(0, io.SeekStart); err != nil {
		_ = w.f.Close() //nolint:errcheck
		return fmt.Errorf("seek WAV header: %w", err)
	}

	header := buildWAVHeader(w.sampleRate, w.channels, w.dataSize)
	if _, err := w.f.Write(header); err != nil {
		_ = w.f.Close() //nolint:errcheck
		return fmt.Errorf("write WAV header: %w", err)
	}

	// Accumulate final part
	w.totalDataSize += uint64(w.dataSize)

	return w.f.Close()
}

// DataSize returns the cumulative number of bytes of audio data written across all parts,
// capped to uint32 for backward compatibility. Use TotalDataSize for the true value.
func (w *WAVWriter) DataSize() uint32 {
	w.mu.Lock()
	defer w.mu.Unlock()
	total := w.totalDataSize + uint64(w.dataSize)
	if total > 0xFFFFFFFF {
		return 0xFFFFFFFF
	}
	return uint32(total)
}

// TotalDataSize returns the exact cumulative number of bytes written across all parts.
func (w *WAVWriter) TotalDataSize() uint64 {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.totalDataSize + uint64(w.dataSize)
}

func buildWAVHeader(sampleRate uint32, channels uint16, dataSize uint32) []byte {
	bitsPerSample := uint16(16)
	byteRate := sampleRate * uint32(channels) * uint32(bitsPerSample/8)
	blockAlign := channels * (bitsPerSample / 8)

	h := make([]byte, 44)
	copy(h[0:4], "RIFF")
	binary.LittleEndian.PutUint32(h[4:8], 36+dataSize)
	copy(h[8:12], "WAVE")
	copy(h[12:16], "fmt ")
	binary.LittleEndian.PutUint32(h[16:20], 16) // PCM format chunk size
	binary.LittleEndian.PutUint16(h[20:22], 1)  // PCM format
	binary.LittleEndian.PutUint16(h[22:24], channels)
	binary.LittleEndian.PutUint32(h[24:28], sampleRate)
	binary.LittleEndian.PutUint32(h[28:32], byteRate)
	binary.LittleEndian.PutUint16(h[32:34], blockAlign)
	binary.LittleEndian.PutUint16(h[34:36], bitsPerSample)
	copy(h[36:40], "data")
	binary.LittleEndian.PutUint32(h[40:44], dataSize)
	return h
}

// ConferenceRecorder manages recording of conference audio in multiple modes.
type ConferenceRecorder struct {
	mu         sync.Mutex
	mode       RecordingMode
	dir        string
	sampleRate uint32
	channels   uint16

	mixWriter    *WAVWriter
	trackWriters map[string]*WAVWriter

	startTime      time.Time
	active         bool
	writeErrors    int
	maxWriteErrors int
	stopReason     string
}

// NewConferenceRecorder creates a recorder. Call Start() to begin recording.
func NewConferenceRecorder(mode RecordingMode, sampleRate uint32, channels uint16) *ConferenceRecorder {
	return &ConferenceRecorder{
		mode:           mode,
		sampleRate:     sampleRate,
		channels:       channels,
		maxWriteErrors: 3,
	}
}

// Start begins recording into a timestamped directory under baseDir.
func (r *ConferenceRecorder) Start(baseDir string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.active {
		return fmt.Errorf("recording already active")
	}

	ts := time.Now().Format("2006-01-02_15-04-05")
	r.dir = filepath.Join(baseDir, ts)

	if r.mode == RecordMix || r.mode == RecordBoth {
		w, err := NewWAVWriter(filepath.Join(r.dir, "mix.wav"), r.sampleRate, r.channels)
		if err != nil {
			return err
		}
		r.mixWriter = w
	}

	r.trackWriters = make(map[string]*WAVWriter)
	r.startTime = time.Now()
	r.active = true
	r.writeErrors = 0
	r.stopReason = ""
	return nil
}

// handleWriteError increments the error counter and auto-stops after maxWriteErrors.
// Must be called with r.mu held.
func (r *ConferenceRecorder) handleWriteError(err error) {
	r.writeErrors++
	if r.writeErrors >= r.maxWriteErrors {
		r.active = false
		r.stopReason = fmt.Sprintf("auto-stopped after %d consecutive write errors: %v", r.writeErrors, err)
	}
}

// handleWriteSuccess resets the consecutive error counter.
// Must be called with r.mu held.
func (r *ConferenceRecorder) handleWriteSuccess() {
	r.writeErrors = 0
}

// WriteMix writes samples to the mix file (if recording mix).
func (r *ConferenceRecorder) WriteMix(samples []float32) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.active || r.mixWriter == nil {
		return nil
	}
	err := r.mixWriter.WriteSamples(samples)
	if err != nil {
		r.handleWriteError(err)
		return err
	}
	r.handleWriteSuccess()
	return nil
}

// maxSilencePadding limits silence padding to 1 hour to prevent excessive blocking.
const maxSilencePadding = time.Hour

// writeSilencePadding writes zero samples to w covering the given elapsed duration.
func (r *ConferenceRecorder) writeSilencePadding(w *WAVWriter, elapsed time.Duration) error {
	// Skip padding for durations shorter than one audio frame (20ms).
	if elapsed < 20*time.Millisecond {
		return nil
	}
	// Cap to prevent unbounded blocking under the lock.
	if elapsed > maxSilencePadding {
		elapsed = maxSilencePadding
	}
	numSamples := int(elapsed.Seconds() * float64(r.sampleRate) * float64(r.channels))
	if numSamples <= 0 {
		return nil
	}
	// Write in chunks to avoid huge allocations
	const chunkSize = 48000 // ~0.5s at 48kHz mono
	silence := make([]float32, chunkSize)
	for numSamples > 0 {
		n := numSamples
		if n > chunkSize {
			n = chunkSize
		}
		if err := w.WriteSamples(silence[:n]); err != nil {
			return err
		}
		numSamples -= n
	}
	return nil
}

// WriteTrack writes samples for a specific participant track.
func (r *ConferenceRecorder) WriteTrack(participantID string, samples []float32) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.active {
		return nil
	}
	if r.mode != RecordTracks && r.mode != RecordBoth {
		return nil
	}

	w, ok := r.trackWriters[participantID]
	if !ok {
		safeName := SanitizeFilename(participantID) + ".wav"
		var err error
		w, err = NewWAVWriter(filepath.Join(r.dir, safeName), r.sampleRate, r.channels)
		if err != nil {
			r.handleWriteError(err)
			return err
		}
		r.trackWriters[participantID] = w

		// Silence padding for late joiners
		elapsed := time.Since(r.startTime)
		if err := r.writeSilencePadding(w, elapsed); err != nil {
			r.handleWriteError(err)
			return err
		}
	}

	err := w.WriteSamples(samples)
	if err != nil {
		r.handleWriteError(err)
		return err
	}
	r.handleWriteSuccess()
	return nil
}

// Stop stops recording and closes all files. Returns duration and total size.
func (r *ConferenceRecorder) Stop() (duration time.Duration, totalSize uint64, fileCount int, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	wasActive := r.active
	r.active = false

	// If already stopped (e.g. auto-stop), still close any open writers.
	if !wasActive && r.mixWriter == nil && r.trackWriters == nil {
		return 0, 0, 0, nil
	}
	duration = time.Since(r.startTime)

	if r.mixWriter != nil {
		totalSize += r.mixWriter.TotalDataSize()
		fileCount++
		if e := r.mixWriter.Close(); e != nil && err == nil {
			err = e
		}
		r.mixWriter = nil
	}

	for _, w := range r.trackWriters {
		totalSize += w.TotalDataSize()
		fileCount++
		if e := w.Close(); e != nil && err == nil {
			err = e
		}
	}
	r.trackWriters = nil

	return duration, totalSize, fileCount, err
}

// IsActive returns whether recording is in progress.
func (r *ConferenceRecorder) IsActive() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.active
}

// Dir returns the recording output directory.
func (r *ConferenceRecorder) Dir() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.dir
}

// FlushHeaders flushes WAV headers on all active writers for crash safety.
func (r *ConferenceRecorder) FlushHeaders() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.active {
		return
	}
	if r.mixWriter != nil {
		_ = r.mixWriter.FlushHeader() //nolint:errcheck
	}
	for _, w := range r.trackWriters {
		_ = w.FlushHeader() //nolint:errcheck
	}
}

// StopReason returns the reason recording was auto-stopped, or empty string.
func (r *ConferenceRecorder) StopReason() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.stopReason
}

// StartTime returns the time recording started.
func (r *ConferenceRecorder) StartTime() time.Time {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.startTime
}

// Duration returns the elapsed recording time, or 0 if not active.
func (r *ConferenceRecorder) Duration() time.Duration {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.active {
		return 0
	}
	return time.Since(r.startTime)
}
