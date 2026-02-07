package store

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestMemoryStoreConcurrentAccess(t *testing.T) {
	t.Parallel()

	store := NewMemoryStore()
	ctx := context.Background()

	const workers = 16
	const iterations = 120

	var wg sync.WaitGroup

	for worker := 0; worker < workers; worker++ {
		worker := worker
		wg.Add(1)
		go func() {
			defer wg.Done()

			for i := 0; i < iterations; i++ {
				roomID := fmt.Sprintf("room-%d-%d", worker, i)
				roomCode := fmt.Sprintf("C%05d", (worker*iterations+i)%99999)
				sessionID := fmt.Sprintf("session-%d-%d", worker, i)

				room := &RoomData{
					ID:         roomID,
					Code:       roomCode,
					PlateCount: 20,
					CreatedAt:  time.Now(),
				}

				if err := store.SaveRoom(ctx, room); err != nil {
					t.Errorf("SaveRoom failed: %v", err)
					return
				}

				if _, err := store.GetRoom(ctx, roomID); err != nil {
					t.Errorf("GetRoom failed: %v", err)
					return
				}

				if _, err := store.GetRoomByCode(ctx, roomCode); err != nil {
					t.Errorf("GetRoomByCode failed: %v", err)
					return
				}

				if err := store.SaveSession(ctx, sessionID, roomID, worker%2); err != nil {
					t.Errorf("SaveSession failed: %v", err)
					return
				}

				if _, _, err := store.GetSession(ctx, sessionID); err != nil {
					t.Errorf("GetSession failed: %v", err)
					return
				}

				if i%5 == 0 {
					if err := store.DeleteSession(ctx, sessionID); err != nil {
						t.Errorf("DeleteSession failed: %v", err)
						return
					}
					if err := store.DeleteRoom(ctx, roomID); err != nil {
						t.Errorf("DeleteRoom failed: %v", err)
						return
					}
				}
			}
		}()
	}

	wg.Wait()
}
