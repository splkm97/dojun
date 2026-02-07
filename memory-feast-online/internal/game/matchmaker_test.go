package game

import (
	"testing"
	"time"
)

func TestCleanupTimedOutRemovesEntryAndCallsCallback(t *testing.T) {
	timedOutSessions := make([]string, 0, 1)
	mm := NewMatchmaker(nil, func(entry *QueueEntry) {
		timedOutSessions = append(timedOutSessions, entry.Player.SessionID)
	})

	player := NewPlayer("player-1", "Tester", "session-timeout", nil)
	position, room := mm.JoinQueue(player, nil, 20)
	if position != 1 {
		t.Fatalf("expected queue position 1, got %d", position)
	}
	if room != nil {
		t.Fatalf("expected no room, got %v", room)
	}

	if len(mm.queue) != 1 {
		t.Fatalf("expected one queue entry, got %d", len(mm.queue))
	}

	mm.queue[0].JoinedAt = time.Now().Add(-QueueTimeout - time.Second)
	mm.cleanupTimedOut()

	if got := mm.QueueSize(); got != 0 {
		t.Fatalf("expected empty queue after cleanup, got %d", got)
	}
	if len(timedOutSessions) != 1 {
		t.Fatalf("expected 1 timeout callback, got %d", len(timedOutSessions))
	}
	if timedOutSessions[0] != "session-timeout" {
		t.Fatalf("expected timeout session 'session-timeout', got %q", timedOutSessions[0])
	}
}

func TestCleanupTimedOutKeepsRecentEntries(t *testing.T) {
	callbackCalled := false
	mm := NewMatchmaker(nil, func(entry *QueueEntry) {
		callbackCalled = true
	})

	player := NewPlayer("player-2", "Active", "session-active", nil)
	position, room := mm.JoinQueue(player, nil, 20)
	if position != 1 {
		t.Fatalf("expected queue position 1, got %d", position)
	}
	if room != nil {
		t.Fatalf("expected no room, got %v", room)
	}

	mm.cleanupTimedOut()

	if got := mm.QueueSize(); got != 1 {
		t.Fatalf("expected queue size 1, got %d", got)
	}
	if callbackCalled {
		t.Fatal("expected timeout callback not to be called")
	}
}

func TestCleanupTimedOutWithNilCallback(t *testing.T) {
	mm := NewMatchmaker(nil, nil)

	player := NewPlayer("player-3", "NoCallback", "session-no-callback", nil)
	position, room := mm.JoinQueue(player, nil, 20)
	if position != 1 {
		t.Fatalf("expected queue position 1, got %d", position)
	}
	if room != nil {
		t.Fatalf("expected no room, got %v", room)
	}

	mm.queue[0].JoinedAt = time.Now().Add(-QueueTimeout - time.Second)
	mm.cleanupTimedOut()

	if got := mm.QueueSize(); got != 0 {
		t.Fatalf("expected empty queue after cleanup, got %d", got)
	}
}
