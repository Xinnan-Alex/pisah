package main

import (
	"context"
	"testing"
)

type mockScanner struct {
	rec ParsedReceipt
	err error
}

func (m *mockScanner) Scan(ctx context.Context, img []byte) (ParsedReceipt, error) {
	return m.rec, m.err
}

func TestScanPipeline_Process(t *testing.T) {
	pipeline := &ScanPipeline{
		scanner: &mockScanner{rec: ParsedReceipt{
			Merchant:    "Mock Cafe",
			SubtotalSen: 1000,
			SstSen:      60,
			TotalSen:    1060,
			Items:       []ParsedItem{{Name: "Kopi", Qty: 1, LineTotalSen: 1000}},
		}},
		provider: "mock",
		model:    "mock",
	}
	img := makeJPEG(t, 800, 600)
	rec, warnings, err := pipeline.Process(context.Background(), img)
	if err != nil {
		t.Fatalf("process: %v", err)
	}
	if rec.Merchant != "Mock Cafe" {
		t.Fatalf("merchant = %q", rec.Merchant)
	}
	if len(warnings) > 0 {
		t.Logf("warnings: %v", warnings)
	}
}
