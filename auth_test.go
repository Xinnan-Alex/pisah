package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
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

func TestWebSignUpPostMissingFields(t *testing.T) {
	s := &Server{cfg: Config{
		SupabaseURL:            "https://example.supabase.co",
		SupabasePublishableKey: "test-key",
		PublicBaseURL:          "https://split.my",
	}}

	req := httptest.NewRequest(http.MethodPost, "/signup", strings.NewReader("email=&password="))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	s.handleWebSignUpPost(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	loc := rec.Header().Get("Location")
	if !strings.Contains(loc, "/signup?error=") || !strings.Contains(loc, "Email+and+password+required") {
		t.Fatalf("unexpected redirect: %s", loc)
	}
}

func TestWebSignUpPostPasswordMismatch(t *testing.T) {
	s := &Server{cfg: Config{
		SupabaseURL:            "https://example.supabase.co",
		SupabasePublishableKey: "test-key",
		PublicBaseURL:          "https://split.my",
	}}

	form := url.Values{}
	form.Set("email", "owner@example.com")
	form.Set("password", "secret123")
	form.Set("password_confirm", "different")
	req := httptest.NewRequest(http.MethodPost, "/signup", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	s.handleWebSignUpPost(rec, req)

	loc := rec.Header().Get("Location")
	if !strings.Contains(loc, "Passwords+do+not+match") {
		t.Fatalf("unexpected redirect: %s", loc)
	}
}

func TestWebSignUpPostWithSession(t *testing.T) {
	mock, cleanup := withMockAuthHTTP(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/auth/v1/signup" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"access_token":  "at-test",
			"refresh_token": "rt-test",
		})
	})
	defer cleanup()

	s := &Server{cfg: Config{
		SupabaseURL:            mock.URL,
		SupabasePublishableKey: "test-key",
		PublicBaseURL:          "https://split.my",
	}}

	form := url.Values{}
	form.Set("email", "owner@example.com")
	form.Set("password", "secret123")
	form.Set("password_confirm", "secret123")
	req := httptest.NewRequest(http.MethodPost, "/signup", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	s.handleWebSignUpPost(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/capture" {
		t.Fatalf("redirect = %q, want /capture", loc)
	}
	var at, rt string
	for _, c := range rec.Result().Cookies() {
		switch c.Name {
		case cookieAccess:
			at = c.Value
		case cookieRefresh:
			rt = c.Value
		}
	}
	if at != "at-test" || rt != "rt-test" {
		t.Fatalf("cookies = %q / %q", at, rt)
	}
}

func TestWebSignUpPostAwaitingConfirmation(t *testing.T) {
	mock, cleanup := withMockAuthHTTP(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"user-id","email":"owner@example.com"}`))
	})
	defer cleanup()

	s := &Server{cfg: Config{
		SupabaseURL:            mock.URL,
		SupabasePublishableKey: "test-key",
		PublicBaseURL:          "https://split.my",
	}}

	form := url.Values{}
	form.Set("email", "owner@example.com")
	form.Set("password", "secret123")
	form.Set("password_confirm", "secret123")
	req := httptest.NewRequest(http.MethodPost, "/signup", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	s.handleWebSignUpPost(rec, req)

	loc := rec.Header().Get("Location")
	if !strings.HasPrefix(loc, "/signin?") {
		t.Fatalf("redirect = %q, want /signin", loc)
	}
	u, err := url.Parse(loc)
	if err != nil {
		t.Fatal(err)
	}
	if u.Query().Get("info") == "" {
		t.Fatalf("expected info message in redirect: %s", loc)
	}
	if u.Query().Get("email") != "owner@example.com" {
		t.Fatalf("email not preserved: %s", loc)
	}
	for _, c := range rec.Result().Cookies() {
		if c.Name == cookieAccess || c.Name == cookieRefresh {
			t.Fatalf("unexpected session cookie %s on confirmation flow", c.Name)
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

func TestSignInPageLinksToSignup(t *testing.T) {
	s := &Server{cfg: Config{PublicBaseURL: "https://split.my"}}
	if err := s.initWeb(); err != nil {
		t.Fatalf("initWeb: %v", err)
	}
	var buf strings.Builder
	if err := s.templates.ExecuteTemplate(&buf, "owner/signin.html", map[string]any{}); err != nil {
		t.Fatalf("render signin: %v", err)
	}
	if !strings.Contains(buf.String(), `href="/signup"`) {
		t.Fatal("expected link to signup page")
	}
}

func TestSignupPageRenders(t *testing.T) {
	s := &Server{cfg: Config{PublicBaseURL: "https://split.my"}}
	if err := s.initWeb(); err != nil {
		t.Fatalf("initWeb: %v", err)
	}
	var buf strings.Builder
	if err := s.templates.ExecuteTemplate(&buf, "owner/signup.html", map[string]any{
		"Email": "owner@example.com",
	}); err != nil {
		t.Fatalf("render signup: %v", err)
	}
	out := buf.String()
	for _, want := range []string{
		`action="/signup"`,
		`name="password_confirm"`,
		`value="owner@example.com"`,
		`href="/signin"`,
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected %q in signup page", want)
		}
	}
}
