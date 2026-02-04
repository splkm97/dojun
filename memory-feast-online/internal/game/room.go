package game

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"memory-feast-online/internal/ws"
)

const (
	ReconnectGracePeriod = 30 * time.Second
	DefaultPlateCount    = 20
	MatchingTimeLimit    = 60
)

// Room represents a game room
type Room struct {
	ID         string
	Code       string // 6-char invite code
	Players    [2]*Player
	State      *GameState
	PlateCount int
	Hub        *ws.Hub // Hub for sending messages

	mu          sync.RWMutex
	timer       *time.Timer
	timerTicker *time.Ticker
	timerDone   chan struct{}

	// Callbacks
	onEmpty func(roomID string)
}

// NewRoom creates a new room
func NewRoom(plateCount int) *Room {
	if plateCount < 4 {
		plateCount = 4
	}
	if plateCount > 20 {
		plateCount = 20
	}
	// Ensure even number
	if plateCount%2 != 0 {
		plateCount++
	}

	return &Room{
		ID:         GenerateID(),
		Code:       generateRoomCode(),
		PlateCount: plateCount,
		State:      NewGameState(plateCount),
	}
}

// SetOnEmpty sets the callback for when room becomes empty
func (r *Room) SetOnEmpty(callback func(roomID string)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.onEmpty = callback
}

// AddPlayer adds a player to the room
func (r *Room) AddPlayer(player *Player) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for i := 0; i < 2; i++ {
		if r.Players[i] == nil {
			r.Players[i] = player
			return i, nil
		}
	}

	return -1, ErrRoomFull
}

// RemovePlayer removes a player from the room
func (r *Room) RemovePlayer(playerIndex int) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if playerIndex < 0 || playerIndex > 1 {
		return
	}

	r.Players[playerIndex] = nil

	// Check if room is empty
	if r.Players[0] == nil && r.Players[1] == nil {
		if r.onEmpty != nil {
			go r.onEmpty(r.ID)
		}
	}
}

// GetPlayer returns a player by index
func (r *Room) GetPlayer(index int) *Player {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if index < 0 || index > 1 {
		return nil
	}
	return r.Players[index]
}

// GetPlayerBySessionID finds a player by session ID
func (r *Room) GetPlayerBySessionID(sessionID string) (*Player, int) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for i, p := range r.Players {
		if p != nil && p.SessionID == sessionID {
			return p, i
		}
	}
	return nil, -1
}

// IsFull checks if room has two players
func (r *Room) IsFull() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.Players[0] != nil && r.Players[1] != nil
}

// GetOpponentNickname returns the opponent's nickname
func (r *Room) GetOpponentNickname(playerIndex int) string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	opponentIndex := 1 - playerIndex
	if r.Players[opponentIndex] != nil {
		return r.Players[opponentIndex].Nickname
	}
	return ""
}

// StartGame begins the game
func (r *Room) StartGame() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.State.Phase = PhasePlacement
	r.State.CurrentTurn = 0
	r.State.PlacementRound = 1
}

// StartMatchingPhase transitions to matching phase
func (r *Room) StartMatchingPhase() {
	r.mu.Lock()
	defer r.mu.Unlock()

	initialTokens := max(5, r.State.MaxRound+1)
	r.Players[0].Tokens = initialTokens
	r.Players[1].Tokens = initialTokens
	r.State.StartMatchingPhase(initialTokens)
}

// HandlePlaceToken handles a token placement
func (r *Room) HandlePlaceToken(playerIndex, plateIndex int) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.State.Phase != PhasePlacement {
		return false
	}
	if r.State.CurrentTurn != playerIndex {
		return false
	}

	return r.State.PlaceToken(plateIndex)
}

// AdvancePlacement moves to next placement turn
// Returns true if placement phase is complete
func (r *Room) AdvancePlacement() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.State.NextPlacementTurn()
}

// HandleSelectPlate handles plate selection during matching
func (r *Room) HandleSelectPlate(playerIndex, plateIndex int) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.State.Phase != PhaseMatching {
		return false
	}
	if r.State.CurrentTurn != playerIndex {
		return false
	}

	return r.State.SelectPlate(plateIndex)
}

