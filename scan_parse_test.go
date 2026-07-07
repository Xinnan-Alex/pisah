package main

import (
	"testing"
)

func TestParseLLMReceiptJSON(t *testing.T) {
	raw := `{
		"merchant": "Warung Pak Ali",
		"subtotalSen": 4500,
		"sstSen": 270,
		"serviceSen": 450,
		"roundingSen": 0,
		"totalSen": 5220,
		"items": [
			{"name": "Nasi Lemak", "qty": 1, "unitPriceSen": 2500, "lineTotalSen": 2500, "confidence": 0.95},
			{"name": "Teh Tarik", "qty": 2, "unitPriceSen": 1000, "lineTotalSen": 2000, "confidence": 0.88}
		],
		"confidence": {"merchant": 0.9, "total": 0.92, "items": 0.85}
	}`
	rec, err := parseLLMReceiptJSON(raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if rec.Merchant != "Warung Pak Ali" {
		t.Fatalf("merchant = %q", rec.Merchant)
	}
	if len(rec.Items) != 2 || rec.SstSen != 270 || rec.ServiceSen != 450 {
		t.Fatalf("unexpected rec: %+v", rec)
	}
}

func TestParseLLMReceiptJSON_markdownFence(t *testing.T) {
	raw := "```json\n{\"merchant\":\"Cafe\",\"totalSen\":500,\"items\":[]}\n```"
	rec, err := parseLLMReceiptJSON(raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if rec.Merchant != "Cafe" || rec.TotalSen != 500 {
		t.Fatalf("got %+v", rec)
	}
}

func TestParseLLMReceiptJSON_taxFallback(t *testing.T) {
	raw := `{"merchant":"X","taxSen":60,"totalSen":1060,"items":[{"name":"A","qty":1,"lineTotalSen":1000}]}`
	rec, err := parseLLMReceiptJSON(raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if rec.SstSen != 60 {
		t.Fatalf("sst = %d, want 60", rec.SstSen)
	}
}
