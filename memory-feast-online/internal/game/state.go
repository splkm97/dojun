package game

// Phase represents the current game phase
type Phase string

const (
	PhaseWaiting   Phase = "waiting"
	PhasePlacement Phase = "placement"
	PhaseMatching  Phase = "matching"
	PhaseAddToken  Phase = "add_token"
	PhaseFinished  Phase = "finished"
)

// Plate represents a plate in the game
type Plate struct {
	Tokens    int  `json:"tokens"`
	Covered   bool `json:"covered"`
	HasTokens bool `json:"hasTokens"`
}

// GameState holds all game-related state
type GameState struct {
	Phase           Phase
	CurrentTurn     int // 0 or 1 (player index)
	PlacementRound  int
	MaxRound        int
	TimeLeft        int
	Plates          []Plate
	SelectedPlates  []int
	MatchedPlates   []int
	LastActionPlate *int // Plate index of last placement/addition for animation
}

// NewGameState creates a new game state with the given plate count
func NewGameState(plateCount int) *GameState {
	plates := make([]Plate, plateCount)
	for i := range plates {
		plates[i] = Plate{
			Tokens:    0,
			Covered:   true,
			HasTokens: false,
		}
	}

	maxRound := plateCount/2 - 1
	if maxRound < 1 {
		maxRound = 1
	}

	return &GameState{
		Phase:          PhaseWaiting,
		CurrentTurn:    0,
		PlacementRound: 1,
		MaxRound:       maxRound,
		TimeLeft:       60,
		Plates:         plates,
		SelectedPlates: []int{},
		MatchedPlates:  []int{},
	}
}

// PlaceToken places tokens on a plate during placement phase
func (gs *GameState) PlaceToken(index int) bool {
	if gs.Phase != PhasePlacement {
		return false
	}
	if index < 0 || index >= len(gs.Plates) {
		return false
	}
	if gs.Plates[index].HasTokens {
		return false
	}

	gs.Plates[index].Tokens = gs.PlacementRound
	gs.Plates[index].HasTokens = true
	gs.Plates[index].Covered = true
	gs.LastActionPlate = &index

	return true
}

// NextPlacementTurn advances to the next placement turn
func (gs *GameState) NextPlacementTurn() bool {
	// Switch player
	gs.CurrentTurn = 1 - gs.CurrentTurn

	// If back to player 0, increment round
	if gs.CurrentTurn == 0 {
		gs.PlacementRound++
	}

	// Check if placement phase is complete
	if gs.PlacementRound > gs.MaxRound {
		return true // Placement complete
	}

	return false
}

// StartMatchingPhase initializes the matching phase
func (gs *GameState) StartMatchingPhase(initialTokens int) {
	gs.Phase = PhaseMatching
	gs.CurrentTurn = 0
	gs.TimeLeft = 60
	gs.SelectedPlates = []int{}
	gs.MatchedPlates = []int{}
}

// SelectPlate toggles plate selection during matching phase
func (gs *GameState) SelectPlate(index int) bool {
	if gs.Phase != PhaseMatching {
		return false
	}
	if index < 0 || index >= len(gs.Plates) {
		return false
	}

	// Check if already selected
	for i, idx := range gs.SelectedPlates {
		if idx == index {
			// Remove from selection
			gs.SelectedPlates = append(gs.SelectedPlates[:i], gs.SelectedPlates[i+1:]...)
			return true
		}
	}

	// Can only select 2 plates
	if len(gs.SelectedPlates) >= 2 {
		return false
	}

	gs.SelectedPlates = append(gs.SelectedPlates, index)
	return true
}

// ConfirmMatch checks if selected plates match
// Returns: matched, plate1Tokens, plate2Tokens
func (gs *GameState) ConfirmMatch() (bool, int, int) {
	if len(gs.SelectedPlates) != 2 {
		return false, 0, 0
	}

	idx1, idx2 := gs.SelectedPlates[0], gs.SelectedPlates[1]
	plate1, plate2 := gs.Plates[idx1], gs.Plates[idx2]

	// Uncover plates
	gs.Plates[idx1].Covered = false
	gs.Plates[idx2].Covered = false

	return plate1.Tokens == plate2.Tokens, plate1.Tokens, plate2.Tokens
}

// SetAddTokenPhase transitions to add token phase after successful match
func (gs *GameState) SetAddTokenPhase() {
	gs.Phase = PhaseAddToken
	gs.MatchedPlates = make([]int, len(gs.SelectedPlates))
	copy(gs.MatchedPlates, gs.SelectedPlates)
}

// AddToken adds a token to a matched plate
func (gs *GameState) AddToken(index int) bool {
	if gs.Phase != PhaseAddToken {
		return false
	}

	// Check if index is in matched plates
	found := false
	for _, idx := range gs.MatchedPlates {
		if idx == index {
			found = true
			break
		}
	}
	if !found {
		return false
	}

	gs.Plates[index].Tokens++
	gs.LastActionPlate = &index
	return true
}

// ResetForNextTurn resets state for the next matching turn
func (gs *GameState) ResetForNextTurn() {
	// Cover all plates
	for i := range gs.Plates {
		gs.Plates[i].Covered = true
	}
	gs.SelectedPlates = []int{}
	gs.MatchedPlates = []int{}
	gs.LastActionPlate = nil
	gs.Phase = PhaseMatching
	gs.TimeLeft = 60
}

// NextMatchingTurn advances to the next matching turn
func (gs *GameState) NextMatchingTurn() {
	gs.CurrentTurn = 1 - gs.CurrentTurn
	gs.ResetForNextTurn()
}

// HasMatchingPairs checks if there are any matching pairs left
func (gs *GameState) HasMatchingPairs() bool {
	tokenCounts := make(map[int]int)
	for _, plate := range gs.Plates {
		tokenCounts[plate.Tokens]++
	}

	for _, count := range tokenCounts {
		if count >= 2 {
			return true
		}
	}
	return false
}

// SetFinished marks the game as finished
func (gs *GameState) SetFinished() {
	gs.Phase = PhaseFinished
}
