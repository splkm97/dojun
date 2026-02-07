package main

import (
	"net/http/httptest"
	"testing"

	"github.com/gorilla/websocket"

	"memory-feast-online/internal/game"
	"memory-feast-online/internal/store"
	"memory-feast-online/internal/ws"
)

func TestHandleLeaveRoomRemovesWaitingPlayerFromQueue(t *testing.T) {
	s := NewServer(store.NewMemoryStore())
	client := ws.NewClient(s.hub, nil, "session-waiting")
	client.SetState(ws.ClientWaiting)

	player := game.NewPlayer("player-1", "Tester", client.SessionID, nil)
	position, room := s.matchmaker.JoinQueue(player, nil, 20)
	if position != 1 {
		t.Fatalf("expected queue position 1, got %d", position)
	}
	if room != nil {
		t.Fatalf("expected no room match, got room %v", room.ID)
	}
	if got := s.matchmaker.GetQueuePosition(client.SessionID); got != 1 {
		t.Fatalf("expected player to be queued before leave, got position %d", got)
	}

	msg, err := ws.NewMessage(ws.MsgLeaveRoom, struct{}{})
	if err != nil {
		t.Fatalf("failed to create leave_room message: %v", err)
	}

	s.handleLeaveRoom(client, msg)

	if got := s.matchmaker.GetQueuePosition(client.SessionID); got != 0 {
		t.Fatalf("expected queue removal on leave_room, got position %d", got)
	}
	if got := client.GetState(); got != ws.ClientLobby {
		t.Fatalf("expected client state lobby, got %s", got)
	}
}

func TestEndGameRemovesRoom(t *testing.T) {
	s := NewServer(store.NewMemoryStore())
	room := game.NewRoom(4)
	room.Hub = s.hub
	room.Players[0] = game.NewPlayer("p1", "Alice", "s1", nil)
	room.Players[1] = game.NewPlayer("p2", "Bob", "s2", nil)

	s.roomsMu.Lock()
	s.rooms[room.ID] = room
	s.roomsMu.Unlock()

	s.endGame(room, 0, "tokens")

	if got := s.getRoom(room.ID); got != nil {
		t.Fatalf("expected room to be removed after endGame")
	}
}

func TestEndGameNoMatchesRemovesRoom(t *testing.T) {
	s := NewServer(store.NewMemoryStore())
	room := game.NewRoom(4)
	room.Hub = s.hub
	room.Players[0] = game.NewPlayer("p1", "Alice", "s1", nil)
	room.Players[1] = game.NewPlayer("p2", "Bob", "s2", nil)

	s.roomsMu.Lock()
	s.rooms[room.ID] = room
	s.roomsMu.Unlock()

	s.endGameNoMatches(room)

	if got := s.getRoom(room.ID); got != nil {
		t.Fatalf("expected room to be removed after endGameNoMatches")
	}
}

func TestHandleClientDisconnectRemovesQueuedPlayer(t *testing.T) {
	s := NewServer(store.NewMemoryStore())
	client := ws.NewClient(s.hub, nil, "session-queued")

	player := game.NewPlayer("player-q", "Queued", client.SessionID, nil)
	position, room := s.matchmaker.JoinQueue(player, nil, 20)
	if position != 1 {
		t.Fatalf("expected queue position 1, got %d", position)
	}
	if room != nil {
		t.Fatalf("expected no room match, got room %v", room.ID)
	}

	s.handleClientDisconnect(client)

	if got := s.matchmaker.GetQueuePosition(client.SessionID); got != 0 {
		t.Fatalf("expected queued player to be removed on disconnect, got position %d", got)
	}
}

func TestHandleClientDisconnectClearsConnectionAndRemovesWaitingRoom(t *testing.T) {
	s := NewServer(store.NewMemoryStore())
	room := game.NewRoom(4)
	room.Hub = s.hub

	conn := &websocket.Conn{}
	player := game.NewPlayer("player-w", "Waiting", "session-waiting-room", conn)
	if _, err := room.AddPlayer(player); err != nil {
		t.Fatalf("failed to add player to room: %v", err)
	}

	s.roomsMu.Lock()
	s.rooms[room.ID] = room
	s.roomsMu.Unlock()

	client := ws.NewClient(s.hub, conn, player.SessionID)
	s.handleClientDisconnect(client)

	if player.IsConnected() {
		t.Fatalf("expected disconnected player connection to be cleared")
	}
	if got := s.getRoom(room.ID); got != nil {
		t.Fatalf("expected waiting room to be removed when lone player disconnects")
	}
}

