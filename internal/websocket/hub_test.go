package websocket

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
	"time"

	ws "nhooyr.io/websocket"
)

func setupTestServer(t *testing.T) (*Hub, *httptest.Server) {
	t.Helper()
	hub := NewHub(context.Background())
	hub.Start()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handler := ServeWS(hub, func(next http.Handler) http.Handler {
			return next
		})
		handler.ServeHTTP(w, r)
	}))
	return hub, ts
}

func dialTestClient(t *testing.T, tsURL string, ep uint, clientType string) *ws.Conn {
	t.Helper()
	u := "ws" + tsURL[4:] + "/ws?client=" + clientType + "&ep=" + strconv.FormatUint(uint64(ep), 10) + "&token=test"
	conn, _, err := ws.Dial(context.Background(), u, nil)
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	return conn
}

func waitForClientCount(hub *Hub, expected int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if hub.GetClientCount() == expected {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return false
}

func readWSMessage(t *testing.T, conn *ws.Conn) WSMessage {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_, data, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	var msg WSMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	return msg
}

func closeConn(t *testing.T, conn *ws.Conn) {
	t.Helper()
	conn.Close(ws.StatusNormalClosure, "test done")
}

func TestNewHubAndStartStop(t *testing.T) {
	hub := NewHub(context.Background())
	hub.Start()
	time.Sleep(50 * time.Millisecond)
	hub.Stop()
}

func TestHubBroadcastToAll(t *testing.T) {
	hub, ts := setupTestServer(t)
	defer ts.Close()
	defer hub.Stop()

	conn1 := dialTestClient(t, ts.URL, 1, "overlay")
	defer closeConn(t, conn1)
	conn2 := dialTestClient(t, ts.URL, 1, "overlay")
	defer closeConn(t, conn2)

	if !waitForClientCount(hub, 2, 3*time.Second) {
		t.Fatalf("expected 2 clients, got %d", hub.GetClientCount())
	}

	hub.BroadcastToAll(WSMessage{Type: MsgTypeDanmaku, Payload: json.RawMessage(`{"test":"hello"}`)})

	msg1 := readWSMessage(t, conn1)
	if msg1.Type != MsgTypeDanmaku {
		t.Errorf("client1 expected danmaku, got %s", msg1.Type)
	}

	msg2 := readWSMessage(t, conn2)
	if msg2.Type != MsgTypeDanmaku {
		t.Errorf("client2 expected danmaku, got %s", msg2.Type)
	}
}

func TestHubBroadcastToEpisode(t *testing.T) {
	hub, ts := setupTestServer(t)
	defer ts.Close()
	defer hub.Stop()

	conn1 := dialTestClient(t, ts.URL, 1, "overlay")
	defer closeConn(t, conn1)
	conn2 := dialTestClient(t, ts.URL, 1, "overlay")
	defer closeConn(t, conn2)
	conn3 := dialTestClient(t, ts.URL, 2, "overlay")
	defer closeConn(t, conn3)

	if !waitForClientCount(hub, 3, 3*time.Second) {
		t.Fatalf("expected 3 clients, got %d", hub.GetClientCount())
	}

	hub.BroadcastToEpisode(1, WSMessage{Type: MsgTypeDanmaku, Payload: json.RawMessage(`{"ep":1}`)})

	msg1 := readWSMessage(t, conn1)
	if msg1.Type != MsgTypeDanmaku {
		t.Errorf("ep1 client1 expected danmaku, got %s", msg1.Type)
	}
	msg2 := readWSMessage(t, conn2)
	if msg2.Type != MsgTypeDanmaku {
		t.Errorf("ep1 client2 expected danmaku, got %s", msg2.Type)
	}

	hub.BroadcastToEpisode(2, WSMessage{Type: MsgTypeDanmaku, Payload: json.RawMessage(`{"ep":2}`)})

	msg3 := readWSMessage(t, conn3)
	if msg3.Type != MsgTypeDanmaku {
		t.Errorf("ep2 client expected danmaku, got %s", msg3.Type)
	}
}

func TestHubRegisterUnregister(t *testing.T) {
	hub, ts := setupTestServer(t)
	defer ts.Close()
	defer hub.Stop()

	conn := dialTestClient(t, ts.URL, 1, "overlay")

	if !waitForClientCount(hub, 1, 3*time.Second) {
		t.Fatalf("expected 1 client, got %d", hub.GetClientCount())
	}

	closeConn(t, conn)

	if !waitForClientCount(hub, 0, 3*time.Second) {
		t.Fatalf("expected 0 clients, got %d", hub.GetClientCount())
	}
}

func TestHubConcurrentClients(t *testing.T) {
	hub, ts := setupTestServer(t)
	defer ts.Close()
	defer hub.Stop()

	var conns []*ws.Conn
	for i := uint(0); i < 10; i++ {
		conn := dialTestClient(t, ts.URL, i, "overlay")
		defer closeConn(t, conn)
		conns = append(conns, conn)
	}

	if !waitForClientCount(hub, 10, 3*time.Second) {
		t.Fatalf("expected 10 clients, got %d", hub.GetClientCount())
	}

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			hub.BroadcastToAll(WSMessage{
				Type:    MsgTypeDanmaku,
				Payload: json.RawMessage(`{"msg":"` + strconv.Itoa(n) + `"}`),
			})
		}(i)
	}
	wg.Wait()

	time.Sleep(200 * time.Millisecond)
}
