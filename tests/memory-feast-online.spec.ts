/**
 * Memory Feast Online Multiplayer - Playwright E2E Tests
 * TDD: Tests designed BEFORE implementation verification
 *
 * Note: These tests require the WebSocket server to be running
 * Some tests mock WebSocket behavior for unit-level testing
 *
 * Test Categories (Priority Order):
 * 1. Error Cases - Connection failures, validation errors
 * 2. Edge Cases - Reconnection, timeout scenarios
 * 3. Happy Paths - Normal lobby and game flow
 */

import { test, expect, type Page, type BrowserContext } from '@playwright/test'

// Helper: Navigate to online game
// Uses port 8080 where Go server runs for full functionality
async function navigateToOnlineGame(page: Page) {
  const goServerUrl = 'http://localhost:8080/'

  // Use Go server which serves both static files and WebSocket
  await page.goto(goServerUrl, { timeout: 10000, waitUntil: 'networkidle' })
  await expect(page.locator('#lobby-screen')).toBeVisible({ timeout: 5000 })
}

// Helper: Fill nickname input
async function fillNickname(page: Page, nickname: string) {
  await page.locator('#nickname').fill(nickname)
}

// Helper: Fill room code
async function fillRoomCode(page: Page, code: string) {
  await page.locator('#room-code').fill(code)
}

async function injectMockGameState(page: Page, phase: 'placement' | 'matching' | 'add_token' = 'placement') {
  await page.evaluate((currentPhase) => {
    const game = (window as any).game
    if (!game || typeof game.handleGameState !== 'function') {
      return
    }

    const mockState = {
      phase: currentPhase,
      currentTurn: 0,
      placementRound: 2,
      maxRound: 9,
      timeLeft: 47,
      players: [
        { nickname: '플레이어1', tokens: 7, isConnected: true },
        { nickname: '플레이어2', tokens: 8, isConnected: true }
      ],
      plates: [
        { tokens: 1, covered: true, hasTokens: true },
        { tokens: 1, covered: true, hasTokens: true },
        { tokens: 2, covered: true, hasTokens: true },
        { tokens: 3, covered: true, hasTokens: true }
      ],
      selectedPlates: [],
      opponentSelectedPlates: [],
      matchedPlates: [0, 1],
      message: '',
      messageType: 'info',
      lastActionPlate: 1
    }

    game.handleGameState(mockState)
  }, phase)
}

// =============================================================================
// 1. ERROR CASES - Validation and connection errors
// =============================================================================

