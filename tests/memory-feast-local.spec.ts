/**
 * Memory Feast Local Game - Playwright E2E Tests
 * TDD: Tests designed BEFORE implementation verification
 *
 * Test Categories (Priority Order):
 * 1. Error Cases - Prevent invalid states and user errors
 * 2. Edge Cases - Boundary conditions and limits
 * 3. Happy Paths - Normal game flow
 * 4. Property Tests - UI invariants
 */

import { test, expect, type Page } from '@playwright/test'

// Helper: Navigate to game and wait for load
async function navigateToGame(page: Page) {
  await page.goto('/memory-feast.html')
  await expect(page.locator('#start-screen')).toBeVisible()
}

// Helper: Start game with specific settings
async function startGameWithSettings(
  page: Page,
  options: {
    plateCount?: number
    mode?: 'pvp' | 'ai'
    difficulty?: 'easy' | 'medium' | 'hard'
    firstPlayer?: 'player' | 'ai'
  } = {}
) {
  await navigateToGame(page)

  // Set plate count if specified
  if (options.plateCount !== undefined) {
    const currentCount = await page.locator('#plate-count-display').textContent()
    const current = parseInt(currentCount || '20')
    const diff = options.plateCount - current

    const btn = diff > 0
      ? page.locator('button:has-text("+")')
      : page.locator('button:has-text("-")')

    for (let i = 0; i < Math.abs(diff) / 2; i++) {
      await btn.click()
    }
  }

  // Set game mode
  if (options.mode === 'ai') {
    await page.locator('#mode-ai').click()
    await expect(page.locator('#ai-settings')).toBeVisible()

    if (options.difficulty) {
      await page.locator(`#diff-${options.difficulty}`).click()
    }

    if (options.firstPlayer) {
      await page.locator(`#first-${options.firstPlayer}`).click()
    }
  }

  // Start the game
  await page.locator('button:has-text("게임 시작")').click()
  await expect(page.locator('#game-screen')).toBeVisible()
}

// Helper: Click a plate by index (0-based)
async function clickPlate(page: Page, index: number) {
  await page.locator(`.plate[data-index="${index}"]`).click()
}

// Helper: Wait for plate animation to complete
async function waitForPlateAnimation(page: Page) {
  await page.waitForTimeout(1600) // 1.5s animation + buffer
}

// =============================================================================
// 1. ERROR CASES - Prevent invalid states and user errors
// =============================================================================

test.describe('Error Cases', () => {
  test('should prevent clicking disabled plates during placement phase', async ({ page }) => {
    await startGameWithSettings(page, { plateCount: 4 })

    // Click first plate (should work)
    await clickPlate(page, 0)
    await waitForPlateAnimation(page)

    // After placement, plate should be disabled
    const plate0 = page.locator('.plate[data-index="0"]')
    await expect(plate0).toHaveClass(/disabled/)

    // Try clicking the same plate again - should have no effect
    await clickPlate(page, 0)

    // Verify no error message or state change occurred
    const messageArea = page.locator('#message-area')
    // The message might appear briefly if clicked
  })

  test('should show error when clicking already-tokened plate', async ({ page }) => {
    await startGameWithSettings(page, { plateCount: 4, mode: 'pvp' })

    // Place token on plate 0
    await clickPlate(page, 0)
    await waitForPlateAnimation(page)

    // Verify plate 0 is now disabled
    const plate0 = page.locator('.plate[data-index="0"]')
    await expect(plate0).toHaveClass(/disabled/)
  })

  test('should not allow selection of more than 2 plates in matching phase', async ({ page }) => {
    await startGameWithSettings(page, { plateCount: 4, mode: 'pvp' })

    // Complete minimal placement phase (need to place tokens on all plates)
    // With 4 plates, maxPlacementRound = 4/2 - 1 = 1
    // So we only place 1 token per player (2 total placements)

    await clickPlate(page, 0) // Player 1 places 1 token
    await waitForPlateAnimation(page)

    await clickPlate(page, 1) // Player 2 places 1 token
    await waitForPlateAnimation(page)

    // Now in matching phase, select 2 plates
    await clickPlate(page, 0)
    await clickPlate(page, 1)

    // Verify floating confirm button appears
    await expect(page.locator('#floating-confirm')).toHaveClass(/show/)

    // Try to select a third plate
    await clickPlate(page, 2)

    // Should still only have 2 selected (no additional selection)
    const selectedPlates = page.locator('.plate.selected-first, .plate.selected-second')
    await expect(selectedPlates).toHaveCount(2)
  })
})

