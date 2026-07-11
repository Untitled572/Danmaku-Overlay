package websocket

import "encoding/json"

type MessageType string

const (
	MsgTypeDanmaku    MessageType = "danmaku"
	MsgTypeTimeSync   MessageType = "time_sync"
	MsgTypeConfigSync MessageType = "config_sync"
	MsgTypePing       MessageType = "ping"
	MsgTypePong       MessageType = "pong"
)

type WSMessage struct {
	Type    MessageType     `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

type TimeSyncPayload struct {
	CurrentTime  float64 `json:"current_time"`
	PlaybackRate float64 `json:"playback_rate"`
	IsSeeking    bool    `json:"is_seeking"`
	IsPaused     bool    `json:"is_paused"`
}

type DanmakuPayload struct {
	Lines []DanmakuLine `json:"lines"`
}

type DanmakuLine struct {
	Time  float64 `json:"time"`
	Text  string  `json:"text"`
	Color int     `json:"color"`
	Type  int     `json:"type"`
}
