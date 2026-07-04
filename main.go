package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Config struct {
	Host                   string // bind address; empty = all interfaces
	Port                   string
	DatabaseURL            string
	SupabaseURL            string // project URL, e.g. https://abc.supabase.co — for JWKS (ES256)
	SupabasePublishableKey string // sb_publishable_... — GoTrue sign-in proxy (apikey header)
	SupabaseSecretKey      string // sb_secret_... — optional; server-side Supabase API (unused today)
	JWTSecret              string // legacy HS256 shared secret (optional)
	PublicBaseURL          string // e.g. https://split.my  — used to build share links
}

func loadConfig() (Config, error) {
	c := Config{
		Host:                   os.Getenv("HOST"),
		Port:                   envOr("PORT", "8080"),
		DatabaseURL:            os.Getenv("DATABASE_URL"),
		SupabaseURL:            os.Getenv("SUPABASE_URL"),
		SupabasePublishableKey: envFirst("SUPABASE_PUBLISHABLE_KEY", "SUPABASE_ANON_KEY"),
		SupabaseSecretKey:      os.Getenv("SUPABASE_SECRET_KEY"),
		JWTSecret:              os.Getenv("SUPABASE_JWT_SECRET"),
		PublicBaseURL:          envOr("PUBLIC_BASE_URL", "https://split.my"),
	}
	if c.DatabaseURL == "" {
		return c, errors.New("DATABASE_URL is required")
	}
	if c.SupabaseURL == "" && c.JWTSecret == "" {
		return c, errors.New("SUPABASE_URL (for JWKS) or SUPABASE_JWT_SECRET is required")
	}
	return c, nil
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func envFirst(keys ...string) string {
	for _, k := range keys {
		if v := os.Getenv(k); v != "" {
			return v
		}
	}
	return ""
}

type Server struct {
	cfg       Config
	store     *Store
	broker    *Broker
	jwks      *jwksCache
	templates *template.Template
	staticFS  fs.FS
}

func main() {
	setupLogger()

	cfg, err := loadConfig()
	if err != nil {
		slog.Error("config load failed", "error", err)
		os.Exit(1)
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("database connect failed", "error", err)
		os.Exit(1)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		slog.Error("database ping failed", "error", err)
		os.Exit(1)
	}

	srv := &Server{cfg: cfg, store: &Store{pool: pool}, broker: newBroker()}
	if cfg.SupabaseURL != "" {
		srv.jwks = newJWKS(cfg.SupabaseURL)
	}
	if err := srv.initWeb(); err != nil {
		slog.Error("web init failed", "error", err)
		os.Exit(1)
	}

	addr := ":" + cfg.Port
	if cfg.Host != "" {
		addr = cfg.Host + ":" + cfg.Port
	}
	httpSrv := &http.Server{
		Addr:              addr,
		Handler:           srv.routes(),
		ReadHeaderTimeout: 10 * time.Second,
		// No WriteTimeout: the SSE endpoint is a long-lived stream.
	}
	slog.Info("server starting",
		"addr", addr,
		"port", cfg.Port,
		"public_base_url", cfg.PublicBaseURL,
		"jwks_enabled", cfg.SupabaseURL != "",
		"auth_proxy_enabled", cfg.SupabaseURL != "" && cfg.SupabasePublishableKey != "",
	)
	if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("server stopped", "error", err)
		os.Exit(1)
	}
}

