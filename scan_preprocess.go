package main

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"

	"github.com/disintegration/imaging"
	"github.com/rwcarlsen/goexif/exif"
)

const maxReceiptEdge = 2048

var (
	errUnreadableImage = errors.New("couldn't read image — try a clearer photo")
	errUnsupportedMIME = errors.New("unsupported image type — use JPEG or PNG")
)

func preprocessReceiptImage(raw []byte) ([]byte, error) {
	if len(raw) == 0 {
		return nil, errUnreadableImage
	}
	mime := sniffImageMIME(raw)
	switch mime {
	case "image/jpeg", "image/png":
	default:
		return nil, errUnsupportedMIME
	}

	img, err := decodeImage(raw, mime)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", errUnreadableImage, err)
	}

	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	if w <= 0 || h <= 0 {
		return nil, errUnreadableImage
	}
	longest := w
	if h > longest {
		longest = h
	}
	if longest > maxReceiptEdge {
		img = imaging.Fit(img, maxReceiptEdge, maxReceiptEdge, imaging.Lanczos)
	}

	var buf bytes.Buffer
	if err := imaging.Encode(&buf, img, imaging.JPEG, imaging.JPEGQuality(85)); err != nil {
		return nil, fmt.Errorf("encode jpeg: %w", err)
	}
	return buf.Bytes(), nil
}

func sniffImageMIME(data []byte) string {
	if len(data) >= 3 && data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF {
		return "image/jpeg"
	}
	if len(data) >= 8 && bytes.Equal(data[:8], []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}) {
		return "image/png"
	}
	return "application/octet-stream"
}

func decodeImage(raw []byte, mime string) (image.Image, error) {
	if mime == "image/jpeg" {
		if oriented, err := applyJPEGOrientation(raw); err == nil {
			return oriented, nil
		}
	}
	return imaging.Decode(bytes.NewReader(raw))
}

func applyJPEGOrientation(raw []byte) (image.Image, error) {
	img, err := imaging.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	x, err := exif.Decode(bytes.NewReader(raw))
	if err != nil {
		return img, nil
	}
	tag, err := x.Get(exif.Orientation)
	if err != nil {
		return img, nil
	}
	orient, err := tag.Int(0)
	if err != nil || orient <= 1 {
		return img, nil
	}
	switch orient {
	case 2:
		return imaging.FlipH(img), nil
	case 3:
		return imaging.Rotate180(img), nil
	case 4:
		return imaging.FlipV(img), nil
	case 5:
		return imaging.Rotate270(imaging.FlipH(img)), nil
	case 6:
		return imaging.Rotate270(img), nil
	case 7:
		return imaging.Rotate90(imaging.FlipH(img)), nil
	case 8:
		return imaging.Rotate90(img), nil
	default:
		return img, nil
	}
}

// decodeUpload reads and bounds-checks an uploaded receipt image.
func decodeUpload(r io.Reader, limit int64) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, errors.New("image too large")
	}
	if len(data) == 0 {
		return nil, errors.New("empty image")
	}
	return data, nil
}
