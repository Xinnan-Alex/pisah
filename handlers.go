package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const maxReceiptBytes = 12 << 20 // 12 MB

var authHTTP = &http.Client{Timeout: 15 * time.Second}

// POST /api/auth/sign-in  (public) — proxy Supabase GoTrue password grant.
func (s *Server) handleSignIn(w http.ResponseWriter, r *http.Request) {
	if s.cfg.SupabaseURL == "" || s.cfg.SupabasePublishableKey == "" {
		writeErr(w, http.StatusNotImplemented, "auth not configured")
		return
	}
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Email == "" || body.Password == "" {
		writeErr(w, http.StatusBadRequest, "email and password required")
		return
	}
	payload, err := json.Marshal(map[string]string{"email": body.Email, "password": body.Password})
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "could not encode sign-in request")
		return
	}
	url := strings.TrimRight(s.cfg.SupabaseURL, "/") + "/auth/v1/token?grant_type=password"
	req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "could not build sign-in request")
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("apikey", s.cfg.SupabasePublishableKey)
	resp, err := authHTTP.Do(req)
	if err != nil {
		writeErrWithLog(r, w, http.StatusBadGateway, "auth service unreachable", err)
		return
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		writeErrWithLog(r, w, http.StatusBadGateway, "auth service unreadable", err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	_, _ = w.Write(data)
}

// GET /api/auth/oauth/google?redirect_to=pisah://auth/callback
// Returns a Supabase GoTrue authorize URL for ASWebAuthenticationSession on iOS.
func (s *Server) handleGoogleOAuthStart(w http.ResponseWriter, r *http.Request) {
	if s.cfg.SupabaseURL == "" || s.cfg.SupabasePublishableKey == "" {
		writeErr(w, http.StatusNotImplemented, "auth not configured")
		return
	}
	redirectTo := r.URL.Query().Get("redirect_to")
	if redirectTo == "" {
		redirectTo = "pisah://auth/callback"
	}
	q := url.Values{}
	q.Set("provider", "google")
	q.Set("redirect_to", redirectTo)
	authURL := strings.TrimRight(s.cfg.SupabaseURL, "/") + "/auth/v1/authorize?" + q.Encode()
	writeJSON(w, http.StatusOK, map[string]string{"url": authURL})
}

// POST /api/receipts/scan  (owner)
// Body: raw image bytes (image/jpeg or image/png). Returns parsed receipt for review.
func (s *Server) handleScan(w http.ResponseWriter, r *http.Request) {
	img, err := io.ReadAll(io.LimitReader(r.Body, maxReceiptBytes))
	if err != nil || len(img) == 0 {
		writeErr(w, http.StatusBadRequest, "empty or unreadable image body")
		return
	}
	parsed, err := scanReceipt(r.Context(), img)
	if err != nil {
		writeErrWithLog(r, w, http.StatusBadGateway, "ocr failed", err)
		return
	}
	writeJSON(w, http.StatusOK, parsed)
}

// POST /api/splits  (owner)  — create a split from reviewed items, get a share link.
func (s *Server) handleCreateSplit(w http.ResponseWriter, r *http.Request) {
	ownerID := r.Context().Value(ctxOwnerID).(string)
	var in CreateSplitInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	if in.TotalSen <= 0 || len(in.Items) == 0 {
		writeErr(w, http.StatusBadRequest, "split needs items and a positive total")
		return
	}
	if claims, ok := r.Context().Value(ctxOwnerClaims).(jwt.MapClaims); ok {
		if n := ownerDisplayName(claims, in.OwnerName); n != "" {
			in.OwnerName = n
		}
	}
	if in.OwnerQRURL == nil || *in.OwnerQRURL == "" {
		if prof, err := s.store.GetOwnerProfile(r.Context(), ownerID); err == nil {
			in.OwnerQRURL = prof.OwnerQRURL
		}
	}

	// Retry slug collisions a few times (4-char space is small but slugs are sparse).
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

	slog.InfoContext(r.Context(), "split created",
		"split_id", split.ID,
		"slug", split.Slug,
		"owner_id", ownerID,
		"items", len(in.Items),
	)

	writeJSON(w, http.StatusCreated, map[string]any{
		"id":              split.ID,
		"slug":            split.Slug,
		"shareUrl":        s.shareURL(split.Slug),
		"shareDisplayUrl": s.shareDisplayURL(split.Slug),
		"split":           split,
	})
}

// GET /api/me/splits  (owner) — recent splits with collected progress.
func (s *Server) handleListMySplits(w http.ResponseWriter, r *http.Request) {
	ownerID := r.Context().Value(ctxOwnerID).(string)
	summaries, err := s.store.ListOwnerSplits(r.Context(), ownerID)
	if err != nil {
		writeErrWithLog(r, w, http.StatusInternalServerError, "could not load splits", err)
		return
	}
	out := make([]map[string]any, 0, len(summaries))
	for _, sum := range summaries {
		out = append(out, map[string]any{
			"split":           sum.Split,
			"shareUrl":        s.shareURL(sum.Split.Slug),
			"shareDisplayUrl": s.shareDisplayURL(sum.Split.Slug),
			"collectedSen":    sum.CollectedSen,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"splits": out})
}

// DELETE /api/splits/{slug}  (owner) — permanently remove a split and its items/participants.
func (s *Server) handleDeleteSplit(w http.ResponseWriter, r *http.Request) {
	ownerID := r.Context().Value(ctxOwnerID).(string)
	split, err := s.store.GetSplitBySlug(r.Context(), r.PathValue("slug"))
	if err != nil {
		writeErr(w, http.StatusNotFound, "split not found")
		return
	}
	if split.OwnerID != ownerID {
		writeErr(w, http.StatusForbidden, "not your split")
		return
	}
	if err := s.store.DeleteSplit(r.Context(), split.ID, ownerID); err != nil {
		if errors.Is(err, errNotFound) {
			writeErr(w, http.StatusNotFound, "split not found")
			return
		}
		writeErrWithLog(r, w, http.StatusInternalServerError, "could not delete split", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// GET /api/splits/{slug}  (public) — friend landing + item list with claim status.
func (s *Server) handleGetSplit(w http.ResponseWriter, r *http.Request) {
	split, err := s.store.GetSplitBySlug(r.Context(), r.PathValue("slug"))
	if err != nil {
		writeErr(w, http.StatusNotFound, "split not found")
		return
	}
	items, err := s.store.ListSplittableItems(r.Context(), split.ID)
	if err != nil {
		writeErrWithLog(r, w, http.StatusInternalServerError, "could not load items", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"split":       split,
		"taxTotalSen": split.TaxTotalSen(),
		"items":       items,
	})
}

// POST /api/splits/{slug}/join  (public) — friend enters a name, gets a token.
func (s *Server) handleJoin(w http.ResponseWriter, r *http.Request) {
	split, err := s.store.GetSplitBySlug(r.Context(), r.PathValue("slug"))
	if err != nil {
		writeErr(w, http.StatusNotFound, "split not found")
		return
	}
	var body struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Name == "" {
		writeErr(w, http.StatusBadRequest, "name is required")
		return
	}
	token := newToken()
	p, err := s.store.CreateParticipant(r.Context(), split.ID, body.Name, token)
	if err != nil {
		writeErrWithLog(r, w, http.StatusInternalServerError, "could not join", err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"token":       token, // friend stores this; sends as Bearer on later calls
		"participant": p,
	})
}

// POST /api/splits/{slug}/claims  (participant) — set the items I ordered.
func (s *Server) handleSetClaims(w http.ResponseWriter, r *http.Request) {
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
	s.writeShareResponse(w, r, splitID, p.ID)
}

// GET /api/splits/{slug}/share  (participant) — my breakdown + owner QR.
func (s *Server) handleShare(w http.ResponseWriter, r *http.Request) {
	p := r.Context().Value(ctxParticipant).(Participant)
	splitID := r.Context().Value(ctxSplitID).(string)
	s.writeShareResponse(w, r, splitID, p.ID)
}

func (s *Server) writeShareResponse(w http.ResponseWriter, r *http.Request, splitID, participantID string) {
	split, err := s.store.GetSplitBySlug(r.Context(), r.PathValue("slug"))
	if err != nil {
		writeErr(w, http.StatusNotFound, "split not found")
		return
	}
	items, err := s.store.ListSplittableItems(r.Context(), split.ID)
	if err != nil {
		writeErrWithLog(r, w, http.StatusInternalServerError, "could not load items", err)
		return
	}
	// Find this participant's current owed + build human-readable breakdown lines.
	parts, err := s.store.Participants(r.Context(), splitID)
	if err != nil {
		writeErrWithLog(r, w, http.StatusInternalServerError, "could not load participant", err)
		return
	}
	var me Participant
	for _, pp := range parts {
		if pp.ID == participantID {
			me = pp
		}
	}

	type line struct {
		Name   string `json:"name"`
		AmtSen int64  `json:"amtSen"`
	}
	var lines []line
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
		lines = append(lines, line{Name: name, AmtSen: portion})
		claimedSen += portion
	}
	taxSen := me.OwedSen - claimedSen // owed already includes proportional tax

	qrURL := s.ownerQRForSplit(r.Context(), split)
	prof, _ := s.store.GetOwnerProfile(r.Context(), split.OwnerID)
	autoFill := prof.AutoFillAmount

	writeJSON(w, http.StatusOK, map[string]any{
		"merchant":       split.Merchant,
		"ownerName":      split.OwnerName,
		"ownerQrUrl":     qrURL,
		"autoFillAmount": autoFill,
		"lines":          lines,
		"taxSen":         taxSen,
		"owedSen":        me.OwedSen,
	})
}

// POST /api/splits/{slug}/paid  (participant) — mark my share paid, notify owner.
func (s *Server) handlePaid(w http.ResponseWriter, r *http.Request) {
	p := r.Context().Value(ctxParticipant).(Participant)
	updated, splitID, err := s.store.MarkPaid(r.Context(), p.ID)
	if err != nil {
		writeErrWithLog(r, w, http.StatusInternalServerError, "could not mark paid", err)
		return
	}
	collected, err := s.store.CollectedSen(r.Context(), splitID)
	if err != nil {
		slog.WarnContext(r.Context(), "could not load collected total after payment",
			"error", err,
			"split_id", splitID,
			"participant_id", p.ID,
		)
	}
	slog.InfoContext(r.Context(), "participant marked paid",
		"split_id", splitID,
		"participant_id", p.ID,
		"owed_sen", updated.OwedSen,
		"collected_sen", collected,
	)
	event, _ := json.Marshal(map[string]any{
		"type":         "paid",
		"participant":  updated,
		"collectedSen": collected,
	})
	s.broker.publish(splitID, event)
	writeJSON(w, http.StatusOK, updated)
}

// GET /api/splits/{slug}/track  (owner) — collected progress + per-participant status.
func (s *Server) handleTrack(w http.ResponseWriter, r *http.Request) {
	ownerID := r.Context().Value(ctxOwnerID).(string)
	split, err := s.store.GetSplitBySlug(r.Context(), r.PathValue("slug"))
	if err != nil {
		writeErr(w, http.StatusNotFound, "split not found")
		return
	}
	if split.OwnerID != ownerID {
		writeErr(w, http.StatusForbidden, "not your split")
		return
	}
	parts, err := s.store.Participants(r.Context(), split.ID)
	if err != nil {
		writeErrWithLog(r, w, http.StatusInternalServerError, "could not load participants", err)
		return
	}
	collected, err := s.store.CollectedSen(r.Context(), split.ID)
	if err != nil {
		writeErrWithLog(r, w, http.StatusInternalServerError, "could not load collected total", err)
		return
	}
	expected, err := s.store.FriendsExpectedSen(r.Context(), split.ID)
	if err != nil {
		writeErrWithLog(r, w, http.StatusInternalServerError, "could not load expected total", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"split":              split,
		"collectedSen":       collected,
		"friendsExpectedSen": expected,
		"participants":       parts,
	})
}

// GET /api/splits/{slug}/events  (public) — Server-Sent Events stream of payments.
func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	split, err := s.store.GetSplitBySlug(r.Context(), r.PathValue("slug"))
	if err != nil {
		writeErr(w, http.StatusNotFound, "split not found")
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeErr(w, http.StatusInternalServerError, "streaming unsupported")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ch := s.broker.subscribe(split.ID)
	defer s.broker.unsubscribe(split.ID, ch)

	slog.InfoContext(r.Context(), "sse client connected",
		"split_id", split.ID,
		"slug", split.Slug,
	)

	fmt.Fprint(w, "retry: 3000\n\n")
	flusher.Flush()

	for {
		select {
		case <-r.Context().Done():
			return
		case msg := <-ch:
			fmt.Fprintf(w, "data: %s\n\n", msg)
			flusher.Flush()
		}
	}
}

// ownerDisplayName prefers the signed-in user's profile name over a client placeholder.
func ownerDisplayName(claims jwt.MapClaims, fallback string) string {
	if meta, ok := claims["user_metadata"].(map[string]any); ok {
		for _, key := range []string{"full_name", "name"} {
			if v, ok := meta[key].(string); ok {
				if n := strings.TrimSpace(v); n != "" {
					return n
				}
			}
		}
	}
	if v, ok := claims["email"].(string); ok {
		if at := strings.Index(v, "@"); at > 0 {
			return v[:at]
		}
	}
	return strings.TrimSpace(fallback)
}

// GET /api/me/payment-settings  (owner) — DuitNow QR URL + preferences.
func (s *Server) handleGetPaymentSettings(w http.ResponseWriter, r *http.Request) {
	ownerID := r.Context().Value(ctxOwnerID).(string)
	prof, err := s.store.GetOwnerProfile(r.Context(), ownerID)
	if err != nil {
		writeErrWithLog(r, w, http.StatusInternalServerError, "could not load payment settings", err)
		return
	}
	writeJSON(w, http.StatusOK, prof)
}

// PUT /api/me/payment-settings  (owner) — update preferences.
func (s *Server) handleUpdatePaymentSettings(w http.ResponseWriter, r *http.Request) {
	ownerID := r.Context().Value(ctxOwnerID).(string)
	var body struct {
		AutoFillAmount *bool `json:"autoFillAmount"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.AutoFillAmount == nil {
		writeErr(w, http.StatusBadRequest, "autoFillAmount is required")
		return
	}
	prof, err := s.store.SetAutoFillAmount(r.Context(), ownerID, *body.AutoFillAmount)
	if err != nil {
		writeErrWithLog(r, w, http.StatusInternalServerError, "could not save payment settings", err)
		return
	}
	writeJSON(w, http.StatusOK, prof)
}

// POST /api/me/duitnow-qr  (owner) — upload a DuitNow QR image (JPEG/PNG).
func (s *Server) handleUploadDuitNowQR(w http.ResponseWriter, r *http.Request) {
	ownerID := r.Context().Value(ctxOwnerID).(string)
	img, err := io.ReadAll(io.LimitReader(r.Body, maxReceiptBytes))
	if err != nil || len(img) == 0 {
		writeErr(w, http.StatusBadRequest, "empty or unreadable image body")
		return
	}
	qrURL, err := uploadDuitNowQR(r.Context(), s.cfg, ownerID, img)
	if err != nil {
		writeErrWithLog(r, w, http.StatusBadGateway, "could not upload qr", err)
		return
	}
	prof, err := s.store.SetOwnerQRURL(r.Context(), ownerID, qrURL)
	if err != nil {
		writeErrWithLog(r, w, http.StatusInternalServerError, "could not save qr url", err)
		return
	}
	writeJSON(w, http.StatusOK, prof)
}
