package main

import (
	"context"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/textract"
	"github.com/aws/aws-sdk-go-v2/service/textract/types"
)

type textractScanner struct{}

func (t *textractScanner) Scan(ctx context.Context, img []byte) (ParsedReceipt, error) {
	start := time.Now()
	slog.InfoContext(ctx, "textract scan start",
		"ocr_provider", "textract",
		"ocr_model", "AnalyzeExpense",
		"ocr_backend", "aws_textract",
	)

	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "textract scan failed",
			"ocr_provider", "textract", "ocr_model", "AnalyzeExpense", "ocr_backend", "aws_textract",
			"error", err,
		)
		return ParsedReceipt{}, err
	}
	client := textract.NewFromConfig(cfg)

	out, err := client.AnalyzeExpense(ctx, &textract.AnalyzeExpenseInput{
		Document: &types.Document{Bytes: img},
	})
	if err != nil {
		slog.ErrorContext(ctx, "textract scan failed",
			"ocr_provider", "textract", "ocr_model", "AnalyzeExpense", "ocr_backend", "aws_textract",
			"error", err,
		)
		return ParsedReceipt{}, err
	}

	var rec ParsedReceipt
	for _, doc := range out.ExpenseDocuments {
		for _, f := range doc.SummaryFields {
			label := textractFieldType(f)
			switch label {
			case "VENDOR_NAME", "NAME":
				if rec.Merchant == "" {
					rec.Merchant = textractValueText(f)
				}
			case "SUBTOTAL":
				rec.SubtotalSen = parseSen(textractValueText(f))
			case "TAX":
				rec.SstSen += parseSen(textractValueText(f))
			case "TOTAL", "AMOUNT_DUE":
				if rec.TotalSen == 0 {
					rec.TotalSen = parseSen(textractValueText(f))
				}
			}
		}
		for _, group := range doc.LineItemGroups {
			for _, li := range group.LineItems {
				item := ParsedItem{Qty: 1}
				for _, f := range li.LineItemExpenseFields {
					switch textractFieldType(f) {
					case "ITEM", "DESCRIPTION":
						item.Name = textractValueText(f)
					case "QUANTITY":
						if q, err := strconv.Atoi(strings.TrimSpace(textractValueText(f))); err == nil && q > 0 {
							item.Qty = q
						}
					case "PRICE", "AMOUNT":
						item.LineTotalSen = parseSen(textractValueText(f))
					case "UNIT_PRICE":
						item.UnitPriceSen = parseSen(textractValueText(f))
					}
				}
				if item.Name == "" {
					continue
				}
				if item.LineTotalSen == 0 && item.UnitPriceSen != 0 {
					item.LineTotalSen = item.UnitPriceSen * int64(item.Qty)
				}
				if item.UnitPriceSen == 0 && item.Qty > 0 {
					item.UnitPriceSen = item.LineTotalSen / int64(item.Qty)
				}
				rec.Items = append(rec.Items, item)
			}
		}
	}

	slog.InfoContext(ctx, "textract scan ok",
		"ocr_provider", "textract",
		"ocr_model", "AnalyzeExpense",
		"ocr_backend", "aws_textract",
		"latency_ms", time.Since(start).Milliseconds(),
		"merchant", rec.Merchant,
		"items", len(rec.Items),
		"total_sen", rec.TotalSen,
	)
	return rec, nil
}

func textractFieldType(f types.ExpenseField) string {
	if f.Type != nil && f.Type.Text != nil {
		return *f.Type.Text
	}
	return ""
}

func textractValueText(f types.ExpenseField) string {
	if f.ValueDetection != nil && f.ValueDetection.Text != nil {
		return *f.ValueDetection.Text
	}
	return ""
}
