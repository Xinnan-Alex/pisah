package main

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

// visionScanReceipt runs a vision LLM call, parses JSON, and retries with a repair prompt on failure.
func visionScanReceipt(ctx context.Context, provider, model string, call func(ctx context.Context, systemPrompt string, img []byte) (string, error), img []byte) (ParsedReceipt, error) {
	start := time.Now()
	content, err := call(ctx, receiptScanSystemPrompt, img)
	if err != nil {
		return ParsedReceipt{}, err
	}
	rec, err := parseLLMReceiptJSON(content)
	if err != nil {
		slog.WarnContext(ctx, "llm json parse failed, retrying repair",
			"ocr_provider", provider, "ocr_model", model, "error", err)
		repaired, err2 := call(ctx, receiptScanRepairPrompt+"\n\nBroken output:\n"+content, img)
		if err2 != nil {
			return ParsedReceipt{}, fmt.Errorf("llm parse failed: %w", err)
		}
		rec, err = parseLLMReceiptJSON(repaired)
		if err != nil {
			return ParsedReceipt{}, fmt.Errorf("llm repair parse failed: %w", err)
		}
	}
	slog.InfoContext(ctx, "vision llm ok",
		"ocr_provider", provider,
		"ocr_model", model,
		"ocr_backend", ocrBackend(provider),
		"latency_ms", time.Since(start).Milliseconds(),
	)
	return rec, nil
}
