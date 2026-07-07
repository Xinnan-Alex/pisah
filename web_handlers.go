package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type capturePageData struct {
	Splits            []SplitSummary
	OutstandingSen    int64
	CollectedTotalSen int64
	ActiveCount       int
	Profile           OwnerProfile
	HasQR             bool
	ShowOnboarding    bool
	SetupRequired     bool
	ScanError         string
}

type reviewPageData struct {
	ScanID      string       `json:"scanId,omitempty"`
	Merchant    string       `json:"merchant"`
	SubtotalSen int64        `json:"subtotalSen"`
	SstSen      int64        `json:"sstSen"`
	ServiceSen  int64        `json:"serviceSen"`
	RoundingSen int64        `json:"roundingSen"`
	TotalSen    int64        `json:"totalSen"`
	Items       []ParsedItem `json:"items"`
	Warnings    []string     `json:"warnings,omitempty"`
	ImageURL    string       `json:"imageUrl,omitempty"`
	CapturedAt  string       `json:"-"`
	ManualEntry bool         `json:"manualEntry,omitempty"`
	RescanError string       `json:"-"`
}

type sharePageData struct {
	Slug            string
	ShareURL        string
	ShareDisplayURL string
}

type trackPageData struct {
	Split              Split
	CollectedSen       int64
	FriendsExpectedSen int64
	Participants       []Participant
	NudgeName          string
	AllPaid            bool
}

type friendLandingData struct {
	Slug      string
	Split     Split
	ItemCount int
	Name      string
}

type friendPickData struct {
	Slug string
	Me   string
	Pick map[string]any
}

type shareLineView struct {
	Name   string
	AmtSen int64
}

type shareBreakdownView struct {
	Merchant       string
	OwnerName      string
	OwnerQRURL     *string
	AutoFillAmount bool
	Lines          []shareLineView
	TaxSen         int64
	OwedSen        int64
}

type friendSharePageData struct {
	Slug  string
	Share shareBreakdownView
}

type friendPayPageData struct {
	Slug  string
	Share shareBreakdownView
}

type friendDonePageData struct {
	Me        string
	OwnerName string
	Merchant  string
	OwedSen   int64
}

