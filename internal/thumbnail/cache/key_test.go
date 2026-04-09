// thumbnail/cache/key_test.go
package cache

import (
	"testing"
)

func TestKeyGenerator(t *testing.T) {
	gen := NewKeyGenerator()

	key := gen.Generate("s3", "files", "/photos/vacation.jpg", 1709234567, 2048000)
	if !gen.ValidateKey(key) {
		t.Errorf("Key should be valid: %s", key)
	}

	if gen.ValidateKey("invalid") {
		t.Error("Invalid key should fail validation")
	}

	key2 := gen.Generate("s3", "files", "/photos/vacation.jpg", 1709234567, 2048000)
	if key != key2 {
		t.Error("Same inputs should produce same key")
	}

	key3 := gen.Generate("s3", "files", "/photos/vacation.jpg", 9999999999, 2048000)
	if key == key3 {
		t.Error("Different mtime should produce different key")
	}
}
