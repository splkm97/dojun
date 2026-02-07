package game

import (
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Player represents a connected player
type Player struct {
	ID             string
	Nickname       string
	SessionID      string
	Tokens         int
	Conn           *websocket.Conn
	ConnMu         sync.Mutex
	WriteMu        sync.Mutex // Mutex for serializing writes to connection
	DisconnectedAt *time.Time
}

// NewPlayer creates a new player
func NewPlayer(id, nickname, sessionID string, conn *websocket.Conn) *Player {
	return &Player{
		ID:        id,
		Nickname:  nickname,
		SessionID: sessionID,
		Tokens:    0,
		Conn:      conn,
	}
}

// IsConnected checks if the player has an active connection
func (p *Player) IsConnected() bool {
	p.ConnMu.Lock()
	defer p.ConnMu.Unlock()
	return p.Conn != nil
}

// SetConnection updates the player's WebSocket connection
func (p *Player) SetConnection(conn *websocket.Conn) {
	p.ConnMu.Lock()
	defer p.ConnMu.Unlock()
	p.Conn = conn
	p.DisconnectedAt = nil
}

// ClearConnection marks the player as disconnected
func (p *Player) ClearConnection() {
	p.ConnMu.Lock()
	defer p.ConnMu.Unlock()
	p.Conn = nil
	now := time.Now()
	p.DisconnectedAt = &now
}

// ClearConnectionIf marks the player disconnected only if connection matches expected.
func (p *Player) ClearConnectionIf(expected *websocket.Conn) bool {
	p.ConnMu.Lock()
	defer p.ConnMu.Unlock()

	if p.Conn != expected {
		return false
	}

	p.Conn = nil
	now := time.Now()
	p.DisconnectedAt = &now
	return true
}

// GetConnection returns the current connection (thread-safe)
func (p *Player) GetConnection() *websocket.Conn {
	p.ConnMu.Lock()
	defer p.ConnMu.Unlock()
	return p.Conn
}

// DisconnectedDuration returns how long the player has been disconnected
func (p *Player) DisconnectedDuration() time.Duration {
	p.ConnMu.Lock()
	defer p.ConnMu.Unlock()
	if p.DisconnectedAt == nil {
		return 0
	}
	return time.Since(*p.DisconnectedAt)
}

// WriteMessage sends a message to the player's connection (thread-safe)
func (p *Player) WriteMessage(messageType int, data []byte) error {
	p.ConnMu.Lock()
	conn := p.Conn
	p.ConnMu.Unlock()

	if conn == nil {
		return nil
	}

	p.WriteMu.Lock()
	defer p.WriteMu.Unlock()
	return conn.WriteMessage(messageType, data)
}
