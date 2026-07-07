package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"time"
)

func (s *Server) runScanCleanup(ctx context.Context) {
	urls, err := s.store.CleanupExpiredScanSessions(ctx)
	if err != nil {
		slog.WarnContext(ctx, "scan session cleanup failed", "error", err)
		return
	}
	for _, u := range urls {
		if err := deleteScanImage(ctx, s.cfg, u); err != nil {
			slog.WarnContext(ctx, "delete expired scan image failed", "url", u, "error", err)
		}
	}
}

func (s *Server) scanLogAttrs(extra ...any) []any {
	if s.scanPipeline == nil {
		return extra
	}
	return s.scanPipeline.logAttrs(extra...)
}

func (s *Server) processScan(ctx context.Context, ownerID string, raw []byte) (ScanResult, error) {
	if s.scanPipeline == nil {
		return ScanResult{}, errors.New("scan pipeline not configured")
	}
	rec, warnings, err := s.scanPipeline.Process(ctx, raw)
	if err != nil {
		return ScanResult{}, err
	}

	preprocessed, err := preprocessReceiptImage(raw)
	if err != nil {
		return ScanResult{}, err
	}

	var session ScanSession
	if s.cfg.SupabaseURL != "" && s.cfg.SupabaseSecretKey != "" {
		scanID, err := s.store.NewScanID(ctx)
		if err != nil {
			return ScanResult{}, fmt.Errorf("new scan id: %w", err)
		}
		imageURL, err := uploadScanImage(ctx, s.cfg, ownerID, scanID, preprocessed)
		if err != nil {
			return ScanResult{}, fmt.Errorf("upload scan image: %w", err)
		}
		session, err = s.store.CreateScanSession(ctx, ownerID, scanID, imageURL)
		if err != nil {
			_ = deleteScanImage(ctx, s.cfg, imageURL)
			return ScanResult{}, fmt.Errorf("create scan session: %w", err)
		}
	}

	result := ScanResult{
		ScanID:   session.ID,
		Receipt:  rec,
		Warnings: warnings,
	}
	if session.ID != "" {
		result.ImageURL = s.scanImageAPIURL(session.ID)
	}
	return result, nil
}

func (s *Server) scanImageAPIURL(scanID string) string {
	return "/api/receipts/scans/" + scanID + "/image"
}

func (s *Server) scanImageWebURL(scanID string) string {
	return "/scan/" + scanID + "/image"
}

func (s *Server) reviewDataFromScan(result ScanResult, web bool) reviewPageData {
	rec := result.Receipt
	imgURL := result.ImageURL
	if web && result.ScanID != "" {
		imgURL = s.scanImageWebURL(result.ScanID)
	}
	return reviewPageData{
		ScanID:      result.ScanID,
		Merchant:    rec.Merchant,
		SubtotalSen: rec.SubtotalSen,
		SstSen:      rec.SstSen,
		ServiceSen:  rec.ServiceSen,
		RoundingSen: rec.RoundingSen,
		TotalSen:    rec.TotalSen,
		Items:       rec.Items,
		Warnings:    result.Warnings,
		ImageURL:    imgURL,
		CapturedAt:  time.Now().Format("2 Jan · 3:04 PM"),
		ManualEntry: false,
	}
}

func (s *Server) handleScanImageWeb(w http.ResponseWriter, r *http.Request) {
	ownerID := r.Context().Value(ctxOwnerID).(string)
	scanID := r.PathValue("id")
	s.serveScanImage(w, r, ownerID, scanID)
}

func (s *Server) handleScanImageAPI(w http.ResponseWriter, r *http.Request) {
	ownerID := r.Context().Value(ctxOwnerID).(string)
	scanID := r.PathValue("id")
	s.serveScanImage(w, r, ownerID, scanID)
}

func (s *Server) serveScanImage(w http.ResponseWriter, r *http.Request, ownerID, scanID string) {
	session, err := s.store.GetScanSession(r.Context(), ownerID, scanID)
	if err != nil {
		if errors.Is(err, errNotFound) {
			http.NotFound(w, r)
			return
		}
		writeErrWithLog(r, w, http.StatusInternalServerError, "could not load scan", err)
		return
	}
	data, ct, err := fetchStorageObject(r.Context(), session.ImageURL)
	if err != nil {
		writeErrWithLog(r, w, http.StatusBadGateway, "could not load receipt image", err)
		return
	}
	w.Header().Set("Content-Type", ct)
	w.Header().Set("Cache-Control", "private, max-age=3600")
	_, _ = w.Write(data)
}

