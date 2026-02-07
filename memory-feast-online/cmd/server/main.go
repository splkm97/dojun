package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"memory-feast-online/internal/game"
	"memory-feast-online/internal/store"
	"memory-feast-online/internal/ws"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for development
	},
}

// Server holds all server state
type Server struct {
	hub        *ws.Hub
	matchmaker *game.Matchmaker
	rooms      map[string]*game.Room
	roomsMu    sync.RWMutex
	store      store.Store
}

// NewServer creates a new server instance
func NewServer(st store.Store) *Server {
	s := &Server{
		hub:   ws.NewHub(),
		rooms: make(map[string]*game.Room),
		store: st,
	}

	// Initialize matchmaker with callback
	s.matchmaker = game.NewMatchmaker(func(entry1, entry2 *game.QueueEntry) *game.Room {
		plateCount := game.ClampPlateCount((entry1.PlateCount + entry2.PlateCount) / 2)

		room := game.NewRoom(plateCount)
		room.Hub = s.hub
		room.SetOnEmpty(func(roomID string) {
			s.removeRoom(roomID)
		})

		// Add players
		room.AddPlayer(entry1.Player)
		room.AddPlayer(entry2.Player)

		// Store the room
		s.roomsMu.Lock()
		s.rooms[room.ID] = room
		s.roomsMu.Unlock()

		// Start the game
		room.StartGame()

		return room
	})

	return s
}

func (s *Server) removeRoom(roomID string) {
	s.roomsMu.Lock()
	delete(s.rooms, roomID)
	s.roomsMu.Unlock()

	// Also remove from store
	if s.store != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		s.store.DeleteRoom(ctx, roomID)
	}

	log.Printf("Room %s removed", roomID)
}

func (s *Server) getRoom(roomID string) *game.Room {
	s.roomsMu.RLock()
	defer s.roomsMu.RUnlock()
	return s.rooms[roomID]
}

func (s *Server) getRoomByCode(code string) *game.Room {
	s.roomsMu.RLock()
	defer s.roomsMu.RUnlock()
	for _, room := range s.rooms {
		if room.Code == code {
			return room
		}
	}
	return nil
}

// handleWebSocket handles WebSocket connections
func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}

	// Get session ID from query params or generate new one
	sessionID := r.URL.Query().Get("sessionId")
	if sessionID == "" {
		sessionID = generateSessionID()
	}

	client := ws.NewClient(s.hub, conn, sessionID)
	s.hub.Register(client)

	// Start write pump (includes ping/pong)
	go client.WritePump()

	// Read messages
	client.ReadPump(func(c *ws.Client, msg *ws.Message) {
		s.handleMessage(c, msg)
	})
}

func (s *Server) handleMessage(client *ws.Client, msg *ws.Message) {
	// Validate message against client state
	if !s.isMessageAllowedForState(client.GetState(), msg.Type) {
		s.sendError(client, "invalid_state",
			"Message "+string(msg.Type)+" not allowed in state "+string(client.GetState()))
		return
	}

	switch msg.Type {
	case ws.MsgJoinQueue:
		s.handleJoinQueue(client, msg)
	case ws.MsgCreateRoom:
		s.handleCreateRoom(client, msg)
	case ws.MsgJoinRoom:
		s.handleJoinRoom(client, msg)
	case ws.MsgPlaceToken:
		s.handlePlaceToken(client, msg)
	case ws.MsgSelectPlate:
		s.handleSelectPlate(client, msg)
	case ws.MsgConfirmMatch:
		s.handleConfirmMatch(client, msg)
	case ws.MsgAddToken:
		s.handleAddToken(client, msg)
	case ws.MsgReconnect:
		s.handleReconnect(client, msg)
	case ws.MsgLeaveRoom:
		s.handleLeaveRoom(client, msg)
	default:
		log.Printf("Unknown message type: %s", msg.Type)
	}
}

// isMessageAllowedForState checks if a message type is allowed for the client's state
func (s *Server) isMessageAllowedForState(state ws.ClientState, msgType ws.MessageType) bool {
	var allowedMsgs []ws.MessageType

	switch state {
	case ws.ClientLobby:
		allowedMsgs = ws.ValidMessagesForLobby
	case ws.ClientWaiting:
		allowedMsgs = ws.ValidMessagesForWaiting
	case ws.ClientInGame:
		allowedMsgs = ws.ValidMessagesForInGame
	default:
		return false
	}

	for _, allowed := range allowedMsgs {
		if allowed == msgType {
			return true
		}
	}
	return false
}

