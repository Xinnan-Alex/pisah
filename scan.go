package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"
)

// ReceiptScanner extracts structured receipt data from a preprocessed image.
type ReceiptScanner interface {
	Scan(ctx context.Context, img []byte) (ParsedReceipt, error)
}

// ReceiptConfidence holds optional per-field confidence scores from the OCR model.
type ReceiptConfidence struct {
	Merchant float64 `json:"merchant,omitempty"`
	Total    float64 `json:"total,omitempty"`
	Items    float64 `json:"items,omitempty"`
}

// ScanResult is the enriched, validated output of a receipt scan.
type ScanResult struct {
	ScanID   string        `json:"scanId"`
	Receipt  ParsedReceipt `json:"receipt"`
	Warnings []string      `json:"warnings"`
	ImageURL string        `json:"imageUrl,omitempty"`
}

// ScanPipeline preprocesses images, runs OCR, normalizes, enriches, and validates.
type ScanPipeline struct {
	scanner  ReceiptScanner
	provider string
	model    string
}

func (p *ScanPipeline) backend() string { return ocrBackend(p.provider) }

func ocrBackend(provider string) string {
	switch provider {
	case "bedrock":
		return "aws_bedrock"
	case "textract":
		return "aws_textract"
	case "openai":
		return "openai"
	default:
		return provider
	}
}

func (p *ScanPipeline) logAttrs(extra ...any) []any {
	attrs := []any{
		"ocr_provider", p.provider,
		"ocr_model", p.model,
		"ocr_backend", p.backend(),
	}
	return append(attrs, extra...)
}

func newScanPipeline() (*ScanPipeline, error) {
	scanner, provider, model, err := newReceiptScanner()
	if err != nil {
		return nil, err
	}
	return &ScanPipeline{scanner: scanner, provider: provider, model: model}, nil
}

func (p *ScanPipeline) Process(ctx context.Context, raw []byte) (ParsedReceipt, []string, error) {
	slog.InfoContext(ctx, "receipt scan start", p.logAttrs()...)

	start := time.Now()
	img, err := preprocessReceiptImage(raw)
	if err != nil {
		slog.ErrorContext(ctx, "receipt scan failed", p.logAttrs("stage", "preprocess", "error", err)...)
		return ParsedReceipt{}, nil, err
	}
	rec, err := p.scanner.Scan(ctx, img)
	if err != nil {
		slog.ErrorContext(ctx, "receipt scan failed", p.logAttrs("stage", "ocr", "error", err)...)
		return ParsedReceipt{}, nil, err
	}
	normalizeParsedReceipt(&rec)
	enrichParsedReceipt(&rec)
	warnings := validateReceipt(&rec)

	slog.InfoContext(ctx, "receipt scan ok", p.logAttrs(
		"latency_ms", time.Since(start).Milliseconds(),
		"merchant", rec.Merchant,
		"items", len(rec.Items),
		"total_sen", rec.TotalSen,
		"warning_count", len(warnings),
	)...)
	return rec, warnings, nil
}

func ocrTimeout() time.Duration {
	timeout := 45 * time.Second
	if v := os.Getenv("OCR_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			timeout = d
		}
	}
	return timeout
}

func newReceiptScanner() (ReceiptScanner, string, string, error) {
	provider := envOr("OCR_PROVIDER", "bedrock")
	switch provider {
	case "bedrock":
		b, err := newBedrockScanner()
		if err != nil {
			return nil, "", "", err
		}
		return b, provider, b.modelID, nil
	case "textract":
		return &textractScanner{}, provider, "AnalyzeExpense", nil
	case "openai":
		key := os.Getenv("OPENAI_API_KEY")
		if key == "" {
			return nil, "", "", fmt.Errorf("OPENAI_API_KEY is required when OCR_PROVIDER=openai")
		}
		model := envOr("OCR_MODEL", "gpt-4o-mini")
		return &visionScanner{
			apiKey:  key,
			model:   model,
			timeout: ocrTimeout(),
		}, provider, model, nil
	default:
		return nil, "", "", fmt.Errorf("unknown OCR_PROVIDER %q (use bedrock, openai, or textract)", provider)
	}
}
