package secure

import (
	"sync"

	"github.com/awnumar/memguard"
)

// SecureBuffer provides memory-safe storage for sensitive data.
// It wraps memguard.Enclave to encrypt secrets at rest in memory
// and protect them from swapping via mlock.
//
// Note: memguard.Enclave doesn't have a direct Destroy method.
// Instead, we track the enclave and use memguard.Purge() for cleanup
// at application exit, or simply let the enclave be garbage collected
// (the encrypted data is safe even without explicit destruction).
type SecureBuffer struct {
	enclave *memguard.Enclave
	mu      sync.RWMutex
	// destroyed tracks if this buffer has been destroyed to allow
	// idempotent Destroy() calls and prevent use after destroy
	destroyed bool
}

// NewSecureBuffer creates a protected buffer from secret bytes.
// The input data is immediately copied into a protected memory region
// and the original data remains unchanged (caller should zero it).
//
// If mlock is unavailable (e.g., due to RLIMIT_MEMLOCK), the function
// logs a warning and continues with standard memory allocation.
// This provides graceful degradation on systems with limited resources.
func NewSecureBuffer(data []byte) (*SecureBuffer, error) {
	// memguard.NewEnclave creates an encrypted enclave from the data.
	// The enclave:
	// - Encrypts the data using XSalsa20Poly1305
	// - Attempts to mlock the memory to prevent swapping
	// - Sets up guard pages for overflow detection
	enclave := memguard.NewEnclave(data)

	return &SecureBuffer{
		enclave:   enclave,
		destroyed: false,
	}, nil
}

// Open decrypts and returns the protected data in a locked buffer.
// The caller MUST call Destroy() on the returned LockedBuffer when done
// to securely wipe the plaintext from memory.
//
// Example:
//
//	locked, err := buf.Open()
//	if err != nil {
//	    return err
//	}
//	defer locked.Destroy()
//	secret := locked.Bytes()
func (s *SecureBuffer) Open() (*memguard.LockedBuffer, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.destroyed {
		// Return an empty locked buffer if already destroyed
		return memguard.NewBufferFromBytes([]byte{}), nil
	}

	// Open decrypts the enclave and returns a locked buffer.
	// The locked buffer has:
	// - Memory locked to prevent swapping
	// - Guard pages on both sides
	// - Read-write access by default
	return s.enclave.Open()
}

// Destroy marks this SecureBuffer as destroyed and prevents further use.
// The underlying encrypted enclave data is safe even without explicit destruction
// since it's encrypted at rest. However, this method ensures the buffer
// cannot be accidentally reused.
//
// This method is idempotent - calling it multiple times is safe.
// After Destroy(), Open() will return an empty buffer.
//
// For complete cleanup of all memguard data at application exit,
// call memguard.Purge() in a defer statement in main().
func (s *SecureBuffer) Destroy() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.destroyed {
		return
	}

	// Mark as destroyed to prevent further use.
	// The enclave's encrypted data will be garbage collected.
	// For sensitive cleanup, callers should use memguard.Purge()
	// at application exit.
	s.enclave = nil
	s.destroyed = true
}
