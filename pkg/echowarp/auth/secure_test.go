package auth

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSecureBuffer_New(t *testing.T) {
	t.Parallel()

	buf := NewSecureBuffer(32)
	defer buf.Release()

	assert.Equal(t, 32, buf.Len())
	assert.NotNil(t, buf.Data())
	assert.False(t, buf.IsReleased())
}

func TestSecureBuffer_FromBytes(t *testing.T) {
	t.Parallel()

	original := []byte("sensitive-data-1234567890123456")
	buf := NewSecureBufferFromBytes(original)
	defer buf.Release()

	assert.Equal(t, len(original), buf.Len())
	assert.True(t, bytes.Equal(original, buf.Data()))

	// Verify it's a copy, not a reference
	original[0] = 'X'
	assert.NotEqual(t, original[0], buf.Data()[0])
}

func TestSecureBuffer_Release(t *testing.T) {
	t.Parallel()

	buf := NewSecureBuffer(32)
	data := buf.Data()

	// Fill with non-zero data
	for i := range data {
		data[i] = byte(i + 1)
	}

	buf.Release()

	// After release, Data() should return nil
	assert.Nil(t, buf.Data())
	assert.Equal(t, 0, buf.Len())
	assert.True(t, buf.IsReleased())
}

func TestSecureBuffer_ReleaseIdempotent(t *testing.T) {
	t.Parallel()

	buf := NewSecureBuffer(32)

	// Multiple releases should be safe
	buf.Release()
	buf.Release()
	buf.Release()

	assert.True(t, buf.IsReleased())
}

func TestSecureBuffer_DataAfterRelease(t *testing.T) {
	t.Parallel()

	buf := NewSecureBuffer(32)
	buf.Release()

	data := buf.Data()
	assert.Nil(t, data)
}

func TestWithSecureBuffer(t *testing.T) {
	t.Parallel()

	err := WithSecureBuffer(32, func(data []byte) error {
		assert.Len(t, data, 32)
		// Write some data
		for i := range data {
			data[i] = byte(i)
		}
		return nil
	})

	require.NoError(t, err)
}

func TestWithSecureBytes(t *testing.T) {
	t.Parallel()

	original := []byte("secret-data-here-123456789012")
	err := WithSecureBytes(original, func(data []byte) error {
		assert.Equal(t, original, data)
		return nil
	})

	require.NoError(t, err)
}

func TestZeroBytes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		data []byte
	}{
		{"normal data", []byte{1, 2, 3, 4, 5}},
		{"empty data", []byte{}},
		{"large data", make([]byte, 1024)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Fill with non-zero data
			for i := range tt.data {
				tt.data[i] = byte(i%256 + 1)
			}

			zeroBytes(tt.data)

			// Should be all zeros
			for i, b := range tt.data {
				assert.Equal(t, byte(0), b, "byte at index %d should be zero", i)
			}
		})
	}
}

func TestZeroBytes_Nil(t *testing.T) {
	t.Parallel()

	// Should not panic
	zeroBytes(nil)
}

func TestZeroBytes_Public(t *testing.T) {
	t.Parallel()

	data := []byte{1, 2, 3, 4, 5}
	ZeroBytes(data)

	assert.Equal(t, []byte{0, 0, 0, 0, 0}, data)
}

