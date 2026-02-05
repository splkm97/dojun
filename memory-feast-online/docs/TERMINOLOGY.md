# 기억의 만찬 - 용어 정리 (Terminology Reference)

## 개요 (Overview)

이 문서는 "기억의 만찬" 온라인 멀티플레이어 게임의 모든 상태(State)와 행동(Action)에 대한 명확한 용어 정리입니다.

---

## 1. 클라이언트 상태 (Client State)

WebSocket 클라이언트의 연결 상태를 나타냅니다.

| 한국어 | English | 코드 | 설명 |
|--------|---------|------|------|
| 로비 | Lobby | `ClientLobby` | 초기 연결 상태. 방 생성/참여/매칭 가능 |
| 대기 | Waiting | `ClientWaiting` | 매칭 대기열 또는 방에서 상대 대기 중 |
| 게임 중 | In Game | `ClientInGame` | 게임 진행 중 |

**코드 참조:** `internal/ws/hub.go:92-96`

```go
const (
    ClientLobby   ClientState = "lobby"
    ClientWaiting ClientState = "waiting"
    ClientInGame  ClientState = "in_game"
)
```

---

## 2. 게임 단계 (Game Phase)

게임 진행의 각 단계를 나타냅니다.

| 한국어 | English | 코드 | JSON 값 | 설명 |
|--------|---------|------|---------|------|
| 대기 중 | Waiting | `PhaseWaiting` | `"waiting"` | 게임 시작 전 대기 |
| 배치 단계 | Placement | `PhasePlacement` | `"placement"` | 플레이어가 번갈아 토큰을 접시에 배치 |
| 매칭 단계 | Matching | `PhaseMatching` | `"matching"` | 같은 토큰 수의 접시 2개를 찾아 맞추기 |
| 토큰 추가 | Add Token | `PhaseAddToken` | `"add_token"` | 매칭 성공 후 접시에 토큰 1개 추가 |
| 종료 | Finished | `PhaseFinished` | `"finished"` | 게임 종료 |

**코드 참조:** `internal/game/state.go:6-12`

```go
const (
    PhaseWaiting   Phase = "waiting"
    PhasePlacement Phase = "placement"
    PhaseMatching  Phase = "matching"
    PhaseAddToken  Phase = "add_token"
    PhaseFinished  Phase = "finished"
)
```

---

## 3. 클라이언트 → 서버 메시지 (Client Actions)

클라이언트가 서버에 보내는 행동 메시지입니다.

### 3.1 로비 행동 (Lobby Actions)

| 한국어 | English | 메시지 타입 | 페이로드 | 설명 |
|--------|---------|-------------|----------|------|
| 랜덤 매칭 참여 | Join Queue | `join_queue` | `{nickname, sessionId}` | 랜덤 매칭 대기열에 참여 |
| 방 생성 | Create Room | `create_room` | `{nickname, sessionId, plateCount}` | 초대 코드로 방 생성 |
| 방 참여 | Join Room | `join_room` | `{nickname, sessionId, roomCode}` | 초대 코드로 방 참여 |
| 재접속 | Reconnect | `reconnect` | `{sessionId}` | 기존 게임에 재접속 시도 |

**코드 참조:** `internal/ws/message.go:10-18`, `internal/ws/message.go:182-186`

### 3.2 대기 행동 (Waiting Actions)

| 한국어 | English | 메시지 타입 | 페이로드 | 설명 |
|--------|---------|-------------|----------|------|
| 방 나가기 | Leave Room | `leave_room` | `{}` | 대기 취소 및 로비로 복귀 |

**코드 참조:** `internal/ws/message.go:189-191`

### 3.3 게임 중 행동 (In-Game Actions)

