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
