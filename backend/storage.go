package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const duitNowQRBucket = "duitnow-qr"

// uploadDuitNowQR stores the image in Supabase Storage and returns its public URL.
func uploadDuitNowQR(ctx context.Context, cfg Config, ownerID string, img []byte) (string, error) {
	if cfg.SupabaseURL == "" || cfg.SupabaseSecretKey == "" {
		return "", errors.New("supabase storage not configured")
	}
	path := ownerID + "/qr.jpg"
	base := strings.TrimRight(cfg.SupabaseURL, "/")
	uploadURL := fmt.Sprintf("%s/storage/v1/object/%s/%s", base, duitNowQRBucket, path)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, uploadURL, bytes.NewReader(img))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+cfg.SupabaseSecretKey)
	req.Header.Set("apikey", cfg.SupabaseSecretKey)
	req.Header.Set("Content-Type", "image/jpeg")
	req.Header.Set("x-upsert", "true")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return "", fmt.Errorf("storage upload failed (%d): %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return fmt.Sprintf("%s/storage/v1/object/public/%s/%s", base, duitNowQRBucket, path), nil
}

// ownerQRForSplit prefers the split's stored QR; falls back to the owner's profile.
func (s *Server) ownerQRForSplit(ctx context.Context, split Split) *string {
	if split.OwnerQRURL != nil && *split.OwnerQRURL != "" {
		return split.OwnerQRURL
	}
	prof, err := s.store.GetOwnerProfile(ctx, split.OwnerID)
	if err != nil || prof.OwnerQRURL == nil || *prof.OwnerQRURL == "" {
		return nil
	}
	return prof.OwnerQRURL
}