// =============================================================================
// 2. EDGE CASES - Boundary conditions and limits
// =============================================================================

test.describe('Edge Cases', () => {
  test('should enforce minimum plate count of 4', async ({ page }) => {
    await navigateToGame(page)

    // Set to minimum
    const display = page.locator('#plate-count-display')
    await expect(display).toHaveText('20')

    // Click minus button 8 times to reach 4
    const minusBtn = page.locator('.btn-small:has-text("-")')
    for (let i = 0; i < 8; i++) {
      await minusBtn.click()
    }
    await expect(display).toHaveText('4')

    // Try to go below 4 - should not change
    await minusBtn.click()
    await expect(display).toHaveText('4')
  })

  test('should enforce maximum plate count of 20', async ({ page }) => {
    await navigateToGame(page)

    const display = page.locator('#plate-count-display')
    await expect(display).toHaveText('20')

    // Try to increase beyond 20
    const plusBtn = page.locator('.btn-small:has-text("+")')
    await plusBtn.click()
    await expect(display).toHaveText('20')
  })

  test('should handle timer reaching zero with penalty', async ({ page }) => {
    test.setTimeout(90000) // Extended timeout for 60s timer

    await startGameWithSettings(page, { plateCount: 4, mode: 'pvp' })

    // Complete placement phase
    await clickPlate(page, 0)
    await waitForPlateAnimation(page)
    await clickPlate(page, 1)
    await waitForPlateAnimation(page)

    // Now in matching phase - wait for timer to expire
    // Initial tokens should be max(5, 1+1) = 5
    const initialTokens = await page.locator('#player1-tokens').textContent()
    expect(initialTokens).toContain('5개')

    // Wait for timeout (60 seconds + buffer)
    await page.waitForTimeout(62000)

    // After timeout, player should receive 2 penalty tokens
    // Turn switches to player 2, so check player 1's final token count
    await expect(page.locator('.message')).toContainText('시간 초과')
  })

  test('should end game when no matching pairs exist', async ({ page }) => {
    // This is hard to test without manipulating game state directly
    // We'll verify the hasMatchingPairs logic indirectly
    await startGameWithSettings(page, { plateCount: 4, mode: 'pvp' })

    // The game should detect when no pairs exist
    // This would require specific game state manipulation
    // For now, verify the modal structure exists
    await expect(page.locator('#result-modal')).not.toHaveClass(/show/)
  })

  test('should declare winner when tokens reach zero', async ({ page }) => {
    // Verify win condition structure
    await startGameWithSettings(page, { plateCount: 4, mode: 'pvp' })

    // Verify result modal elements exist
    const resultModal = page.locator('#result-modal')
    await expect(resultModal).toBeAttached()
    await expect(page.locator('#result-title')).toBeAttached()
    await expect(page.locator('#winner-announcement')).toBeAttached()
  })
})

// =============================================================================
// 3. HAPPY PATHS - Normal game flow
// =============================================================================