| 한국어 | English | 메시지 타입 | 페이로드 | 허용 단계 | 설명 |
|--------|---------|-------------|----------|-----------|------|
| 토큰 배치 | Place Token | `place_token` | `{index}` | Placement | 접시에 토큰 배치 |
| 접시 선택 | Select Plate | `select_plate` | `{index}` | Matching | 매칭할 접시 선택/해제 |
| 매칭 확인 | Confirm Match | `confirm_match` | `{}` | Matching | 선택한 2개 접시 매칭 확인 |
| 토큰 추가 | Add Token | `add_token` | `{index}` | Add Token | 매칭된 접시 중 하나에 토큰 추가 |
| 방 나가기 | Leave Room | `leave_room` | `{}` | Any | 게임 포기 (상대 승리) |

**코드 참조:** `internal/ws/message.go:193-198`

---

## 4. 서버 → 클라이언트 메시지 (Server Events)

서버가 클라이언트에게 보내는 이벤트 메시지입니다.

| 한국어 | English | 메시지 타입 | 설명 |
|--------|---------|-------------|------|
| 오류 | Error | `error` | 오류 발생 알림 (`{code, message}`) |
| 대기열 참여됨 | Queue Joined | `queue_joined` | 랜덤 매칭 대기열 참여 확인 (`{position}`) |
| 매칭됨 | Matched | `matched` | 상대와 매칭 완료 (`{roomId, playerIndex, opponent}`) |
| 방 생성됨 | Room Created | `room_created` | 방 생성 완료 (`{roomId, roomCode}`) |
| 방 참여됨 | Room Joined | `room_joined` | 방 참여 완료 |
| 게임 상태 | Game State | `game_state` | 현재 게임 상태 전체 전송 |
| 게임 종료 | Game End | `game_end` | 게임 종료 및 결과 (`{winner, reason, finalTokens}`) |
| 플레이어 퇴장 | Player Left | `player_left` | 상대 연결 끊김 알림 (`{gracePeriod}`) |
| 재접속 완료 | Reconnected | `reconnected` | 재접속 성공 (`{playerIndex}`) |

**코드 참조:** `internal/ws/message.go:21-29`

---

## 5. 게임 오브젝트 (Game Objects)

### 5.1 접시 (Plate)

| 한국어 | English | 필드명 | 타입 | 설명 |
|--------|---------|--------|------|------|
| 토큰 수 | Token Count | `tokens` | `int` | 접시에 놓인 토큰 개수 |
| 덮개 상태 | Covered | `covered` | `bool` | 접시 덮개가 덮여 있는지 |
| 토큰 존재 | Has Tokens | `hasTokens` | `bool` | 배치 단계에서 토큰이 배치되었는지 |

**코드 참조:** `internal/game/state.go:15-19`

### 5.2 플레이어 (Player)

| 한국어 | English | 필드명 | 타입 | 설명 |
|--------|---------|--------|------|------|
| 닉네임 | Nickname | `nickname` | `string` | 플레이어 표시 이름 |
| 보유 토큰 | Tokens | `tokens` | `int` | 플레이어가 보유한 토큰 수 (0이 되면 승리) |
| 연결 상태 | Connected | `isConnected` | `bool` | WebSocket 연결 상태 |

**코드 참조:** `internal/ws/message.go:123-128`

---

## 6. 게임 상태 필드 (Game State Fields)

`game_state` 메시지에 포함되는 필드들입니다.

