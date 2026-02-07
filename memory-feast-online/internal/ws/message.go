package ws

import "encoding/json"

// MessageType defines the type of WebSocket message
type MessageType string

const (
	// Client -> Server messages
	MsgJoinQueue    MessageType = "join_queue"
	MsgCreateRoom   MessageType = "create_room"
	MsgJoinRoom     MessageType = "join_room"
	MsgPlaceToken   MessageType = "place_token"
	MsgSelectPlate  MessageType = "select_plate"
	MsgConfirmMatch MessageType = "confirm_match"
	MsgAddToken     MessageType = "add_token"
	MsgReconnect    MessageType = "reconnect"
	MsgLeaveRoom    MessageType = "leave_room"

	// Server -> Client messages
	MsgError        MessageType = "error"
	MsgQueueJoined  MessageType = "queue_joined"
	MsgQueueTimeout MessageType = "queue_timeout"
	MsgMatched      MessageType = "matched"
	MsgRoomCreated  MessageType = "room_created"
	MsgRoomJoined   MessageType = "room_joined"
	MsgGameState    MessageType = "game_state"
	MsgGameEnd      MessageType = "game_end"
	MsgPlayerLeft   MessageType = "player_left"
	MsgReconnected  MessageType = "reconnected"
)

// Message is the base WebSocket message structure
type Message struct {
	Type    MessageType     `json:"type"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

// JoinQueuePayload for joining random matchmaking
type JoinQueuePayload struct {
	Nickname  string `json:"nickname"`
	SessionID string `json:"sessionId"`
}

// CreateRoomPayload for creating a room with invite code
type CreateRoomPayload struct {
	Nickname   string `json:"nickname"`
	SessionID  string `json:"sessionId"`
	PlateCount int    `json:"plateCount"`
}

// JoinRoomPayload for joining a room by code
type JoinRoomPayload struct {
	Nickname  string `json:"nickname"`
	SessionID string `json:"sessionId"`
	RoomCode  string `json:"roomCode"`
}

// PlaceTokenPayload for placement phase action
type PlaceTokenPayload struct {
	Index int `json:"index"`
}

// SelectPlatePayload for matching phase selection
type SelectPlatePayload struct {
	Index int `json:"index"`
}

// ConfirmMatchPayload for confirming selected plates
type ConfirmMatchPayload struct{}

// AddTokenPayload for adding token to matched plate
type AddTokenPayload struct {
	Index int `json:"index"`
}

// ReconnectPayload for reconnecting with session ID
type ReconnectPayload struct {
	SessionID string `json:"sessionId"`
}

// ErrorPayload for error messages
type ErrorPayload struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// QueueJoinedPayload confirmation of queue join
type QueueJoinedPayload struct {
	Position int `json:"position"`
}

// QueueTimeoutPayload informs client that queue wait timed out
type QueueTimeoutPayload struct {
	TimeoutSeconds int `json:"timeoutSeconds"`
}

// MatchedPayload when two players are matched
type MatchedPayload struct {
	RoomID      string `json:"roomId"`
	RoomCode    string `json:"roomCode,omitempty"`
	PlayerIndex int    `json:"playerIndex"` // 0 or 1
	Opponent    string `json:"opponent"`
}

// RoomCreatedPayload when a room is created
type RoomCreatedPayload struct {
	RoomID   string `json:"roomId"`
	RoomCode string `json:"roomCode"`
}

// GameStatePayload contains the full game state
type GameStatePayload struct {
	Phase                  string       `json:"phase"` // waiting, placement, matching, add_token, finished
	CurrentTurn            int          `json:"currentTurn"`
	PlacementRound         int          `json:"placementRound"`
	MaxRound               int          `json:"maxRound"`
	TimeLeft               int          `json:"timeLeft"`
	Players                []PlayerInfo `json:"players"`
	Plates                 []PlateInfo  `json:"plates"`
	SelectedPlates         []int        `json:"selectedPlates"`
	OpponentSelectedPlates []int        `json:"opponentSelectedPlates,omitempty"` // Opponent's selections visible to this player
	MatchedPlates          []int        `json:"matchedPlates,omitempty"`
	LastActionPlate        *int         `json:"lastActionPlate,omitempty"` // Plate index of last placement/addition for animation
	Message                string       `json:"message,omitempty"`
	MessageType            string       `json:"messageType,omitempty"` // success, fail, info
}

// PlayerInfo for game state
type PlayerInfo struct {
	Nickname    string `json:"nickname"`
	Tokens      int    `json:"tokens"`
	IsConnected bool   `json:"isConnected"`
}

// PlateInfo for game state
type PlateInfo struct {
	Tokens    int  `json:"tokens"`
	Covered   bool `json:"covered"`
	HasTokens bool `json:"hasTokens"`
}

// GameEndPayload when game ends
type GameEndPayload struct {
	Winner      int    `json:"winner"` // 0 for draw, 1 or 2 for winner
	WinnerName  string `json:"winnerName"`
	Reason      string `json:"reason"` // tokens, no_matches, forfeit
	FinalTokens []int  `json:"finalTokens"`
}

// PlayerLeftPayload when opponent disconnects
type PlayerLeftPayload struct {
	PlayerIndex int `json:"playerIndex"`
	GracePeriod int `json:"gracePeriod"` // seconds until forfeit
}

// ReconnectedPayload when player successfully reconnects
type ReconnectedPayload struct {
	PlayerIndex int `json:"playerIndex"`
}

// Helper functions for creating messages

func NewMessage(msgType MessageType, payload interface{}) (*Message, error) {
	var payloadBytes json.RawMessage
	if payload != nil {
		bytes, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		payloadBytes = bytes
	}
	return &Message{
		Type:    msgType,
		Payload: payloadBytes,
	}, nil
}

func NewErrorMessage(code, message string) (*Message, error) {
	return NewMessage(MsgError, ErrorPayload{
		Code:    code,
		Message: message,
	})
}

// ValidMessagesForState maps ClientState to allowed message types
// ClientState is defined in hub.go
var ValidMessagesForLobby = []MessageType{
	MsgJoinQueue,
	MsgCreateRoom,
	MsgJoinRoom,
	MsgReconnect,
}

var ValidMessagesForWaiting = []MessageType{
	MsgLeaveRoom,
}

var ValidMessagesForInGame = []MessageType{
	MsgPlaceToken,
	MsgSelectPlate,
	MsgConfirmMatch,
	MsgAddToken,
	MsgLeaveRoom,
}