// HandleConfirmMatch handles match confirmation
// Returns: success, matched, plate1Tokens, plate2Tokens
func (r *Room) HandleConfirmMatch(playerIndex int) (bool, bool, int, int) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.State.Phase != PhaseMatching {
		return false, false, 0, 0
	}
	if r.State.CurrentTurn != playerIndex {
		return false, false, 0, 0
	}
	if len(r.State.SelectedPlates) != 2 {
		return false, false, 0, 0
	}

	matched, t1, t2 := r.State.ConfirmMatch()
	return true, matched, t1, t2
}

// SetAddTokenPhase transitions to add token phase
func (r *Room) SetAddTokenPhase() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.State.SetAddTokenPhase()
}

// HandleAddToken handles adding a token to matched plate
// Returns: success, newTokenCount, playerWon
func (r *Room) HandleAddToken(playerIndex, plateIndex int) (bool, int, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.State.Phase != PhaseAddToken {
		return false, 0, false
	}
	if r.State.CurrentTurn != playerIndex {
		return false, 0, false
	}

	if !r.State.AddToken(plateIndex) {
		return false, 0, false
	}

	// Decrease player tokens
	r.Players[playerIndex].Tokens--
	playerWon := r.Players[playerIndex].Tokens <= 0

	return true, r.State.Plates[plateIndex].Tokens, playerWon
}

// HandleMatchFail handles a failed match
func (r *Room) HandleMatchFail(playerIndex int) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Add penalty token
	r.Players[playerIndex].Tokens++
}

// HandleTimeout handles turn timeout
func (r *Room) HandleTimeout(playerIndex int) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Add penalty tokens (2 for timeout)
	r.Players[playerIndex].Tokens += 2
}

// AdvanceMatching moves to next matching turn
// Returns true if game should continue, false if no more matches
func (r *Room) AdvanceMatching() bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.State.HasMatchingPairs() {
		return false
	}

	r.State.NextMatchingTurn()
	return true
}

// GetWinner determines the winner when no matches remain
// Returns: winner index (0, 1) or -1 for draw
func (r *Room) GetWinner() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	t0 := r.Players[0].Tokens
	t1 := r.Players[1].Tokens

	if t0 < t1 {
		return 0
	} else if t1 < t0 {
		return 1
	}
	return -1 // Draw
}

// SetFinished marks the game as finished
func (r *Room) SetFinished() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.State.SetFinished()
}

// StartTimer starts the matching phase timer
func (r *Room) StartTimer(onTick func(timeLeft int), onTimeout func()) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.stopTimerLocked()

	r.State.TimeLeft = MatchingTimeLimit
	r.timerDone = make(chan struct{})

	r.timerTicker = time.NewTicker(1 * time.Second)
	go func() {
		for {
			select {
			case <-r.timerDone:
				return
			case <-r.timerTicker.C:
				r.mu.Lock()
				r.State.TimeLeft--
				timeLeft := r.State.TimeLeft
				r.mu.Unlock()

				if onTick != nil {
					onTick(timeLeft)
				}

				if timeLeft <= 0 {
					r.StopTimer()
					if onTimeout != nil {
						onTimeout()
					}
					return
				}
			}
		}
	}()
}

// StopTimer stops the current timer
func (r *Room) StopTimer() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.stopTimerLocked()
}

func (r *Room) stopTimerLocked() {
	if r.timerTicker != nil {
		r.timerTicker.Stop()
		r.timerTicker = nil
	}
	if r.timerDone != nil {
		close(r.timerDone)
		r.timerDone = nil
	}
}

// GetGameState returns a snapshot of the game state (generic, no player-specific data)
func (r *Room) GetGameState() ws.GameStatePayload {
	return r.GetGameStateForPlayer(-1) // -1 means generic state
}