func (s *Server) handleJoinQueue(client *ws.Client, msg *ws.Message) {
	var payload ws.JoinQueuePayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		s.sendError(client, "invalid_payload", "Invalid join queue payload")
		return
	}

	if payload.Nickname == "" {
		s.sendError(client, "invalid_nickname", "Nickname is required")
		return
	}

	player := game.NewPlayer(client.SessionID, payload.Nickname, client.SessionID, client.Conn)

	// Join queue (default 20 plates)
	position, room := s.matchmaker.JoinQueue(player, client.Conn, 20)

	if room != nil {
		// Matched! Send matched message to both players
		for i := 0; i < 2; i++ {
			p := room.GetPlayer(i)
			if p != nil {
				matchedMsg, _ := ws.NewMessage(ws.MsgMatched, ws.MatchedPayload{
					RoomID:      room.ID,
					PlayerIndex: i,
					Opponent:    room.GetOpponentNickname(i),
				})

				c := s.hub.GetClient(p.SessionID)
				if c != nil {
					c.SetState(ws.ClientInGame) // Transition to InGame
					c.SendMessage(matchedMsg)
				}
			}
		}

		// Send initial game state
		room.BroadcastState()
	} else {
		// Added to queue - transition to Waiting
		client.SetState(ws.ClientWaiting)
		queueMsg, _ := ws.NewMessage(ws.MsgQueueJoined, ws.QueueJoinedPayload{
			Position: position,
		})
		client.SendMessage(queueMsg)
	}
}

func (s *Server) handleCreateRoom(client *ws.Client, msg *ws.Message) {
	var payload ws.CreateRoomPayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		s.sendError(client, "invalid_payload", "Invalid create room payload")
		return
	}

	if payload.Nickname == "" {
		s.sendError(client, "invalid_nickname", "Nickname is required")
		return
	}

	plateCount := payload.PlateCount
	if plateCount == 0 {
		plateCount = game.DefaultPlateCount
	}
	plateCount = game.ClampPlateCount(plateCount)

	room := game.NewRoom(plateCount)
	room.Hub = s.hub
	room.SetOnEmpty(func(roomID string) {
		s.removeRoom(roomID)
	})

	player := game.NewPlayer(client.SessionID, payload.Nickname, client.SessionID, client.Conn)
	room.AddPlayer(player)

	s.roomsMu.Lock()
	s.rooms[room.ID] = room
	s.roomsMu.Unlock()

	// Transition to Waiting state
	client.SetState(ws.ClientWaiting)

	// Send room created message
	createdMsg, _ := ws.NewMessage(ws.MsgRoomCreated, ws.RoomCreatedPayload{
		RoomID:   room.ID,
		RoomCode: room.Code,
	})
	client.SendMessage(createdMsg)
}

func (s *Server) handleJoinRoom(client *ws.Client, msg *ws.Message) {
	var payload ws.JoinRoomPayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		s.sendError(client, "invalid_payload", "Invalid join room payload")
		return
	}

	if payload.Nickname == "" {
		s.sendError(client, "invalid_nickname", "Nickname is required")
		return
	}

	room := s.getRoomByCode(payload.RoomCode)
	if room == nil {
		s.sendError(client, "room_not_found", "Room not found")
		return
	}

	if room.IsFull() {
		s.sendError(client, "room_full", "Room is full")
		return
	}

	player := game.NewPlayer(client.SessionID, payload.Nickname, client.SessionID, client.Conn)
	playerIndex, err := room.AddPlayer(player)
	if err != nil {
		s.sendError(client, "join_failed", err.Error())
		return
	}

	// Send joined message
	joinedMsg, _ := ws.NewMessage(ws.MsgRoomJoined, ws.MatchedPayload{
		RoomID:      room.ID,
		RoomCode:    room.Code,
		PlayerIndex: playerIndex,
		Opponent:    room.GetOpponentNickname(playerIndex),
	})
	client.SendMessage(joinedMsg)

	// If room is now full, start the game
	if room.IsFull() {
		room.StartGame()

		// Transition both players to InGame state
		client.SetState(ws.ClientInGame)

		// Notify first player and transition them
		for i := 0; i < 2; i++ {
			p := room.GetPlayer(i)
			if p != nil && i != playerIndex {
				matchedMsg, _ := ws.NewMessage(ws.MsgMatched, ws.MatchedPayload{
					RoomID:      room.ID,
					PlayerIndex: i,
					Opponent:    room.GetOpponentNickname(i),
				})
				c := s.hub.GetClient(p.SessionID)
				if c != nil {
					c.SetState(ws.ClientInGame)
					c.SendMessage(matchedMsg)
				}
			}
		}

		room.BroadcastState()
	} else {
		// Room not full yet, waiting for opponent
		client.SetState(ws.ClientWaiting)
	}
}

