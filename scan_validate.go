package main

import (
	"strconv"
	"strings"
)

// ParsedReceipt is the owner-reviewable result of an OCR scan. All money in sen.
type ParsedReceipt struct {
	Merchant    string             `json:"merchant"`
	SubtotalSen int64              `json:"subtotalSen"`
	SstSen      int64              `json:"sstSen"`
	ServiceSen  int64              `json:"serviceSen"`
	RoundingSen int64              `json:"roundingSen"`
	TotalSen    int64              `json:"totalSen"`
	Items       []ParsedItem       `json:"items"`
	Confidence  *ReceiptConfidence `json:"confidence,omitempty"`
}

type ParsedItem struct {
	Name         string  `json:"name"`
	Qty          int     `json:"qty"`
	UnitPriceSen int64   `json:"unitPriceSen"`
	LineTotalSen int64   `json:"lineTotalSen"`
	Confidence   float64 `json:"confidence,omitempty"`
}

func itemsSumSen(items []ParsedItem) int64 {
	var sum int64
	for _, it := range items {
		sum += it.LineTotalSen
	}
	return sum
}

func taxTotalSen(rec *ParsedReceipt) int64 {
	return rec.SstSen + rec.ServiceSen + rec.RoundingSen
}

// enrichParsedReceipt fills missing subtotal/rounding from line items and totals.
func enrichParsedReceipt(rec *ParsedReceipt) {
	if rec.SubtotalSen <= 0 && len(rec.Items) > 0 {
		rec.SubtotalSen = itemsSumSen(rec.Items)
	}
	if rec.RoundingSen == 0 && rec.TotalSen > 0 && rec.SubtotalSen > 0 {
		tax := rec.SstSen + rec.ServiceSen
		rounding := rec.TotalSen - rec.SubtotalSen - tax
		if rounding > 0 && rounding < 100 {
			rec.RoundingSen = rounding
		}
	}
	if rec.TotalSen <= 0 && rec.SubtotalSen > 0 {
		rec.TotalSen = rec.SubtotalSen + taxTotalSen(rec)
	}
}

// validateReceipt returns human-facing warning codes for the review UI.
func validateReceipt(rec *ParsedReceipt) []string {
	var warnings []string
	if strings.TrimSpace(rec.Merchant) == "" {
		warnings = append(warnings, "no_merchant")
	}
	if len(rec.Items) == 0 {
		warnings = append(warnings, "no_items")
	}
	if rec.TotalSen <= 0 {
		warnings = append(warnings, "no_total")
	}
	itemsSum := itemsSumSen(rec.Items)
	computed := rec.SubtotalSen + taxTotalSen(rec)
	if rec.TotalSen > 0 && computed > 0 && !senClose(computed, rec.TotalSen) {
		// Also check items-only sum when subtotal missing.
		alt := itemsSum + taxTotalSen(rec)
		if !senClose(alt, rec.TotalSen) {
			warnings = append(warnings, "total_mismatch")
		}
	}
	if rec.Confidence != nil {
		if rec.Confidence.Merchant > 0 && rec.Confidence.Merchant < 0.7 {
			warnings = append(warnings, "low_confidence_merchant")
		}
		if rec.Confidence.Total > 0 && rec.Confidence.Total < 0.7 {
			warnings = append(warnings, "low_confidence_total")
		}
		if rec.Confidence.Items > 0 && rec.Confidence.Items < 0.7 {
			warnings = append(warnings, "low_confidence_items")
		}
	}
	for _, it := range rec.Items {
		if it.Confidence > 0 && it.Confidence < 0.7 {
			warnings = append(warnings, "low_confidence_item")
			break
		}
	}
	seen := map[string]int{}
	for _, it := range rec.Items {
		key := strings.ToLower(strings.TrimSpace(it.Name)) + "|" + strconv.FormatInt(it.LineTotalSen, 10)
		seen[key]++
		if seen[key] > 1 {
			warnings = append(warnings, "possible_duplicate")
			break
		}
	}
	return warnings
}

// normalizeParsedReceipt fixes Malaysian receipts (e.g. KFC) where line-item prices
// are tax-inclusive but the summary shows a pre-tax subtotal plus SST.
func normalizeParsedReceipt(rec *ParsedReceipt) {
	tax := rec.SstSen + rec.ServiceSen
	if rec.SubtotalSen <= 0 || tax <= 0 || len(rec.Items) == 0 {
		return
	}
	itemsSum := itemsSumSen(rec.Items)
	if itemsSum <= 0 {
		return
	}
	expectedTotal := rec.SubtotalSen + tax + rec.RoundingSen
	if rec.TotalSen == 0 {
		rec.TotalSen = expectedTotal
	}
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
	return a-b <= 2
}

func roundDiv(a, b int64) int64 {
	if b <= 0 {
		return 0
	}
	return (a + b/2) / b
}

// parseSen turns a money string like "RM 12.50", "12.50", "1,234.5" into sen.
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
	return int64(f*100 + 0.5)
}

// warningMessage returns user-facing copy for a warning code.
func warningMessage(code string) string {
	switch code {
	case "no_merchant":
		return "We couldn't read the restaurant name — please fill it in."
	case "no_items":
		return "No line items were found — add items manually."
	case "no_total":
		return "We couldn't read the total — please check the amount."
	case "total_mismatch":
		return "Items and tax don't add up to the total — please double-check SST, service charge, and rounding."
	case "low_confidence_merchant":
		return "The merchant name may be wrong — please verify."
	case "low_confidence_total":
		return "The total may be wrong — please verify against your receipt."
	case "low_confidence_items", "low_confidence_item":
		return "Some line items may be wrong — check highlighted rows."
	case "possible_duplicate":
		return "Some items look duplicated — remove any extras."
	default:
		return "Please review the details before sharing."
	}
}