func (s *Server) handleWebRoot(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := s.ownerFromRequest(w, r); ok {
		http.Redirect(w, r, "/capture", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/signin", http.StatusSeeOther)
}

func (s *Server) handleWebSignInGet(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := s.ownerFromRequest(w, r); ok {
		http.Redirect(w, r, "/capture", http.StatusSeeOther)
		return
	}
	data := map[string]any{
		"Error":     r.URL.Query().Get("error"),
		"Info":      r.URL.Query().Get("info"),
		"Email":     r.URL.Query().Get("email"),
		"GoogleURL": "",
	}
	if s.cfg.SupabaseURL != "" && s.cfg.SupabasePublishableKey != "" {
		data["GoogleURL"] = s.webGoogleOAuthURL(r)
	}
	s.render(w, r, "owner/signin.html", "Sign in · Pisah", data)
}

func (s *Server) handleWebSignInPost(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/signin?error=Invalid+form", http.StatusSeeOther)
		return
	}
	email := strings.TrimSpace(r.FormValue("email"))
	password := r.FormValue("password")
	if email == "" || password == "" {
		http.Redirect(w, r, "/signin?error=Email+and+password+required&email="+url.QueryEscape(email), http.StatusSeeOther)
		return
	}
	if !s.authConfigured() {
		http.Redirect(w, r, "/signin?error=Auth+not+configured", http.StatusSeeOther)
		return
	}
	status, data, err := s.supabaseSignIn(r.Context(), email, password)
	if err != nil {
		http.Redirect(w, r, "/signin?error=Auth+service+unreachable&email="+url.QueryEscape(email), http.StatusSeeOther)
		return
	}
	if status >= 400 {
		http.Redirect(w, r, "/signin?error=Invalid+email+or+password&email="+url.QueryEscape(email), http.StatusSeeOther)
		return
	}
	tok := parseSupabaseSession(data)
	if tok.AccessToken == "" {
		http.Redirect(w, r, "/signin?error=Auth+response+invalid", http.StatusSeeOther)
		return
	}
	s.setSessionCookies(w, tok.AccessToken, tok.RefreshToken)
	http.Redirect(w, r, "/capture", http.StatusSeeOther)
}

func (s *Server) handleWebSignUpGet(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := s.ownerFromRequest(w, r); ok {
		http.Redirect(w, r, "/capture", http.StatusSeeOther)
		return
	}
	data := map[string]any{
		"Error": r.URL.Query().Get("error"),
		"Email": r.URL.Query().Get("email"),
	}
	s.render(w, r, "owner/signup.html", "Sign up · Pisah", data)
}

func (s *Server) handleWebSignUpPost(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/signup?error=Invalid+form", http.StatusSeeOther)
		return
	}
	email := strings.TrimSpace(r.FormValue("email"))
	password := r.FormValue("password")
	confirm := r.FormValue("password_confirm")
	if email == "" || password == "" {
		http.Redirect(w, r, "/signup?error=Email+and+password+required&email="+url.QueryEscape(email), http.StatusSeeOther)
		return
	}
	if password != confirm {
		http.Redirect(w, r, "/signup?error=Passwords+do+not+match&email="+url.QueryEscape(email), http.StatusSeeOther)
		return
	}
	if !s.authConfigured() {
		http.Redirect(w, r, "/signup?error=Auth+not+configured", http.StatusSeeOther)
		return
	}
	redirectTo := strings.TrimRight(s.cfg.PublicBaseURL, "/") + "/auth/callback"
	status, data, err := s.supabaseSignUp(r.Context(), email, password, redirectTo)
	if err != nil {
		http.Redirect(w, r, "/signup?error=Auth+service+unreachable&email="+url.QueryEscape(email), http.StatusSeeOther)
		return
	}
	if status >= 400 {
		msg := supabaseErrorMessage(data)
		if msg == "" {
			msg = "Could not create account"
		}
		http.Redirect(w, r, "/signup?error="+url.QueryEscape(msg)+"&email="+url.QueryEscape(email), http.StatusSeeOther)
		return
	}
	tok := parseSupabaseSession(data)
	if tok.AccessToken != "" {
		s.setSessionCookies(w, tok.AccessToken, tok.RefreshToken)
		http.Redirect(w, r, "/capture", http.StatusSeeOther)
		return
	}
	info := "Check your email to confirm your account, then sign in"
	http.Redirect(w, r, "/signin?info="+url.QueryEscape(info)+"&email="+url.QueryEscape(email), http.StatusSeeOther)
}

func (s *Server) handleWebSignOut(w http.ResponseWriter, r *http.Request) {
	s.clearSessionCookies(w)
	http.Redirect(w, r, "/signin", http.StatusSeeOther)
}

func (s *Server) handleWebAuthCallback(w http.ResponseWriter, r *http.Request) {
	if err := s.templates.ExecuteTemplate(w, "auth/callback.html", nil); err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
	}
}

func (s *Server) handleWebAuthSession(w http.ResponseWriter, r *http.Request) {
	var body struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.AccessToken == "" {
		writeErr(w, http.StatusBadRequest, "access_token required")
		return
	}
	if _, _, err := s.verifyAccessToken(body.AccessToken); err != nil {
		writeErr(w, http.StatusUnauthorized, "invalid token")
		return
	}
	s.setSessionCookies(w, body.AccessToken, body.RefreshToken)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) loadCapturePageData(ctx context.Context, ownerID string, setupRequired bool) (capturePageData, error) {
	summaries, err := s.store.ListOwnerSplits(ctx, ownerID)
	if err != nil {
		return capturePageData{}, err
	}
	var outstanding, collected int64
	active := 0
	for _, sum := range summaries {
		collected += sum.CollectedSen
		rem := sum.Split.TotalSen - sum.CollectedSen
		if rem > 0 {
			outstanding += rem
			active++
		}
	}
	prof, err := s.store.GetOwnerProfile(ctx, ownerID)
	if err != nil {
		return capturePageData{}, err
	}
	hasQR := ownerProfileHasQR(prof)
	return capturePageData{
		Splits:            summaries,
		OutstandingSen:    outstanding,
		CollectedTotalSen: collected,
		ActiveCount:       active,
		Profile:           prof,
		HasQR:             hasQR,
		ShowOnboarding:    prof.OnboardingSeenAt == nil,
		SetupRequired:     setupRequired,
	}, nil
}

