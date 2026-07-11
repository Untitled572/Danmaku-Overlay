package websocket

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	ws "nhooyr.io/websocket"
)

func NewClient(conn *ws.Conn, clientType ClientType, episodeID uint, hub *Hub) *Client {
	return &Client{
		conn:       conn,
		clientType: clientType,
		episodeID:  episodeID,
		hub:        hub,
		sendCh:     make(chan []byte, 256),
	}
}

func (c *Client) ReadPump() {
	defer func() {
		c.hub.UnregisterClient(c)
	}()

	c.conn.SetReadLimit(maxMessageSize)
	ctx := context.Background()

	for {
		_, data, err := c.conn.Read(ctx)
		if err != nil {
			if ws.CloseStatus(err) == ws.StatusNormalClosure || ws.CloseStatus(err) == ws.StatusGoingAway {
				slog.Info("ws client closed connection normally", "type", c.clientType, "ep", c.episodeID)
			} else {
				slog.Warn("ws read error", "error", err, "type", c.clientType, "ep", c.episodeID)
			}
			return
		}

		var msg WSMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			slog.Warn("ws invalid message", "error", err, "type", c.clientType, "ep", c.episodeID)
			continue
		}

		switch msg.Type {
		case MsgTypePong:
			c.lastPongTime = time.Now()
		case MsgTypePing:
			pong, _ := json.Marshal(WSMessage{Type: MsgTypePong})
			c.mu.Lock()
			if !c.closed {
				c.sendCh <- pong
			}
			c.mu.Unlock()
		default:
			slog.Debug("ws unhandled message type", "type", msg.Type, "ep", c.episodeID)
		}
	}
}

func (c *Client) WritePump() {
	pingTicker := time.NewTicker(pingPeriod)
	defer func() {
		pingTicker.Stop()
		c.conn.Close(ws.StatusNormalClosure, "closed")
	}()

	for {
		select {
		case msg, ok := <-c.sendCh:
			if !ok {
				return
			}
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			err := c.conn.Write(ctx, ws.MessageText, msg)
			cancel()
			if err != nil {
				return
			}
		case <-pingTicker.C:
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			err := c.conn.Ping(ctx)
			cancel()
			if err != nil {
				return
			}
		}
	}
}

func (c *Client) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.closed {
		c.closed = true
		close(c.sendCh)
		c.conn.Close(ws.StatusNormalClosure, "server closed")
	}
}

func (c *Client) SendMessage(msg WSMessage) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal message: %w", err)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return fmt.Errorf("client closed")
	}
	c.sendCh <- data
	return nil
}