test.describe('Error Cases', () => {
  test('should require nickname for random matching', async ({ page }) => {
    await navigateToOnlineGame(page)

    // Don't fill nickname
    page.on('dialog', async dialog => {
      expect(dialog.message()).toContain('닉네임')
      await dialog.accept()
    })

    await page.locator('button:has-text("랜덤 매칭")').click()

    // Should remain on lobby screen
    await expect(page.locator('#lobby-screen')).toBeVisible()
  })

  test('should require nickname for creating room', async ({ page }) => {
    await navigateToOnlineGame(page)

    page.on('dialog', async dialog => {
      expect(dialog.message()).toContain('닉네임')
      await dialog.accept()
    })

    await page.locator('button:has-text("방 만들기")').click()

    // Should remain on lobby screen
    await expect(page.locator('#lobby-screen')).toBeVisible()
  })

  test('should require nickname for joining room', async ({ page }) => {
    await navigateToOnlineGame(page)

    await fillRoomCode(page, 'ABC123')

    page.on('dialog', async dialog => {
      expect(dialog.message()).toContain('닉네임')
      await dialog.accept()
    })

    await page.locator('.room-code-input button').click()

    // Should remain on lobby screen
    await expect(page.locator('#lobby-screen')).toBeVisible()
  })

  test('should validate room code format (must be 6 characters)', async ({ page }) => {
    await navigateToOnlineGame(page)

    await fillNickname(page, 'TestPlayer')
    await fillRoomCode(page, 'ABC') // Too short

    page.on('dialog', async dialog => {
      expect(dialog.message()).toContain('방 코드')
      await dialog.accept()
    })

    await page.locator('.room-code-input button').click()

    // Should remain on lobby screen
    await expect(page.locator('#lobby-screen')).toBeVisible()
  })

  test('should validate room code format (empty)', async ({ page }) => {
    await navigateToOnlineGame(page)

    await fillNickname(page, 'TestPlayer')
    // Don't fill room code

    page.on('dialog', async dialog => {
      expect(dialog.message()).toContain('방 코드')
      await dialog.accept()
    })

    await page.locator('.room-code-input button').click()

    await expect(page.locator('#lobby-screen')).toBeVisible()
  })

  test('should show connection status indicator', async ({ page }) => {
    await navigateToOnlineGame(page)

    // Connection status should be visible
    const status = page.locator('#connection-status')
    await expect(status).toBeVisible()

    // Should show one of the status classes
    const hasClass = await status.evaluate(el => {
      return el.classList.contains('connecting') ||
             el.classList.contains('connected') ||
             el.classList.contains('disconnected')
    })
    expect(hasClass).toBe(true)
  })

  test('should limit nickname to 12 characters', async ({ page }) => {
    await navigateToOnlineGame(page)

    const input = page.locator('#nickname')
    await input.fill('ThisIsAVeryLongNickname')

    const value = await input.inputValue()
    expect(value.length).toBeLessThanOrEqual(12)
  })
})

// =============================================================================
// 2. EDGE CASES - Connection and timeout scenarios
// =============================================================================

test.describe('Edge Cases', () => {
  test('should handle WebSocket disconnection gracefully', async ({ page }) => {
    await navigateToOnlineGame(page)

    // Initially might be connecting
    const status = page.locator('#connection-status')

    // Simulate disconnect by closing WebSocket (if possible via console)
    await page.evaluate(() => {
      const game = (window as any).game
      if (game && game.ws) {
        game.ws.close()
      }
    })

    // Should show disconnected status
    await expect(status).toHaveClass(/disconnected/, { timeout: 5000 })
  })

  test('should attempt reconnection after disconnect', async ({ page }) => {
    test.setTimeout(15000)

    await navigateToOnlineGame(page)

    // Close WebSocket
    await page.evaluate(() => {
      const game = (window as any).game
      if (game && game.ws) {
        game.ws.close()
      }
    })

    // Should show disconnected
    const status = page.locator('#connection-status')
    await expect(status).toHaveClass(/disconnected/, { timeout: 5000 })

    // Should attempt reconnection (3 second timeout in code)
    // Note: Will only succeed if server is running
    await page.waitForTimeout(4000)

    // Status should change to a retrying/connected state
    const currentClass = await status.getAttribute('class')
    expect(currentClass).toMatch(/connecting|connected/)
  })

  test('should expose exponential reconnect delay strategy', async ({ page }) => {
    await navigateToOnlineGame(page)

    const result = await page.evaluate(() => {
      const game = (window as any).game
      if (!game || typeof game.getReconnectDelay !== 'function') {
        return { hasStrategy: false, d1: 0, d2: 0, d3: 0 }
      }

      game.reconnectAttempts = 0
      const d1 = game.getReconnectDelay()
      game.reconnectAttempts = 1
      const d2 = game.getReconnectDelay()
      game.reconnectAttempts = 2
      const d3 = game.getReconnectDelay()

      return { hasStrategy: true, d1, d2, d3 }
    })

    expect(result.hasStrategy).toBe(true)
    expect(result.d2).toBeGreaterThanOrEqual(result.d1)
    expect(result.d3).toBeGreaterThanOrEqual(result.d2)
  })

  test('should persist sessionId in localStorage', async ({ page }) => {
    await navigateToOnlineGame(page)

    // Check localStorage has sessionId
    const sessionId = await page.evaluate(() => {
      return localStorage.getItem('sessionId')
    })

    expect(sessionId).toBeTruthy()
    expect(sessionId).toMatch(/^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i)
  })

  test('should reuse sessionId across page reloads', async ({ page }) => {
    await navigateToOnlineGame(page)

    const sessionId1 = await page.evaluate(() => localStorage.getItem('sessionId'))

    await page.reload()
    await expect(page.locator('#lobby-screen')).toBeVisible()

    const sessionId2 = await page.evaluate(() => localStorage.getItem('sessionId'))

    expect(sessionId1).toBe(sessionId2)
  })

  test('should transform room code to uppercase', async ({ page }) => {
    await navigateToOnlineGame(page)

    const input = page.locator('#room-code')
    await input.fill('abc123')

    // CSS transforms to uppercase visually, but we can verify the input styling
    await expect(input).toHaveCSS('text-transform', 'uppercase')
  })
})

