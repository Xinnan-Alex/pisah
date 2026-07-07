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
const receiptScansBucket = "receipt-scans"

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

// fetchStorageObject streams a Supabase Storage public URL through the server.
// Server-side fetch can reach 127.0.0.1 even when the client (phone) cannot.
func fetchStorageObject(ctx context.Context, publicURL string) ([]byte, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, publicURL, nil)
	if err != nil {
		return nil, "", err
	}
	resp, err := (&http.Client{Timeout: 15 * time.Second}).Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
		return nil, "", fmt.Errorf("storage fetch failed (%d): %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxReceiptBytes))
	if err != nil {
		return nil, "", err
	}
	ct := resp.Header.Get("Content-Type")
	if ct == "" {
		ct = "image/jpeg"
	}
	return data, ct, nil
}

// uploadScanImage stores a receipt scan in Supabase Storage and returns its URL.
func uploadScanImage(ctx context.Context, cfg Config, ownerID, scanID string, img []byte) (string, error) {
	if cfg.SupabaseURL == "" || cfg.SupabaseSecretKey == "" {
		return "", errors.New("supabase storage not configured")
	}
	path := ownerID + "/" + scanID + ".jpg"
	base := strings.TrimRight(cfg.SupabaseURL, "/")
	uploadURL := fmt.Sprintf("%s/storage/v1/object/%s/%s", base, receiptScansBucket, path)

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
	return fmt.Sprintf("%s/storage/v1/object/public/%s/%s", base, receiptScansBucket, path), nil
}

// deleteScanImage removes a stored receipt scan image.
func deleteScanImage(ctx context.Context, cfg Config, imageURL string) error {
	if cfg.SupabaseURL == "" || cfg.SupabaseSecretKey == "" || imageURL == "" {
		return nil
	}
	base := strings.TrimRight(cfg.SupabaseURL, "/")
	prefix := base + "/storage/v1/object/public/" + receiptScansBucket + "/"
	if !strings.HasPrefix(imageURL, prefix) {
		return nil
	}
	path := strings.TrimPrefix(imageURL, prefix)
	deleteURL := fmt.Sprintf("%s/storage/v1/object/%s/%s", base, receiptScansBucket, path)
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, deleteURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+cfg.SupabaseSecretKey)
	req.Header.Set("apikey", cfg.SupabaseSecretKey)
	resp, err := (&http.Client{Timeout: 15 * time.Second}).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
	return fmt.Errorf("storage delete failed (%d): %s", resp.StatusCode, strings.TrimSpace(string(body)))
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
