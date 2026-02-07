package store

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	roomKeyPrefix    = "room:"
	sessionKeyPrefix = "session:"
	roomTTL          = 24 * time.Hour
	sessionTTL       = 1 * time.Hour
)

// Store defines the interface for game state persistence
type Store interface {
	SaveRoom(ctx context.Context, room *RoomData) error
	GetRoom(ctx context.Context, roomID string) (*RoomData, error)
	DeleteRoom(ctx context.Context, roomID string) error
	GetRoomByCode(ctx context.Context, code string) (*RoomData, error)
	SaveSession(ctx context.Context, sessionID, roomID string, playerIndex int) error
	GetSession(ctx context.Context, sessionID string) (roomID string, playerIndex int, err error)
	DeleteSession(ctx context.Context, sessionID string) error
}

// RoomData is the serializable room state for Redis
type RoomData struct {
	ID         string       `json:"id"`
	Code       string       `json:"code"`
	PlateCount int          `json:"plateCount"`
	Players    []PlayerData `json:"players"`
	State      StateData    `json:"state"`
	CreatedAt  time.Time    `json:"createdAt"`
}

// PlayerData is the serializable player state
type PlayerData struct {
	ID        string `json:"id"`
	Nickname  string `json:"nickname"`
	SessionID string `json:"sessionId"`
	Tokens    int    `json:"tokens"`
}

// StateData is the serializable game state
type StateData struct {
	Phase          string      `json:"phase"`
	CurrentTurn    int         `json:"currentTurn"`
	PlacementRound int         `json:"placementRound"`
	MaxRound       int         `json:"maxRound"`
	TimeLeft       int         `json:"timeLeft"`
	Plates         []PlateData `json:"plates"`
	SelectedPlates []int       `json:"selectedPlates"`
	MatchedPlates  []int       `json:"matchedPlates"`
}

// PlateData is the serializable plate state
type PlateData struct {
	Tokens    int  `json:"tokens"`
	Covered   bool `json:"covered"`
	HasTokens bool `json:"hasTokens"`
}

// SessionData is the session-to-room mapping
type SessionData struct {
	RoomID      string `json:"roomId"`
	PlayerIndex int    `json:"playerIndex"`
}

// RedisStore implements Store using Redis
type RedisStore struct {
	client *redis.Client
}

// NewRedisStore creates a new Redis store
func NewRedisStore(addr, password string, db int) (*RedisStore, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return &RedisStore{client: client}, nil
}

// Close closes the Redis connection
func (s *RedisStore) Close() error {
	return s.client.Close()
}

// SaveRoom saves room data to Redis
func (s *RedisStore) SaveRoom(ctx context.Context, room *RoomData) error {
	data, err := json.Marshal(room)
	if err != nil {
		return fmt.Errorf("failed to marshal room: %w", err)
	}

	// Save by ID
	key := roomKeyPrefix + room.ID
	if err := s.client.Set(ctx, key, data, roomTTL).Err(); err != nil {
		return fmt.Errorf("failed to save room: %w", err)
	}

	// Also save code -> ID mapping
	codeKey := "code:" + room.Code
	if err := s.client.Set(ctx, codeKey, room.ID, roomTTL).Err(); err != nil {
		return fmt.Errorf("failed to save room code mapping: %w", err)
	}

	return nil
}

// GetRoom retrieves room data from Redis
func (s *RedisStore) GetRoom(ctx context.Context, roomID string) (*RoomData, error) {
	key := roomKeyPrefix + roomID
	data, err := s.client.Get(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get room: %w", err)
	}

	var room RoomData
	if err := json.Unmarshal(data, &room); err != nil {
		return nil, fmt.Errorf("failed to unmarshal room: %w", err)
	}

	return &room, nil
}

