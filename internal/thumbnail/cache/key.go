// thumbnail/cache/key.go
package cache

import (
	"crypto/sha256"
	"fmt"
)

type KeyGenerator struct{}

func NewKeyGenerator() *KeyGenerator {
	return &KeyGenerator{}
}

func (g *KeyGenerator) Generate(provider, bucket, filePath string, mtime int64, size int64) string {
	input := fmt.Sprintf("%s:%s:%s:%d:%d", provider, bucket, filePath, mtime, size)
	hash := sha256.Sum256([]byte(input))
	return fmt.Sprintf("sha256:%x", hash)
}

func (g *KeyGenerator) ValidateKey(key string) bool {
	if len(key) < 7 || key[:7] != "sha256:" {
		return false
	}
	return len(key[7:]) == 64
}
