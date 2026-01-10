// Package secure provides memory-safe handling of sensitive data.
//
// This package wraps the memguard library to provide secure storage for
// secrets in memory. It ensures that sensitive data is:
//
//   - Encrypted at rest in memory (XSalsa20Poly1305)
//   - Protected from swapping via mlock
//   - Securely wiped when no longer needed
//   - Protected from buffer overflow via guard pages
//
// # Usage
//
// Create a secure buffer from sensitive bytes:
//
//	buf, err := secure.NewSecureBuffer([]byte("my-secret"))
//	if err != nil {
//	    // Handle error - may indicate mlock unavailable
//	}
//	defer buf.Destroy() // Always destroy when done
//
//	// When you need to use the secret:
//	locked, err := buf.Open()
//	if err != nil {
//	    // Handle error
//	}
//	defer locked.Destroy() // Destroy the unlocked buffer when done
//
//	// Use locked.Bytes() to access the plaintext
//	secretBytes := locked.Bytes()
//
// # Platform Behavior
//
// Memory locking behavior varies by platform:
//
//   - Linux: Requires RLIMIT_MEMLOCK to be set appropriately
//   - macOS: Works out of the box
//   - Windows: Uses VirtualLock
//
// If mlock is unavailable or fails, the package logs a warning and
// continues with standard Go memory (graceful degradation).
//
// # Security Guarantees
//
// This package provides defense-in-depth against memory-based attacks:
//
//   - Core dumps will not contain plaintext secrets
//   - Secrets won't be swapped to disk
//   - Memory is overwritten with zeros on destruction
//   - Guard pages detect buffer overflows
//
// It does NOT protect against:
//
//   - Attackers with root access to the running process
//   - Hardware-level attacks (cold boot, DMA)
//   - Spectre/Meltdown side-channel attacks
package secure