func (s *Server) renderCapture(w http.ResponseWriter, r *http.Request, setupRequired bool) {
	ownerID := r.Context().Value(ctxOwnerID).(string)
	data, err := s.loadCapturePageData(r.Context(), ownerID, setupRequired)
	if err != nil {
		writeErrWithLog(r, w, http.StatusInternalServerError, "could not load capture page", err)
		return
	}
	s.render(w, r, "owner/capture.html", "Pisah", data)
}

func (s *Server) ownerQRRequired(w http.ResponseWriter, r *http.Request, ownerID string) bool {
	prof, err := s.store.GetOwnerProfile(r.Context(), ownerID)
	if err != nil {
		writeErrWithLog(r, w, http.StatusInternalServerError, "could not load profile", err)
		return false
	}
	if ownerProfileHasQR(prof) {
		return true
	}
	if r.Header.Get("HX-Request") != "" {
		s.renderCapture(w, r, true)
		return false
	}
	http.Redirect(w, r, "/capture?setup=qr", http.StatusSeeOther)
	return false
}

func (s *Server) handleWebCapture(w http.ResponseWriter, r *http.Request) {
	ownerID := r.Context().Value(ctxOwnerID).(string)
	setupRequired := r.URL.Query().Get("setup") == "qr"
	data, err := s.loadCapturePageData(r.Context(), ownerID, setupRequired)
	if err != nil {
		writeErrWithLog(r, w, http.StatusInternalServerError, "could not load splits", err)
		return
	}
	if scanErr := r.URL.Query().Get("error"); scanErr != "" {
		data.ScanError = scanErr
	}
	s.render(w, r, "owner/capture.html", "Pisah", data)
}

