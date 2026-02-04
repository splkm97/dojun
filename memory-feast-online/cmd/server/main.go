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
		// Calculate plate count (average, rounded to even)
		plateCount := (entry1.PlateCount + entry2.PlateCount) / 2
		if plateCount%2 != 0 {
			plateCount++
		}
		if plateCount < 4 {
			plateCount = 4
		}
		if plateCount > 20 {
			plateCount = 20
		}

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
					c.SendMessage(matchedMsg)
				}
			}
		}

		// Send initial game state
		room.BroadcastState()
	} else {
		// Added to queue
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
	if plateCount < 4 {
		plateCount = 20
	}

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

		// Notify first player
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
					c.SendMessage(matchedMsg)
				}
			}
		}

		room.BroadcastState()
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
		// Cover the plate
		room.State.Plates[payload.Index].Covered = true

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
		if matched {
			// Success - transition to add token phase
			room.SetAddTokenPhase()
			state := room.GetGameState()
			state.Message = "매치 성공! 토큰을 추가할 접시를 선택하세요."
			state.MessageType = "success"
			s.broadcastStateWithMessage(room, state)
		} else {
			// Fail - add penalty and advance turn
			room.HandleMatchFail(playerIndex)
			state := room.GetGameState()
			state.Message = "매치 실패! " + room.GetPlayer(playerIndex).Nickname + "에게 페널티 토큰 +1"
			state.MessageType = "fail"

			time.AfterFunc(2*time.Second, func() {
				if room.AdvanceMatching() {
					s.startMatchingTimer(room)
					room.BroadcastState()
				} else {
					s.endGameNoMatches(room)
				}
			})

			s.broadcastStateWithMessage(room, state)
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

	if playerWon {
		s.endGame(room, playerIndex, "tokens")
		return
	}

	// Continue to next turn
	if room.AdvanceMatching() {
		s.startMatchingTimer(room)
		room.BroadcastState()
	} else {
		s.endGameNoMatches(room)
	}
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
		return
	}

	player := room.GetPlayer(playerIndex)
	if player != nil {
		player.ClearConnection()
	}

	// Notify opponent
	opponentIndex := 1 - playerIndex
	opponent := room.GetPlayer(opponentIndex)
	if opponent != nil && opponent.IsConnected() {
		leftMsg, _ := ws.NewMessage(ws.MsgPlayerLeft, ws.PlayerLeftPayload{
			PlayerIndex: playerIndex,
			GracePeriod: int(game.ReconnectGracePeriod.Seconds()),
		})
		c := s.hub.GetClient(opponent.SessionID)
		if c != nil {
			c.SendMessage(leftMsg)
		}

		// Start forfeit timer
		time.AfterFunc(game.ReconnectGracePeriod, func() {
			p := room.GetPlayer(playerIndex)
			if p != nil && !p.IsConnected() {
				s.endGame(room, opponentIndex, "forfeit")
			}
		})
	}
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
			// Tick - broadcast updated time
			room.BroadcastState()
		},
		func() {
			// Timeout
			currentTurn := room.State.CurrentTurn
			room.HandleTimeout(currentTurn)

			state := room.GetGameState()
			state.Message = "시간 초과! 페널티 토큰 +2"
			state.MessageType = "fail"
			s.broadcastStateWithMessage(room, state)

			time.AfterFunc(2*time.Second, func() {
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

func (s *Server) broadcastStateWithMessage(room *game.Room, state ws.GameStatePayload) {
	msg, _ := ws.NewMessage(ws.MsgGameState, state)
	room.BroadcastMessage(msg)
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
}

func (s *Server) sendError(client *ws.Client, code, message string) {
	errMsg, _ := ws.NewErrorMessage(code, message)
	client.SendMessage(errMsg)
}

func generateSessionID() string {
	return game.NewRoom(4).ID // Reuse ID generation
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
