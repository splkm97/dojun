package main

import (
	"testing"

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
