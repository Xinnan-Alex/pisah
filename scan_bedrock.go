package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types"
	"github.com/aws/smithy-go"
)

const defaultBedrockOCRModel = "global.anthropic.claude-haiku-4-5-20251001-v1:0"

type bedrockScanner struct {
	client  *bedrockruntime.Client
	modelID string
	timeout time.Duration
}

func newBedrockScanner() (*bedrockScanner, error) {
	region := envOr("AWS_REGION", "ap-southeast-1")
	cfg, err := config.LoadDefaultConfig(context.Background(), config.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}
	timeout := 45 * time.Second
	if v := os.Getenv("OCR_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			timeout = d
		}
	}
	return &bedrockScanner{
		client:  bedrockruntime.NewFromConfig(cfg),
		modelID: bedrockConverseModelID(envOr("OCR_MODEL", defaultBedrockOCRModel)),
		timeout: timeout,
	}, nil
}

// bedrockConverseModelID maps foundation-model IDs to inference profile IDs.
// Recent Anthropic models (e.g. Haiku 4.5) reject on-demand calls to the raw
// model ID and require global/us/eu/apac inference profile prefixes instead.
func bedrockConverseModelID(model string) string {
	if prefix, _, ok := strings.Cut(model, "."); ok {
		switch prefix {
		case "global", "us", "eu", "apac", "au", "jp":
			return model
		}
	}
	switch model {
	case "anthropic.claude-haiku-4-5-20251001-v1:0":
		return "global.anthropic.claude-haiku-4-5-20251001-v1:0"
	case "anthropic.claude-sonnet-4-5-20250929-v1:0":
		return "global.anthropic.claude-sonnet-4-5-20250929-v1:0"
	case "anthropic.claude-sonnet-4-6":
		return "global.anthropic.claude-sonnet-4-6"
	case "anthropic.claude-opus-4-6-v1":
		return "global.anthropic.claude-opus-4-6-v1"
	}
	return model
}

func (b *bedrockScanner) Scan(ctx context.Context, img []byte) (ParsedReceipt, error) {
	return visionScanReceipt(ctx, "bedrock", b.modelID, b.converse, img)
}

func (b *bedrockScanner) converse(ctx context.Context, systemPrompt string, img []byte) (string, error) {
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
		text, retryable, err := b.doConverse(ctx, systemPrompt, img)
		if err == nil {
			return text, nil
		}
		lastErr = err
		if !retryable {
			break
		}
		slog.WarnContext(ctx, "bedrock converse retry",
			"ocr_provider", "bedrock",
			"ocr_model", b.modelID,
			"ocr_backend", "aws_bedrock",
			"attempt", attempt+1,
			"error", err,
		)
	}
	return "", lastErr
}

func (b *bedrockScanner) doConverse(ctx context.Context, systemPrompt string, img []byte) (string, bool, error) {
	callCtx, cancel := context.WithTimeout(ctx, b.timeout)
	defer cancel()

	out, err := b.client.Converse(callCtx, &bedrockruntime.ConverseInput{
		ModelId: aws.String(b.modelID),
		System: []types.SystemContentBlock{
			&types.SystemContentBlockMemberText{Value: systemPrompt},
		},
		Messages: []types.Message{
			{
				Role: types.ConversationRoleUser,
				Content: []types.ContentBlock{
					&types.ContentBlockMemberText{Value: "Extract all receipt fields from this image. Return only valid JSON."},
					&types.ContentBlockMemberImage{Value: types.ImageBlock{
						Format: types.ImageFormatJpeg,
						Source: &types.ImageSourceMemberBytes{Value: img},
					}},
				},
			},
		},
		InferenceConfig: &types.InferenceConfiguration{
			Temperature: aws.Float32(0),
			MaxTokens:   aws.Int32(4096),
		},
	})
	if err != nil {
		return "", bedrockRetryable(err), fmt.Errorf("bedrock converse: %w", err)
	}
	if out.Output == nil {
		return "", false, errors.New("empty bedrock response")
	}
	msgOut, ok := out.Output.(*types.ConverseOutputMemberMessage)
	if !ok {
		return "", false, errors.New("unexpected bedrock output type")
	}
	text := bedrockMessageText(msgOut.Value.Content)
	if strings.TrimSpace(text) == "" {
		return "", false, errors.New("empty bedrock message")
	}
	return text, false, nil
}

func bedrockMessageText(blocks []types.ContentBlock) string {
	var b strings.Builder
	for _, block := range blocks {
		if t, ok := block.(*types.ContentBlockMemberText); ok {
			b.WriteString(t.Value)
		}
	}
	return b.String()
}

func bedrockRetryable(err error) bool {
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.ErrorCode() {
		case "ThrottlingException", "ServiceUnavailableException", "ModelTimeoutException", "InternalServerException":
			return true
		}
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "throttl") || strings.Contains(msg, "timeout") || strings.Contains(msg, "503")
}
