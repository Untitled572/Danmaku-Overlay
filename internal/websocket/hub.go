package websocket

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	ws "nhooyr.io/websocket"
)

const (
	heartbeatInterval = 30 * time.Second
	pongWait          = 60 * time.Second
	pingPeriod        = (pongWait * 9) / 10
	maxMessageSize    = 64 * 1024
)

type ClientType string

const (
	ClientOverlay ClientType = "overlay"
	ClientTauri   ClientType = "tauri"
	ClientUI      ClientType = "ui"
)

type Client struct {
	conn         *ws.Conn
	clientType   ClientType
	episodeID    uint
	hub          *Hub
	sendCh       chan []byte
	closed       bool
	mu           sync.Mutex
	lastPongTime time.Time
}

type Hub struct {
	clients    map[*Client]bool
	rooms      map[uint]map[*Client]bool
	register   chan *Client
	unregister chan *Client
	broadcast  chan []byte
	ctx        context.Context
	cancel     context.CancelFunc
	mu         sync.RWMutex
}

func NewHub(ctx context.Context) *Hub {
	ctx, cancel := context.WithCancel(ctx)
	return &Hub{
		clients:    make(map[*Client]bool),
		rooms:      make(map[uint]map[*Client]bool),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		broadcast:  make(chan []byte, 256),
		ctx:        ctx,
		cancel:     cancel,
	}
}

func (h *Hub) Start() {
	go h.run()
}

func (h *Hub) Stop() {
	h.cancel()
}

func (h *Hub) run() {
	heartbeatTicker := time.NewTicker(heartbeatInterval)
	defer heartbeatTicker.Stop()

	for {
		select {
		case <-h.ctx.Done():
			h.mu.Lock()
			for c := range h.clients {
				c.Close()
			}
			h.clients = make(map[*Client]bool)
			h.rooms = make(map[uint]map[*Client]bool)
			h.mu.Unlock()
			return
		case c := <-h.register:
			h.mu.Lock()
			h.clients[c] = true
			if h.rooms[c.episodeID] == nil {
				h.rooms[c.episodeID] = make(map[*Client]bool)
			}
			h.rooms[c.episodeID][c] = true
			h.mu.Unlock()
			slog.Info("ws client connected", "type", c.clientType, "ep", c.episodeID)
		case c := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[c]; ok {
				delete(h.clients, c)
				if clients, ok := h.rooms[c.episodeID]; ok {
					delete(clients, c)
					if len(clients) == 0 {
						delete(h.rooms, c.episodeID)
					}
				}
				c.Close()
			}
			h.mu.Unlock()
			slog.Info("ws client disconnected", "type", c.clientType, "ep", c.episodeID)
		case msg := <-h.broadcast:
			h.mu.RLock()
			for c := range h.clients {
				c.mu.Lock()
				if !c.closed {
					c.sendCh <- msg
				}
				c.mu.Unlock()
			}
			h.mu.RUnlock()
		case <-heartbeatTicker.C:
			payload, err := json.Marshal(WSMessage{Type: MsgTypePing})
			if err != nil {
				slog.Error("failed to marshal ping", "error", err)
				continue
			}
			h.mu.RLock()
			for c := range h.clients {
				c.mu.Lock()
				if !c.closed {
					c.sendCh <- payload
				}
				c.mu.Unlock()
			}
			h.mu.RUnlock()
		}
	}
}

func (h *Hub) RegisterClient(c *Client) {
	h.register <- c
}

func (h *Hub) UnregisterClient(c *Client) {
	h.unregister <- c
}

func (h *Hub) BroadcastToEpisode(episodeID uint, msg WSMessage) {
	data, err := json.Marshal(msg)
	if err != nil {
		slog.Error("failed to marshal broadcast message", "error", err)
		return
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	if clients, ok := h.rooms[episodeID]; ok {
		for c := range clients {
			c.mu.Lock()
			if !c.closed {
				c.sendCh <- data
			}
			c.mu.Unlock()
		}
	}
}

func (h *Hub) BroadcastToAll(msg WSMessage) {
	data, err := json.Marshal(msg)
	if err != nil {
		slog.Error("failed to marshal broadcast message", "error", err)
		return
	}
	h.broadcast <- data
}

func (h *Hub) GetClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}
