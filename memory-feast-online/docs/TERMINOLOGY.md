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
| 대기열 시간 초과 | Queue Timeout | `queue_timeout` | 대기열 제한 시간 초과 알림 (`{timeoutSeconds}`) |
| 매칭됨 | Matched | `matched` | 상대와 매칭 완료 (`{roomId, roomCode?, playerIndex, opponent}`) |
| 방 생성됨 | Room Created | `room_created` | 방 생성 완료 (`{roomId, roomCode}`) |
| 방 참여됨 | Room Joined | `room_joined` | 방 참여 완료 (`{roomId, roomCode, playerIndex, opponent}`) |
| 게임 상태 | Game State | `game_state` | 현재 게임 상태 전체 전송 |
| 게임 종료 | Game End | `game_end` | 게임 종료 및 결과 (`{winner, reason, finalTokens}`) |
| 플레이어 퇴장 | Player Left | `player_left` | 상대 연결 끊김 알림 (`{gracePeriod}`) |
| 재접속 완료 | Reconnected | `reconnected` | 재접속 성공 (`{playerIndex}`) |

**코드 참조:** `internal/ws/message.go`, `cmd/server/main.go`

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
| 기권 | Forfeit | `forfeit` | 명시적 방 나가기(`leave_room`) 또는 재접속 유예 시간 초과 |

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

## 10. 운영 상수 및 제한 (Runtime Constants)

| 항목 | 코드 심볼 | 기본값 | 설명 |
|------|-----------|--------|------|
| 재접속 유예 시간 | `ReconnectGracePeriod` | `30s` | 상대 연결 끊김 후 복귀 허용 시간 |
| 기본 접시 수 | `DefaultPlateCount` | `20` | 방 생성 시 `plateCount` 미지정(0) 기본값 |
| 매칭 제한 시간 | `MatchingTimeLimit` | `60` | 매칭 단계 턴 제한 시간(초) |

**코드 참조:** `internal/game/room.go`

---

## 11. 튜토리얼/가이드 UI 용어 (Tutorial/Guide UI Terms)

프론트엔드에서 제공하는 튜토리얼/규칙 상세 UI 관련 용어입니다.

| 한국어 | English | 키/식별자 | 설명 |
|--------|---------|-----------|------|
| 가이드 저장 키 | Guide Storage Key | `memoryFeastOnlineGuideStateV1` | 로컬스토리지에 가이드 진행 상태 저장 |
| 가이드 탭 | Guide Tab | `tutorial` / `rules` | 모달의 튜토리얼/규칙 상세 탭 |
| 가이드 상태 | Guide State | `completed`, `lastTab`, `lastStep` | 가이드 완료 여부/마지막 탭/마지막 단계 저장 필드 |
| 튜토리얼 단계 목록 | Tutorial Steps | `tutorialSteps` | 5단계 튜토리얼 카드 데이터(제목/요약/포인트) |
| 규칙 상세 목록 | Rules List | `rules` | 단계별 상세 규칙 목록(`placement`, `matching`, `add_token`) |
| 단계 힌트 맵 | Phase Hints | `phaseHints` | 현재 게임 단계 기준 도움말 텍스트 매핑 |
| 단계 도움말 | Phase Help | `phase-help` panel | 현재 게임 단계 기반 설명 표시 |
| 메시지 타입 | Message Type | `success` / `fail` / `info` | 인게임 알림 스타일 구분 |

가이드 모달/패널 식별자(프론트엔드 UI):

| 용어 | 식별자 | 설명 |
|------|--------|------|
| 가이드 모달 | `guide-modal` | 튜토리얼/규칙 상세 모달 컨테이너 |
| 튜토리얼 패널 | `guide-tutorial-panel` | 튜토리얼 단계 카드 영역 |
| 규칙 패널 | `guide-rules-panel` | 단계별 규칙 목록 영역 |
| 현재 단계 라벨 | `guide-current-phase-label` | 현재 게임 단계 라벨 텍스트 |
| 단계 도움말 제목 | `phase-help-title` | 인게임 도움말 제목 텍스트 |
| 단계 도움말 본문 | `phase-help-text` | 인게임 도움말 본문 텍스트 |

가이드 동작 메서드(클라이언트):

