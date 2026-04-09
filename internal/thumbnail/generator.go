package thumbnail

import (
	"bytes"
	"context"
	"errors"
	"io"
	"mime"
	"net/http"
	"strings"

	"github.com/disintegration/imaging"
)

var (
	ErrUnsupportedFormat = errors.New("unsupported image format")
	ErrImageDecode       = errors.New("failed to decode image")
)

type ImageGenerator struct {
	maxSize int
}

func NewImageGenerator(maxSize int) *ImageGenerator {
	return &ImageGenerator{maxSize: maxSize}
}

var supportedFormats = map[string]imaging.Format{
	".jpeg": imaging.JPEG,
	".jpg":  imaging.JPEG,
	".png":  imaging.PNG,
	".gif":  imaging.GIF,
	".bmp":  imaging.BMP,
}

func (g *ImageGenerator) Generate(ctx context.Context, input io.Reader) ([]byte, error) {
	data, err := io.ReadAll(input)
	if err != nil {
		return nil, err
	}

	if _, ok := g.detectFormat(data); !ok {
		return nil, ErrUnsupportedFormat
	}

	img, err := imaging.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, ErrImageDecode
	}

	thumb := imaging.Thumbnail(img, g.maxSize, g.maxSize, imaging.Lanczos)

	var buf bytes.Buffer
	err = imaging.Encode(&buf, thumb, imaging.JPEG, imaging.JPEGQuality(85))
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func (g *ImageGenerator) detectFormat(data []byte) (imaging.Format, bool) {
	contentType := http.DetectContentType(data)
	ext, err := mime.ExtensionsByType(contentType)
	if err != nil || len(ext) == 0 {
		return imaging.Format(-1), false
	}
	format, ok := supportedFormats[strings.ToLower(ext[0])]
	if !ok {
		return imaging.Format(-1), false
	}
	return format, true
}

func (g *ImageGenerator) IsFormatSupported(filename string) bool {
	idx := strings.LastIndex(filename, ".")
	if idx < 0 {
		return false
	}
	ext := strings.ToLower(filename[idx:])
	_, ok := supportedFormats[ext]
	return ok
}