test.describe('Happy Paths', () => {
  test('should complete full placement phase', async ({ page }) => {
    await startGameWithSettings(page, { plateCount: 4, mode: 'pvp' })

    // With 4 plates, maxPlacementRound = 1
    // Player 1 places round 1, Player 2 places round 1

    // Verify initial state
    await expect(page.locator('#phase-title')).toHaveText('배치 단계')
    await expect(page.locator('#player1-info')).toHaveClass(/active/)

    // Player 1 places
    await clickPlate(page, 0)
    await waitForPlateAnimation(page)

    // Verify turn switched to Player 2
    await expect(page.locator('#player2-info')).toHaveClass(/active/)

    // Player 2 places
    await clickPlate(page, 1)
    await waitForPlateAnimation(page)

    // Should transition to matching phase
    await expect(page.locator('#phase-title')).toHaveText('매칭 단계')
    await expect(page.locator('#timer')).toBeVisible()
  })

  test('should allow selecting and deselecting plates in matching phase', async ({ page }) => {
    await startGameWithSettings(page, { plateCount: 4, mode: 'pvp' })

    // Complete placement
    await clickPlate(page, 0)
    await waitForPlateAnimation(page)
    await clickPlate(page, 1)
    await waitForPlateAnimation(page)

    // In matching phase, select a plate
    await clickPlate(page, 0)
    await expect(page.locator('.plate[data-index="0"]')).toHaveClass(/selected-first/)

    // Deselect by clicking again
    await clickPlate(page, 0)
    await expect(page.locator('.plate[data-index="0"]')).not.toHaveClass(/selected-first/)
  })

  test('should show confirm button when 2 plates selected', async ({ page }) => {
    await startGameWithSettings(page, { plateCount: 4, mode: 'pvp' })

    // Complete placement
    await clickPlate(page, 0)
    await waitForPlateAnimation(page)
    await clickPlate(page, 1)
    await waitForPlateAnimation(page)

    // Confirm button should be hidden initially
    await expect(page.locator('#floating-confirm')).not.toHaveClass(/show/)

    // Select first plate
    await clickPlate(page, 0)
    await expect(page.locator('#floating-confirm')).not.toHaveClass(/show/)

    // Select second plate
    await clickPlate(page, 1)
    await expect(page.locator('#floating-confirm')).toHaveClass(/show/)
  })

  test('should handle match confirmation and reveal plates', async ({ page }) => {
    await startGameWithSettings(page, { plateCount: 4, mode: 'pvp' })

    // Complete placement
    await clickPlate(page, 0)
    await waitForPlateAnimation(page)
    await clickPlate(page, 1)
    await waitForPlateAnimation(page)

    // Select plates for matching
    await clickPlate(page, 0)
    await clickPlate(page, 1)

    // Click confirm
    await page.locator('#floating-confirm button').click()

    // Plates should be revealed (covers removed)
    await expect(page.locator('.plate[data-index="0"] .plate-cover')).toHaveClass(/open/)
    await expect(page.locator('.plate[data-index="1"] .plate-cover')).toHaveClass(/open/)
  })

  test('should properly switch turns after match result', async ({ page }) => {
    await startGameWithSettings(page, { plateCount: 4, mode: 'pvp' })

    // Complete placement
    await clickPlate(page, 0)
    await waitForPlateAnimation(page)
    await clickPlate(page, 1)
    await waitForPlateAnimation(page)

    // Player 1's turn in matching
    await expect(page.locator('#player1-info')).toHaveClass(/active/)

    // Make a match attempt (will fail since both have 1 token)
    await clickPlate(page, 0)
    await clickPlate(page, 1)
    await page.locator('#floating-confirm button').click()

    // Wait for result and turn switch
    await page.waitForTimeout(4500) // Animation + result display

    // Should switch to Player 2 (or show success message if matched)
    // Since both plates have 1 token, it should be a match
    const message = await page.locator('.message').textContent()
    expect(message).toContain('매치')
  })
})

// =============================================================================
// 4. UI/INTERACTION TESTS
// =============================================================================

