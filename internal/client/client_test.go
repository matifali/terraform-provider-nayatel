package client

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func testJWT(t *testing.T, expiresAt time.Time) string {
	t.Helper()

	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none","typ":"JWT"}`))
	payload, err := json.Marshal(struct {
		Exp int64 `json:"exp"`
	}{Exp: expiresAt.Unix()})
	if err != nil {
		t.Fatalf("failed to marshal JWT payload: %v", err)
	}

	return fmt.Sprintf("%s.%s.signature", header, base64.RawURLEncoding.EncodeToString(payload))
}

func TestRequestFetchesAndSendsCSRFSession(t *testing.T) {
	ctx := context.Background()

	var csrfCalls atomic.Int32
	var apiCalls atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/csrf-token":
			csrfCalls.Add(1)
			if r.Method != http.MethodGet {
				t.Errorf("csrf method = %s, want %s", r.Method, http.MethodGet)
			}
			http.SetCookie(w, &http.Cookie{Name: "csrf-token", Value: "csrf-1", Path: "/"})
			w.Header().Set("x-csrf-token", "csrf-1")
			_, _ = w.Write([]byte(`{"token":"csrf-1"}`))
		case "/api/resource":
			apiCalls.Add(1)
			if r.Method != http.MethodGet {
				t.Errorf("api method = %s, want %s", r.Method, http.MethodGet)
			}
			if got, want := r.Header.Get("Authorization"), "Bearer api-token"; got != want {
				t.Errorf("Authorization = %q, want %q", got, want)
			}
			if got, want := r.Header.Get("X-CSRF-Token"), "csrf-1"; got != want {
				t.Errorf("X-CSRF-Token = %q, want %q", got, want)
			}
			cookie, err := r.Cookie("csrf-token")
			if err != nil {
				t.Errorf("missing csrf-token cookie: %v", err)
			} else if cookie.Value != "csrf-1" {
				t.Errorf("csrf-token cookie = %q, want csrf-1", cookie.Value)
			}
			_, _ = w.Write([]byte(`{"ok":true}`))
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	httpClient := server.Client()
	if httpClient.Jar != nil {
		t.Fatalf("test expected httptest client to start without a cookie jar")
	}

	c := NewClient("user", "api-token", WithBaseURL(server.URL+"/api"), WithHTTPClient(httpClient))
	if httpClient.Jar == nil {
		t.Fatalf("NewClient did not install a cookie jar on a custom HTTP client")
	}

	resp, err := c.Get(ctx, "/resource")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if got, want := string(resp), `{"ok":true}`; got != want {
		t.Fatalf("response = %s, want %s", got, want)
	}
	if got := csrfCalls.Load(); got != 1 {
		t.Fatalf("csrf calls = %d, want 1", got)
	}
	if got := apiCalls.Load(); got != 1 {
		t.Fatalf("api calls = %d, want 1", got)
	}
}

func TestRequestReadsCSRFTokenFromJSONBody(t *testing.T) {
	ctx := context.Background()

	var csrfCalls atomic.Int32
	var apiCalls atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/csrf-token":
			csrfCalls.Add(1)
			_, _ = w.Write([]byte(`{"csrf_token":"json-csrf"}`))
		case "/api/resource":
			apiCalls.Add(1)
			if got, want := r.Header.Get("X-CSRF-Token"), "json-csrf"; got != want {
				t.Errorf("X-CSRF-Token = %q, want %q", got, want)
			}
			_, _ = w.Write([]byte(`{"ok":true}`))
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	c := NewClient("user", "api-token", WithBaseURL(server.URL+"/api"), WithHTTPClient(server.Client()))
	resp, err := c.Get(ctx, "/resource")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if got, want := string(resp), `{"ok":true}`; got != want {
		t.Fatalf("response = %s, want %s", got, want)
	}
	if got := csrfCalls.Load(); got != 1 {
		t.Fatalf("csrf calls = %d, want 1", got)
	}
	if got := apiCalls.Load(); got != 1 {
		t.Fatalf("api calls = %d, want 1", got)
	}
}

