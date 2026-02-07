package game

import (
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	QueueTimeout = 60 * time.Second
)

// QueueEntry represents a player waiting for a match
type QueueEntry struct {
	Player     *Player
	Conn       *websocket.Conn
	JoinedAt   time.Time
	PlateCount int
}

// Matchmaker handles random matchmaking
type Matchmaker struct {
	queue     []*QueueEntry
	mu        sync.Mutex
	onMatched func(entry1, entry2 *QueueEntry) *Room
}

// NewMatchmaker creates a new matchmaker instance
func NewMatchmaker(onMatched func(entry1, entry2 *QueueEntry) *Room) *Matchmaker {
	mm := &Matchmaker{
		queue:     make([]*QueueEntry, 0),
		onMatched: onMatched,
	}

	// Start cleanup goroutine
	go mm.cleanupLoop()

	return mm
}

// JoinQueue adds a player to the matchmaking queue
// Returns: position in queue, matched room (if immediately matched), or nil
func (mm *Matchmaker) JoinQueue(player *Player, conn *websocket.Conn, plateCount int) (int, *Room) {
	mm.mu.Lock()
	defer mm.mu.Unlock()

	// Check if player is already in queue
	for i, entry := range mm.queue {
		if entry.Player.SessionID == player.SessionID {
			// Update connection
			entry.Conn = conn
			entry.Player = player
			return i + 1, nil
		}
	}

	entry := &QueueEntry{
		Player:     player,
		Conn:       conn,
		JoinedAt:   time.Now(),
		PlateCount: ClampPlateCount(plateCount),
	}

	// Check for a match (FIFO - take first available)
	if len(mm.queue) > 0 {
		// Match with first player in queue
		opponent := mm.queue[0]
		mm.queue = mm.queue[1:]

		// Create room via callback
		if mm.onMatched != nil {
			room := mm.onMatched(opponent, entry)
			return 0, room
		}
	}

	// No match found, add to queue
	mm.queue = append(mm.queue, entry)
	return len(mm.queue), nil
}

// LeaveQueue removes a player from the queue
func (mm *Matchmaker) LeaveQueue(sessionID string) bool {
	mm.mu.Lock()
	defer mm.mu.Unlock()

	for i, entry := range mm.queue {
		if entry.Player.SessionID == sessionID {
			mm.queue = append(mm.queue[:i], mm.queue[i+1:]...)
			return true
		}
	}
	return false
}

// GetQueuePosition returns the player's position in queue (1-indexed)
// Returns 0 if not in queue
func (mm *Matchmaker) GetQueuePosition(sessionID string) int {
	mm.mu.Lock()
	defer mm.mu.Unlock()

	for i, entry := range mm.queue {
		if entry.Player.SessionID == sessionID {
			return i + 1
		}
	}
	return 0
}

// QueueSize returns the current queue size
func (mm *Matchmaker) QueueSize() int {
	mm.mu.Lock()
	defer mm.mu.Unlock()
	return len(mm.queue)
}

// cleanupLoop removes timed-out entries from the queue
func (mm *Matchmaker) cleanupLoop() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		mm.cleanupTimedOut()
	}
}

func (mm *Matchmaker) cleanupTimedOut() {
	mm.mu.Lock()
	defer mm.mu.Unlock()

	now := time.Now()
	newQueue := make([]*QueueEntry, 0, len(mm.queue))

	for _, entry := range mm.queue {
		if now.Sub(entry.JoinedAt) < QueueTimeout {
			newQueue = append(newQueue, entry)
		}
		// Timed out entries are simply not added to newQueue
	}

	mm.queue = newQueue
}

// GetEntryBySessionID finds a queue entry by session ID
func (mm *Matchmaker) GetEntryBySessionID(sessionID string) *QueueEntry {
	mm.mu.Lock()
	defer mm.mu.Unlock()

	for _, entry := range mm.queue {
		if entry.Player.SessionID == sessionID {
			return entry
		}
	}
	return nil
}