func (s *Server) handleWebOnboardingSeen(w http.ResponseWriter, r *http.Request) {
	ownerID := r.Context().Value(ctxOwnerID).(string)
	if err := s.store.MarkOnboardingSeen(r.Context(), ownerID); err != nil {
		writeErrWithLog(r, w, http.StatusInternalServerError, "could not save onboarding", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleWebScan(w http.ResponseWriter, r *http.Request) {
	ownerID := r.Context().Value(ctxOwnerID).(string)
	if !s.ownerQRRequired(w, r, ownerID) {
		return
	}
	if err := r.ParseMultipartForm(maxReceiptBytes); err != nil {
		s.renderCaptureWithError(w, r, false, "Image too large — try a smaller photo")
		return
	}
	file, _, err := r.FormFile("receipt")
	if err != nil {
		s.renderCaptureWithError(w, r, false, "No receipt image selected")
		return
	}
	defer file.Close()
	img, err := decodeUpload(file, maxReceiptBytes)
	if err != nil {
		s.renderCaptureWithError(w, r, false, scanErrorMessage(err))
		return
	}

	result, err := s.processScan(r.Context(), ownerID, img)
	if err != nil {
		slog.ErrorContext(r.Context(), "web scan failed", s.scanLogAttrs("error", err)...)
		s.renderCaptureWithError(w, r, false, scanErrorMessage(err))
		return
	}

	slog.InfoContext(r.Context(), "web scan ok", s.scanLogAttrs(
		"scan_id", result.ScanID,
		"merchant", result.Receipt.Merchant,
		"items", len(result.Receipt.Items),
		"total_sen", result.Receipt.TotalSen,
		"warnings", len(result.Warnings),
	)...)
	data := s.reviewDataFromScan(result, true)
	s.render(w, r, "owner/review.html", "Review receipt · Pisah", data)
}

func (s *Server) handleWebCreateSplit(w http.ResponseWriter, r *http.Request) {
	ownerID := r.Context().Value(ctxOwnerID).(string)
	if !s.ownerQRRequired(w, r, ownerID) {
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/capture", http.StatusSeeOther)
		return
	}
	merchant := strings.TrimSpace(r.FormValue("merchant"))
	subtotalSen, _ := strconv.ParseInt(r.FormValue("subtotal"), 10, 64)
	sstSen, _ := strconv.ParseInt(r.FormValue("sst"), 10, 64)
	serviceSen, _ := strconv.ParseInt(r.FormValue("service"), 10, 64)
	roundingSen, _ := strconv.ParseInt(r.FormValue("rounding"), 10, 64)
	totalSen, _ := strconv.ParseInt(r.FormValue("total"), 10, 64)

	var items []struct {
		Name            string `json:"name"`
		Qty             int    `json:"qty"`
		UnitPriceSen    int64  `json:"unitPriceSen"`
		LineTotalSen    int64  `json:"lineTotalSen"`
		IncludedInSplit bool   `json:"includedInSplit"`
	}
	for i := 0; i < 100; i++ {
		name := r.FormValue(fmt.Sprintf("item_name_%d", i))
		if name == "" {
			if i == 0 {
				continue
			}
			break
		}
		qty, _ := strconv.Atoi(r.FormValue(fmt.Sprintf("item_qty_%d", i)))
		if qty < 1 {
			qty = 1
		}
		lineSen, _ := strconv.ParseInt(r.FormValue(fmt.Sprintf("item_line_%d", i)), 10, 64)
		included := r.FormValue(fmt.Sprintf("item_in_split_%d", i)) != "0"
		items = append(items, struct {
			Name            string `json:"name"`
			Qty             int    `json:"qty"`
			UnitPriceSen    int64  `json:"unitPriceSen"`
			LineTotalSen    int64  `json:"lineTotalSen"`
			IncludedInSplit bool   `json:"includedInSplit"`
		}{Name: name, Qty: qty, UnitPriceSen: lineSen / int64(qty), LineTotalSen: lineSen, IncludedInSplit: included})
	}
	if len(items) == 0 || totalSen <= 0 {
		http.Redirect(w, r, "/capture", http.StatusSeeOther)
		return
	}
	if subtotalSen <= 0 {
		for _, it := range items {
			subtotalSen += it.LineTotalSen
		}
	}

	ownerName := "You"
	if claims, ok := r.Context().Value(ctxOwnerClaims).(jwt.MapClaims); ok {
		ownerName = ownerDisplayName(claims, ownerName)
	}

	now := time.Now()
	in := CreateSplitInput{
		Merchant:    merchant,
		OwnerName:   ownerName,
		CapturedAt:  &now,
		SubtotalSen: subtotalSen,
		SSTSen:      sstSen,
		ServiceSen:  serviceSen,
		RoundingSen: roundingSen,
		TotalSen:    totalSen,
	}
	for _, it := range items {
		included := it.IncludedInSplit
		in.Items = append(in.Items, struct {
			Name            string `json:"name"`
			Qty             int    `json:"qty"`
			UnitPriceSen    int64  `json:"unitPriceSen"`
			LineTotalSen    int64  `json:"lineTotalSen"`
			IncludedInSplit *bool  `json:"includedInSplit"`
		}{Name: it.Name, Qty: it.Qty, UnitPriceSen: it.UnitPriceSen, LineTotalSen: it.LineTotalSen, IncludedInSplit: &included})
	}
	if prof, err := s.store.GetOwnerProfile(r.Context(), ownerID); err == nil {
		in.OwnerQRURL = prof.OwnerQRURL
	}

	scanID := strings.TrimSpace(r.FormValue("scan_id"))

	var split Split
	var err error
	for attempt := 0; attempt < 5; attempt++ {
		split, err = s.store.CreateSplit(r.Context(), ownerID, newSlug(), in)
		if err == nil {
			break
		}
	}
	if err != nil {
		writeErrWithLog(r, w, http.StatusInternalServerError, "could not create split", err)
		return
	}
	s.deleteScanSessionForOwner(r.Context(), ownerID, scanID)
	data := sharePageData{
		Slug:            split.Slug,
		ShareURL:        s.shareURL(split.Slug),
		ShareDisplayURL: s.shareDisplayURL(split.Slug),
	}
	s.render(w, r, "owner/share.html", "Share split · Pisah", data)
}

func (s *Server) handleWebDeleteSplit(w http.ResponseWriter, r *http.Request) {
	ownerID := r.Context().Value(ctxOwnerID).(string)
	slug := r.PathValue("slug")
	split, err := s.store.GetSplitBySlug(r.Context(), slug)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	if split.OwnerID != ownerID {
		w.WriteHeader(http.StatusForbidden)
		return
	}
	if err := s.store.DeleteSplit(r.Context(), split.ID, ownerID); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleWebTrack(w http.ResponseWriter, r *http.Request) {
	data, ok := s.loadTrackPageData(r)
	if !ok {
		http.Redirect(w, r, "/capture", http.StatusSeeOther)
		return
	}
	s.render(w, r, "owner/track.html", "Track · "+data.Split.Merchant, data)
}

func (s *Server) handleWebTrackParticipants(w http.ResponseWriter, r *http.Request) {
	data, ok := s.loadTrackPageData(r)
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	s.renderPartial(w, r, "partials/track_participants.html", data)
}

func (s *Server) loadTrackPageData(r *http.Request) (trackPageData, bool) {
	ownerID := r.Context().Value(ctxOwnerID).(string)
	slug := r.PathValue("slug")
	split, err := s.store.GetSplitBySlug(r.Context(), slug)
	if err != nil || split.OwnerID != ownerID {
		return trackPageData{}, false
	}
	parts, err := s.store.Participants(r.Context(), split.ID)
	if err != nil {
		return trackPageData{}, false
	}
	collected, err := s.store.CollectedSen(r.Context(), split.ID)
	if err != nil {
		return trackPageData{}, false
	}
	expected, err := s.store.FriendsExpectedSen(r.Context(), split.ID)
	if err != nil {
		return trackPageData{}, false
	}
	data := trackPageData{Split: split, CollectedSen: collected, FriendsExpectedSen: expected, Participants: parts}
	allPaid := true
	for _, p := range parts {
		if p.IsOwner {
			continue
		}
		if !p.Paid {
			allPaid = false
			if data.NudgeName == "" {
				data.NudgeName = p.Name
			}
		}
	}
	data.AllPaid = trackAllFriendsPaidBanner(collected, split.TotalSen, allPaid, len(parts))
	return data, true
}

// trackAllFriendsPaidBanner is true only when every friend marked paid and
// collected amount reaches the full bill total (not just friends' share).
func trackAllFriendsPaidBanner(collected, billTotalSen int64, everyFriendMarkedPaid bool, participantCount int) bool {
	return participantCount > 1 && everyFriendMarkedPaid && collected >= billTotalSen
}

func (s *Server) handleWebSettingsQRImage(w http.ResponseWriter, r *http.Request) {
	ownerID := r.Context().Value(ctxOwnerID).(string)
	prof, err := s.store.GetOwnerProfile(r.Context(), ownerID)
	if err != nil || prof.OwnerQRURL == nil || *prof.OwnerQRURL == "" {
		http.NotFound(w, r)
		return
	}
	s.serveStorageImage(w, r, *prof.OwnerQRURL)
}

func (s *Server) handleFriendQRImage(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	split, err := s.store.GetSplitBySlug(r.Context(), slug)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	qrURL := s.ownerQRForSplit(r.Context(), split)
	if qrURL == nil || *qrURL == "" {
		http.NotFound(w, r)
		return
	}
	s.serveStorageImage(w, r, *qrURL)
}

func (s *Server) serveStorageImage(w http.ResponseWriter, r *http.Request, storageURL string) {
	data, ct, err := fetchStorageObject(r.Context(), storageURL)
	if err != nil {
		slog.WarnContext(r.Context(), "qr image fetch failed", "error", err, "url", storageURL)
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", ct)
	w.Header().Set("Cache-Control", "private, max-age=300")
	w.Write(data)
}

func (s *Server) renderSettings(w http.ResponseWriter, r *http.Request, ownerID string) {
	prof, err := s.store.GetOwnerProfile(r.Context(), ownerID)
	if err != nil {
		slog.WarnContext(r.Context(), "could not load settings, using defaults", "error", err, "owner_id", ownerID)
		prof = OwnerProfile{AutoFillAmount: true}
	}
	s.render(w, r, "owner/settings.html", "Settings · Pisah", map[string]any{"Profile": prof})
}

func (s *Server) handleWebSettingsGet(w http.ResponseWriter, r *http.Request) {
	ownerID := r.Context().Value(ctxOwnerID).(string)
	s.renderSettings(w, r, ownerID)
}

func (s *Server) handleWebSettingsPut(w http.ResponseWriter, r *http.Request) {
	ownerID := r.Context().Value(ctxOwnerID).(string)
	if err := r.ParseForm(); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid form")
		return
	}
	autoFill := r.FormValue("autoFillAmount") == "true"
	if _, err := s.store.SetAutoFillAmount(r.Context(), ownerID, autoFill); err != nil {
		slog.ErrorContext(r.Context(), "could not save settings", "error", err, "owner_id", ownerID)
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleWebUploadQR(w http.ResponseWriter, r *http.Request) {
	ownerID := r.Context().Value(ctxOwnerID).(string)
	if err := r.ParseMultipartForm(maxReceiptBytes); err == nil {
		file, _, err := r.FormFile("qr")
		if err == nil {
			defer file.Close()
			img, err := io.ReadAll(io.LimitReader(file, maxReceiptBytes))
			if err == nil && len(img) > 0 {
				qrURL, err := uploadDuitNowQR(r.Context(), s.cfg, ownerID, img)
				if err != nil {
					slog.ErrorContext(r.Context(), "qr upload failed", "error", err)
				} else if _, err := s.store.SetOwnerQRURL(r.Context(), ownerID, qrURL); err != nil {
					slog.ErrorContext(r.Context(), "qr save failed", "error", err)
				}
			}
		}
	}
	if r.Header.Get("HX-Request") != "" {
		s.renderSettings(w, r, ownerID)
		return
	}
	if r.FormValue("next") == "capture" {
		http.Redirect(w, r, "/capture", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/settings", http.StatusSeeOther)
}

// ---- friend web ----

func (s *Server) handleFriendLanding(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	if tok := s.participantToken(r, slug); tok != "" {
		if _, _, err := s.store.ParticipantByToken(r.Context(), tok); err == nil {
			http.Redirect(w, r, "/r/"+slug+"/pick", http.StatusSeeOther)
			return
		}
	}
	split, err := s.store.GetSplitBySlug(r.Context(), slug)
	if err != nil {
		http.Error(w, "Split not found", http.StatusNotFound)
		return
	}
	items, _ := s.store.ListSplittableItems(r.Context(), split.ID)
	data := friendLandingData{Slug: slug, Split: split, ItemCount: len(items)}
	s.render(w, r, "friend/landing.html", "Join split · Pisah", data)
}

func (s *Server) handleFriendJoin(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	split, err := s.store.GetSplitBySlug(r.Context(), slug)
	if err != nil {
		http.Error(w, "Split not found", http.StatusNotFound)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/r/"+slug, http.StatusSeeOther)
		return
	}
	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		http.Redirect(w, r, "/r/"+slug, http.StatusSeeOther)
		return
	}
	token := newToken()
	if _, err := s.store.CreateParticipant(r.Context(), split.ID, name, token); err != nil {
		writeErrWithLog(r, w, http.StatusInternalServerError, "could not join", err)
		return
	}
	s.setParticipantCookie(w, slug, token)
	http.Redirect(w, r, "/r/"+slug+"/pick", http.StatusSeeOther)
}

func (s *Server) handleFriendPick(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	p := r.Context().Value(ctxParticipant).(Participant)
	data, err := s.buildFriendPickData(r, slug, p)
	if err != nil {
		http.Redirect(w, r, "/r/"+slug, http.StatusSeeOther)
		return
	}
	s.render(w, r, "friend/pick.html", "Pick items · Pisah", data)
}

func (s *Server) handleFriendPickData(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	p := r.Context().Value(ctxParticipant).(Participant)
	split, err := s.store.GetSplitBySlug(r.Context(), slug)
	if err != nil {
		writeErr(w, http.StatusNotFound, "split not found")
		return
	}
	items, err := s.store.ListSplittableItems(r.Context(), split.ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "could not load items")
		return
	}
	splittableSub, _ := s.store.SplittableSubtotalSen(r.Context(), split.ID)
	selected := selectedIDs(items, p.Name)
	writeJSON(w, http.StatusOK, map[string]any{
		"items":                 items,
		"owedSen":               p.OwedSen,
		"selected":              selected,
		"splittableSubtotalSen": splittableSub,
	})
}

func selectedIDs(items []Item, me string) []string {
	var ids []string
	for _, it := range items {
		for _, n := range it.ClaimedBy {
			if n == me {
				ids = append(ids, it.ID)
			}
		}
	}
	return ids
}

func (s *Server) buildFriendPickData(r *http.Request, slug string, p Participant) (friendPickData, error) {
	split, err := s.store.GetSplitBySlug(r.Context(), slug)
	if err != nil {
		return friendPickData{}, err
	}
	items, err := s.store.ListSplittableItems(r.Context(), split.ID)
	if err != nil {
		return friendPickData{}, err
	}
	splittableSub, err := s.store.SplittableSubtotalSen(r.Context(), split.ID)
	if err != nil {
		return friendPickData{}, err
	}
	splitMap := map[string]any{
		"id":                    split.ID,
		"slug":                  split.Slug,
		"merchant":              split.Merchant,
		"subtotalSen":           split.SubtotalSen,
		"splittableSubtotalSen": splittableSub,
		"totalSen":              split.TotalSen,
	}
	cfg := map[string]any{
		"slug":        slug,
		"me":          p.Name,
		"split":       splitMap,
		"items":       items,
		"taxTotalSen": split.TaxTotalSen(),
		"selected":    selectedIDs(items, p.Name),
		"owedSen":     p.OwedSen,
	}
	return friendPickData{Slug: slug, Me: p.Name, Pick: cfg}, nil
}

func (s *Server) handleFriendClaims(w http.ResponseWriter, r *http.Request) {
	p := r.Context().Value(ctxParticipant).(Participant)
	splitID := r.Context().Value(ctxSplitID).(string)
	var body struct {
		ItemIDs []string `json:"itemIds"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	if err := s.store.SetClaims(r.Context(), splitID, p.ID, body.ItemIDs); err != nil {
		writeErrWithLog(r, w, http.StatusInternalServerError, "could not save claims", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) buildShareBreakdown(r *http.Request, slug string, participantID string) (shareBreakdownView, error) {
	split, err := s.store.GetSplitBySlug(r.Context(), slug)
	if err != nil {
		return shareBreakdownView{}, err
	}
	items, err := s.store.ListSplittableItems(r.Context(), split.ID)
	if err != nil {
		return shareBreakdownView{}, err
	}
	parts, err := s.store.Participants(r.Context(), split.ID)
	if err != nil {
		return shareBreakdownView{}, err
	}
	var me Participant
	for _, pp := range parts {
		if pp.ID == participantID {
			me = pp
		}
	}
	var lines []shareLineView
	var claimedSen int64
	for _, it := range items {
		mine := false
		for _, n := range it.ClaimedBy {
			if n == me.Name {
				mine = true
			}
		}
		if !mine {
			continue
		}
		portion := it.LineTotalSen
		if it.Claimants > 1 {
			portion = (it.LineTotalSen + int64(it.Claimants)/2) / int64(it.Claimants)
		}
		name := it.Name
		if it.Claimants > 1 {
			name = fmt.Sprintf("%s · shared ÷%d", it.Name, it.Claimants)
		}
		lines = append(lines, shareLineView{Name: name, AmtSen: portion})
		claimedSen += portion
	}
	qrURL := s.ownerQRForSplit(r.Context(), split)
	prof, _ := s.store.GetOwnerProfile(r.Context(), split.OwnerID)
	return shareBreakdownView{
		Merchant:       split.Merchant,
		OwnerName:      split.OwnerName,
		OwnerQRURL:     qrURL,
		AutoFillAmount: prof.AutoFillAmount,
		Lines:          lines,
		TaxSen:         me.OwedSen - claimedSen,
		OwedSen:        me.OwedSen,
	}, nil
}

func (s *Server) handleFriendShare(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	p := r.Context().Value(ctxParticipant).(Participant)
	sh, err := s.buildShareBreakdown(r, slug, p.ID)
	if err != nil {
		if errors.Is(err, errNotFound) {
			http.NotFound(w, r)
			return
		}
		writeErrWithLog(r, w, http.StatusInternalServerError, "could not load share", err)
		return
	}
	s.render(w, r, "friend/share.html", "Your share · Pisah", friendSharePageData{Slug: slug, Share: sh})
}

func (s *Server) handleFriendPay(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	p := r.Context().Value(ctxParticipant).(Participant)
	sh, err := s.buildShareBreakdown(r, slug, p.ID)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	s.render(w, r, "friend/pay.html", "Pay · Pisah", friendPayPageData{Slug: slug, Share: sh})
}

func (s *Server) handleFriendPaid(w http.ResponseWriter, r *http.Request) {
	p := r.Context().Value(ctxParticipant).(Participant)
	slug := r.PathValue("slug")
	updated, splitID, err := s.store.MarkPaid(r.Context(), p.ID)
	if err != nil {
		writeErrWithLog(r, w, http.StatusInternalServerError, "could not mark paid", err)
		return
	}
	collected, _ := s.store.CollectedSen(r.Context(), splitID)
	event, _ := json.Marshal(map[string]any{
		"type":         "paid",
		"participant":  updated,
		"collectedSen": collected,
	})
	s.broker.publish(splitID, event)
	split, _ := s.store.GetSplitBySlug(r.Context(), slug)
	data := friendDonePageData{
		Me:        p.Name,
		OwnerName: split.OwnerName,
		Merchant:  split.Merchant,
		OwedSen:   updated.OwedSen,
	}
	s.render(w, r, "friend/done.html", "Done · Pisah", data)
}

func (s *Server) requireParticipantWeb(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		slug := r.PathValue("slug")
		tok := s.participantToken(r, slug)
		if tok == "" {
			http.Redirect(w, r, "/r/"+slug, http.StatusSeeOther)
			return
		}
		p, splitID, err := s.store.ParticipantByToken(r.Context(), tok)
		if err != nil {
			http.Redirect(w, r, "/r/"+slug, http.StatusSeeOther)
			return
		}
		ctx := contextWithParticipant(r.Context(), p, splitID)
		next(w, r.WithContext(ctx))
	}
}

func contextWithParticipant(ctx context.Context, p Participant, splitID string) context.Context {
	ctx = context.WithValue(ctx, ctxParticipant, p)
	return context.WithValue(ctx, ctxSplitID, splitID)
}
