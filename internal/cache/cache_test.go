package cache

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCache_SetAndGet(t *testing.T) {
	// Create temp directory for test cache
	tmpDir := t.TempDir()
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	c, err := New(1*time.Hour, "test-cache")
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	type testData struct {
		Name  string
		Value int
	}

	tests := []struct {
		name    string
		key     string
		value   testData
		wantErr bool
	}{
		{
			name:    "simple data",
			key:     "test-key",
			value:   testData{Name: "test", Value: 42},
			wantErr: false,
		},
		{
			name:    "empty struct",
			key:     "empty",
			value:   testData{},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set value
			err := c.Set(tt.key, tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("Set() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				return
			}

			// Get value
			var got testData
			err = c.Get(tt.key, &got)
			if err != nil {
				t.Errorf("Get() error = %v", err)
				return
			}

			if got.Name != tt.value.Name || got.Value != tt.value.Value {
				t.Errorf("Get() = %+v, want %+v", got, tt.value)
			}
		})
	}
}

func TestCache_GetNonExistent(t *testing.T) {
	tmpDir := t.TempDir()
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	c, err := New(1*time.Hour, "test-cache")
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	var data map[string]string
	err = c.Get("nonexistent", &data)

	// Should not error on non-existent key, just return nil data
	if err != nil {
		t.Errorf("Get() on non-existent key error = %v, want nil", err)
	}

	if data != nil {
		t.Errorf("Get() on non-existent key = %v, want nil", data)
	}
}

func TestCache_Expiration(t *testing.T) {
	tmpDir := t.TempDir()
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	// Create cache with very short TTL
	c, err := New(50*time.Millisecond, "test-cache")
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	testValue := "test-value"
	err = c.Set("expire-test", testValue)
	if err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	// Should be able to get it immediately
	var got string
	err = c.Get("expire-test", &got)
	if err != nil {
		t.Errorf("Get() before expiration error = %v", err)
	}
	if got != testValue {
		t.Errorf("Get() before expiration = %q, want %q", got, testValue)
	}

	// Wait for expiration
	time.Sleep(100 * time.Millisecond)

	// Should not be able to get it after expiration
	var expired string
	err = c.Get("expire-test", &expired)
	if err != nil {
		t.Errorf("Get() after expiration error = %v", err)
	}
	if expired != "" {
		t.Errorf("Get() after expiration = %q, want empty", expired)
	}

	// Verify cache file was cleaned up
	cacheFile := filepath.Join(c.baseDir, "expire-test.json")
	if _, err := os.Stat(cacheFile); !os.IsNotExist(err) {
		t.Errorf("Expired cache file still exists")
	}
}

func TestCache_Delete(t *testing.T) {
	tmpDir := t.TempDir()
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	c, err := New(1*time.Hour, "test-cache")
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	// Set a value
	err = c.Set("delete-test", "value")
	if err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	// Delete it
	err = c.Delete("delete-test")
	if err != nil {
		t.Errorf("Delete() error = %v", err)
	}

	// Verify it's gone
	var got string
	err = c.Get("delete-test", &got)
	if err != nil {
		t.Errorf("Get() after delete error = %v", err)
	}
	if got != "" {
		t.Errorf("Get() after delete = %q, want empty", got)
	}

	// Deleting non-existent key should not error
	err = c.Delete("nonexistent")
	if err != nil {
		t.Errorf("Delete() non-existent key error = %v", err)
	}
}

func TestCache_Clear(t *testing.T) {
	tmpDir := t.TempDir()
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	c, err := New(1*time.Hour, "test-cache")
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	// Set multiple values
	keys := []string{"key1", "key2", "key3"}
	for _, key := range keys {
		err = c.Set(key, "value")
		if err != nil {
			t.Fatalf("Set(%q) error = %v", key, err)
		}
	}

	// Clear cache
	err = c.Clear()
	if err != nil {
		t.Errorf("Clear() error = %v", err)
	}

	// Verify all keys are gone
	for _, key := range keys {
		var got string
		err = c.Get(key, &got)
		if err != nil {
			t.Errorf("Get(%q) after clear error = %v", key, err)
		}
		if got != "" {
			t.Errorf("Get(%q) after clear = %q, want empty", key, got)
		}
	}
}

func TestCache_Info(t *testing.T) {
	tmpDir := t.TempDir()
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	c, err := New(1*time.Hour, "test-cache")
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	// Initially empty
	info, err := c.Info()
	if err != nil {
		t.Fatalf("Info() error = %v", err)
	}
	if info.EntryCount != 0 {
		t.Errorf("Info().EntryCount = %d, want 0", info.EntryCount)
	}

	// Add some entries
	err = c.Set("key1", "value1")
	if err != nil {
		t.Fatalf("Set() error = %v", err)
	}
	err = c.Set("key2", "value2")
	if err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	// Should have 2 entries
	info, err = c.Info()
	if err != nil {
		t.Fatalf("Info() after adding entries error = %v", err)
	}
	if info.EntryCount != 2 {
		t.Errorf("Info().EntryCount = %d, want 2", info.EntryCount)
	}
	if info.TotalSize == 0 {
		t.Errorf("Info().TotalSize = 0, want > 0")
	}
	if info.Directory == "" {
		t.Errorf("Info().Directory is empty")
	}
}

func TestNew_DefaultTTL(t *testing.T) {
	tmpDir := t.TempDir()
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	// Create with zero TTL should use default
	c, err := New(0, "test-cache")
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if c.ttl != DefaultTTL {
		t.Errorf("New(0) ttl = %v, want %v", c.ttl, DefaultTTL)
	}
}

func TestCache_ConcurrentAccess(t *testing.T) {
	tmpDir := t.TempDir()
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	c, err := New(1*time.Hour, "test-cache")
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	// Test concurrent writes don't panic
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(n int) {
			_ = c.Set("concurrent", n)
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Should be able to read the value
	var got int
	err = c.Get("concurrent", &got)
	if err != nil {
		t.Errorf("Get() after concurrent writes error = %v", err)
	}
}
