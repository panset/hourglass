// SPDX-License-Identifier: MIT

// fakeidp is a throwaway OIDC issuer for verification task 23 (stdlib only).
// GET /control?sub=&email=&name=&mode=ok|error sets what the next sign-in gets.
// mode=error makes the token endpoint answer 400 and echo the posted form,
// so a test can check hourglassd never logs or returns that body.
package main

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/url"
	"os"
	"sync"
	"time"
)

var (
	mu    sync.Mutex
	key   *rsa.PrivateKey
	iss   string
	codes = map[string]map[string]string{}
	next  = map[string]string{"sub": "sub-alice", "email": "alice@example.test", "name": "Alice", "mode": "ok"}
)

func b64(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }

func sign(claims map[string]any) string {
	h, _ := json.Marshal(map[string]string{"alg": "RS256", "typ": "JWT", "kid": "k1"})
	c, _ := json.Marshal(claims)
	in := b64(h) + "." + b64(c)
	sum := sha256.Sum256([]byte(in))
	sig, _ := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, sum[:])
	return in + "." + b64(sig)
}

func main() {
	addr := os.Args[1]
	iss = "http://" + addr
	key, _ = rsa.GenerateKey(rand.Reader, 2048)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"issuer": iss, "authorization_endpoint": iss + "/authorize",
			"token_endpoint": iss + "/token", "jwks_uri": iss + "/jwks", "response_types_supported": []string{"code"},
			"subject_types_supported": []string{"public"}, "id_token_signing_alg_values_supported": []string{"RS256"}})
	})
	mux.HandleFunc("GET /jwks", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": []map[string]string{{"kty": "RSA", "kid": "k1", "use": "sig", "alg": "RS256",
			"n": b64(key.PublicKey.N.Bytes()), "e": b64(big.NewInt(int64(key.PublicKey.E)).Bytes())}}})
	})
	mux.HandleFunc("GET /control", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		for k, v := range r.URL.Query() {
			next[k] = v[0]
		}
		mu.Unlock()
	})
	mux.HandleFunc("GET /authorize", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		code := b64([]byte(q.Get("state")))[:12] + "code"
		mu.Lock()
		codes[code] = map[string]string{"nonce": q.Get("nonce"), "challenge": q.Get("code_challenge")}
		mu.Unlock()
		http.Redirect(w, r, q.Get("redirect_uri")+"?code="+url.QueryEscape(code)+"&state="+url.QueryEscape(q.Get("state")), http.StatusFound)
	})
	mux.HandleFunc("POST /token", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		id, secret, _ := r.BasicAuth()
		mu.Lock()
		a := codes[r.PostForm.Get("code")]
		delete(codes, r.PostForm.Get("code"))
		n := map[string]string{}
		for k, v := range next {
			n[k] = v
		}
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if n["mode"] == "error" || a == nil {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": "invalid_grant", "error_description": "echo id=" + id + " secret=" + secret + " form=" + r.PostForm.Encode()})
			return
		}
		now := time.Now()
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "SENTINEL-ACCESSTOKEN-1d9e", "refresh_token": "SENTINEL-REFRESHTOKEN-2e8f", "token_type": "Bearer",
			"id_token": sign(map[string]any{"iss": iss, "sub": n["sub"], "aud": id, "nonce": a["nonce"], "iat": now.Unix(), "exp": now.Add(5 * time.Minute).Unix(), "email": n["email"], "name": n["name"]})})
	})
	_ = http.ListenAndServe(addr, mux)
}