// =============================================================================
// 3. HAPPY PATHS - Normal lobby flow
// =============================================================================

test.describe('Lobby UI', () => {
  test('should display lobby screen with all elements', async ({ page }) => {
    await navigateToOnlineGame(page)

    // Title and subtitle (use first() to handle multiple h1 elements)
    await expect(page.locator('#lobby-screen h1')).toContainText('기억의 만찬')
    await expect(page.locator('.subtitle')).toContainText('온라인')

    // Input fields
    await expect(page.locator('#nickname')).toBeVisible()
    await expect(page.locator('#room-code')).toBeVisible()

    // Buttons
    await expect(page.locator('button:has-text("랜덤 매칭")')).toBeVisible()
    await expect(page.locator('button:has-text("방 만들기")')).toBeVisible()
    await expect(page.locator('.room-code-input button:has-text("참여")')).toBeVisible()

    // Rules panel
    await expect(page.locator('.lobby-panel:has-text("게임 규칙")')).toBeVisible()
  })

  test('should display game rules correctly', async ({ page }) => {
    await navigateToOnlineGame(page)

    const rules = page.locator('.lobby-panel:has-text("게임 규칙")')

    await expect(rules).toContainText('배치 단계')
    await expect(rules).toContainText('매칭 단계')
    await expect(rules).toContainText('성공')
    await expect(rules).toContainText('실패')
    await expect(rules).toContainText('승리')
  })

  test('should allow typing nickname', async ({ page }) => {
    await navigateToOnlineGame(page)

    await fillNickname(page, 'TestPlayer')

    await expect(page.locator('#nickname')).toHaveValue('TestPlayer')
  })

  test('should open guide modal from lobby tutorial button', async ({ page }) => {
    await navigateToOnlineGame(page)

    await expect(page.locator('#guide-modal')).not.toHaveClass(/show/)
    await page.locator('#open-guide-lobby').click()

    await expect(page.locator('#guide-modal')).toHaveClass(/show/)
    await expect(page.locator('#guide-tab-tutorial')).toHaveClass(/active/)
    await expect(page.locator('#guide-step-title')).toContainText('게임 목표')
  })

  test('should switch to rules tab and show phase-highlighted rules', async ({ page }) => {
    await navigateToOnlineGame(page)

    await page.locator('#open-guide-lobby').click()
    await page.locator('#guide-tab-rules').click()

    await expect(page.locator('#guide-rules-panel')).toHaveClass(/active/)
    await expect(page.locator('#guide-current-phase-label')).toContainText('배치 단계')
    await expect(page.locator('.guide-rule-item.active-phase')).toContainText('배치 단계')
  })

  test('should persist guide completion in localStorage', async ({ page }) => {
    await navigateToOnlineGame(page)

    await page.locator('#open-guide-lobby').click()

    for (let i = 0; i < 5; i++) {
      await page.locator('#guide-next').click()
    }

    await expect(page.locator('#guide-modal')).not.toHaveClass(/show/)

    const guideState = await page.evaluate(() => localStorage.getItem('memoryFeastOnlineGuideStateV1'))
    expect(guideState).toBeTruthy()
    const parsed = JSON.parse(guideState || '{}')
    expect(parsed.completed).toBe(true)
  })
})

