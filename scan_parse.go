package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

// llmReceiptResponse is the raw JSON shape returned by the vision model.
type llmReceiptResponse struct {
	Merchant    string             `json:"merchant"`
	SubtotalSen int64              `json:"subtotalSen"`
	SstSen      int64              `json:"sstSen"`
	ServiceSen  int64              `json:"serviceSen"`
	RoundingSen int64              `json:"roundingSen"`
	TotalSen    int64              `json:"totalSen"`
	TaxSen      int64              `json:"taxSen"` // legacy fallback
	Items       []llmParsedItem    `json:"items"`
	Confidence  *ReceiptConfidence `json:"confidence"`
}

type llmParsedItem struct {
	Name         string  `json:"name"`
	Qty          int     `json:"qty"`
	UnitPriceSen int64   `json:"unitPriceSen"`
	LineTotalSen int64   `json:"lineTotalSen"`
	Confidence   float64 `json:"confidence"`
}

func parseLLMReceiptJSON(raw string) (ParsedReceipt, error) {
	raw = extractJSON(raw)
	var resp llmReceiptResponse
	dec := json.NewDecoder(bytes.NewReader([]byte(raw)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&resp); err != nil {
		// Retry without DisallowUnknownFields for slightly malformed payloads.
		if err2 := json.Unmarshal([]byte(raw), &resp); err2 != nil {
			return ParsedReceipt{}, fmt.Errorf("parse llm json: %w", err)
		}
	}
	return mapLLMResponse(resp), nil
}

func mapLLMResponse(resp llmReceiptResponse) ParsedReceipt {
	rec := ParsedReceipt{
		Merchant:    strings.TrimSpace(resp.Merchant),
		SubtotalSen: resp.SubtotalSen,
		SstSen:      resp.SstSen,
		ServiceSen:  resp.ServiceSen,
		RoundingSen: resp.RoundingSen,
		TotalSen:    resp.TotalSen,
		Confidence:  resp.Confidence,
	}
	if rec.SstSen == 0 && resp.TaxSen > 0 {
		rec.SstSen = resp.TaxSen
	}
	for _, it := range resp.Items {
		name := strings.TrimSpace(it.Name)
		if name == "" {
			continue
		}
		qty := it.Qty
		if qty < 1 {
			qty = 1
		}
		item := ParsedItem{
			Name:         name,
			Qty:          qty,
			UnitPriceSen: it.UnitPriceSen,
			LineTotalSen: it.LineTotalSen,
			Confidence:   it.Confidence,
		}
		if item.LineTotalSen == 0 && item.UnitPriceSen != 0 {
			item.LineTotalSen = item.UnitPriceSen * int64(qty)
		}
		if item.UnitPriceSen == 0 && qty > 0 && item.LineTotalSen != 0 {
			item.UnitPriceSen = item.LineTotalSen / int64(qty)
		}
		rec.Items = append(rec.Items, item)
	}
	return rec
}

func extractJSON(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```") {
		s = strings.TrimPrefix(s, "```json")
		s = strings.TrimPrefix(s, "```")
		if i := strings.LastIndex(s, "```"); i >= 0 {
			s = s[:i]
		}
		s = strings.TrimSpace(s)
	}
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start >= 0 && end > start {
		return s[start : end+1]
	}
	return s
}
