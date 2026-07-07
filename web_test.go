package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"
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

func TestPercentCollected(t *testing.T) {
	tests := []struct {
		collected, total int64
		want             int
	}{
		{0, 9080, 0},
		{2959, 9080, 32},  // friends paid, bill not fully collected
		{9080, 9080, 100}, // full bill collected
		{100, 0, 0},
		{150, 100, 100}, // capped at 100
	}
	for _, tc := range tests {
		if got := percentCollected(tc.collected, tc.total); got != tc.want {
			t.Errorf("percentCollected(%d, %d) = %d, want %d", tc.collected, tc.total, got, tc.want)
		}
	}
}

func TestTrackPageProgressUsesBillTotal(t *testing.T) {
	s := &Server{cfg: Config{PublicBaseURL: "https://split.my"}}
	if err := s.initWeb(); err != nil {
		t.Fatalf("initWeb: %v", err)
	}
	created := mustParseTime(t, "2026-07-07T12:00:00Z")
	data := trackPageData{
		Split: Split{
			Merchant:  "KFC",
			Slug:      "abc123",
			TotalSen:  9080,
			CreatedAt: &created,
		},
		CollectedSen:       2959,
		FriendsExpectedSen: 2959, // all friends paid their share
	}
	var buf bytes.Buffer
	if err := s.templates.ExecuteTemplate(&buf, "owner/track.html", data); err != nil {
		t.Fatalf("render track: %v", err)
	}
	out := buf.String()
	for _, want := range []string{
		"RM 29.59",
		"of RM 90.80 collected",
		`width:32%`,
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected %q in track page, got:\n%s", want, out)
		}
	}
	for _, bad := range []string{"of RM 29.59 collected", `width:100%`, "All friends paid"} {
		if strings.Contains(out, bad) {
			t.Fatalf("unexpected %q in track page (friends-only denominator bug)", bad)
		}
	}
}

func TestTrackAllFriendsPaidBanner(t *testing.T) {
	tests := []struct {
		name      string
		collected int64
		total     int64
		allMarked bool
		parts     int
		want      bool
	}{
		{"friends paid partial bill", 2959, 9080, true, 2, false},
		{"full bill collected", 9080, 9080, true, 2, true},
		{"unpaid friend", 9080, 9080, false, 2, false},
		{"solo split", 9080, 9080, true, 1, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := trackAllFriendsPaidBanner(tc.collected, tc.total, tc.allMarked, tc.parts)
			if got != tc.want {
				t.Fatalf("got %v want %v", got, tc.want)
			}
		})
	}
}

func TestTrackPageAllPaidRequiresFullBill(t *testing.T) {
	s := &Server{cfg: Config{PublicBaseURL: "https://split.my"}}
	if err := s.initWeb(); err != nil {
		t.Fatalf("initWeb: %v", err)
	}
	created := mustParseTime(t, "2026-07-07T12:00:00Z")
	// All friends marked paid but bill not fully collected — no success banner.
	data := trackPageData{
		Split: Split{
			Merchant:  "KFC",
			Slug:      "abc123",
			TotalSen:  9080,
			CreatedAt: &created,
		},
		CollectedSen:       2959,
		FriendsExpectedSen: 2959,
		AllPaid:            false,
		Participants: []Participant{
			{Name: "owner", IsOwner: true},
			{Name: "Alex", Paid: true, OwedSen: 2959},
		},
	}
	var buf bytes.Buffer
	if err := s.templates.ExecuteTemplate(&buf, "owner/track.html", data); err != nil {
		t.Fatalf("render track: %v", err)
	}
	if strings.Contains(buf.String(), "All friends paid") {
		t.Fatal("expected no All friends paid banner when collected < total")
	}

	data.AllPaid = true
	data.CollectedSen = 9080
	buf.Reset()
	if err := s.templates.ExecuteTemplate(&buf, "owner/track.html", data); err != nil {
		t.Fatalf("render track: %v", err)
	}
	if !strings.Contains(buf.String(), "All friends paid") {
		t.Fatal("expected All friends paid banner when collected >= total")
	}
}

func mustParseTime(t *testing.T, s string) time.Time {
	t.Helper()
	ts, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatalf("parse time: %v", err)
	}
	return ts
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
