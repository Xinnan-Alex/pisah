package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

const (
	cookieAccess  = "pisah_at"
	cookieRefresh = "pisah_rt"
)

func (s *Server) setSessionCookies(w http.ResponseWriter, access, refresh string) {
	http.SetCookie(w, &http.Cookie{
		Name:     cookieAccess,
		Value:    access,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   3600,
	})
	if refresh != "" {
		http.SetCookie(w, &http.Cookie{
			Name:     cookieRefresh,
			Value:    refresh,
			Path:     "/",
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
			MaxAge:   60 * 60 * 24 * 30,
		})
	}
}

func (s *Server) clearSessionCookies(w http.ResponseWriter) {
	for _, name := range []string{cookieAccess, cookieRefresh} {
		http.SetCookie(w, &http.Cookie{Name: name, Value: "", Path: "/", MaxAge: -1})
	}
}

func (s *Server) verifyAccessToken(tok string) (string, jwt.MapClaims, error) {
	claims := jwt.MapClaims{}
	_, err := jwt.ParseWithClaims(tok, claims, func(t *jwt.Token) (any, error) {
		switch t.Method.(type) {
		case *jwt.SigningMethodECDSA:
			if s.jwks == nil {
				return nil, errors.New("no JWKS configured")
			}
			kid, _ := t.Header["kid"].(string)
			return s.jwks.key(kid)
		case *jwt.SigningMethodHMAC:
			if s.cfg.JWTSecret == "" {
				return nil, errors.New("no HS256 secret configured")
			}
			return []byte(s.cfg.JWTSecret), nil
		default:
			return nil, errors.New("unexpected signing method")
		}
	})
	if err != nil {
		return "", nil, err
	}
	sub, _ := claims["sub"].(string)
	if sub == "" {
		return "", nil, errors.New("token missing subject")
	}
	return sub, claims, nil
}

func (s *Server) refreshSession(ctx context.Context, refreshToken string) (string, string, error) {
	if s.cfg.SupabaseURL == "" || s.cfg.SupabasePublishableKey == "" {
		return "", "", errors.New("auth not configured")
	}
	payload, _ := json.Marshal(map[string]string{"refresh_token": refreshToken})
	url := strings.TrimRight(s.cfg.SupabaseURL, "/") + "/auth/v1/token?grant_type=refresh_token"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("apikey", s.cfg.SupabasePublishableKey)
	resp, err := authHTTP.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", "", err
	}
	if resp.StatusCode >= 400 {
		return "", "", errors.New("refresh failed")
	}
	var out struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return "", "", err
	}
	if out.AccessToken == "" {
		return "", "", errors.New("no access token in refresh response")
	}
	return out.AccessToken, out.RefreshToken, nil
}

func (s *Server) ownerFromRequest(w http.ResponseWriter, r *http.Request) (string, jwt.MapClaims, bool) {
	at, err := r.Cookie(cookieAccess)
	if err != nil || at.Value == "" {
		return "", nil, false
	}
	ownerID, claims, err := s.verifyAccessToken(at.Value)
	if err == nil {
		return ownerID, claims, true
	}
	rt, err := r.Cookie(cookieRefresh)
	if err != nil || rt.Value == "" {
		return "", nil, false
	}
	newAt, newRt, err := s.refreshSession(r.Context(), rt.Value)
	if err != nil {
		return "", nil, false
	}
	s.setSessionCookies(w, newAt, newRt)
	ownerID, claims, err = s.verifyAccessToken(newAt)
	if err != nil {
		return "", nil, false
	}
	return ownerID, claims, true
}

func (s *Server) requireOwnerWeb(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ownerID, claims, ok := s.ownerFromRequest(w, r)
		if !ok {
			http.Redirect(w, r, "/signin", http.StatusSeeOther)
			return
		}
		ctx := context.WithValue(r.Context(), ctxOwnerID, ownerID)
		ctx = context.WithValue(ctx, ctxOwnerClaims, claims)
		next(w, r.WithContext(ctx))
	}
}

func participantCookieName(slug string) string {
	return "pisah_p_" + slug
}

func (s *Server) participantToken(r *http.Request, slug string) string {
	c, err := r.Cookie(participantCookieName(slug))
	if err != nil {
		return ""
	}
	return c.Value
}

func (s *Server) setParticipantCookie(w http.ResponseWriter, slug, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     participantCookieName(slug),
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   60 * 60 * 24 * 90,
	})
}

func (s *Server) webGoogleOAuthURL(r *http.Request) string {
	base := strings.TrimRight(s.cfg.PublicBaseURL, "/")
	redirectTo := url.QueryEscape(base + "/auth/callback")
	return strings.TrimRight(s.cfg.SupabaseURL, "/") + "/auth/v1/authorize?provider=google&redirect_to=" + redirectTo
}
