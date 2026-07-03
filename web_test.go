package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestWebTemplatesParse(t *testing.T) {
	s := &Server{cfg: Config{PublicBaseURL: "https://split.my"}}
	if err := s.initWeb(); err != nil {
		t.Fatalf("initWeb: %v", err)
	}
	pages := []string{
		"layout.html",
		"owner/signin.html",
		"owner/capture.html",
		"owner/review.html",
		"owner/share.html",
		"owner/track.html",
		"owner/settings.html",
		"friend/landing.html",
		"friend/pick.html",
		"friend/share.html",
		"friend/pay.html",
		"friend/done.html",
		"partials/splits_list.html",
		"partials/track_participants.html",
		"auth/callback.html",
	}
	for _, p := range pages {
		if s.templates.Lookup(p) == nil {
			t.Fatalf("missing template %q", p)
		}
	}
}

func TestReviewPageEmbedsReceiptJSONOnce(t *testing.T) {
	s := &Server{cfg: Config{PublicBaseURL: "https://split.my"}}
	if err := s.initWeb(); err != nil {
		t.Fatalf("initWeb: %v", err)
	}
	data := reviewPageData{
		Merchant:    "Test Cafe",
		SubtotalSen: 1500,
		Items: []ParsedItem{
			{Name: "Latte", Qty: 1, UnitPriceSen: 1500, LineTotalSen: 1500},
		},
		CapturedAt: "3 Jul · 9:24 PM",
	}
	var buf bytes.Buffer
	if err := s.templates.ExecuteTemplate(&buf, "owner/review.html", data); err != nil {
		t.Fatalf("render review: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, `id="receipt-data"`) {
		t.Fatalf("expected receipt JSON script tag, got:\n%s", out)
	}
	start := strings.Index(out, `<script type="application/json" id="receipt-data">`)
	if start < 0 {
		t.Fatal("missing receipt-data script")
	}
	start += len(`<script type="application/json" id="receipt-data">`)
	end := strings.Index(out[start:], "</script>")
	if end < 0 {
		t.Fatal("unclosed receipt-data script")
	}
	payload := out[start : start+end]
	var parsed map[string]any
	if err := json.Unmarshal([]byte(payload), &parsed); err != nil {
		t.Fatalf("receipt JSON invalid: %v\npayload: %s", err, payload)
	}
	items, _ := parsed["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("expected 1 item in JSON payload, got %v", parsed["items"])
	}
	if strings.Contains(out, "Test Cafe") == false {
		t.Fatal("expected merchant in page")
	}
}
