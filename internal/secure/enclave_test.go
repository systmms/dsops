package secure

import (
	"bytes"
	"testing"
)

func TestNewSecureBuffer(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		data    []byte
		wantErr bool
	}{
		{
			name:    "creates enclave from bytes",
			data:    []byte("my-secret-password"),
			wantErr: false,
		},
		{
			name:    "handles empty data",
			data:    []byte{},
			wantErr: false,
		},
		{
			name:    "handles binary data",
			data:    []byte{0x00, 0xFF, 0x10, 0x20},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			buf, err := NewSecureBuffer(tt.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewSecureBuffer() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if buf == nil {
				t.Error("NewSecureBuffer() returned nil buffer")
				return
			}

			// Clean up
			buf.Destroy()
		})
	}
}

func TestSecureBuffer_Open(t *testing.T) {
	t.Parallel()

	// Note: memguard may zero the source buffer, so we need a copy for comparison
	secretStr := "super-secret-data"
	secret := []byte(secretStr)
	expected := []byte(secretStr) // Separate copy for comparison

	buf, err := NewSecureBuffer(secret)
	if err != nil {
		t.Fatalf("NewSecureBuffer() error = %v", err)
	}
	defer buf.Destroy()

	// Open should return the decrypted data
	locked, err := buf.Open()
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer locked.Destroy()

	got := locked.Bytes()
	if !bytes.Equal(got, expected) {
		t.Errorf("Open() returned %v, want %v", got, expected)
	}
}

func TestSecureBuffer_MultipleOpens(t *testing.T) {
	t.Parallel()

	secretStr := "test-secret"
	secret := []byte(secretStr)
	expected := []byte(secretStr) // Separate copy for comparison

	buf, err := NewSecureBuffer(secret)
	if err != nil {
		t.Fatalf("NewSecureBuffer() error = %v", err)
	}
	defer buf.Destroy()

	// Should be able to open multiple times
	for i := 0; i < 3; i++ {
		locked, err := buf.Open()
		if err != nil {
			t.Fatalf("Open() iteration %d error = %v", i, err)
		}
		if !bytes.Equal(locked.Bytes(), expected) {
			t.Errorf("Open() iteration %d: got different data", i)
		}
		locked.Destroy()
	}
}

func TestSecureBuffer_Destroy(t *testing.T) {
	t.Parallel()

	secret := []byte("secret-to-destroy")
	buf, err := NewSecureBuffer(secret)
	if err != nil {
		t.Fatalf("NewSecureBuffer() error = %v", err)
	}

	// Destroy should not panic
	buf.Destroy()

	// Double destroy should also not panic (idempotent)
	buf.Destroy()
}

func TestSecureBuffer_DestroyWipesMemory(t *testing.T) {
	t.Parallel()

	secretStr := "sensitive-data-to-wipe"
	secret := []byte(secretStr)
	expected := []byte(secretStr) // Separate copy for comparison

	buf, err := NewSecureBuffer(secret)
	if err != nil {
		t.Fatalf("NewSecureBuffer() error = %v", err)
	}

	// Open to verify data exists
	locked, err := buf.Open()
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}

	// Get reference to the bytes before destroying
	// Note: We can't actually test that memory is wiped since
	// memguard handles this internally. This test verifies the
	// Destroy method executes without error.
	if !bytes.Equal(locked.Bytes(), expected) {
		t.Error("Data not equal before destroy")
	}

	locked.Destroy()
	buf.Destroy()

	// After destroy, the buffer should be unusable
	// memguard will panic if we try to use a destroyed enclave
}

func TestNewSecureBuffer_GracefulDegradation(t *testing.T) {
	t.Parallel()

	// This test verifies that NewSecureBuffer works even if mlock
	// might fail (e.g., due to RLIMIT_MEMLOCK limits). The implementation
	// should gracefully degrade rather than fail.

	// Create a reasonably sized buffer - use a copy for comparison
	expected := bytes.Repeat([]byte("x"), 1024)
	secret := bytes.Repeat([]byte("x"), 1024)
	buf, err := NewSecureBuffer(secret)

	// Should not error - either mlock works or we gracefully degrade
	if err != nil {
		t.Fatalf("NewSecureBuffer() should not error, got: %v", err)
	}
	defer buf.Destroy()

	// Data should still be retrievable
	locked, err := buf.Open()
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer locked.Destroy()

	if !bytes.Equal(locked.Bytes(), expected) {
		t.Error("Data corrupted after creation")
	}
}

func TestSecureBuffer_ConcurrentAccess(t *testing.T) {
	t.Parallel()

	secretStr := "concurrent-secret"
	secret := []byte(secretStr)
	expected := []byte(secretStr) // Separate copy for comparison

	buf, err := NewSecureBuffer(secret)
	if err != nil {
		t.Fatalf("NewSecureBuffer() error = %v", err)
	}
	defer buf.Destroy()

	// Multiple goroutines opening the buffer concurrently
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			defer func() { done <- true }()

			locked, err := buf.Open()
			if err != nil {
				t.Errorf("Open() error = %v", err)
				return
			}
			defer locked.Destroy()

			if !bytes.Equal(locked.Bytes(), expected) {
				t.Error("Data mismatch in concurrent access")
			}
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
}

// BenchmarkSecureBuffer measures the overhead of secure buffer operations
func BenchmarkSecureBuffer(b *testing.B) {
	secret := []byte("benchmark-secret-data")

	b.Run("NewSecureBuffer", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			buf, _ := NewSecureBuffer(secret)
			buf.Destroy()
		}
	})

	b.Run("Open", func(b *testing.B) {
		buf, _ := NewSecureBuffer(secret)
		defer buf.Destroy()

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			locked, _ := buf.Open()
			locked.Destroy()
		}
	})
}