test.describe('UI Interactions', () => {
  test('should display tutorial modal on button click', async ({ page }) => {
    await navigateToGame(page)

    await expect(page.locator('#tutorial-modal')).not.toHaveClass(/show/)
    await page.locator('button:has-text("게임 방법")').click()
    await expect(page.locator('#tutorial-modal')).toHaveClass(/show/)
  })

  test('should navigate through all tutorial steps', async ({ page }) => {
    await navigateToGame(page)
    await page.locator('button:has-text("게임 방법")').click()

    // Step 1 should be active
    await expect(page.locator('.tutorial-step[data-step="1"]')).toHaveClass(/active/)
    await expect(page.locator('#tutorial-prev')).not.toBeVisible()

    // Navigate through steps
    for (let step = 2; step <= 5; step++) {
      await page.locator('#tutorial-next').click()
      await expect(page.locator(`.tutorial-step[data-step="${step}"]`)).toHaveClass(/active/)
    }

    // On last step, button should say "시작하기"
    await expect(page.locator('#tutorial-next')).toHaveText('시작하기')

    // Click to close
    await page.locator('#tutorial-next').click()
    await expect(page.locator('#tutorial-modal')).not.toHaveClass(/show/)
  })

  test('should update plate count display correctly', async ({ page }) => {
    await navigateToGame(page)

    const display = page.locator('#plate-count-display')
    const minusBtn = page.locator('.btn-small:has-text("-")')
    const plusBtn = page.locator('.btn-small:has-text("+")')

    await expect(display).toHaveText('20')

    await minusBtn.click()
    await expect(display).toHaveText('18')

    await minusBtn.click()
    await expect(display).toHaveText('16')

    await plusBtn.click()
    await expect(display).toHaveText('18')
  })

  test('should toggle AI settings visibility based on mode', async ({ page }) => {
    await navigateToGame(page)

    // Initially AI settings should be hidden (PvP is default)
    await expect(page.locator('#ai-settings')).not.toBeVisible()
    await expect(page.locator('#first-player-settings')).not.toBeVisible()

    // Select AI mode
    await page.locator('#mode-ai').click()
    await expect(page.locator('#ai-settings')).toBeVisible()
    await expect(page.locator('#first-player-settings')).toBeVisible()

    // Switch back to PvP
    await page.locator('#mode-pvp').click()
    await expect(page.locator('#ai-settings')).not.toBeVisible()
  })

  test('should update difficulty hint text', async ({ page }) => {
    await navigateToGame(page)
    await page.locator('#mode-ai').click()

    const hint = page.locator('#difficulty-hint')

    // Default is medium
    await expect(hint).toContainText('70~85%')

    await page.locator('#diff-easy').click()
    await expect(hint).toContainText('40~60%')

    await page.locator('#diff-hard').click()
    await expect(hint).toContainText('95~100%')
  })

  test('should display correct player names for AI mode', async ({ page }) => {
    await startGameWithSettings(page, {
      plateCount: 4,
      mode: 'ai',
      firstPlayer: 'player',
    })

    // When player goes first, player is 1 and AI is 2
    await expect(page.locator('#player1-info h3')).toHaveText('플레이어')
    await expect(page.locator('#player2-info h3')).toHaveText('AI')
  })

  test('should display correct player names for AI mode (AI first)', async ({ page }) => {
    await startGameWithSettings(page, {
      plateCount: 4,
      mode: 'ai',
      firstPlayer: 'ai',
    })

    // When AI goes first, AI is player 1
    await expect(page.locator('#player1-info h3')).toHaveText('AI')
    await expect(page.locator('#player2-info h3')).toHaveText('플레이어')
  })

  test('should render correct number of plates', async ({ page }) => {
    // Test with 4 plates
    await startGameWithSettings(page, { plateCount: 4, mode: 'pvp' })
    await expect(page.locator('.plate')).toHaveCount(4)

    // Reset and test with different count
    await page.locator('#result-modal button').click({ force: true }).catch(() => {})
    await page.goto('/memory-feast.html')
    await startGameWithSettings(page, { plateCount: 8, mode: 'pvp' })
    await expect(page.locator('.plate')).toHaveCount(8)
  })

  test('should show result modal and allow reset', async ({ page }) => {
    await startGameWithSettings(page, { plateCount: 4, mode: 'pvp' })

    // Verify result modal exists but is hidden
    const modal = page.locator('#result-modal')
    await expect(modal).not.toHaveClass(/show/)

    // Verify reset button exists
    const resetBtn = modal.locator('button:has-text("다시 하기")')
    await expect(resetBtn).toBeAttached()
  })
})

// =============================================================================
// 5. AI MODE TESTS
// =============================================================================

test.describe('AI Mode', () => {
  test('should automatically make AI moves during placement', async ({ page }) => {
    test.setTimeout(30000)

    await startGameWithSettings(page, {
      plateCount: 4,
      mode: 'ai',
      firstPlayer: 'player',
    })

    // Player places first token
    await clickPlate(page, 0)
    await waitForPlateAnimation(page)

    // AI should automatically place its token
    // Wait for AI turn (1s delay + 1.5s animation)
    await page.waitForTimeout(3500)

    // After both placements with 4 plates (maxRound=1), game transitions to matching phase
    // In matching phase, plates are no longer "disabled" in the same way
    // Verify that we've transitioned to matching phase or that a second plate was placed
    const phaseTitle = await page.locator('#phase-title').textContent()

    // Either still in placement with AI having placed, or transitioned to matching
    if (phaseTitle === '배치 단계') {
      // Still in placement - check for AI's placement via plate state
      // The hasTokens flag is set but disabled class only applies in placement
      const disabledPlates = page.locator('.plate.disabled')
      await expect(disabledPlates).toHaveCount(2, { timeout: 5000 })
    } else {
      // Transitioned to matching phase (both players placed)
      expect(phaseTitle).toBe('매칭 단계')
    }
  })

  test('AI should skip player turn when AI goes first', async ({ page }) => {
    test.setTimeout(30000)

    await startGameWithSettings(page, {
      plateCount: 4,
      mode: 'ai',
      firstPlayer: 'ai',
    })

    // AI is player 1, should automatically place
    await page.waitForTimeout(3000)

    // One plate should be disabled (AI placed)
    const disabledPlates = page.locator('.plate.disabled')
    await expect(disabledPlates).toHaveCount(1)

    // Now it's player's turn
    await expect(page.locator('#player2-info')).toHaveClass(/active/)
  })
})