// =============================================================================
// 4. WAITING ROOM TESTS
// =============================================================================

test.describe('Waiting Room UI', () => {
  test('should have waiting screen elements', async ({ page }) => {
    await navigateToOnlineGame(page)

    // Waiting screen should exist but be hidden initially
    const waitingScreen = page.locator('#waiting-screen')
    await expect(waitingScreen).toBeAttached()
    await expect(waitingScreen).not.toBeVisible()

    // Check waiting screen elements exist
    await expect(page.locator('#waiting-title')).toBeAttached()
    await expect(page.locator('#room-code-display')).toBeAttached()
    await expect(page.locator('#waiting-message')).toBeAttached()
    await expect(page.locator('.spinner')).toBeAttached()
  })

  test('should have cancel button in waiting screen', async ({ page }) => {
    await navigateToOnlineGame(page)

    const cancelBtn = page.locator('#waiting-screen button:has-text("취소")')
    await expect(cancelBtn).toBeAttached()
  })
})

// =============================================================================
// 5. GAME SCREEN UI
// =============================================================================

test.describe('Game Screen UI', () => {
  test('should have game screen elements', async ({ page }) => {
    await navigateToOnlineGame(page)

    // Game screen should exist but be hidden
    const gameScreen = page.locator('#game-screen')
    await expect(gameScreen).toBeAttached()
    await expect(gameScreen).not.toBeVisible()

    // Player info panels
    await expect(page.locator('#player0-info')).toBeAttached()
    await expect(page.locator('#player1-info')).toBeAttached()

    // Phase info
    await expect(page.locator('#phase-title')).toBeAttached()
    await expect(page.locator('#timer')).toBeAttached()

    // Plates container
    await expect(page.locator('#plates-container')).toBeAttached()

    // Confirm button
    await expect(page.locator('#floating-confirm')).toBeAttached()
  })

  test('should have result modal', async ({ page }) => {
    await navigateToOnlineGame(page)

    const modal = page.locator('#result-modal')
    await expect(modal).toBeAttached()
    await expect(modal).not.toHaveClass(/show/)

    // Modal content
    await expect(page.locator('#result-title')).toBeAttached()
    await expect(page.locator('#winner-announcement')).toBeAttached()
    await expect(page.locator('#result-description')).toBeAttached()
    await expect(modal.locator('button:has-text("로비로")')).toBeAttached()
  })

  test('should update phase-aware help content when game phase changes', async ({ page }) => {
    await navigateToOnlineGame(page)

    await injectMockGameState(page, 'placement')
    await expect(page.locator('#phase-help-title')).toContainText('배치 단계')

    await injectMockGameState(page, 'matching')
    await expect(page.locator('#phase-help-title')).toContainText('매칭 단계')

    await injectMockGameState(page, 'add_token')
    await expect(page.locator('#phase-help-title')).toContainText('토큰 추가 단계')
  })

  test('should open guide from in-game button with current phase label', async ({ page }) => {
    await navigateToOnlineGame(page)

    await injectMockGameState(page, 'matching')
    await page.locator('#open-guide-game').click()
    await page.locator('#guide-tab-rules').click()

    await expect(page.locator('#guide-modal')).toHaveClass(/show/)
    await expect(page.locator('#guide-current-phase-label')).toContainText('매칭 단계')
    await expect(page.locator('.guide-rule-item.active-phase')).toContainText('매칭 단계')
  })
})

// =============================================================================
// 6. CONNECTION STATUS TESTS
// =============================================================================

