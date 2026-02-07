package game

import "testing"

func TestCoverPlateSetsCoveredForValidIndex(t *testing.T) {
	room := NewRoom(4)
	room.StartGame()

	room.mu.Lock()
	room.State.Plates[0].Covered = false
	room.mu.Unlock()

	if ok := room.CoverPlate(0); !ok {
		t.Fatalf("expected CoverPlate to succeed for valid index")
	}

	room.mu.RLock()
	covered := room.State.Plates[0].Covered
	room.mu.RUnlock()
	if !covered {
		t.Fatalf("expected plate to be covered after CoverPlate")
	}
}

func TestCoverPlateReturnsFalseForInvalidIndex(t *testing.T) {
	room := NewRoom(4)

	if ok := room.CoverPlate(-1); ok {
		t.Fatalf("expected CoverPlate(-1) to fail")
	}
	if ok := room.CoverPlate(len(room.State.Plates)); ok {
		t.Fatalf("expected CoverPlate(out of range) to fail")
	}
}

func TestHandleAddToken_BlocksDoubleAdd(t *testing.T) {
	room := NewRoom(4)
	room.StartGame()

	// Setup: Initialize players and create PhaseAddToken state with matched plates
	room.mu.Lock()
	room.Players[0] = &Player{Tokens: 5}
	room.Players[1] = &Player{Tokens: 5}
	room.State.Phase = PhaseAddToken
	room.State.CurrentTurn = 0
	room.State.MatchedPlates = []int{0, 1}
	room.State.Plates[0].Covered = true
	room.State.Plates[1].Covered = true
	room.mu.Unlock()

	// Act: First call (should succeed)
	ok1, _, _ := room.HandleAddToken(0, 0)
	if !ok1 {
		t.Fatalf("expected first HandleAddToken to succeed")
	}

	// Act: Second call (currently succeeds — BUG; should fail after fix)
	ok2, _, _ := room.HandleAddToken(0, 1)
	if ok2 {
		t.Fatalf("expected second HandleAddToken to be blocked (bug: currently succeeds)")
	}
}
