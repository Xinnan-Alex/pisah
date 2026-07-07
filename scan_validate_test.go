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

func TestRepairMisassignedTotals_totalInSST(t *testing.T) {
	rec := ParsedReceipt{
		Merchant:    "KFC TRX",
		SubtotalSen: 2168,
		SstSen:      2298, // LLM put printed Total here
		TotalSen:    0,
		Items: []ParsedItem{
			{Name: "SEAWEED SHAKER FRIES (M)", LineTotalSen: 599},
			{Name: "1-PC NASI LEMAK KFC COMBO", LineTotalSen: 1699},
		},
	}
	repairMisassignedTotals(&rec)
	if rec.SstSen != 130 {
		t.Fatalf("sst = %d, want 130", rec.SstSen)
	}
	if rec.TotalSen != 2298 {
		t.Fatalf("total = %d, want 2298", rec.TotalSen)
	}
}

func TestRepairMisassignedTotals_taxInServiceCharge(t *testing.T) {
	rec := ParsedReceipt{
		SubtotalSen: 2168,
		SstSen:      2298,
		ServiceSen:  130,
		TotalSen:    2300,
	}
	repairMisassignedTotals(&rec)
	if rec.SstSen != 130 {
		t.Fatalf("sst = %d, want 130", rec.SstSen)
	}
	if rec.ServiceSen != 0 {
		t.Fatalf("service = %d, want 0", rec.ServiceSen)
	}
	if rec.TotalSen != 2298 {
		t.Fatalf("total = %d, want 2298", rec.TotalSen)
	}
	if rec.RoundingSen != 2 {
		t.Fatalf("rounding = %d, want 2", rec.RoundingSen)
	}
}

func TestRepairMisassignedTotals_correctParseUnchanged(t *testing.T) {
	rec := ParsedReceipt{
		SubtotalSen: 2168,
		SstSen:      130,
		TotalSen:    2298,
		RoundingSen: 0,
	}
	repairMisassignedTotals(&rec)
	if rec.SubtotalSen != 2168 || rec.SstSen != 130 || rec.TotalSen != 2298 || rec.RoundingSen != 0 {
		t.Fatalf("correct receipt changed: %+v", rec)
	}
}

func TestRepairMisassignedLineItems_paymentOnLastItem(t *testing.T) {
	rec := ParsedReceipt{
		SubtotalSen: 2168,
		SstSen:      130,
		TotalSen:    2298,
		RoundingSen: 2,
		Items: []ParsedItem{
			{Name: "SEAWEED SHAKER FRIES (M)", Qty: 1, LineTotalSen: 599},
			{Name: "1-PC NASI LEMAK KFC COMBO", Qty: 1, LineTotalSen: 2300},
		},
	}
	repairMisassignedLineItems(&rec)
	if rec.Items[1].LineTotalSen != 1699 {
		t.Fatalf("nasi lemak = %d, want 1699", rec.Items[1].LineTotalSen)
	}
	normalizeParsedReceipt(&rec)
	var sum int64
	for _, it := range rec.Items {
		sum += it.LineTotalSen
	}
	if sum != rec.SubtotalSen {
		t.Fatalf("items sum = %d, want subtotal %d", sum, rec.SubtotalSen)
	}
}

func TestRepairMisassignedLineItems_correctItemsUnchanged(t *testing.T) {
	rec := ParsedReceipt{
		SubtotalSen: 2168,
		SstSen:      130,
		TotalSen:    2298,
		Items: []ParsedItem{
			{Name: "SEAWEED SHAKER FRIES (M)", Qty: 1, LineTotalSen: 599},
			{Name: "1-PC NASI LEMAK KFC COMBO", Qty: 1, LineTotalSen: 1699},
		},
	}
	before := []int64{rec.Items[0].LineTotalSen, rec.Items[1].LineTotalSen}
	repairMisassignedLineItems(&rec)
	if rec.Items[0].LineTotalSen != before[0] || rec.Items[1].LineTotalSen != before[1] {
		t.Fatalf("correct items changed: %+v", rec.Items)
	}
}