func (s *Server) routes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("ok")) })
	mux.HandleFunc("GET /static/", s.serveStatic)

	// Web app (htmx + templates)
	mux.HandleFunc("GET /", s.handleWebRoot)
	mux.HandleFunc("GET /signin", s.handleWebSignInGet)
	mux.HandleFunc("POST /signin", s.handleWebSignInPost)
	mux.HandleFunc("GET /signup", s.handleWebSignUpGet)
	mux.HandleFunc("POST /signup", s.handleWebSignUpPost)
	mux.HandleFunc("POST /signout", s.handleWebSignOut)
	mux.HandleFunc("GET /auth/callback", s.handleWebAuthCallback)
	mux.HandleFunc("POST /auth/session", s.handleWebAuthSession)

	mux.Handle("GET /capture", s.requireOwnerWeb(s.handleWebCapture))
	mux.Handle("POST /onboarding/seen", s.requireOwnerWeb(s.handleWebOnboardingSeen))
	mux.Handle("POST /scan", s.requireOwnerWeb(s.handleWebScan))
	mux.Handle("POST /splits", s.requireOwnerWeb(s.handleWebCreateSplit))
	mux.Handle("DELETE /splits/{slug}", s.requireOwnerWeb(s.handleWebDeleteSplit))
	mux.Handle("GET /splits/{slug}/track", s.requireOwnerWeb(s.handleWebTrack))
	mux.Handle("GET /splits/{slug}/track/participants", s.requireOwnerWeb(s.handleWebTrackParticipants))
	mux.Handle("GET /settings", s.requireOwnerWeb(s.handleWebSettingsGet))
	mux.Handle("GET /settings/qr-image", s.requireOwnerWeb(s.handleWebSettingsQRImage))
	mux.Handle("PUT /settings", s.requireOwnerWeb(s.handleWebSettingsPut))
	mux.Handle("POST /settings/duitnow-qr", s.requireOwnerWeb(s.handleWebUploadQR))

	// Friend web flow
	mux.HandleFunc("GET /r/{slug}", s.handleFriendLanding)
	mux.HandleFunc("GET /r/{slug}/qr-image", s.handleFriendQRImage)
	mux.HandleFunc("POST /r/{slug}/join", s.handleFriendJoin)
	mux.Handle("GET /r/{slug}/pick", s.requireParticipantWeb(s.handleFriendPick))
	mux.Handle("GET /r/{slug}/pick-data", s.requireParticipantWeb(s.handleFriendPickData))
	mux.Handle("POST /r/{slug}/claims", s.requireParticipantWeb(s.handleFriendClaims))
	mux.Handle("GET /r/{slug}/share", s.requireParticipantWeb(s.handleFriendShare))
	mux.Handle("GET /r/{slug}/pay", s.requireParticipantWeb(s.handleFriendPay))
	mux.Handle("POST /r/{slug}/paid", s.requireParticipantWeb(s.handleFriendPaid))

	// Owner sign-in API (proxies GoTrue so clients only need the server URL).
	mux.HandleFunc("POST /api/auth/sign-in", s.handleSignIn)
	mux.HandleFunc("POST /api/auth/sign-up", s.handleSignUp)
	mux.HandleFunc("GET /api/auth/oauth/google", s.handleGoogleOAuthStart)

	// Owner API (Supabase-authenticated).
	mux.Handle("POST /api/receipts/scan", s.requireOwner(s.handleScan))
	mux.Handle("POST /api/splits", s.requireOwner(s.handleCreateSplit))
	mux.Handle("GET /api/splits/{slug}/track", s.requireOwner(s.handleTrack))
	mux.Handle("DELETE /api/splits/{slug}", s.requireOwner(s.handleDeleteSplit))
	mux.Handle("GET /api/me/splits", s.requireOwner(s.handleListMySplits))
	mux.Handle("GET /api/me/payment-settings", s.requireOwner(s.handleGetPaymentSettings))
	mux.Handle("PUT /api/me/payment-settings", s.requireOwner(s.handleUpdatePaymentSettings))
	mux.Handle("POST /api/me/duitnow-qr", s.requireOwner(s.handleUploadDuitNowQR))

	// Friend API (public link / participant token).
	mux.HandleFunc("GET /api/splits/{slug}", s.handleGetSplit)
	mux.HandleFunc("POST /api/splits/{slug}/join", s.handleJoin)
	mux.Handle("POST /api/splits/{slug}/claims", s.requireParticipant(s.handleSetClaims))
	mux.Handle("GET /api/splits/{slug}/share", s.requireParticipant(s.handleShare))
	mux.Handle("POST /api/splits/{slug}/paid", s.requireParticipant(s.handlePaid))

	// Live payment updates (public; clients pass ?slug=). Read-only stream.
	mux.HandleFunc("GET /api/splits/{slug}/events", s.handleEvents)

	return withRecover(withLogging(withCORS(mux)))
}

// ---- auth ----

type ctxKey string

const (
	ctxOwnerID     ctxKey = "ownerID"
	ctxOwnerClaims ctxKey = "ownerClaims"
	ctxParticipant ctxKey = "participant"
	ctxSplitID     ctxKey = "splitID"
)

