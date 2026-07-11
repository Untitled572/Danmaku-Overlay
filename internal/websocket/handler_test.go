package websocket

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	ws "nhooyr.io/websocket"
)

func TestServeWS_ValidConnection(t *testing.T) {
	hub := NewHub(context.Background())
	hub.Start()
	defer hub.Stop()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handler := ServeWS(hub, func(next http.Handler) http.Handler {
			return next
		})
		handler.ServeHTTP(w, r)
	}))
	defer ts.Close()

	u := "ws" + ts.URL[4:] + "/ws?client=overlay&ep=1&token=test"
	conn, _, err := ws.Dial(context.Background(), u, nil)
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	defer conn.Close(ws.StatusNormalClosure, "done")

	if !waitForClientCount(hub, 1, 3*time.Second) {
		t.Fatalf("expected 1 client, got %d", hub.GetClientCount())
	}

	hub.BroadcastToAll(WSMessage{Type: MsgTypeDanmaku, Payload: json.RawMessage(`{"test":"hello"}`)})
	msg := readWSMessage(t, conn)
	if msg.Type != MsgTypeDanmaku {
		t.Errorf("expected danmaku, got %s", msg.Type)
	}
}

func TestServeWS_MissingClient(t *testing.T) {
	hub := NewHub(context.Background())
	hub.Start()
	defer hub.Stop()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handler := ServeWS(hub, func(next http.Handler) http.Handler {
			return next
		})
		handler.ServeHTTP(w, r)
	}))
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/ws?ep=1&token=test")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestServeWS_MissingEpisode(t *testing.T) {
	hub := NewHub(context.Background())
	hub.Start()
	defer hub.Stop()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handler := ServeWS(hub, func(next http.Handler) http.Handler {
			return next
		})
		handler.ServeHTTP(w, r)
	}))
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/ws?client=overlay&token=test")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestServeWS_MissingToken(t *testing.T) {
	hub := NewHub(context.Background())
	hub.Start()
	defer hub.Stop()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handler := ServeWS(hub, func(next http.Handler) http.Handler {
			return next
		})
		handler.ServeHTTP(w, r)
	}))
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/ws?client=overlay&ep=1")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}

func TestServeWS_InvalidClientType(t *testing.T) {
	hub := NewHub(context.Background())
	hub.Start()
	defer hub.Stop()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handler := ServeWS(hub, func(next http.Handler) http.Handler {
			return next
		})
		handler.ServeHTTP(w, r)
	}))
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/ws?client=invalid&ep=1&token=test")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestServeWS_AuthMiddleware(t *testing.T) {
	hub := NewHub(context.Background())
	hub.Start()
	defer hub.Stop()

	validToken := "valid-token-123"

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handler := ServeWS(hub, func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				auth := r.Header.Get("Authorization")
				if auth != "Bearer "+validToken {
					http.Error(w, "unauthorized", http.StatusUnauthorized)
					return
				}
				next.ServeHTTP(w, r)
			})
		})
		handler.ServeHTTP(w, r)
	}))
	defer ts.Close()

	// valid token -> success
	u := "ws" + ts.URL[4:] + "/ws?client=overlay&ep=1&token=" + validToken
	conn, _, err := ws.Dial(context.Background(), u, nil)
	if err != nil {
		t.Fatalf("valid token dial failed: %v", err)
	}
	defer conn.Close(ws.StatusNormalClosure, "done")

	if !waitForClientCount(hub, 1, 3*time.Second) {
		t.Errorf("expected 1 client with valid token, got %d", hub.GetClientCount())
	}

	// invalid token -> rejected (use http:// URL for http.Get)
	resp, err := http.Get(ts.URL + "/ws?client=overlay&ep=2&token=wrong")
	if err != nil {
		t.Fatalf("invalid token request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401 for invalid token, got %d", resp.StatusCode)
	}
}
