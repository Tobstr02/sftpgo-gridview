package thumbnail

import (
	"bytes"
	"context"
	"testing"
)

func TestImageGenerator(t *testing.T) {
	gen := NewImageGenerator(256)

	if !gen.IsFormatSupported("test.jpg") {
		t.Error("jpg should be supported")
	}
	if !gen.IsFormatSupported("test.png") {
		t.Error("png should be supported")
	}
	if gen.IsFormatSupported("test.txt") {
		t.Error("txt should not be supported")
	}

	testImg := []byte{0x89, 0x50, 0x4E, 0x47} // PNG header
	_, err := gen.Generate(context.Background(), bytes.NewReader(testImg))
	if err == nil {
		t.Error("Should fail for invalid image data")
	}
}