func TestRequestCachesCSRFTokenAcrossRequests(t *testing.T) {
	ctx := context.Background()

	var csrfCalls atomic.Int32
	var apiCalls atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/csrf-token":
			csrfCalls.Add(1)
			http.SetCookie(w, &http.Cookie{Name: "csrf-token", Value: "csrf-cached", Path: "/"})
			w.Header().Set("x-csrf-token", "csrf-cached")
			_, _ = w.Write([]byte(`{"token":"csrf-cached"}`))
		case "/api/resource":
			apiCalls.Add(1)
			if got, want := r.Header.Get("X-CSRF-Token"), "csrf-cached"; got != want {
				t.Errorf("X-CSRF-Token = %q, want %q", got, want)
			}
			if _, err := r.Cookie("csrf-token"); err != nil {
				t.Errorf("missing csrf-token cookie: %v", err)
			}
			_, _ = w.Write([]byte(`{"ok":true}`))
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	c := NewClient("user", "api-token", WithBaseURL(server.URL+"/api"), WithHTTPClient(server.Client()))
	for i := 0; i < 2; i++ {
		if _, err := c.Get(ctx, "/resource"); err != nil {
			t.Fatalf("Get #%d returned error: %v", i+1, err)
		}
	}

	if got := csrfCalls.Load(); got != 1 {
		t.Fatalf("csrf calls = %d, want 1", got)
	}
	if got := apiCalls.Load(); got != 2 {
		t.Fatalf("api calls = %d, want 2", got)
	}
}

func TestRequestRefreshesCSRFAndRetriesOnce(t *testing.T) {
	ctx := context.Background()

	var csrfCalls atomic.Int32
	var apiCalls atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/csrf-token":
			call := csrfCalls.Add(1)
			token := fmt.Sprintf("csrf-%d", call)
			http.SetCookie(w, &http.Cookie{Name: "csrf-token", Value: token, Path: "/"})
			w.Header().Set("x-csrf-token", token)
			_, _ = fmt.Fprintf(w, `{"token":%q}`, token)
		case "/api/retry":
			call := apiCalls.Add(1)
			if r.Method != http.MethodPost {
				t.Errorf("api method = %s, want %s", r.Method, http.MethodPost)
			}

			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Errorf("failed to read request body: %v", err)
			}
			var payload struct {
				Name string `json:"name"`
			}
			if err := json.Unmarshal(body, &payload); err != nil {
				t.Errorf("failed to decode request body %q: %v", string(body), err)
			}
			if payload.Name != "terraform" {
				t.Errorf("payload name = %q, want terraform", payload.Name)
			}

			if call == 1 {
				if got, want := r.Header.Get("X-CSRF-Token"), "csrf-1"; got != want {
					t.Errorf("first X-CSRF-Token = %q, want %q", got, want)
				}
				http.Error(w, "Invalid or missing CSRF token", http.StatusForbidden)
				return
			}

			if got, want := r.Header.Get("X-CSRF-Token"), "csrf-2"; got != want {
				t.Errorf("retry X-CSRF-Token = %q, want %q", got, want)
			}
			cookie, err := r.Cookie("csrf-token")
			if err != nil {
				t.Errorf("missing retry csrf-token cookie: %v", err)
			} else if cookie.Value != "csrf-2" {
				t.Errorf("retry csrf-token cookie = %q, want csrf-2", cookie.Value)
			}
			_, _ = w.Write([]byte(`{"ok":true}`))
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	c := NewClient("user", "api-token", WithBaseURL(server.URL+"/api"), WithHTTPClient(server.Client()))
	resp, err := c.Post(ctx, "/retry", struct {
		Name string `json:"name"`
	}{Name: "terraform"})
	if err != nil {
		t.Fatalf("Post returned error: %v", err)
	}
	if got, want := string(resp), `{"ok":true}`; got != want {
		t.Fatalf("response = %s, want %s", got, want)
	}
	if got := csrfCalls.Load(); got != 2 {
		t.Fatalf("csrf calls = %d, want 2", got)
	}
	if got := apiCalls.Load(); got != 2 {
		t.Fatalf("api calls = %d, want 2", got)
	}
}

