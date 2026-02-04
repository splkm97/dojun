package ws

import (
	"encoding/json"
	"log"
	"sync"

	"github.com/gorilla/websocket"
)

// Hub maintains the set of active clients and manages rooms
type Hub struct {
	// Registered clients by session ID
	clients map[string]*Client

	// Mutex for clients map
	mu sync.RWMutex

	// Register requests from clients
	register chan *Client

	// Unregister requests from clients
	unregister chan *Client
}

// NewHub creates a new Hub instance
func NewHub() *Hub {
	return &Hub{
		clients:    make(map[string]*Client),
		register:   make(chan *Client),
		unregister: make(chan *Client),
	}
}

// Run starts the hub's event loop
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			// Check for existing client with same session
			if existing, ok := h.clients[client.SessionID]; ok {
				// Close old connection
				existing.Close()
			}
			h.clients[client.SessionID] = client
			h.mu.Unlock()
			log.Printf("Client registered: %s", client.SessionID)

		case client := <-h.unregister:
			h.mu.Lock()
			if existing, ok := h.clients[client.SessionID]; ok {
				// Only unregister if it's the same client instance
				if existing == client {
					delete(h.clients, client.SessionID)
					log.Printf("Client unregistered: %s", client.SessionID)
				}
			}
			h.mu.Unlock()
		}
	}
}

// Register adds a client to the hub
func (h *Hub) Register(client *Client) {
	h.register <- client
}

// Unregister removes a client from the hub
func (h *Hub) Unregister(client *Client) {
	h.unregister <- client
}

// GetClient returns a client by session ID
func (h *Hub) GetClient(sessionID string) *Client {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.clients[sessionID]
}

// ClientCount returns the number of connected clients
func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// Client represents a connected WebSocket client
type Client struct {
	Hub       *Hub
	Conn      *websocket.Conn
	SessionID string
	Send      chan []byte

	closeMu sync.Mutex
	closed  bool
}

// NewClient creates a new client
func NewClient(hub *Hub, conn *websocket.Conn, sessionID string) *Client {
	return &Client{
		Hub:       hub,
		Conn:      conn,
		SessionID: sessionID,
		Send:      make(chan []byte, 256),
	}
}

// Close closes the client connection
func (c *Client) Close() {
	c.closeMu.Lock()
	defer c.closeMu.Unlock()

	if c.closed {
		return
	}
	c.closed = true
	close(c.Send)
	c.Conn.Close()
}

// IsClosed returns whether the client is closed
func (c *Client) IsClosed() bool {
	c.closeMu.Lock()
	defer c.closeMu.Unlock()
	return c.closed
}

// WritePump pumps messages from the hub to the websocket connection
func (c *Client) WritePump() {
	defer func() {
		c.Conn.Close()
	}()

	for message := range c.Send {
		if err := c.Conn.WriteMessage(websocket.TextMessage, message); err != nil {
			log.Printf("Error writing to client %s: %v", c.SessionID, err)
			return
		}
	}
}

// SendMessage sends a message to this client
func (c *Client) SendMessage(msg *Message) error {
	c.closeMu.Lock()
	if c.closed {
		c.closeMu.Unlock()
		return nil
	}
	c.closeMu.Unlock()

	bytes, err := marshalMessage(msg)
	if err != nil {
		return err
	}

	select {
	case c.Send <- bytes:
		return nil
	default:
		return ErrChannelFull
	}
}

// Error types
type HubError string

func (e HubError) Error() string { return string(e) }

const (
	ErrChannelFull HubError = "send channel full"
)

// marshalMessage marshals a message to JSON bytes
func marshalMessage(msg *Message) ([]byte, error) {
	return json.Marshal(msg)
}
