package main

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"testing"
)

func TestPreprocessReceiptImage_jpeg(t *testing.T) {
	src := makeJPEG(t, 3000, 2000)
	out, err := preprocessReceiptImage(src)
	if err != nil {
		t.Fatalf("preprocess: %v", err)
	}
	if sniffImageMIME(out) != "image/jpeg" {
		t.Fatalf("expected jpeg output")
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(out))
	if err != nil {
		t.Fatalf("decode config: %v", err)
	}
	if cfg.Width > maxReceiptEdge || cfg.Height > maxReceiptEdge {
		t.Fatalf("image not downscaled: %dx%d", cfg.Width, cfg.Height)
	}
}

func TestPreprocessReceiptImage_rejectEmpty(t *testing.T) {
	_, err := preprocessReceiptImage(nil)
	if err == nil {
		t.Fatal("expected error for empty input")
	}
}

func TestPreprocessReceiptImage_rejectUnknown(t *testing.T) {
	_, err := preprocessReceiptImage([]byte("not an image"))
	if err == nil {
		t.Fatal("expected error for unknown mime")
	}
}

func makeJPEG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{uint8(x % 255), uint8(y % 255), 100, 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatalf("encode: %v", err)
	}
	return buf.Bytes()
}