func (s *Server) handleWebRescan(w http.ResponseWriter, r *http.Request) {
	ownerID := r.Context().Value(ctxOwnerID).(string)
	if !s.ownerQRRequired(w, r, ownerID) {
		return
	}
	scanID := r.PathValue("id")
	session, err := s.store.GetScanSession(r.Context(), ownerID, scanID)
	if err != nil {
		if errors.Is(err, errNotFound) {
			http.Redirect(w, r, "/capture?error="+url.QueryEscape("Scan expired — please take a new photo"), http.StatusSeeOther)
			return
		}
		s.renderCaptureWithError(w, r, false, "Could not rescan — try again")
		return
	}
	img, _, err := fetchStorageObject(r.Context(), session.ImageURL)
	if err != nil || len(img) == 0 {
		s.renderCaptureWithError(w, r, false, "Could not load receipt image — take a new photo")
		return
	}
	rec, warnings, err := s.scanPipeline.Process(r.Context(), img)
	if err != nil {
		slog.ErrorContext(r.Context(), "rescan failed", s.scanLogAttrs("scan_id", scanID, "error", err)...)
		data := s.reviewDataFromScan(ScanResult{ScanID: scanID, Receipt: rec, Warnings: warnings}, true)
		data.RescanError = scanErrorMessage(err)
		s.render(w, r, "owner/review.html", "Review receipt · Pisah", data)
		return
	}
	data := s.reviewDataFromScan(ScanResult{ScanID: scanID, Receipt: rec, Warnings: warnings}, true)
	s.render(w, r, "owner/review.html", "Review receipt · Pisah", data)
	_ = session
}

func (s *Server) handleWebManualReview(w http.ResponseWriter, r *http.Request) {
	ownerID := r.Context().Value(ctxOwnerID).(string)
	if !s.ownerQRRequired(w, r, ownerID) {
		return
	}
	data := reviewPageData{
		Merchant:    "",
		Items:       []ParsedItem{{Name: "", Qty: 1, UnitPriceSen: 0, LineTotalSen: 0}},
		CapturedAt:  time.Now().Format("2 Jan · 3:04 PM"),
		ManualEntry: true,
	}
	s.render(w, r, "owner/review.html", "Enter receipt · Pisah", data)
}

func (s *Server) handleRescanAPI(w http.ResponseWriter, r *http.Request) {
	ownerID := r.Context().Value(ctxOwnerID).(string)
	scanID := r.PathValue("id")
	session, err := s.store.GetScanSession(r.Context(), ownerID, scanID)
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeErr(w, http.StatusNotFound, "scan session not found or expired")
			return
		}
		writeErrWithLog(r, w, http.StatusInternalServerError, "could not load scan", err)
		return
	}
	img, _, err := fetchStorageObject(r.Context(), session.ImageURL)
	if err != nil || len(img) == 0 {
		writeErrWithLog(r, w, http.StatusBadGateway, "could not load receipt image", err)
		return
	}
	rec, warnings, err := s.scanPipeline.Process(r.Context(), img)
	if err != nil {
		writeErrWithLog(r, w, http.StatusBadGateway, "ocr failed", err)
		return
	}
	writeJSON(w, http.StatusOK, ScanResult{
		ScanID:   scanID,
		Receipt:  rec,
		Warnings: warnings,
		ImageURL: s.scanImageAPIURL(scanID),
	})
}

func scanErrorMessage(err error) string {
	if errors.Is(err, errUnreadableImage) || errors.Is(err, errUnsupportedMIME) {
		return err.Error()
	}
	return "Couldn't read receipt — try better lighting or enter details manually"
}

func (s *Server) renderCaptureWithError(w http.ResponseWriter, r *http.Request, setupRequired bool, scanError string) {
	ownerID := r.Context().Value(ctxOwnerID).(string)
	data, err := s.loadCapturePageData(r.Context(), ownerID, setupRequired)
	if err != nil {
		writeErrWithLog(r, w, http.StatusInternalServerError, "could not load capture page", err)
		return
	}
	data.ScanError = scanError
	s.render(w, r, "owner/capture.html", "Pisah", data)
}

func (s *Server) deleteScanSessionForOwner(ctx context.Context, ownerID, scanID string) {
	if scanID == "" {
		return
	}
	imageURL, err := s.store.DeleteScanSession(ctx, ownerID, scanID)
	if err != nil && !errors.Is(err, errNotFound) {
		slog.WarnContext(ctx, "delete scan session failed", "scan_id", scanID, "error", err)
		return
	}
	if imageURL != "" {
		if err := deleteScanImage(ctx, s.cfg, imageURL); err != nil {
			slog.WarnContext(ctx, "delete scan image failed", "scan_id", scanID, "error", err)
		}
	}
}