func TestHandleClientDisconnectIgnoresStaleConnectionAfterRebind(t *testing.T) {
	s := NewServer(store.NewMemoryStore())
	room := game.NewRoom(4)
	room.Hub = s.hub

	oldConn := &websocket.Conn{}
	newConn := &websocket.Conn{}

	player := game.NewPlayer("player-r", "Rebound", "session-rebound", oldConn)
	if _, err := room.AddPlayer(player); err != nil {
		t.Fatalf("failed to add player to room: %v", err)
	}

	player.SetConnection(newConn)

	s.roomsMu.Lock()
	s.rooms[room.ID] = room
	s.roomsMu.Unlock()

	staleClient := ws.NewClient(s.hub, oldConn, player.SessionID)
	s.handleClientDisconnect(staleClient)

	if got := player.GetConnection(); got != newConn {
		t.Fatalf("expected stale disconnect to keep rebound connection")
	}
	if got := s.getRoom(room.ID); got == nil {
		t.Fatalf("expected room to remain active after stale disconnect")
	}
}

func TestIsOriginAllowed(t *testing.T) {
	tests := []struct {
		name           string
		origin         string
		allowedOrigins string
		want           bool
	}{
		{
			name:           "exact origin allowed",
			origin:         "https://app.example.com",
			allowedOrigins: "https://app.example.com,https://admin.example.com",
			want:           true,
		},
		{
			name:           "wildcard not supported",
			origin:         "https://app.example.com",
			allowedOrigins: "*",
			want:           false,
		},
		{
			name:           "case insensitive scheme and host",
			origin:         "HTTPS://APP.EXAMPLE.COM",
			allowedOrigins: "https://app.example.com",
			want:           true,
		},
		{
			name:           "empty origin rejected",
			origin:         "",
			allowedOrigins: "https://app.example.com",
			want:           false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isOriginAllowed(tt.origin, tt.allowedOrigins); got != tt.want {
				t.Fatalf("isOriginAllowed(%q, %q) = %v, want %v", tt.origin, tt.allowedOrigins, got, tt.want)
			}
		})
	}
}

func TestIsAllowedWebSocketOriginDefaultLocalhostOnly(t *testing.T) {
	t.Setenv("ALLOWED_WS_ORIGINS", "")

	req := httptest.NewRequest("GET", "/ws", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	if !isAllowedWebSocketOrigin(req) {
		t.Fatalf("expected localhost origin to be allowed by default")
	}

	req = httptest.NewRequest("GET", "/ws", nil)
	req.Header.Set("Origin", "https://evil.example")
	if isAllowedWebSocketOrigin(req) {
		t.Fatalf("expected non-local origin to be rejected by default")
	}
}

func TestIsAllowedWebSocketOriginUsesConfiguredList(t *testing.T) {
	t.Setenv("ALLOWED_WS_ORIGINS", "https://app.example.com")

	req := httptest.NewRequest("GET", "/ws", nil)
	req.Header.Set("Origin", "https://app.example.com")
	if !isAllowedWebSocketOrigin(req) {
		t.Fatalf("expected configured origin to be allowed")
	}

	req = httptest.NewRequest("GET", "/ws", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	if isAllowedWebSocketOrigin(req) {
		t.Fatalf("expected localhost origin to be rejected when explicit allowlist is set")
	}
}

func TestIsAllowedWebSocketOriginWildcardRejected(t *testing.T) {
	t.Setenv("ALLOWED_WS_ORIGINS", "*")

	req := httptest.NewRequest("GET", "/ws", nil)
	req.Header.Set("Origin", "https://app.example.com")
	if isAllowedWebSocketOrigin(req) {
		t.Fatalf("expected wildcard allowlist to be rejected")
	}
}