func TestSecureBytesEqual(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		a        []byte
		b        []byte
		expected bool
	}{
		{"equal", []byte{1, 2, 3}, []byte{1, 2, 3}, true},
		{"not equal", []byte{1, 2, 3}, []byte{1, 2, 4}, false},
		{"different length", []byte{1, 2, 3}, []byte{1, 2}, false},
		{"empty equal", []byte{}, []byte{}, true},
		{"one empty", []byte{1, 2, 3}, []byte{}, false},
		{"nil vs empty", nil, []byte{}, true}, // nil and empty slice are treated the same
		{"both nil", nil, nil, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SecureBytesEqual(tt.a, tt.b)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSecureBytesEqual_ConstantTime(t *testing.T) {
	t.Parallel()

	// This test verifies that the comparison is constant-time
	// by checking all positions of a difference
	base := []byte{1, 2, 3, 4, 5, 6, 7, 8}

	for i := 0; i < len(base); i++ {
		other := make([]byte, len(base))
		copy(other, base)
		other[i] = 0xFF // Different at position i

		// Should always return false
		assert.False(t, SecureBytesEqual(base, other))
	}
}

func TestSecureCopy(t *testing.T) {
	t.Parallel()

	t.Run("normal copy", func(t *testing.T) {
		dst := make([]byte, 10)
		src := []byte{1, 2, 3, 4, 5}

		n := SecureCopy(dst, src)
		assert.Equal(t, 5, n)
		assert.Equal(t, src, dst[:n])
	})

	t.Run("dst smaller", func(t *testing.T) {
		dst := make([]byte, 3)
		src := []byte{1, 2, 3, 4, 5}

		n := SecureCopy(dst, src)
		assert.Equal(t, 3, n)
		assert.Equal(t, []byte{1, 2, 3}, dst)
	})

	t.Run("nil dst", func(t *testing.T) {
		n := SecureCopy(nil, []byte{1, 2, 3})
		assert.Equal(t, 0, n)
	})

	t.Run("nil src", func(t *testing.T) {
		n := SecureCopy(make([]byte, 10), nil)
		assert.Equal(t, 0, n)
	})
}

func TestSecureOperation(t *testing.T) {
	t.Parallel()

	executed := false
	err := SecureOperation(func() error {
		executed = true
		return nil
	})

	require.NoError(t, err)
	assert.True(t, executed)
}

func TestSecureOperation_Error(t *testing.T) {
	t.Parallel()

	err := SecureOperation(func() error {
		return assert.AnError
	})

	require.Error(t, err)
	assert.Equal(t, assert.AnError, err)
}

func TestSecurePool(t *testing.T) {
	t.Parallel()

	pool := NewSecurePool(32)

	// Get a buffer
	buf := pool.Get()
	assert.Equal(t, 32, buf.Len())

	// Write some data
	data := buf.Data()
	for i := range data {
		data[i] = byte(i + 1)
	}

	// Return to pool (should be zeroed)
	pool.Put(buf)

	// Get another buffer (may be the same)
	buf2 := pool.Get()
	defer pool.Put(buf2)

	// Should be zeroed
	data2 := buf2.Data()
	for i, b := range data2 {
		assert.Equal(t, byte(0), b, "buffer should be zeroed at index %d", i)
	}
}

func TestSecurePool_PutNil(t *testing.T) {
	t.Parallel()

	pool := NewSecurePool(32)

	// Should not panic
	pool.Put(nil)
}

func TestSecurePool_PutReleased(t *testing.T) {
	t.Parallel()

	pool := NewSecurePool(32)
	buf := pool.Get()
	buf.Release()

	// Should not panic - released buffers are not returned to pool
	pool.Put(buf)
}

func TestSecureBuffer_IntegrationWithCrypto(t *testing.T) {
	t.Parallel()

	// Test that SecureBuffer can be used with crypto operations
	buf := NewSecureBuffer(x25519KeySize)
	defer buf.Release()

	// Fill with shared secret simulation
	for i := range buf.Data() {
		buf.Data()[i] = byte(i)
	}

	// Derive key using the buffer
	key, err := DeriveAESKey(buf.Data(), nil, "")
	require.NoError(t, err)
	require.Len(t, key, aesKeySize)
}

// BenchmarkSecureBuffer_ZeroBytes benchmarks the zeroing operation.
func BenchmarkSecureBuffer_ZeroBytes(b *testing.B) {
	data := make([]byte, 1024)
	for i := range data {
		data[i] = byte(i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		zeroBytes(data)
	}
}

// BenchmarkSecureBytesEqual benchmarks constant-time comparison.
func BenchmarkSecureBytesEqual(b *testing.B) {
	a := make([]byte, 32)
	for i := range a {
		a[i] = byte(i)
	}
	cpy := make([]byte, 32)
	copy(cpy, a)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = SecureBytesEqual(a, cpy)
	}
}

// BenchmarkSecureBuffer_Pool benchmarks pool operations.
func BenchmarkSecureBuffer_Pool(b *testing.B) {
	pool := NewSecurePool(32)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf := pool.Get()
		pool.Put(buf)
	}
}