func (s *Server) handlePlaceToken(client *ws.Client, msg *ws.Message) {
	var payload ws.PlaceTokenPayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		s.sendError(client, "invalid_payload", "Invalid place token payload")
		return
	}

	room, playerIndex := s.findPlayerRoom(client.SessionID)
	if room == nil {
		s.sendError(client, "not_in_room", "You are not in a room")
		return
	}

	if !room.HandlePlaceToken(playerIndex, payload.Index) {
		s.sendError(client, "invalid_action", "Cannot place token here")
		return
	}

	// Show token briefly, then cover
	room.BroadcastState()

	// Wait briefly then advance turn
	time.AfterFunc(1500*time.Millisecond, func() {
		if !s.isRoomActive(room) {
			return
		}

		room.CoverPlate(payload.Index)

		if room.AdvancePlacement() {
			// Placement complete, start matching
			room.StartMatchingPhase()
			s.startMatchingTimer(room)
		}
		room.BroadcastState()
	})
}

func (s *Server) handleSelectPlate(client *ws.Client, msg *ws.Message) {
	var payload ws.SelectPlatePayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		s.sendError(client, "invalid_payload", "Invalid select plate payload")
		return
	}

	room, playerIndex := s.findPlayerRoom(client.SessionID)
	if room == nil {
		s.sendError(client, "not_in_room", "You are not in a room")
		return
	}

	if !room.HandleSelectPlate(playerIndex, payload.Index) {
		return // Silently ignore invalid selections
	}

	room.BroadcastState()
}

func (s *Server) handleConfirmMatch(client *ws.Client, msg *ws.Message) {
	room, playerIndex := s.findPlayerRoom(client.SessionID)
	if room == nil {
		s.sendError(client, "not_in_room", "You are not in a room")
		return
	}

	room.StopTimer()

	success, matched, _, _ := room.HandleConfirmMatch(playerIndex)
	if !success {
		s.sendError(client, "invalid_action", "Cannot confirm match")
		return
	}

	// Show plates
	room.BroadcastState()

	time.AfterFunc(2*time.Second, func() {
		if !s.isRoomActive(room) {
			return
		}

		if matched {
			// Success - transition to add token phase
			room.SetAddTokenPhase()
			s.broadcastStateWithMessage(room, "매치 성공! 토큰을 추가할 접시를 선택하세요.", "success")
		} else {
			// Fail - add penalty and advance turn
			room.HandleMatchFail(playerIndex)
			player := room.GetPlayer(playerIndex)
			nickname := "해당 플레이어"
			if player != nil {
				nickname = player.Nickname
			}
			message := "매치 실패! " + nickname + "에게 페널티 토큰 +1"

			time.AfterFunc(2*time.Second, func() {
				if !s.isRoomActive(room) {
					return
				}

				if room.AdvanceMatching() {
					s.startMatchingTimer(room)
					room.BroadcastState()
				} else {
					s.endGameNoMatches(room)
				}
			})

			s.broadcastStateWithMessage(room, message, "fail")
		}
	})
}