test.describe('Connection Status UI', () => {
  test('should display correct connection labels', async ({ page }) => {
    await navigateToOnlineGame(page)

    // Verify status element structure
    const status = page.locator('#connection-status')
    await expect(status.locator('.status-dot')).toBeVisible()
    await expect(status.locator('span')).toBeVisible()

    // Check text content matches one of expected states
    const text = await status.locator('span').textContent()
    expect(['연결 중...', '연결됨', '연결 끊김']).toContain(text)
  })

  test('should have status dot with correct styling', async ({ page }) => {
    await navigateToOnlineGame(page)

    const dot = page.locator('.status-dot')

    // Should have border-radius for circular shape
    await expect(dot).toHaveCSS('border-radius', '50%')

    // Should have appropriate size
    await expect(dot).toHaveCSS('width', '8px')
    await expect(dot).toHaveCSS('height', '8px')
  })
})

// =============================================================================
// 7. RESPONSIVE DESIGN TESTS
// =============================================================================

test.describe('Responsive Design', () => {
  test('should adapt layout for mobile viewport', async ({ page }) => {
    await page.setViewportSize({ width: 375, height: 667 }) // iPhone SE
    await navigateToOnlineGame(page)

    // Lobby should still be functional
    await expect(page.locator('#lobby-screen')).toBeVisible()
    await expect(page.locator('#nickname')).toBeVisible()
    await expect(page.locator('button:has-text("랜덤 매칭")')).toBeVisible()
  })

  test('should adapt layout for tablet viewport', async ({ page }) => {
    await page.setViewportSize({ width: 768, height: 1024 }) // iPad
    await navigateToOnlineGame(page)

    await expect(page.locator('#lobby-screen')).toBeVisible()
  })
})

// =============================================================================
// 8. MULTI-PLAYER SCENARIO TESTS (Requires Server)
// =============================================================================