func TestShouldRetryCSRFRecognizesExpiryResponses(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       []byte
		want       bool
	}{
		{
			name:       "forbidden token mismatch",
			statusCode: http.StatusForbidden,
			body:       []byte("Token mismatch"),
			want:       true,
		},
		{
			name:       "http 419 page expired",
			statusCode: 419,
			body:       []byte("Page Expired"),
			want:       true,
		},
		{
			name:       "ordinary forbidden",
			statusCode: http.StatusForbidden,
			body:       []byte("permission denied"),
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldRetryCSRF(tt.statusCode, tt.body); got != tt.want {
				t.Fatalf("shouldRetryCSRF() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRequestDoesNotRetryNonCSRFForbidden(t *testing.T) {
	ctx := context.Background()

	var csrfCalls atomic.Int32
	var apiCalls atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/csrf-token":
			csrfCalls.Add(1)
			http.SetCookie(w, &http.Cookie{Name: "csrf-token", Value: "csrf-1", Path: "/"})
			w.Header().Set("x-csrf-token", "csrf-1")
			_, _ = w.Write([]byte(`{"token":"csrf-1"}`))
		case "/api/forbidden":
			apiCalls.Add(1)
			http.Error(w, "permission denied", http.StatusForbidden)
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	c := NewClient("user", "api-token", WithBaseURL(server.URL+"/api"), WithHTTPClient(server.Client()))
	_, err := c.Get(ctx, "/forbidden")
	if err == nil {
		t.Fatalf("Get returned nil error, want APIError")
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("error = %T, want *APIError", err)
	}
	if apiErr.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", apiErr.StatusCode, http.StatusForbidden)
	}
	if got := csrfCalls.Load(); got != 1 {
		t.Fatalf("csrf calls = %d, want 1", got)
	}
	if got := apiCalls.Load(); got != 1 {
		t.Fatalf("api calls = %d, want 1", got)
	}
}

func TestNewClientWithLoginUsesCSRFFormFlow(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	ctx := context.Background()

	var csrfCalls atomic.Int32
	var authCalls atomic.Int32
	var tokenCalls atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/csrf-token":
			csrfCalls.Add(1)
			if r.Method != http.MethodGet {
				t.Errorf("csrf method = %s, want %s", r.Method, http.MethodGet)
			}
			http.SetCookie(w, &http.Cookie{Name: "csrf-token", Value: "login-csrf", Path: "/"})
			w.Header().Set("x-csrf-token", "login-csrf")
			_, _ = w.Write([]byte(`{"token":"login-csrf"}`))
		case "/api/authenticate":
			authCalls.Add(1)
			if r.Method != http.MethodPost {
				t.Errorf("auth method = %s, want %s", r.Method, http.MethodPost)
			}
			if got := r.Header.Get("Authorization"); got != "" {
				t.Errorf("Authorization header = %q, want empty", got)
			}
			if got, want := r.Header.Get("X-CSRF-Token"), "login-csrf"; got != want {
				t.Errorf("X-CSRF-Token = %q, want %q", got, want)
			}
			cookie, err := r.Cookie("csrf-token")
			if err != nil {
				t.Errorf("missing csrf-token cookie: %v", err)
			} else if cookie.Value != "login-csrf" {
				t.Errorf("csrf-token cookie = %q, want login-csrf", cookie.Value)
			}
			if got := r.Header.Get("Content-Type"); !strings.HasPrefix(got, "application/x-www-form-urlencoded") {
				t.Errorf("Content-Type = %q, want form urlencoded", got)
			}
			if err := r.ParseForm(); err != nil {
				t.Errorf("ParseForm failed: %v", err)
			}
			if got, want := r.Form.Get("userid"), "login-user"; got != want {
				t.Errorf("userid = %q, want %q", got, want)
			}
			if got, want := r.Form.Get("password"), "login-pass"; got != want {
				t.Errorf("password = %q, want %q", got, want)
			}
			_, _ = w.Write([]byte(`{"token":"login-token"}`))
		case "/api/token":
			tokenCalls.Add(1)
			http.Error(w, "old token endpoint should not be called", http.StatusInternalServerError)
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	c, err := NewClientWithLogin(ctx, "login-user", "login-pass", WithBaseURL(server.URL+"/api"), WithHTTPClient(server.Client()))
	if err != nil {
		t.Fatalf("NewClientWithLogin returned error: %v", err)
	}
	if got, want := c.Token, "login-token"; got != want {
		t.Fatalf("client token = %q, want %q", got, want)
	}
	if got := csrfCalls.Load(); got != 1 {
		t.Fatalf("csrf calls = %d, want 1", got)
	}
	if got := authCalls.Load(); got != 1 {
		t.Fatalf("auth calls = %d, want 1", got)
	}
	if got := tokenCalls.Load(); got != 0 {
		t.Fatalf("legacy token calls = %d, want 0", got)
	}
}

func TestNewClientWithLoginUsesValidCachedToken(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	ctx := context.Background()

	cachedToken := testJWT(t, time.Now().Add(time.Hour))
	if err := saveCachedToken("cache-user", cachedToken); err != nil {
		t.Fatalf("saveCachedToken returned error: %v", err)
	}

	var unexpectedCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		unexpectedCalls.Add(1)
		t.Errorf("unexpected auth request to %s", r.URL.Path)
		http.Error(w, "unexpected request", http.StatusInternalServerError)
	}))
	defer server.Close()

	c, err := NewClientWithLogin(ctx, "cache-user", "cache-pass", WithBaseURL(server.URL+"/api"), WithHTTPClient(server.Client()))
	if err != nil {
		t.Fatalf("NewClientWithLogin returned error: %v", err)
	}
	if got := c.Token; got != cachedToken {
		t.Fatalf("client token = %q, want cached token", got)
	}
	if got := unexpectedCalls.Load(); got != 0 {
		t.Fatalf("unexpected auth calls = %d, want 0", got)
	}
}

