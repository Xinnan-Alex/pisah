package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"encoding/base64"
	"encoding/json"
	"errors"
	"log/slog"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"
)

// jwksCache fetches and caches Supabase's ECC public keys (ES256). Modern Supabase
// signs user sessions with an asymmetric signing key, so we verify against the
// project's JWKS rather than the legacy HS256 shared secret.
type jwksCache struct {
	url     string
	mu      sync.RWMutex
	keys    map[string]*ecdsa.PublicKey
	fetched time.Time
}

func newJWKS(supabaseURL string) *jwksCache {
	return &jwksCache{
		url:  strings.TrimRight(supabaseURL, "/") + "/auth/v1/.well-known/jwks.json",
		keys: map[string]*ecdsa.PublicKey{},
	}
}

// key returns the public key for a kid, refreshing once if it isn't cached
// (handles signing-key rotation) but rate-limiting refreshes to avoid hammering.
func (j *jwksCache) key(kid string) (*ecdsa.PublicKey, error) {
	j.mu.RLock()
	k := j.keys[kid]
	age := time.Since(j.fetched)
	j.mu.RUnlock()
	if k != nil {
		return k, nil
	}
	if age < 30*time.Second {
		return nil, errors.New("unknown signing key")
	}
	if err := j.refresh(); err != nil {
		slog.Warn("jwks refresh failed", "error", err, "url", j.url)
		return nil, err
	}
	j.mu.RLock()
	defer j.mu.RUnlock()
	if k := j.keys[kid]; k != nil {
		return k, nil
	}
	return nil, errors.New("unknown signing key")
}

func (j *jwksCache) refresh() error {
	resp, err := http.Get(j.url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	var doc struct {
		Keys []struct {
			Kid, Kty, Crv, X, Y string
		} `json:"keys"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		return err
	}
	m := map[string]*ecdsa.PublicKey{}
	for _, k := range doc.Keys {
		if k.Kty != "EC" || k.Crv != "P-256" {
			continue
		}
		x, err1 := base64.RawURLEncoding.DecodeString(k.X)
		y, err2 := base64.RawURLEncoding.DecodeString(k.Y)
		if err1 != nil || err2 != nil {
			continue
		}
		m[k.Kid] = &ecdsa.PublicKey{
			Curve: elliptic.P256(),
			X:     new(big.Int).SetBytes(x),
			Y:     new(big.Int).SetBytes(y),
		}
	}
	if len(m) == 0 {
		return errors.New("jwks: no usable keys")
	}
	j.mu.Lock()
	j.keys = m
	j.fetched = time.Now()
	j.mu.Unlock()
	slog.Info("jwks refreshed", "url", j.url, "keys", len(m))
	return nil
}