test.describe('Multi-Player Scenarios', () => {
  // Use serial mode to avoid WebSocket conflicts between tests
  test.describe.configure({ mode: 'serial' })

  test('should match two players in random queue', async ({ browser }) => {
    // This test requires a running server

    const context1 = await browser.newContext()
    const context2 = await browser.newContext()

    const page1 = await context1.newPage()
    const page2 = await context2.newPage()

    await navigateToOnlineGame(page1)
    await navigateToOnlineGame(page2)

    // Player 1 joins queue
    await fillNickname(page1, 'Player1')
    await page1.locator('button:has-text("랜덤 매칭")').click()

    // Player 2 joins queue
    await fillNickname(page2, 'Player2')
    await page2.locator('button:has-text("랜덤 매칭")').click()

    // Both should transition to game screen
    await expect(page1.locator('#game-screen')).toBeVisible({ timeout: 10000 })
    await expect(page2.locator('#game-screen')).toBeVisible({ timeout: 10000 })

    await context1.close()
    await context2.close()
  })

  test('should allow room creation and joining', async ({ browser }) => {
    test.setTimeout(30000) // Extended timeout for this test

    // This test requires a running server
    const context1 = await browser.newContext()
    const context2 = await browser.newContext()

    const page1 = await context1.newPage()
    const page2 = await context2.newPage()

    try {
      await navigateToOnlineGame(page1)
      await navigateToOnlineGame(page2)

      // Wait for WebSocket connection on both pages
      await expect(page1.locator('#connection-status')).toHaveClass(/connected/, { timeout: 10000 })
      await expect(page2.locator('#connection-status')).toHaveClass(/connected/, { timeout: 10000 })

      // Player 1 creates room
      await fillNickname(page1, 'Host')
      await page1.locator('button:has-text("방 만들기")').click()

      // Wait for room code to appear
      await expect(page1.locator('#waiting-screen')).toBeVisible({ timeout: 10000 })
      await expect(page1.locator('#room-code-display')).toBeVisible({ timeout: 10000 })

      // Wait for room code to have actual content
      await page1.waitForFunction(() => {
        const el = document.getElementById('room-code-display')
        return el && el.textContent && el.textContent.length === 6
      }, { timeout: 10000 })

      const roomCode = await page1.locator('#room-code-display').textContent()
      expect(roomCode).toBeTruthy()
      expect(roomCode?.length).toBe(6)

      // Player 2 joins with room code
      await fillNickname(page2, 'Guest')
      await fillRoomCode(page2, roomCode!)
      await page2.locator('.room-code-input button').click()

      // Wait for game state to sync - both should transition to game screen
      await Promise.all([
        expect(page1.locator('#game-screen')).toBeVisible({ timeout: 20000 }),
        expect(page2.locator('#game-screen')).toBeVisible({ timeout: 20000 })
      ])
    } finally {
      await context1.close()
      await context2.close()
    }
  })

  test('should reconnect and restore game state after browser refresh', async ({ browser }) => {
    test.setTimeout(45000)

    const context1 = await browser.newContext()
    const context2 = await browser.newContext()

    const page1 = await context1.newPage()
    const page2 = await context2.newPage()

    try {
      // Setup: Create a game between two players
      await navigateToOnlineGame(page1)
      await navigateToOnlineGame(page2)

      await expect(page1.locator('#connection-status')).toHaveClass(/connected/, { timeout: 10000 })
      await expect(page2.locator('#connection-status')).toHaveClass(/connected/, { timeout: 10000 })

      // Player 1 creates room
      await fillNickname(page1, 'Host')
      await page1.locator('button:has-text("방 만들기")').click()

      await expect(page1.locator('#waiting-screen')).toBeVisible({ timeout: 10000 })
      await page1.waitForFunction(() => {
        const el = document.getElementById('room-code-display')
        return el && el.textContent && el.textContent.length === 6
      }, { timeout: 10000 })

      const roomCode = await page1.locator('#room-code-display').textContent()

      // Player 2 joins
      await fillNickname(page2, 'Guest')
      await fillRoomCode(page2, roomCode!)
      await page2.locator('.room-code-input button').click()

      // Both in game
      await Promise.all([
        expect(page1.locator('#game-screen')).toBeVisible({ timeout: 20000 }),
        expect(page2.locator('#game-screen')).toBeVisible({ timeout: 20000 })
      ])

      // Verify game state before refresh
      await expect(page1.locator('#phase-title')).toContainText('배치')

      // ACTION: Player 1 refreshes browser
      await page1.reload()

      // EXPECTED: Player 1 should automatically reconnect to the game
      await expect(page1.locator('#connection-status')).toHaveClass(/connected/, { timeout: 10000 })

      // CRITICAL: Should be back in game screen, NOT lobby
      await expect(page1.locator('#game-screen')).toBeVisible({ timeout: 10000 })

      // Game state should be preserved
      await expect(page1.locator('#phase-title')).toContainText('배치')

      // Player 2 should still see opponent as connected
      await expect(page2.locator('#player0-info')).not.toHaveClass(/disconnected/, { timeout: 5000 })

    } finally {
      await context1.close()
      await context2.close()
    }
  })
})

// =============================================================================
// 9. ACCESSIBILITY TESTS
// =============================================================================

test.describe('Accessibility', () => {
  test('should have accessible input labels', async ({ page }) => {
    await navigateToOnlineGame(page)

    // Nickname input should have label
    const nicknameLabel = page.locator('label[for="nickname"]')
    await expect(nicknameLabel).toBeVisible()

    // Room code input should have label
    const roomCodeLabel = page.locator('label[for="room-code"]')
    await expect(roomCodeLabel).toBeVisible()
  })

  test('should have appropriate focus styles', async ({ page }) => {
    await navigateToOnlineGame(page)

    const nicknameInput = page.locator('#nickname')
    await nicknameInput.focus()

    // Should have visible focus indicator (border-color change)
    await expect(nicknameInput).toHaveCSS('border-color', 'rgb(255, 215, 0)') // #ffd700
  })

  test('should support keyboard navigation', async ({ page }) => {
    await navigateToOnlineGame(page)

    // Tab through form elements
    await page.keyboard.press('Tab')
    await expect(page.locator('#nickname')).toBeFocused()

    await page.keyboard.press('Tab')
    await expect(page.locator('button:has-text("랜덤 매칭")')).toBeFocused()
  })
})