// DeleteRoom removes room data from Redis
func (s *RedisStore) DeleteRoom(ctx context.Context, roomID string) error {
	// Get room first to delete code mapping
	room, err := s.GetRoom(ctx, roomID)
	if err != nil {
		return err
	}
	if room == nil {
		return nil
	}

	// Delete room
	key := roomKeyPrefix + roomID
	if err := s.client.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("failed to delete room: %w", err)
	}

	// Delete code mapping
	codeKey := "code:" + room.Code
	if err := s.client.Del(ctx, codeKey).Err(); err != nil {
		return fmt.Errorf("failed to delete room code mapping: %w", err)
	}

	return nil
}

// GetRoomByCode retrieves room data by invite code
func (s *RedisStore) GetRoomByCode(ctx context.Context, code string) (*RoomData, error) {
	codeKey := "code:" + code
	roomID, err := s.client.Get(ctx, codeKey).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get room by code: %w", err)
	}

	return s.GetRoom(ctx, roomID)
}

// SaveSession saves session-to-room mapping
func (s *RedisStore) SaveSession(ctx context.Context, sessionID, roomID string, playerIndex int) error {
	data := SessionData{
		RoomID:      roomID,
		PlayerIndex: playerIndex,
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal session: %w", err)
	}

	key := sessionKeyPrefix + sessionID
	if err := s.client.Set(ctx, key, jsonData, sessionTTL).Err(); err != nil {
		return fmt.Errorf("failed to save session: %w", err)
	}

	return nil
}

// GetSession retrieves session data
func (s *RedisStore) GetSession(ctx context.Context, sessionID string) (string, int, error) {
	key := sessionKeyPrefix + sessionID
	data, err := s.client.Get(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			return "", -1, nil
		}
		return "", -1, fmt.Errorf("failed to get session: %w", err)
	}

	var session SessionData
	if err := json.Unmarshal(data, &session); err != nil {
		return "", -1, fmt.Errorf("failed to unmarshal session: %w", err)
	}

	return session.RoomID, session.PlayerIndex, nil
}

// DeleteSession removes session data
func (s *RedisStore) DeleteSession(ctx context.Context, sessionID string) error {
	key := sessionKeyPrefix + sessionID
	if err := s.client.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("failed to delete session: %w", err)
	}
	return nil
}

// MemoryStore implements Store using in-memory maps (for testing/simple deployments)
type MemoryStore struct {
	mu       sync.RWMutex
	rooms    map[string]*RoomData
	codes    map[string]string // code -> roomID
	sessions map[string]*SessionData
}

// NewMemoryStore creates a new in-memory store
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		rooms:    make(map[string]*RoomData),
		codes:    make(map[string]string),
		sessions: make(map[string]*SessionData),
	}
}

func (s *MemoryStore) SaveRoom(ctx context.Context, room *RoomData) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.rooms[room.ID] = room
	s.codes[room.Code] = room.ID
	return nil
}

func (s *MemoryStore) GetRoom(ctx context.Context, roomID string) (*RoomData, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.rooms[roomID], nil
}

func (s *MemoryStore) DeleteRoom(ctx context.Context, roomID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if room, ok := s.rooms[roomID]; ok {
		delete(s.codes, room.Code)
		delete(s.rooms, roomID)
	}
	return nil
}

func (s *MemoryStore) GetRoomByCode(ctx context.Context, code string) (*RoomData, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if roomID, ok := s.codes[code]; ok {
		return s.rooms[roomID], nil
	}
	return nil, nil
}

func (s *MemoryStore) SaveSession(ctx context.Context, sessionID, roomID string, playerIndex int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.sessions[sessionID] = &SessionData{
		RoomID:      roomID,
		PlayerIndex: playerIndex,
	}
	return nil
}

func (s *MemoryStore) GetSession(ctx context.Context, sessionID string) (string, int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if session, ok := s.sessions[sessionID]; ok {
		return session.RoomID, session.PlayerIndex, nil
	}
	return "", -1, nil
}

func (s *MemoryStore) DeleteSession(ctx context.Context, sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.sessions, sessionID)
	return nil
}
