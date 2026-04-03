package audio

import (
	"encoding/binary"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

// bytesToFloat32Slice is the allocating variant of bytesToFloat32SliceInto.
// Used only in tests; production code uses bytesToFloat32SliceInto instead.
func bytesToFloat32Slice(b []byte) []float32 {
	if len(b) == 0 {
		return nil
	}
	numSamples := len(b) / 4
	samples := make([]float32, numSamples)
	for i := 0; i < numSamples; i++ {
		bits := binary.LittleEndian.Uint32(b[i*4 : (i+1)*4])
		samples[i] = math.Float32frombits(bits)
	}
	return samples
}

func TestBytesToFloat32Slice_Empty(t *testing.T) {
	t.Parallel()
	result := bytesToFloat32Slice([]byte{})
	assert.Nil(t, result)
}

func TestBytesToFloat32Slice_SingleValue(t *testing.T) {
	t.Parallel()
	value := float32(0.5)
	bits := math.Float32bits(value)
	buf := make([]byte, 4)
	buf[0] = byte(bits)
	buf[1] = byte(bits >> 8)
	buf[2] = byte(bits >> 16)
	buf[3] = byte(bits >> 24)

	result := bytesToFloat32Slice(buf)
	assert.Len(t, result, 1)
	assert.InDelta(t, 0.5, result[0], 0.0001)
}

func TestBytesToFloat32Slice_MultipleValues(t *testing.T) {
	t.Parallel()
	values := []float32{0.5, -0.5, 0.0, 1.0, -1.0}
	buf := make([]byte, len(values)*4)
	for i, v := range values {
		bits := math.Float32bits(v)
		buf[i*4] = byte(bits)
		buf[i*4+1] = byte(bits >> 8)
		buf[i*4+2] = byte(bits >> 16)
		buf[i*4+3] = byte(bits >> 24)
	}

	result := bytesToFloat32Slice(buf)
	assert.Len(t, result, len(values))
	for i, v := range values {
		assert.InDelta(t, v, result[i], 0.0001, "value mismatch at index %d", i)
	}
}

func TestFloat32SliceToBytes_Empty(t *testing.T) {
	t.Parallel()
	result := float32SliceToBytes([]float32{})
	assert.Len(t, result, 0)
}

func TestFloat32SliceToBytes_SingleValue(t *testing.T) {
	t.Parallel()
	result := float32SliceToBytes([]float32{0.5})
	assert.Len(t, result, 4)
}

func TestFloat32SliceToBytes_MultipleValues(t *testing.T) {
	t.Parallel()
	values := []float32{0.5, -0.5, 0.0, 1.0, -1.0}
	result := float32SliceToBytes(values)
	assert.Len(t, result, len(values)*4)
}

func TestBytesToFloat32Slice_Float32SliceToBytes_Roundtrip(t *testing.T) {
	t.Parallel()
	original := []float32{0.5, -0.5, 0.0, 1.0, -1.0, 0.12345, -0.98765}

	encoded := float32SliceToBytes(original)
	decoded := bytesToFloat32Slice(encoded)

	assert.Len(t, decoded, len(original))
	for i, v := range original {
		assert.InDelta(t, v, decoded[i], 0.0001, "roundtrip mismatch at index %d", i)
	}
}

func TestFloat32SliceToBytes_SpecialValues(t *testing.T) {
	tests := []struct {
		name  string
		value float32
	}{
		{"zero", 0.0},
		{"positive_one", 1.0},
		{"negative_one", -1.0},
		{"small_positive", 0.0001},
		{"small_negative", -0.0001},
		{"large_positive", 0.9999},
		{"large_negative", -0.9999},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			encoded := float32SliceToBytes([]float32{tt.value})
			decoded := bytesToFloat32Slice(encoded)
			assert.InDelta(t, tt.value, decoded[0], 0.0001)
		})
	}
}

func TestBytesToFloat32SliceInto_Empty(t *testing.T) {
	t.Parallel()
	dst := make([]float32, 10)
	result := bytesToFloat32SliceInto(dst, []byte{})
	assert.Nil(t, result)
}

func TestBytesToFloat32SliceInto_SufficientCapacity(t *testing.T) {
	t.Parallel()
	values := []float32{0.5, -0.5, 0.0, 1.0, -1.0}
	buf := make([]byte, len(values)*4)
	for i, v := range values {
		bits := math.Float32bits(v)
		buf[i*4] = byte(bits)
		buf[i*4+1] = byte(bits >> 8)
		buf[i*4+2] = byte(bits >> 16)
		buf[i*4+3] = byte(bits >> 24)
	}

	dst := make([]float32, 0, 10)
	result := bytesToFloat32SliceInto(dst, buf)
	assert.Len(t, result, len(values))
	for i, v := range values {
		assert.InDelta(t, v, result[i], 0.0001, "value mismatch at index %d", i)
	}
}

func TestBytesToFloat32SliceInto_InsufficientCapacity(t *testing.T) {
	t.Parallel()
	values := []float32{0.5, -0.5, 0.0, 1.0, -1.0}
	buf := make([]byte, len(values)*4)
	for i, v := range values {
		bits := math.Float32bits(v)
		buf[i*4] = byte(bits)
		buf[i*4+1] = byte(bits >> 8)
		buf[i*4+2] = byte(bits >> 16)
		buf[i*4+3] = byte(bits >> 24)
	}

	dst := make([]float32, 0, 2)
	result := bytesToFloat32SliceInto(dst, buf)
	assert.Len(t, result, len(values))
	for i, v := range values {
		assert.InDelta(t, v, result[i], 0.0001, "value mismatch at index %d", i)
	}
}

func TestBytesToFloat32SliceInto_Roundtrip(t *testing.T) {
	t.Parallel()
	original := []float32{0.5, -0.5, 0.0, 1.0, -1.0, 0.12345, -0.98765}

	encoded := float32SliceToBytes(original)
	dst := make([]float32, 0, len(original))
	decoded := bytesToFloat32SliceInto(dst, encoded)

	assert.Len(t, decoded, len(original))
	for i, v := range original {
		assert.InDelta(t, v, decoded[i], 0.0001, "roundtrip mismatch at index %d", i)
	}
}

func BenchmarkBytesToFloat32Slice_Old(b *testing.B) {
	buf := make([]byte, 960*4)
	for i := range buf {
		buf[i] = byte(i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = bytesToFloat32Slice(buf)
	}
}

func BenchmarkBytesToFloat32Slice_New(b *testing.B) {
	buf := make([]byte, 960*4)
	for i := range buf {
		buf[i] = byte(i)
	}

	dst := make([]float32, 0, 960)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = bytesToFloat32SliceInto(dst, buf)
	}
}

func BenchmarkBytesToFloat32Slice_WithPool(b *testing.B) {
	buf := make([]byte, 960*4)
	for i := range buf {
		buf[i] = byte(i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dst := getPCMBuffer(960)
		_ = bytesToFloat32SliceInto(dst, buf)
		putPCMBuffer(dst)
	}
}