| 메서드 | 설명 |
|--------|------|
| `openGuide(tab)` | 가이드 모달 열기 및 시작 탭 선택 |
| `closeGuide()` | 가이드 모달 닫기 및 상태 저장 |
| `selectGuideTab(tab)` | 튜토리얼/규칙 탭 전환 |
| `guideNext()` / `guidePrev()` | 튜토리얼 단계 이동 |
| `updateGuideUI()` | 가이드 탭/패널/버튼 상태 갱신 |
| `updatePhaseHelp(phase)` | 현재 단계 기반 도움말 텍스트 갱신 |

연결 상태 표시값(클라이언트 UI):

| 상태 값 | 표시 텍스트 |
|---------|-------------|
| `connecting` | 연결 중... |
| `connected` | 연결됨 |
| `disconnected` | 연결 끊김 |

주요 UI 영역(게임 화면):

| 한국어 | English | 식별자 | 설명 |
|--------|---------|--------|------|
| 게임 정보 영역 | Game Info Area | `.game-info` | 플레이어 정보, 단계 정보, 메시지 영역을 포함한 상단 정보 레이아웃 |
| 플레이어 정보 카드 | Player Info Card | `player0-info`, `player1-info` (`.player-info`) | 플레이어 닉네임/토큰/접속 상태 표시, 현재 턴 강조 |
| 단계 정보 영역 | Phase Info Area | `.phase-info` | 현재 게임 단계/남은 시간/단계 제목 표시 |
| 배치 라운드 정보 | Placement Info | `placement-info` | 배치 단계 라운드/최대 라운드 안내 표시 |
| 단계 도움말 패널 | Phase Help Panel | `phase-help` (`phase-help-title`, `phase-help-text`) | 현재 단계 행동 가이드(규칙 힌트) 표시 |
| 인게임 메시지 영역 | In-Game Message Area | `message-area` | 서버 `game_state.message`를 렌더링하는 지시문/알림 영역 |
| 메시지 박스 | Message Box | `.message.success` / `.message.fail` / `.message.info` | 메시지 스타일(초록/빨강/중립) 구분 |

인게임 메시지 영역 렌더링 규칙:

| 항목 | 식별자/필드 | 설명 |
|------|-------------|------|
| 메시지 텍스트 | `state.message` | 서버가 보낸 지시문 본문 |
| 메시지 타입 | `state.messageType` | `success` / `fail` / `info` 스타일 결정 |
| 렌더링 메서드 | `showMessage(text, type)` | `message-area` 내부에 메시지 박스를 생성하고 3초 후 제거 |

**코드 참조:** `web/index.html`

---

## 12. 내부 동기화/보호 용어 (Internal Safety Terms)

동시성 및 중복 행동 방지를 위해 내부에서 사용하는 보호 용어입니다.

| 용어 | 코드 식별자 | 설명 |
|------|-------------|------|
| 배치 중복 방지 잠금 | `placementPending` | 배치 단계에서 턴당 1회 클릭만 허용 |
| 매칭 확인 중 잠금 | `confirmPending` | 매칭 확인 처리 중 추가 선택 차단 |
| 토큰 추가 중 잠금 | `addTokenPending` | 토큰 추가 단계에서 중복 추가 차단 |
| 방 활성 상태 확인 | `isRoomActive(room)` | 지연 콜백(`time.AfterFunc`) 실행 전 방 유효성 확인 |

**코드 참조:** `internal/game/room.go`, `cmd/server/main.go`

---

## 13. 파일 참조 요약

| 파일 | 내용 |
|------|------|
| `internal/game/state.go` | 게임 단계(Phase), 게임 상태(GameState), 상태 변경 함수 |
| `internal/ws/message.go` | 메시지 타입, 페이로드 구조체, 상태별 허용 메시지 |
| `internal/ws/hub.go` | 클라이언트 상태(ClientState), WebSocket 클라이언트 관리 |
| `internal/ws/client.go` | WebSocket read/write 루프, ping/pong, 메시지 크기 제한 |
| `internal/game/room.go` | 방 관리, 행동 핸들러, 상태 브로드캐스트 |
| `internal/game/player.go` | 플레이어 연결/재접속 상태, 연결 끊김 시간 관리 |
| `internal/game/matchmaker.go` | 랜덤 매칭 큐, 큐 타임아웃, 매칭 페어링 |
| `internal/store/redis.go` | Redis/Memory 저장소 모델, 세션-방 매핑 |
| `cmd/server/main.go` | 메시지 라우팅, HTTP/WebSocket 핸들러 |
| `web/index.html` | 클라이언트 상태 렌더링, 게임 UI, 튜토리얼/가이드 UI |