// requireOwner verifies a Supabase HS256 JWT and stashes the user id (sub).
func (s *Server) requireOwner(h http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tok := bearer(r)
		if tok == "" {
			writeErr(w, http.StatusUnauthorized, "missing bearer token")
			return
		}
		claims := jwt.MapClaims{}
		_, err := jwt.ParseWithClaims(tok, claims, func(t *jwt.Token) (any, error) {
			switch t.Method.(type) {
			case *jwt.SigningMethodECDSA: // modern Supabase: ES256 via JWKS
				if s.jwks == nil {
					return nil, errors.New("no JWKS configured")
				}
				kid, _ := t.Header["kid"].(string)
				return s.jwks.key(kid)
			case *jwt.SigningMethodHMAC: // legacy: HS256 shared secret
				if s.cfg.JWTSecret == "" {
					return nil, errors.New("no HS256 secret configured")
				}
				return []byte(s.cfg.JWTSecret), nil
			default:
				return nil, errors.New("unexpected signing method")
			}
		})
		if err != nil {
			slog.DebugContext(r.Context(), "owner auth failed", "error", err)
			writeErr(w, http.StatusUnauthorized, "invalid token")
			return
		}
		sub, _ := claims["sub"].(string)
		if sub == "" {
			writeErr(w, http.StatusUnauthorized, "token missing subject")
			return
		}
		ctx := context.WithValue(r.Context(), ctxOwnerID, sub)
		ctx = context.WithValue(ctx, ctxOwnerClaims, claims)
		h(w, r.WithContext(ctx))
	})
}

// requireParticipant resolves a friend's bearer token to their participant row.
func (s *Server) requireParticipant(h http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tok := bearer(r)
		if tok == "" {
			writeErr(w, http.StatusUnauthorized, "missing participant token")
			return
		}
		p, splitID, err := s.store.ParticipantByToken(r.Context(), tok)
		if err != nil {
			if !errors.Is(err, errNotFound) {
				slog.ErrorContext(r.Context(), "participant lookup failed", "error", err)
			}
			writeErr(w, http.StatusUnauthorized, "invalid participant token")
			return
		}
		ctx := context.WithValue(r.Context(), ctxParticipant, p)
		ctx = context.WithValue(ctx, ctxSplitID, splitID)
		h(w, r.WithContext(ctx))
	})
}

func bearer(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if len(h) > 7 && h[:7] == "Bearer " {
		return h[7:]
	}
	return ""
}

// ---- token / slug generation ----

const slugAlphabet = "23456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnpqrstuvwxyz" // no 0/O/1/I/l

func newSlug() string {
	b := make([]byte, 4)
	rand.Read(b)
	out := make([]byte, 4)
	for i, x := range b {
		out[i] = slugAlphabet[int(x)%len(slugAlphabet)]
	}
	return string(out)
}

func newToken() string {
	b := make([]byte, 18)
	rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

// ---- tiny http helpers ----

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

func withCORS(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*") // ponytail: open CORS for the web view; lock to your domain in prod
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		h.ServeHTTP(w, r)
	})
}

// ---- SSE broker ----
// In-memory pub/sub for payment events.
// ponytail: single-process only — events published on one instance won't reach
// SSE clients on another. Fine for one container. To scale horizontally, back it
// with Postgres LISTEN/NOTIFY or subscribe clients to Supabase Realtime directly.
type Broker struct {
	mu   sync.Mutex
	subs map[string]map[chan []byte]struct{} // splitID -> set of subscriber channels
}

func newBroker() *Broker { return &Broker{subs: map[string]map[chan []byte]struct{}{}} }

func (b *Broker) subscribe(splitID string) chan []byte {
	ch := make(chan []byte, 8)
	b.mu.Lock()
	if b.subs[splitID] == nil {
		b.subs[splitID] = map[chan []byte]struct{}{}
	}
	b.subs[splitID][ch] = struct{}{}
	b.mu.Unlock()
	return ch
}

func (b *Broker) unsubscribe(splitID string, ch chan []byte) {
	b.mu.Lock()
	if set := b.subs[splitID]; set != nil {
		delete(set, ch)
		if len(set) == 0 {
			delete(b.subs, splitID)
		}
	}
	b.mu.Unlock()
	close(ch)
}

func (b *Broker) publish(splitID string, msg []byte) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for ch := range b.subs[splitID] {
		select {
		case ch <- msg:
		default: // drop if a slow client's buffer is full; next event still fresh
		}
	}
}
