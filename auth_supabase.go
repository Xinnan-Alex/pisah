package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

type supabaseSession struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

func (s *Server) authConfigured() bool {
	return s.cfg.SupabaseURL != "" && s.cfg.SupabasePublishableKey != ""
}

func (s *Server) supabaseAuthPost(ctx context.Context, path string, payload any) (int, []byte, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return 0, nil, err
	}
	authURL := strings.TrimRight(s.cfg.SupabaseURL, "/") + path
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, authURL, bytes.NewReader(body))
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("apikey", s.cfg.SupabasePublishableKey)
	resp, err := authHTTP.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return resp.StatusCode, nil, err
	}
	return resp.StatusCode, data, nil
}

func (s *Server) supabaseSignIn(ctx context.Context, email, password string) (int, []byte, error) {
	return s.supabaseAuthPost(ctx, "/auth/v1/token?grant_type=password", map[string]string{
		"email":    email,
		"password": password,
	})
}

func (s *Server) supabaseSignUp(ctx context.Context, email, password, emailRedirectTo string) (int, []byte, error) {
	payload := map[string]any{
		"email":    email,
		"password": password,
	}
	if emailRedirectTo != "" {
		payload["options"] = map[string]string{"email_redirect_to": emailRedirectTo}
	}
	return s.supabaseAuthPost(ctx, "/auth/v1/signup", payload)
}

func parseSupabaseSession(data []byte) supabaseSession {
	var tok supabaseSession
	_ = json.Unmarshal(data, &tok)
	return tok
}

func supabaseErrorMessage(data []byte) string {
	var errBody struct {
		Msg              string `json:"msg"`
		ErrorDescription string `json:"error_description"`
		Message          string `json:"message"`
	}
	if json.Unmarshal(data, &errBody) != nil {
		return ""
	}
	if errBody.Msg != "" {
		return errBody.Msg
	}
	if errBody.ErrorDescription != "" {
		return errBody.ErrorDescription
	}
	return errBody.Message
}
