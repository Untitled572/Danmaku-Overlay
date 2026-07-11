package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTokenAuth(t *testing.T) {
	validToken := "abcdef1234567890abcdef1234567890"

	tests := []struct {
		name       string
		token      string
		setHeader  func(r *http.Request)
		wantStatus int
	}{
		{
			name:       "empty token skips auth",
			token:      "",
			setHeader:  func(r *http.Request) {},
			wantStatus: http.StatusOK,
		},
		{
			name:       "valid token returns 200",
			token:      validToken,
			setHeader:  func(r *http.Request) { r.Header.Set("Authorization", "Bearer "+validToken) },
			wantStatus: http.StatusOK,
		},
		{
			name:       "invalid token returns 401",
			token:      validToken,
			setHeader:  func(r *http.Request) { r.Header.Set("Authorization", "Bearer wrongtoken") },
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "missing authorization header returns 401",
			token:      validToken,
			setHeader:  func(r *http.Request) {},
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "non bearer schema returns 401",
			token:      validToken,
			setHeader:  func(r *http.Request) { r.Header.Set("Authorization", "Basic xxx") },
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "empty bearer token returns 401",
			token:      validToken,
			setHeader:  func(r *http.Request) { r.Header.Set("Authorization", "Bearer ") },
			wantStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := TokenAuth(tt.token)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			tt.setHeader(req)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("expected status %d, got %d", tt.wantStatus, rec.Code)
			}

			if rec.Code == http.StatusUnauthorized {
				var body map[string]string
				if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
					t.Fatalf("failed to decode response body: %v", err)
				}
				if body["error"] != "unauthorized" {
					t.Errorf("expected error 'unauthorized', got %q", body["error"])
				}
			}
		})
	}
}

func TestTokenAuthConstantTime(t *testing.T) {
	token := "supersecrettoken123"
	handler := TokenAuth(token)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Tokens of varying lengths should not panic and should all fail auth
	for _, tok := range []string{"a", "ab", "abc", "a very long token that is much longer than the real one"} {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Authorization", "Bearer "+tok)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected 401 for token %q, got %d", tok, rec.Code)
		}
	}
}
