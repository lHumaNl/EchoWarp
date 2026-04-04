// Package auth provides authentication mechanisms for EchoWarp.
// This file implements secure memory handling for sensitive data.
package auth

import (
	"runtime"
	"sync"
)

// SecureBuffer wraps a byte slice with secure memory handling.
// It provides memory locking (where supported), zeroing on release,
// and protection against compiler optimizations that might remove zeroing.
//
// Security features:
//   - Memory zeroing on release
//   - runtime.KeepAlive to prevent premature garbage collection
//   - runtime.LockOSThread for operations on sensitive data
//   - Optional mlock on supported platforms (Unix-like systems)
//
// Usage:
//
//	buf := NewSecureBuffer(32)
//	defer buf.Release()
//	// Use buf.Data() for sensitive operations
//	// Memory is zeroed when Release() is called
type SecureBuffer struct {
	data     []byte
	locked   bool
	released bool
	mu       sync.Mutex
}

// NewSecureBuffer creates a new SecureBuffer of the specified size.
// The buffer is initialized with zeros and attempts to lock memory
// if the platform supports it.
func NewSecureBuffer(size int) *SecureBuffer {
	buf := &SecureBuffer{
		data: make([]byte, size),
	}

	// Attempt to lock memory (platform-specific)
	buf.locked = lockMemory(buf.data)

	return buf
}

// NewSecureBufferFromBytes creates a SecureBuffer from existing data.
// The data is copied to prevent references to the original slice.
func NewSecureBufferFromBytes(data []byte) *SecureBuffer {
	buf := NewSecureBuffer(len(data))
	copy(buf.data, data)
	return buf
}

// Data returns the underlying byte slice.
// DO NOT retain references to this slice beyond the buffer's lifetime.
func (sb *SecureBuffer) Data() []byte {
	sb.mu.Lock()
	defer sb.mu.Unlock()

	if sb.released {
		return nil
	}

	runtime.KeepAlive(sb)
	return sb.data
}

// Len returns the length of the buffer.
func (sb *SecureBuffer) Len() int {
	sb.mu.Lock()
	defer sb.mu.Unlock()

	if sb.released {
		return 0
	}
	return len(sb.data)
}

// Release securely zeroes and releases the buffer.
// This method should be called when the buffer is no longer needed.
// After Release, Data() returns nil and Len() returns 0.
func (sb *SecureBuffer) Release() {
	sb.mu.Lock()
	defer sb.mu.Unlock()

	if sb.released {
		return
	}

	// Zero the memory
	zeroBytes(sb.data)

	// Unlock memory if it was locked
	if sb.locked {
		unlockMemory(sb.data)
	}

	// Clear reference
	sb.data = nil
	sb.released = true
}

// IsReleased returns true if the buffer has been released.
func (sb *SecureBuffer) IsReleased() bool {
	sb.mu.Lock()
	defer sb.mu.Unlock()
	return sb.released
}

// WithSecureBuffer creates a secure buffer, executes the function,
// and releases the buffer automatically.
//
// Example:
//
//	err := WithSecureBuffer(32, func(data []byte) error {
//	    // Use data for sensitive operations
//	    copy(data, secret)
//	    return nil
//	})
func WithSecureBuffer(size int, fn func([]byte) error) error {
	buf := NewSecureBuffer(size)
	defer buf.Release()
	return fn(buf.Data())
}

// WithSecureBytes creates a secure buffer from existing data, executes
// the function, and releases the buffer automatically.
func WithSecureBytes(data []byte, fn func([]byte) error) error {
	buf := NewSecureBufferFromBytes(data)
	defer buf.Release()
	return fn(buf.Data())
}

// zeroBytes securely zeroes a byte slice in memory.
// Uses runtime.KeepAlive to prevent compiler optimizations from removing the zeroing.
func zeroBytes(b []byte) {
	if b == nil {
		return
	}

	// Lock OS thread to ensure the zeroing happens on a predictable goroutine
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	for i := range b {
		b[i] = 0
	}

	// Ensure the write is not optimized away
	runtime.KeepAlive(b)
}

// ZeroBytes is the public version of zeroBytes for external use.
func ZeroBytes(b []byte) {
	zeroBytes(b)
}

// SecureOperation executes a function with runtime.LockOSThread to ensure
// the operation runs on a dedicated OS thread. This is useful for operations
// that involve sensitive data and may benefit from thread locality.
//
// Example:
//
//	err := SecureOperation(func() error {
//	    // Sensitive operation here
//	    return nil
//	})
func SecureOperation(fn func() error) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	return fn()
}

// SecureBytesEqual performs a constant-time comparison of two byte slices.
// This prevents timing attacks when comparing sensitive data like keys or HMACs.
// nil and empty slices are treated as equivalent.
func SecureBytesEqual(a, b []byte) bool {
	// Treat nil and empty slices as equivalent
	if len(a) == 0 && len(b) == 0 {
		return true
	}

	if len(a) != len(b) {
		return false
	}

	var result byte
	for i := 0; i < len(a); i++ {
		result |= a[i] ^ b[i]
	}

	return result == 0
}

// SecureCopy copies data from src to dst using secure memory handling.
// Returns the number of bytes copied.
func SecureCopy(dst, src []byte) int {
	if dst == nil || src == nil {
		return 0
	}

	n := len(src)
	if n > len(dst) {
		n = len(dst)
	}

	// Use a simple copy without any extra allocations
	for i := 0; i < n; i++ {
		dst[i] = src[i]
	}

	runtime.KeepAlive(dst)
	runtime.KeepAlive(src)

	return n
}

// SecurePool is a pool of secure buffers for reuse.
type SecurePool struct {
	pool    sync.Pool
	bufSize int
}

// NewSecurePool creates a pool of secure buffers of the specified size.
func NewSecurePool(bufSize int) *SecurePool {
	return &SecurePool{
		bufSize: bufSize,
		pool: sync.Pool{
			New: func() interface{} {
				return NewSecureBuffer(bufSize)
			},
		},
	}
}

// Get retrieves a secure buffer from the pool.
// The buffer's data is zeroed before being returned.
func (p *SecurePool) Get() *SecureBuffer {
	buf := p.pool.Get().(*SecureBuffer) //nolint:errcheck

	// Zero the buffer before returning it
	if buf.data != nil {
		zeroBytes(buf.data)
	}
	buf.released = false

	return buf
}

// Put returns a secure buffer to the pool.
// The buffer's data is zeroed before being pooled.
func (p *SecurePool) Put(buf *SecureBuffer) {
	if buf == nil || buf.released {
		return
	}

	// Zero the buffer before returning to pool
	if buf.data != nil {
		zeroBytes(buf.data)
	}

	p.pool.Put(buf)
}
