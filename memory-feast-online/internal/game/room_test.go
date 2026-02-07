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

func TestClampPlateCount(t *testing.T) {
	tests := []struct {
		name  string
		input int
		want  int
	}{
		{name: "below minimum defaults to 4", input: 3, want: 4},
		{name: "zero defaults to 4", input: 0, want: 4},
		{name: "negative defaults to 4", input: -1, want: 4},
		{name: "minimum remains 4", input: 4, want: 4},
		{name: "odd rounds up", input: 5, want: 6},
		{name: "odd near max rounds to max", input: 19, want: 20},
		{name: "max remains 20", input: 20, want: 20},
		{name: "above max clamps to 20", input: 21, want: 20},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ClampPlateCount(tt.input)
			if got != tt.want {
				t.Fatalf("ClampPlateCount(%d) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestGetWinnerHandlesMissingPlayers(t *testing.T) {
	t.Run("both missing draw", func(t *testing.T) {
		room := NewRoom(4)
		if got := room.GetWinner(); got != -1 {
			t.Fatalf("expected draw (-1), got %d", got)
		}
	})

	t.Run("player0 missing player1 wins", func(t *testing.T) {
		room := NewRoom(4)
		room.mu.Lock()
		room.Players[1] = &Player{Tokens: 3}
		room.mu.Unlock()

		if got := room.GetWinner(); got != 1 {
			t.Fatalf("expected player 1 to win, got %d", got)
		}
	})

	t.Run("player1 missing player0 wins", func(t *testing.T) {
		room := NewRoom(4)
		room.mu.Lock()
		room.Players[0] = &Player{Tokens: 2}
		room.mu.Unlock()

		if got := room.GetWinner(); got != 0 {
			t.Fatalf("expected player 0 to win, got %d", got)
		}
	})
}

func TestHandleConfirmMatchBlocksReentry(t *testing.T) {
	room := NewRoom(4)

	room.mu.Lock()
	room.State.Phase = PhaseMatching
	room.State.CurrentTurn = 0
	room.State.SelectedPlates = []int{0, 1}
	room.State.Plates[0].Tokens = 1
	room.State.Plates[1].Tokens = 1
	room.mu.Unlock()

	ok, _, _, _ := room.HandleConfirmMatch(0)
	if !ok {
		t.Fatalf("expected first HandleConfirmMatch to succeed")
	}

	ok, _, _, _ = room.HandleConfirmMatch(0)
	if ok {
		t.Fatalf("expected second HandleConfirmMatch to be blocked while confirmPending")
	}
}