func TestNewClientWithLoginFallsBackWhenCachedTokenExpired(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	ctx := context.Background()

	if err := saveCachedToken("cache-user", testJWT(t, time.Now().Add(-time.Hour))); err != nil {
		t.Fatalf("saveCachedToken returned error: %v", err)
	}
	freshToken := testJWT(t, time.Now().Add(time.Hour))

	var csrfCalls atomic.Int32
	var authCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/csrf-token":
			csrfCalls.Add(1)
			w.Header().Set("x-csrf-token", "fresh-csrf")
		case "/api/authenticate":
			authCalls.Add(1)
			if got, want := r.Header.Get("X-CSRF-Token"), "fresh-csrf"; got != want {
				t.Errorf("X-CSRF-Token = %q, want %q", got, want)
			}
			_, _ = fmt.Fprintf(w, `{"token":%q}`, freshToken)
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	c, err := NewClientWithLogin(ctx, "cache-user", "cache-pass", WithBaseURL(server.URL+"/api"), WithHTTPClient(server.Client()))
	if err != nil {
		t.Fatalf("NewClientWithLogin returned error: %v", err)
	}
	if got := c.Token; got != freshToken {
		t.Fatalf("client token = %q, want fresh token", got)
	}
	if got := csrfCalls.Load(); got != 1 {
		t.Fatalf("csrf calls = %d, want 1", got)
	}
	if got := authCalls.Load(); got != 1 {
		t.Fatalf("auth calls = %d, want 1", got)
	}
}

func TestTokenCachePathEscapesUsername(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configDir)

	cachePath, err := getTokenCachePath(`../team/user@example.com`)
	if err != nil {
		t.Fatalf("getTokenCachePath returned error: %v", err)
	}

	if got, want := filepath.Dir(cachePath), filepath.Join(configDir, tokenCacheDir); got != want {
		t.Fatalf("cache dir = %q, want %q", got, want)
	}
	if got := filepath.Base(cachePath); !strings.Contains(got, "%2F") {
		t.Fatalf("cache filename = %q, want escaped path separators", got)
	}
}
