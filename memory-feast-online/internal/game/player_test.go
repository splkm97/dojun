package game

import (
	"testing"

	"github.com/gorilla/websocket"
)

func TestClearConnectionIfClearsMatchingConnection(t *testing.T) {
	conn := &websocket.Conn{}
	player := NewPlayer("p1", "Alice", "s1", conn)

	if ok := player.ClearConnectionIf(conn); !ok {
		t.Fatalf("expected ClearConnectionIf to clear matching connection")
	}
	if player.GetConnection() != nil {
		t.Fatalf("expected connection to be nil after clear")
	}
}

func TestClearConnectionIfKeepsDifferentConnection(t *testing.T) {
	current := &websocket.Conn{}
	expected := &websocket.Conn{}
	player := NewPlayer("p1", "Alice", "s1", current)

	if ok := player.ClearConnectionIf(expected); ok {
		t.Fatalf("expected ClearConnectionIf to fail when expected conn mismatches")
	}
	if player.GetConnection() != current {
		t.Fatalf("expected connection to remain unchanged")
	}
}