// GetGameStateForPlayer returns game state with player-specific selection visibility
// playerIndex: 0 or 1 for specific player, -1 for generic state
func (r *Room) GetGameStateForPlayer(playerIndex int) ws.GameStatePayload {
	r.mu.RLock()
	defer r.mu.RUnlock()

	players := make([]ws.PlayerInfo, 2)
	for i, p := range r.Players {
		if p != nil {
			players[i] = ws.PlayerInfo{
				Nickname:    p.Nickname,
				Tokens:      p.Tokens,
				IsConnected: p.IsConnected(),
			}
		}
	}

	plates := make([]ws.PlateInfo, len(r.State.Plates))
	for i, p := range r.State.Plates {
		plates[i] = ws.PlateInfo{
			Tokens:    p.Tokens,
			Covered:   p.Covered,
			HasTokens: p.HasTokens,
		}
	}

	state := ws.GameStatePayload{
		Phase:          string(r.State.Phase),
		CurrentTurn:    r.State.CurrentTurn,
		PlacementRound: r.State.PlacementRound,
		MaxRound:       r.State.MaxRound,
		TimeLeft:       r.State.TimeLeft,
		Players:        players,
		Plates:         plates,
		SelectedPlates: []int{},
		MatchedPlates:  r.State.MatchedPlates,
	}

	// During matching phase, show selections appropriately
	if r.State.Phase == PhaseMatching && playerIndex >= 0 {
		if playerIndex == r.State.CurrentTurn {
			// Current turn player sees their own selections as "selected"
			state.SelectedPlates = r.State.SelectedPlates
		} else {
			// Opponent sees current turn player's selections as "opponent selected"
			state.OpponentSelectedPlates = r.State.SelectedPlates
		}
	} else {
		// Default: include selected plates as-is (used during non-matching phases
		// like placement/add_token where selection visibility doesn't matter)
		state.SelectedPlates = r.State.SelectedPlates
	}

	return state
}

// BroadcastState sends game state to all connected players
// Each player receives state with appropriate selection visibility
func (r *Room) BroadcastState() {
	// Collect session IDs first while holding lock
	r.mu.RLock()
	sessionIDs := make([]string, 2)
	playerIDs := make([]string, 2)
	for i, p := range r.Players {
		if p != nil {
			sessionIDs[i] = p.SessionID
			playerIDs[i] = p.ID
		}
	}
	hub := r.Hub
	r.mu.RUnlock()

	if hub == nil {
		return
	}

	// Send to each player with their specific state
	for i := 0; i < 2; i++ {
		if sessionIDs[i] == "" {
			continue
		}

		client := hub.GetClient(sessionIDs[i])
		if client == nil {
			continue
		}

		state := r.GetGameStateForPlayer(i)
		msg, err := ws.NewMessage(ws.MsgGameState, state)
		if err != nil {
			log.Printf("Error creating game state message: %v", err)
			continue
		}

		msgBytes, err := json.Marshal(msg)
		if err != nil {
			log.Printf("Error marshaling game state: %v", err)
			continue
		}

		if err := client.WriteMessageDirect(websocket.TextMessage, msgBytes); err != nil {
			log.Printf("Error sending game state to player %s: %v", playerIDs[i], err)
		}
	}
}

// BroadcastMessage sends a message to all connected players
func (r *Room) BroadcastMessage(msg *ws.Message) {
	msgBytes, err := json.Marshal(msg)
	if err != nil {
		log.Printf("Error marshaling message: %v", err)
		return
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, p := range r.Players {
		if p != nil && r.Hub != nil {
			client := r.Hub.GetClient(p.SessionID)
			if client != nil {
				if err := client.WriteMessageDirect(websocket.TextMessage, msgBytes); err != nil {
					log.Printf("Error sending message to player %s: %v", p.ID, err)
				}
			}
		}
	}
}

// SendToPlayer sends a message to a specific player
func (r *Room) SendToPlayer(playerIndex int, msg *ws.Message) error {
	r.mu.RLock()
	player := r.Players[playerIndex]
	r.mu.RUnlock()

	if player == nil || r.Hub == nil {
		return nil
	}

	msgBytes, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	client := r.Hub.GetClient(player.SessionID)
	if client != nil {
		return client.WriteMessageDirect(websocket.TextMessage, msgBytes)
	}
	return nil
}

// Error types
type RoomError string

func (e RoomError) Error() string { return string(e) }

const (
	ErrRoomFull     RoomError = "room is full"
	ErrRoomNotFound RoomError = "room not found"
	ErrNotYourTurn  RoomError = "not your turn"
	ErrInvalidPhase RoomError = "invalid phase for this action"
)

// Helper functions

// GenerateID creates a random 32-char hex ID
func GenerateID() string {
	bytes := make([]byte, 16)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

func generateRoomCode() string {
	const charset = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789" // Excluding confusing chars
	code := make([]byte, 6)
	rand.Read(code)
	for i := range code {
		code[i] = charset[int(code[i])%len(charset)]
	}
	return string(code)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
