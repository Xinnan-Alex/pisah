package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

type visionScanner struct {
	apiKey  string
	model   string
	timeout time.Duration
	client  *http.Client
}

func (v *visionScanner) Scan(ctx context.Context, img []byte) (ParsedReceipt, error) {
	return visionScanReceipt(ctx, "openai", v.model, v.chat, img)
}

func (v *visionScanner) chat(ctx context.Context, systemPrompt string, img []byte) (string, error) {
	b64 := base64.StdEncoding.EncodeToString(img)
	return v.chatB64(ctx, systemPrompt, b64)
}

func (v *visionScanner) chatB64(ctx context.Context, systemPrompt, imageB64 string) (string, error) {
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			wait := time.Duration(attempt*attempt) * time.Second
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(wait):
			}
		}
		text, status, err := v.doChat(ctx, systemPrompt, imageB64)
		if err == nil {
			return text, nil
		}
		lastErr = err
		if status != 429 && status < 500 {
			break
		}
		slog.WarnContext(ctx, "openai chat retry",
			"ocr_provider", "openai",
			"ocr_model", v.model,
			"ocr_backend", "openai",
			"attempt", attempt+1,
			"status", status,
			"error", err,
		)
	}
	return "", lastErr
}

func (v *visionScanner) doChat(ctx context.Context, systemPrompt, imageB64 string) (string, int, error) {
	reqBody := map[string]any{
		"model":           v.model,
		"response_format": map[string]string{"type": "json_object"},
		"messages": []map[string]any{
			{"role": "system", "content": systemPrompt},
			{
				"role": "user",
				"content": []map[string]any{
					{"type": "text", "text": "Extract all receipt fields from this image."},
					{
						"type": "image_url",
						"image_url": map[string]string{
							"url":    "data:image/jpeg;base64," + imageB64,
							"detail": "high",
						},
					},
				},
			},
		},
		"temperature": 0,
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", 0, err
	}

	callCtx, cancel := context.WithTimeout(ctx, v.timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(callCtx, http.MethodPost, "https://api.openai.com/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", 0, err
	}
	req.Header.Set("Authorization", "Bearer "+v.apiKey)
	req.Header.Set("Content-Type", "application/json")

	client := v.client
	if client == nil {
		client = &http.Client{Timeout: v.timeout + 5*time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", 0, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := strings.TrimSpace(string(raw))
		if len(msg) > 200 {
			msg = msg[:200]
		}
		return "", resp.StatusCode, fmt.Errorf("openai api %d: %s", resp.StatusCode, msg)
	}

	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", resp.StatusCode, err
	}
	if len(out.Choices) == 0 || strings.TrimSpace(out.Choices[0].Message.Content) == "" {
		return "", resp.StatusCode, errors.New("empty openai response")
	}
	return out.Choices[0].Message.Content, resp.StatusCode, nil
}