func (s *Server) handleAddToken(client *ws.Client, msg *ws.Message) {
	var payload ws.AddTokenPayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		s.sendError(client, "invalid_payload", "Invalid add token payload")
		return
	}

	room, playerIndex := s.findPlayerRoom(client.SessionID)
	if room == nil {
		s.sendError(client, "not_in_room", "You are not in a room")
		return
	}

	success, _, playerWon := room.HandleAddToken(playerIndex, payload.Index)
	if !success {
		s.sendError(client, "invalid_action", "Cannot add token here")
		return
	}

	// Broadcast state with lastActionPlate for animation
	room.BroadcastState()

	if playerWon {
		// Delay to show animation before ending game
		time.AfterFunc(1500*time.Millisecond, func() {
			if !s.isRoomActive(room) {
				return
			}
			s.endGame(room, playerIndex, "tokens")
		})
		return
	}

	// Delay to show animation, then continue to next turn
	time.AfterFunc(1500*time.Millisecond, func() {
		if !s.isRoomActive(room) {
			return
		}

		if room.AdvanceMatching() {
			s.startMatchingTimer(room)
			room.BroadcastState()
		} else {
			s.endGameNoMatches(room)
		}
	})
}

func (s *Server) handleReconnect(client *ws.Client, msg *ws.Message) {
	var payload ws.ReconnectPayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		s.sendError(client, "invalid_payload", "Invalid reconnect payload")
		return
	}

	room, playerIndex := s.findPlayerRoom(payload.SessionID)
	if room == nil {
		s.sendError(client, "no_active_game", "No active game found")
		return
	}

	// Check if game is already finished
	if room.GetPhase() == game.PhaseFinished {
		s.sendError(client, "game_finished", "Game has already ended")
		return
	}

	player := room.GetPlayer(playerIndex)
	if player == nil {
		s.sendError(client, "player_not_found", "Player not found")
		return
	}

	// Check grace period
	if player.DisconnectedDuration() > game.ReconnectGracePeriod {
		s.sendError(client, "grace_period_expired", "Reconnection grace period expired")
		return
	}

	// Update connection
	player.SetConnection(client.Conn)

	// Transition to InGame state
	client.SetState(ws.ClientInGame)

	// Send reconnected message
	reconnectedMsg, _ := ws.NewMessage(ws.MsgReconnected, ws.ReconnectedPayload{
		PlayerIndex: playerIndex,
	})
	client.SendMessage(reconnectedMsg)

	// Send current game state
	room.BroadcastState()
}

func (s *Server) handleLeaveRoom(client *ws.Client, msg *ws.Message) {
	room, playerIndex := s.findPlayerRoom(client.SessionID)
	if room == nil {
		s.matchmaker.LeaveQueue(client.SessionID)
		// If not in a room, just reset to lobby
		client.SetState(ws.ClientLobby)
		return
	}

	player := room.GetPlayer(playerIndex)
	if player != nil {
		player.ClearConnection()
	}

	// Transition leaving player to Lobby
	client.SetState(ws.ClientLobby)

	// Explicit leave = immediate forfeit, opponent wins
	opponentIndex := 1 - playerIndex
	s.endGame(room, opponentIndex, "forfeit")
}

func (s *Server) findPlayerRoom(sessionID string) (*game.Room, int) {
	s.roomsMu.RLock()
	defer s.roomsMu.RUnlock()

	for _, room := range s.rooms {
		if player, idx := room.GetPlayerBySessionID(sessionID); player != nil {
			return room, idx
		}
	}
	return nil, -1
}

func (s *Server) startMatchingTimer(room *game.Room) {
	room.StartTimer(
		func(timeLeft int) {
			if !s.isRoomActive(room) {
				return
			}

			// Tick - broadcast updated time
			room.BroadcastState()
		},
		func() {
			if !s.isRoomActive(room) {
				return
			}

			// Timeout
			currentTurn := room.GetCurrentTurn()
			room.HandleTimeout(currentTurn)

			s.broadcastStateWithMessage(room, "시간 초과! 페널티 토큰 +2", "fail")

			time.AfterFunc(2*time.Second, func() {
				if !s.isRoomActive(room) {
					return
				}

				if room.AdvanceMatching() {
					s.startMatchingTimer(room)
					room.BroadcastState()
				} else {
					s.endGameNoMatches(room)
				}
			})
		},
	)
}

