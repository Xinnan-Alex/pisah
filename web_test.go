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
	if !strings.Contains(out, `placeholder="Restaurant name"`) {
		t.Fatal("expected editable merchant input")
	}
}

func TestShareDisplayURLStripsScheme(t *testing.T) {
	s := &Server{cfg: Config{PublicBaseURL: "https://pisah.leongxinnan.com"}}
	got := s.shareDisplayURL("sLJi")
	want := "pisah.leongxinnan.com/r/sLJi"
	if got != want {
		t.Fatalf("shareDisplayURL = %q, want %q", got, want)
	}
}

func TestSharePageShowsShortURL(t *testing.T) {
	s := &Server{cfg: Config{PublicBaseURL: "https://pisah.leongxinnan.com"}}
	if err := s.initWeb(); err != nil {
		t.Fatalf("initWeb: %v", err)
	}
	data := sharePageData{
		Slug:            "sLJi",
		ShareURL:        s.shareURL("sLJi"),
		ShareDisplayURL: s.shareDisplayURL("sLJi"),
	}
	var buf bytes.Buffer
	if err := s.templates.ExecuteTemplate(&buf, "owner/share.html", data); err != nil {
		t.Fatalf("render share: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, `value="pisah.leongxinnan.com/r/sLJi"`) {
		t.Fatalf("expected short display URL in input, got:\n%s", out)
	}
	if !strings.Contains(out, `data-full-url="https://pisah.leongxinnan.com/r/sLJi"`) {
		t.Fatalf("expected full URL in data-full-url, got:\n%s", out)
	}
}
