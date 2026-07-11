package websocket

import (
	"log/slog"
	"net/http"
	"strconv"

	ws "nhooyr.io/websocket"
)

func ServeWS(hub *Hub, authMiddleware func(http.Handler) http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		clientType := r.URL.Query().Get("client")
		if clientType == "" {
			http.Error(w, "missing client type", http.StatusBadRequest)
			return
		}

		switch ClientType(clientType) {
		case ClientOverlay, ClientTauri, ClientUI:
			// valid
		default:
			http.Error(w, "invalid client type", http.StatusBadRequest)
			return
		}

		epStr := r.URL.Query().Get("ep")
		if epStr == "" {
			http.Error(w, "missing episode ID", http.StatusBadRequest)
			return
		}

		epID, err := strconv.ParseUint(epStr, 10, 32)
		if err != nil {
			http.Error(w, "invalid episode ID", http.StatusBadRequest)
			return
		}

		token := r.URL.Query().Get("token")
		if token == "" {
			http.Error(w, "missing token", http.StatusUnauthorized)
			return
		}

		// Create a mock request with Bearer token for auth middleware
		mockReq := r.Clone(r.Context())
		mockReq.Header.Set("Authorization", "Bearer "+token)

		authHandler := authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			opts := &ws.AcceptOptions{
				InsecureSkipVerify: true,
				CompressionMode:    ws.CompressionDisabled,
			}

			conn, err := ws.Accept(w, r, opts)
			if err != nil {
				slog.Error("ws upgrade failed", "error", err)
				return
			}

			client := NewClient(conn, ClientType(clientType), uint(epID), hub)
			hub.RegisterClient(client)

			go client.WritePump()
			go client.ReadPump()
		}))

		authHandler.ServeHTTP(w, mockReq)
	}
}