func (s *Server) broadcastStateWithMessage(room *game.Room, message, messageType string) {
	for i := 0; i < 2; i++ {
		if room.GetPlayer(i) == nil {
			continue
		}

		state := room.GetGameStateForPlayer(i)
		state.Message = message
		state.MessageType = messageType

		msg, err := ws.NewMessage(ws.MsgGameState, state)
		if err != nil {
			log.Printf("failed to create game_state message for player %d in room %s: %v", i, room.ID, err)
			continue
		}

		if err := room.SendToPlayer(i, msg); err != nil {
			log.Printf("failed to send game_state to player %d in room %s: %v", i, room.ID, err)
		}
	}
}

func (s *Server) isRoomActive(room *game.Room) bool {
	if room == nil {
		return false
	}
	if room.GetPhase() == game.PhaseFinished {
		return false
	}
	return s.getRoom(room.ID) == room
}

// resetPlayersToLobby transitions all players in a room back to Lobby state
func (s *Server) resetPlayersToLobby(room *game.Room) {
	for i := 0; i < 2; i++ {
		if p := room.GetPlayer(i); p != nil {
			if c := s.hub.GetClient(p.SessionID); c != nil {
				c.SetState(ws.ClientLobby)
			}
		}
	}
}

func (s *Server) endGame(room *game.Room, winner int, reason string) {
	room.StopTimer()
	room.SetFinished()

	winnerName := ""
	if p := room.GetPlayer(winner); p != nil {
		winnerName = p.Nickname
	}

	finalTokens := []int{0, 0}
	if p0 := room.GetPlayer(0); p0 != nil {
		finalTokens[0] = p0.Tokens
	}
	if p1 := room.GetPlayer(1); p1 != nil {
		finalTokens[1] = p1.Tokens
	}

	endMsg, _ := ws.NewMessage(ws.MsgGameEnd, ws.GameEndPayload{
		Winner:      winner + 1, // 1-indexed for display
		WinnerName:  winnerName,
		Reason:      reason,
		FinalTokens: finalTokens,
	})
	room.BroadcastMessage(endMsg)

	s.resetPlayersToLobby(room)
	s.removeRoom(room.ID)
}

func (s *Server) endGameNoMatches(room *game.Room) {
	room.StopTimer()
	room.SetFinished()

	winner := room.GetWinner()
	winnerName := ""
	if winner >= 0 {
		if p := room.GetPlayer(winner); p != nil {
			winnerName = p.Nickname
		}
	}

	finalTokens := []int{0, 0}
	if p0 := room.GetPlayer(0); p0 != nil {
		finalTokens[0] = p0.Tokens
	}
	if p1 := room.GetPlayer(1); p1 != nil {
		finalTokens[1] = p1.Tokens
	}

	displayWinner := 0 // 0 for draw
	if winner >= 0 {
		displayWinner = winner + 1
	}

	endMsg, _ := ws.NewMessage(ws.MsgGameEnd, ws.GameEndPayload{
		Winner:      displayWinner,
		WinnerName:  winnerName,
		Reason:      "no_matches",
		FinalTokens: finalTokens,
	})
	room.BroadcastMessage(endMsg)

	s.resetPlayersToLobby(room)
	s.removeRoom(room.ID)
}

func (s *Server) sendError(client *ws.Client, code, message string) {
	errMsg, _ := ws.NewErrorMessage(code, message)
	client.SendMessage(errMsg)
}

func generateSessionID() string {
	return game.GenerateID()
}

func main() {
	// Get configuration from environment
	redisAddr := os.Getenv("REDIS_ADDR")
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	var st store.Store
	if redisAddr != "" {
		var err error
		st, err = store.NewRedisStore(redisAddr, "", 0)
		if err != nil {
			log.Printf("Failed to connect to Redis, using memory store: %v", err)
			st = store.NewMemoryStore()
		} else {
			log.Printf("Connected to Redis at %s", redisAddr)
		}
	} else {
		log.Println("REDIS_ADDR not set, using memory store")
		st = store.NewMemoryStore()
	}

	server := NewServer(st)

	// Start hub
	go server.hub.Run()

	// Routes
	http.HandleFunc("/ws", server.handleWebSocket)

	// Serve static files
	fs := http.FileServer(http.Dir("./web"))
	http.Handle("/", fs)

	log.Printf("Server starting on :%s", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
