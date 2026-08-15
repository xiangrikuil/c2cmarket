package evidence

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"strings"

	"github.com/liyue201/goqr"
	"golang.org/x/image/webp"
)

var (
	ErrFileTooLarge      = errors.New("evidence image exceeds 5 MiB")
	ErrUnsupportedFormat = errors.New("evidence image must be JPEG, PNG, or WebP")
	ErrInvalidDimensions = errors.New("evidence image dimensions are invalid")
	ErrQRCodeDetected    = errors.New("evidence image contains a QR code")
)

func ProcessImage(reader io.Reader) (ProcessedImage, error) {
	limited := io.LimitReader(reader, MaxFileBytes+1)
	raw, err := io.ReadAll(limited)
	if err != nil {
		return ProcessedImage{}, fmt.Errorf("read evidence image: %w", err)
	}
	if int64(len(raw)) > MaxFileBytes {
		return ProcessedImage{}, ErrFileTooLarge
	}
	if len(raw) == 0 {
		return ProcessedImage{}, ErrUnsupportedFormat
	}

	config, format, err := decodeSupportedImageConfig(raw)
	if err != nil {
		return ProcessedImage{}, err
	}
	width, height := config.Width, config.Height
	if width < 1 || height < 1 || width > MaxDimension || height > MaxDimension {
		return ProcessedImage{}, ErrInvalidDimensions
	}
	img, err := decodeSupportedImage(raw, format)
	if err != nil {
		return ProcessedImage{}, err
	}
	if codes, qrErr := goqr.Recognize(img); qrErr == nil && len(codes) > 0 {
		return ProcessedImage{}, ErrQRCodeDetected
	} else if qrErr != nil && !errors.Is(qrErr, goqr.ErrNoQRCode) {
		return ProcessedImage{}, fmt.Errorf("scan evidence QR code: %w", qrErr)
	}

	var output bytes.Buffer
	outputMIME := "image/png"
	if format == "jpeg" {
		outputMIME = "image/jpeg"
		err = jpeg.Encode(&output, img, &jpeg.Options{Quality: 90})
	} else {
		err = png.Encode(&output, img)
	}
	if err != nil {
		return ProcessedImage{}, fmt.Errorf("re-encode evidence image: %w", err)
	}
	if int64(output.Len()) > MaxFileBytes {
		return ProcessedImage{}, ErrFileTooLarge
	}
	encoded := output.Bytes()
	return ProcessedImage{
		Bytes: append([]byte(nil), encoded...), MIME: outputMIME,
		Width: width, Height: height, SHA256: sha256.Sum256(encoded),
	}, nil
}

func decodeSupportedImageConfig(raw []byte) (image.Config, string, error) {
	format := sniffFormat(raw)
	var (
		config image.Config
		err    error
	)
	switch format {
	case "jpeg":
		config, err = jpeg.DecodeConfig(bytes.NewReader(raw))
	case "png":
		config, err = png.DecodeConfig(bytes.NewReader(raw))
	case "webp":
		config, err = webp.DecodeConfig(bytes.NewReader(raw))
	default:
		return image.Config{}, "", ErrUnsupportedFormat
	}
	if err != nil {
		return image.Config{}, "", ErrUnsupportedFormat
	}
	return config, format, nil
}

func decodeSupportedImage(raw []byte, format string) (image.Image, error) {
	var (
		img image.Image
		err error
	)
	switch format {
	case "jpeg":
		img, err = jpeg.Decode(bytes.NewReader(raw))
	case "png":
		img, err = png.Decode(bytes.NewReader(raw))
	case "webp":
		img, err = webp.Decode(bytes.NewReader(raw))
	default:
		return nil, ErrUnsupportedFormat
	}
	if err != nil {
		return nil, ErrUnsupportedFormat
	}
	return img, nil
}

func sniffFormat(raw []byte) string {
	if len(raw) >= 3 && raw[0] == 0xff && raw[1] == 0xd8 && raw[2] == 0xff {
		return "jpeg"
	}
	if len(raw) >= 8 && bytes.Equal(raw[:8], []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a}) {
		return "png"
	}
	if len(raw) >= 12 && string(raw[:4]) == "RIFF" && strings.EqualFold(string(raw[8:12]), "WEBP") {
		return "webp"
	}
	return ""
}
