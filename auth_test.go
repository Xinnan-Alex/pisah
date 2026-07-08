package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func withMockAuthHTTP(t *testing.T, handler http.HandlerFunc) (*httptest.Server, func()) {
	t.Helper()
	mock := httptest.NewServer(handler)
	prev := authHTTP
	authHTTP = mock.Client()
	return mock, func() {
		mock.Close()
		authHTTP = prev
	}
}

func TestAuthGoneRedirectsToCapture(t *testing.T) {
	s := &Server{cfg: Config{PublicBaseURL: "https://split.my"}}
	for _, path := range []string{"/signin", "/signup", "/signout", "/auth/callback"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		s.handleAuthGone(rec, req)
		if rec.Code != http.StatusSeeOther {
			t.Fatalf("%s status = %d, want %d", path, rec.Code, http.StatusSeeOther)
		}
		if loc := rec.Header().Get("Location"); loc != "/capture" {
			t.Fatalf("%s redirect = %q, want /capture", path, loc)
		}
	}
}

func TestHandleSignUpAPI(t *testing.T) {
	mock, cleanup := withMockAuthHTTP(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/auth/v1/signup" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		var payload map[string]any
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatal(err)
		}
		if payload["email"] != "owner@example.com" {
			t.Fatalf("email = %v", payload["email"])
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"access_token":"at","refresh_token":"rt"}`))
	})
	defer cleanup()

	s := &Server{cfg: Config{
		SupabaseURL:            mock.URL,
		SupabasePublishableKey: "test-key",
	}}

	req := httptest.NewRequest(http.MethodPost, "/api/auth/sign-up", strings.NewReader(`{"email":"owner@example.com","password":"secret123"}`))
	rec := httptest.NewRecorder()
	s.handleSignUp(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	var out map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out["access_token"] != "at" {
		t.Fatalf("response = %#v", out)
	}
}
