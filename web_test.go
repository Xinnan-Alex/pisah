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
		"owner/signup.html",
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
		"partials/onboarding_walkthrough.html",
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

func TestCapturePageOnboardingAndQRGate(t *testing.T) {
	s := &Server{cfg: Config{PublicBaseURL: "https://split.my"}}
	if err := s.initWeb(); err != nil {
		t.Fatalf("initWeb: %v", err)
	}

	t.Run("first visit shows welcome and qr gate", func(t *testing.T) {
		data := capturePageData{
			Profile:        OwnerProfile{AutoFillAmount: true},
			ShowOnboarding: true,
			HasQR:          false,
		}
		var buf bytes.Buffer
		if err := s.templates.ExecuteTemplate(&buf, "owner/capture.html", data); err != nil {
			t.Fatalf("render capture: %v", err)
		}
		out := buf.String()
		for _, want := range []string{
			`data-show-onboarding="1"`,
			`id="onboarding-modal"`,
			`aria-label="How to use Pisah"`,
			`Upload your DuitNow QR`,
			`viewfinder-section-locked`,
			`Upload QR to unlock scanning`,
		} {
			if !strings.Contains(out, want) {
				t.Fatalf("expected %q in capture page, got:\n%s", want, out)
			}
		}
	})

	t.Run("summary strip always visible", func(t *testing.T) {
		data := capturePageData{
			Profile:        OwnerProfile{AutoFillAmount: true},
			ShowOnboarding: false,
			HasQR:          true,
		}
		var buf bytes.Buffer
		if err := s.templates.ExecuteTemplate(&buf, "owner/capture.html", data); err != nil {
			t.Fatalf("render capture: %v", err)
		}
		out := buf.String()
		if !strings.Contains(out, `id="summary-strip"`) {
			t.Fatal("expected summary strip with zero splits")
		}
		if strings.Contains(out, `id="capture-splits"`) {
			t.Fatal("expected no splits section when list is empty")
		}
		for _, bad := range []string{"⚙", "↪"} {
			if strings.Contains(out, bad) {
				t.Fatalf("expected SVG header icons, found unicode %q", bad)
			}
		}
		if !strings.Contains(out, `aria-label="Payment settings"`) || !strings.Contains(out, `viewBox="0 0 24 24"`) {
			t.Fatal("expected SVG icons in capture header")
		}
	})

	t.Run("returning owner with qr unlocks scanning", func(t *testing.T) {
		qr := "https://example.com/qr.png"
		data := capturePageData{
			Profile:        OwnerProfile{OwnerQRURL: &qr, AutoFillAmount: true},
			ShowOnboarding: false,
			HasQR:          true,
		}
		var buf bytes.Buffer
		if err := s.templates.ExecuteTemplate(&buf, "owner/capture.html", data); err != nil {
			t.Fatalf("render capture: %v", err)
		}
		out := buf.String()
		if strings.Contains(out, `viewfinder-section-locked`) {
			t.Fatal("expected unlocked capture UI when QR exists")
		}
		if !strings.Contains(out, `Snap your receipt`) {
			t.Fatal("expected scan hero when QR exists")
		}
		if !strings.Contains(out, `id="scan-form"`) {
			t.Fatal("expected scan form when QR exists")
		}
	})
}

func TestOwnerProfileHasQR(t *testing.T) {
	if ownerProfileHasQR(OwnerProfile{}) {
		t.Fatal("empty profile should not have QR")
	}
	qr := "https://example.com/qr.png"
	if !ownerProfileHasQR(OwnerProfile{OwnerQRURL: &qr}) {
		t.Fatal("profile with URL should have QR")
	}
	empty := ""
	if ownerProfileHasQR(OwnerProfile{OwnerQRURL: &empty}) {
		t.Fatal("empty URL should not count as QR")
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
