package main

import "testing"

func TestValidateReceipt_warnings(t *testing.T) {
	rec := ParsedReceipt{
		Merchant: "",
		TotalSen: 1000,
		Items:    []ParsedItem{{Name: "Tea", LineTotalSen: 1000}},
	}
	w := validateReceipt(&rec)
	if len(w) == 0 {
		t.Fatal("expected warnings")
	}
	found := false
	for _, code := range w {
		if code == "no_merchant" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected no_merchant, got %v", w)
	}
}

func TestValidateReceipt_totalMismatch(t *testing.T) {
	rec := ParsedReceipt{
		Merchant:    "Cafe",
		SubtotalSen: 1000,
		SstSen:      60,
		TotalSen:    2000,
		Items:       []ParsedItem{{Name: "Tea", LineTotalSen: 1000}},
	}
	enrichParsedReceipt(&rec)
	w := validateReceipt(&rec)
	mismatch := false
	for _, code := range w {
		if code == "total_mismatch" {
			mismatch = true
		}
	}
	if !mismatch {
		t.Fatalf("expected total_mismatch, got %v", w)
	}
}

func TestEnrichParsedReceipt_infersSubtotal(t *testing.T) {
	rec := ParsedReceipt{
		Items: []ParsedItem{
			{Name: "A", LineTotalSen: 500},
			{Name: "B", LineTotalSen: 700},
		},
	}
	enrichParsedReceipt(&rec)
	if rec.SubtotalSen != 1200 {
		t.Fatalf("subtotal = %d, want 1200", rec.SubtotalSen)
	}
}

func TestEnrichParsedReceipt_infersRounding(t *testing.T) {
	rec := ParsedReceipt{
		SubtotalSen: 2168,
		SstSen:      130,
		TotalSen:    2298,
		Items:       []ParsedItem{{Name: "Meal", LineTotalSen: 2168}},
	}
	enrichParsedReceipt(&rec)
	if rec.RoundingSen != 0 && rec.TotalSen != 2298 {
		t.Fatalf("unexpected rounding/total: %+v", rec)
	}
}
