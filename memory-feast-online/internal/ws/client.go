package ws

import (
	"encoding/json"
	"log"
	"time"

	"github.com/gorilla/websocket"
)

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer.
	maxMessageSize = 4096
)

// MessageHandler is a function that handles incoming messages
type MessageHandler func(client *Client, msg *Message)

// ReadPump pumps messages from the websocket connection to the handler
func (c *Client) ReadPump(handler MessageHandler) {
	defer func() {
		if callback := c.OnDisconnect(); callback != nil {
			callback(c)
		}
		c.Hub.Unregister(c)
		c.Close()
	}()

	c.Conn.SetReadLimit(maxMessageSize)
	c.Conn.SetReadDeadline(time.Now().Add(pongWait))
	c.Conn.SetPongHandler(func(string) error {
		c.Conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, message, err := c.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error for client %s: %v", c.SessionID, err)
			}
			break
		}

		var msg Message
		if err := json.Unmarshal(message, &msg); err != nil {
			log.Printf("Error unmarshaling message from client %s: %v", c.SessionID, err)
			continue
		}

		handler(c, &msg)
	}
}

// Note: Ping/pong is now handled in WritePump to avoid concurrent writes