| 한국어 | English | 필드명 | 타입 | 설명 |
|--------|---------|--------|------|------|
| 현재 단계 | Phase | `phase` | `string` | 현재 게임 단계 |
| 현재 턴 | Current Turn | `currentTurn` | `int` | 현재 차례 플레이어 인덱스 (0 또는 1) |
| 배치 라운드 | Placement Round | `placementRound` | `int` | 현재 배치 라운드 (1부터 시작) |
| 최대 라운드 | Max Round | `maxRound` | `int` | 배치 단계 총 라운드 수 |
| 남은 시간 | Time Left | `timeLeft` | `int` | 매칭 단계 남은 시간 (초) |
| 플레이어 목록 | Players | `players` | `[]PlayerInfo` | 양 플레이어 정보 |
| 접시 목록 | Plates | `plates` | `[]PlateInfo` | 모든 접시 상태 |
| 선택된 접시 | Selected Plates | `selectedPlates` | `[]int` | 내가 선택한 접시 인덱스 |
| 상대 선택 접시 | Opponent Selected | `opponentSelectedPlates` | `[]int` | 상대가 선택한 접시 인덱스 |
| 매칭된 접시 | Matched Plates | `matchedPlates` | `[]int` | 매칭 성공한 접시 인덱스 |
| 최근 행동 접시 | Last Action Plate | `lastActionPlate` | `int?` | 마지막으로 토큰이 배치/추가된 접시 (애니메이션용) |
| 메시지 | Message | `message` | `string` | 화면에 표시할 메시지 |
| 메시지 타입 | Message Type | `messageType` | `string` | 메시지 스타일 (success/fail/info) |

**코드 참조:** `internal/ws/message.go:107-121`

---

## 7. 게임 종료 사유 (Game End Reasons)

| 한국어 | English | 코드 값 | 설명 |
|--------|---------|---------|------|
| 토큰 소진 | Tokens Depleted | `tokens` | 플레이어가 토큰을 모두 소진하여 승리 |
| 매칭 불가 | No Matches | `no_matches` | 더 이상 매칭 가능한 쌍이 없음 (토큰 수 비교로 승패 결정) |
| 기권 | Forfeit | `forfeit` | 상대방이 재접속 유예 기간 내 복귀하지 않음 |

**코드 참조:** `internal/ws/message.go:137-142`

---

## 8. 상태 전이 다이어그램 (State Transitions)

### 8.1 클라이언트 상태 전이

```
┌─────────┐  join_queue/create_room/join_room  ┌─────────┐
│  Lobby  │ ─────────────────────────────────► │ Waiting │
└─────────┘                                    └─────────┘
     ▲                                              │
     │ leave_room                                   │ matched / room_full
     │                                              ▼
     │                                         ┌─────────┐
     └──────────────── game_end ◄───────────── │ In Game │
                                               └─────────┘
```

### 8.2 게임 단계 전이

```
┌─────────┐          ┌───────────┐          ┌──────────┐
│ Waiting │ ───────► │ Placement │ ───────► │ Matching │
└─────────┘  start   └───────────┘  done    └──────────┘
                                                 │   ▲
                                    success      │   │ next turn /
                                                 ▼   │ fail / timeout
                                            ┌───────────┐
                                            │ Add Token │
                                            └───────────┘
                                                 │
                                    no matches / └───► ┌──────────┐
                                    win              │ Finished │
                                                     └──────────┘
```

---

## 9. 용어 사용 예시

### 지시문 예시

| 의도 | 올바른 표현 |
|------|-------------|
| 토큰을 접시에 놓을 때 | "배치 단계에서 `place_token` 행동 수행" |
| 접시를 클릭하여 선택할 때 | "매칭 단계에서 `select_plate` 행동 수행" |
| 선택 확정할 때 | "`confirm_match` 행동으로 매칭 확인" |
| 매칭 성공 후 토큰 추가할 때 | "토큰 추가 단계에서 `add_token` 행동 수행" |
| 게임 상태 동기화 | "서버가 `game_state` 이벤트 브로드캐스트" |

---

## 10. 파일 참조 요약

| 파일 | 내용 |
|------|------|
| `internal/game/state.go` | 게임 단계(Phase), 게임 상태(GameState), 상태 변경 함수 |
| `internal/ws/message.go` | 메시지 타입, 페이로드 구조체, 상태별 허용 메시지 |
| `internal/ws/hub.go` | 클라이언트 상태(ClientState), WebSocket 클라이언트 관리 |
| `internal/game/room.go` | 방 관리, 행동 핸들러, 상태 브로드캐스트 |
| `cmd/server/main.go` | 메시지 라우팅, HTTP/WebSocket 핸들러 |
