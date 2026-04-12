package thumbnail

import (
	"bytes"
	"context"
	"errors"
	"image"
	"io"
	"mime"
	"net/http"
	"strings"

	"github.com/disintegration/imaging"
	_ "github.com/vegidio/heif-go"
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
	".heic": imaging.Format(-1), // HEIC - use heif-go decoder
	".heif": imaging.Format(-1), // HEIF - use heif-go decoder
}

func (g *ImageGenerator) Generate(ctx context.Context, input io.Reader) ([]byte, error) {
	data, err := io.ReadAll(input)
	if err != nil {
		return nil, err
	}

	format, isHeic := g.detectFormat(data)
	if !isHeic && format == imaging.Format(-1) {
		return nil, ErrUnsupportedFormat
	}

	var img image.Image
	if isHeic && format == imaging.Format(-1) {
		img, _, err = image.Decode(bytes.NewReader(data))
		if err != nil {
			return nil, ErrImageDecode
		}
	} else {
		img, err = imaging.Decode(bytes.NewReader(data))
		if err != nil {
			return nil, ErrImageDecode
		}
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
	if isHEIC(data) {
		return imaging.Format(-1), true
	}
	contentType := http.DetectContentType(data)
	exts, err := mime.ExtensionsByType(contentType)
	if err != nil || len(exts) == 0 {
		return imaging.Format(-1), false
	}
	// Check all returned extensions - mime might return .jfif first for JPEG files
	for _, ext := range exts {
		format, ok := supportedFormats[strings.ToLower(ext)]
		if ok {
			return format, true
		}
	}
	return imaging.Format(-1), false
}

func isHEIC(data []byte) bool {
	if len(data) < 12 {
		return false
	}
	if string(data[4:8]) != "ftyp" {
		return false
	}
	brand := string(data[8:12])
	return brand == "heic" || brand == "mif1" || brand == "heix" || brand == "hevc" || brand == "hevx"
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
