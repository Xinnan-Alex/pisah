package main

import (
	"context"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/textract"
	"github.com/aws/aws-sdk-go-v2/service/textract/types"
)

// ParsedReceipt is the owner-reviewable result of an OCR scan. All money in sen.
type ParsedReceipt struct {
	Merchant    string       `json:"merchant"`
	SubtotalSen int64        `json:"subtotalSen"`
	TaxSen      int64        `json:"taxSen"` // Textract lumps SST + service under TAX
	TotalSen    int64        `json:"totalSen"`
	Items       []ParsedItem `json:"items"`
}

type ParsedItem struct {
	Name         string `json:"name"`
	Qty          int    `json:"qty"`
	UnitPriceSen int64  `json:"unitPriceSen"`
	LineTotalSen int64  `json:"lineTotalSen"`
}

// scanReceipt runs AWS Textract AnalyzeExpense on a receipt image and maps the
// typed expense document to ParsedReceipt. AWS credentials + region come from the
// standard environment (AWS_REGION, AWS_ACCESS_KEY_ID, ...).
func scanReceipt(ctx context.Context, img []byte) (ParsedReceipt, error) {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return ParsedReceipt{}, err
	}
	client := textract.NewFromConfig(cfg)

	out, err := client.AnalyzeExpense(ctx, &textract.AnalyzeExpenseInput{
		Document: &types.Document{Bytes: img},
	})
	if err != nil {
		return ParsedReceipt{}, err
	}

	var rec ParsedReceipt
	for _, doc := range out.ExpenseDocuments {
		for _, f := range doc.SummaryFields {
			label := fieldType(f)
			switch label {
			case "VENDOR_NAME", "NAME":
				if rec.Merchant == "" {
					rec.Merchant = valueText(f)
				}
			case "SUBTOTAL":
				rec.SubtotalSen = parseSen(valueText(f))
			case "TAX":
				rec.TaxSen += parseSen(valueText(f))
			case "TOTAL", "AMOUNT_DUE":
				if rec.TotalSen == 0 {
					rec.TotalSen = parseSen(valueText(f))
				}
			}
		}
		for _, group := range doc.LineItemGroups {
			for _, li := range group.LineItems {
				item := ParsedItem{Qty: 1}
				for _, f := range li.LineItemExpenseFields {
					switch fieldType(f) {
					case "ITEM", "DESCRIPTION":
						item.Name = valueText(f)
					case "QUANTITY":
						if q, err := strconv.Atoi(strings.TrimSpace(valueText(f))); err == nil && q > 0 {
							item.Qty = q
						}
					case "PRICE", "AMOUNT":
						item.LineTotalSen = parseSen(valueText(f))
					case "UNIT_PRICE":
						item.UnitPriceSen = parseSen(valueText(f))
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
	normalizeParsedReceipt(&rec)
	return rec, nil
}

// normalizeParsedReceipt fixes Malaysian receipts (e.g. KFC) where line-item prices
// are tax-inclusive but the summary shows a pre-tax subtotal plus SST. Without this,
// split math treats item prices as net and adds tax again on top.
func normalizeParsedReceipt(rec *ParsedReceipt) {
	if rec.SubtotalSen <= 0 || rec.TaxSen <= 0 || len(rec.Items) == 0 {
		return
	}
	var itemsSum int64
	for _, it := range rec.Items {
		itemsSum += it.LineTotalSen
	}
	if itemsSum <= 0 {
		return
	}
	expectedTotal := rec.SubtotalSen + rec.TaxSen
	if rec.TotalSen == 0 {
		rec.TotalSen = expectedTotal
	}
	// Tax-inclusive when item prices sum to the bill total, not the pre-tax subtotal.
	if itemsSum <= rec.SubtotalSen {
		return
	}
	if !senClose(itemsSum, rec.TotalSen) || !senClose(expectedTotal, rec.TotalSen) {
		return
	}
	scaleItemsToNet(rec, rec.SubtotalSen, itemsSum)
}

func scaleItemsToNet(rec *ParsedReceipt, netTotal, grossTotal int64) {
	var scaled int64
	for i := range rec.Items {
		it := &rec.Items[i]
		it.LineTotalSen = roundDiv(it.LineTotalSen*netTotal, grossTotal)
		if it.Qty > 0 {
			it.UnitPriceSen = it.LineTotalSen / int64(it.Qty)
		}
		scaled += it.LineTotalSen
	}
	if drift := netTotal - scaled; drift != 0 {
		for i := len(rec.Items) - 1; i >= 0; i-- {
			if rec.Items[i].LineTotalSen > 0 {
				rec.Items[i].LineTotalSen += drift
				if rec.Items[i].Qty > 0 {
					rec.Items[i].UnitPriceSen = rec.Items[i].LineTotalSen / int64(rec.Items[i].Qty)
				}
				break
			}
		}
	}
}

func senClose(a, b int64) bool {
	if a < b {
		a, b = b, a
	}
	return a-b <= 2 // tolerate small OCR drift
}

func roundDiv(a, b int64) int64 {
	if b <= 0 {
		return 0
	}
	return (a + b/2) / b
}

func fieldType(f types.ExpenseField) string {
	if f.Type != nil && f.Type.Text != nil {
		return *f.Type.Text
	}
	return ""
}

func valueText(f types.ExpenseField) string {
	if f.ValueDetection != nil && f.ValueDetection.Text != nil {
		return *f.ValueDetection.Text
	}
	return ""
}

// parseSen turns a money string like "RM 12.50", "12.50", "1,234.5" into sen.
// Best-effort: returns 0 on anything it can't read (owner edits before saving).
func parseSen(s string) int64 {
	var b strings.Builder
	for _, r := range s {
		if (r >= '0' && r <= '9') || r == '.' {
			b.WriteRune(r)
		}
	}
	f, err := strconv.ParseFloat(b.String(), 64)
	if err != nil {
		return 0
	}
	return int64(f*100 + 0.5) // round to nearest sen
}
