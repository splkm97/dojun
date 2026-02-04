import { test, expect, Page, BrowserContext } from '@playwright/test';

/**
 * Opponent Selection Visibility Tests
 *
 * Feature: Both players should see each other's plate selections during matching phase
 * - Player A selects plate → Player B sees it highlighted (different color)
 * - Player B selects plate → Player A sees it highlighted (different color)
 */

const BASE_URL = 'http://localhost:8080';

// Helper to connect to WebSocket
async function connectWebSocket(page: Page): Promise<void> {
  await page.goto(BASE_URL);
  await expect(page.locator('#lobby-screen')).toBeVisible({ timeout: 10000 });
  await page.waitForFunction(() => {
    const game = (window as any).game;
    return game && game.ws && game.ws.readyState === WebSocket.OPEN;
  }, { timeout: 5000 });
}

// Helper to setup two players and reach matching phase
async function setupMatchingPhase(browser: any): Promise<{
  page1: Page;
  page2: Page;
  context1: BrowserContext;
  context2: BrowserContext;
}> {
  const context1 = await browser.newContext();
  const context2 = await browser.newContext();
  const page1 = await context1.newPage();
  const page2 = await context2.newPage();

  await connectWebSocket(page1);
  await connectWebSocket(page2);

  // Player 1 creates room
  await page1.locator('#nickname').fill('Player1');
  await page1.getByRole('button', { name: '방 만들기' }).click();
  await expect(page1.locator('#waiting-screen')).toBeVisible({ timeout: 5000 });

  // Get room code
  const roomCodeText = await page1.locator('#room-code-display').textContent();
  const roomCode = roomCodeText?.replace('초대 코드: ', '').trim() || '';

  // Player 2 joins
  await page2.locator('#nickname').fill('Player2');
  await page2.locator('#room-code').fill(roomCode);
  await page2.getByRole('button', { name: '참여' }).click();

  // Wait for game to start
  await expect(page1.locator('#game-screen')).toBeVisible({ timeout: 10000 });
  await expect(page2.locator('#game-screen')).toBeVisible({ timeout: 10000 });

  // Wait for placement phase
  await expect(page1.locator('.phase-info')).toContainText('배치');

  // Complete placement phase (simplified - just need to reach matching)
  // For this test, we'll use a smaller plate count or skip placement
  // Actually, we need to complete placement to reach matching phase

  // Get max rounds and complete placement
  const maxRound = await page1.evaluate(() => {
    const game = (window as any).game;
    return game.gameState?.maxRound || 9;
  });

  // Alternate placing tokens between players
  for (let round = 1; round <= maxRound; round++) {
    // Determine current player
    const currentTurn = await page1.evaluate(() => {
      return (window as any).game.gameState?.currentTurn;
    });

    const currentPlayer = currentTurn === 0 ? page1 : page2;

    // Find an unoccupied plate and click it
    const plateIndex = (round - 1) * 2;
    await currentPlayer.locator(`.plate[data-index="${plateIndex}"]`).click();
    await currentPlayer.waitForTimeout(2000); // Wait for animation

    // Second placement of the round
    const currentTurn2 = await page1.evaluate(() => {
      return (window as any).game.gameState?.currentTurn;
    });
    const currentPlayer2 = currentTurn2 === 0 ? page1 : page2;

    const plateIndex2 = (round - 1) * 2 + 1;
    await currentPlayer2.locator(`.plate[data-index="${plateIndex2}"]`).click();
    await currentPlayer2.waitForTimeout(2000); // Wait for animation
  }

  // Should now be in matching phase
  await expect(page1.locator('.phase-info')).toContainText('매칭', { timeout: 10000 });

  return { page1, page2, context1, context2 };
}

test.describe('Opponent Selection Visibility', () => {
  test.describe.configure({ mode: 'serial', timeout: 120000 });

  test('should show current player selection highlighted with primary color', async ({ browser }) => {
    const { page1, page2, context1, context2 } = await setupMatchingPhase(browser);

    // Determine whose turn it is
    const currentTurn = await page1.evaluate(() => {
      return (window as any).game.gameState?.currentTurn;
    });
    const activePlayer = currentTurn === 0 ? page1 : page2;
    const playerIndex = currentTurn;

    // Active player selects a plate
    await activePlayer.locator('.plate[data-index="0"]').click();
    await activePlayer.waitForTimeout(500);

    // Verify the plate has 'selected-first' class on the active player's screen
    const hasSelectedClass = await activePlayer.locator('.plate[data-index="0"]').evaluate(
      el => el.classList.contains('selected-first')
    );
    expect(hasSelectedClass).toBe(true);

    await context1.close();
    await context2.close();
  });

  test('should show opponent selection highlighted with different color', async ({ browser }) => {
    const { page1, page2, context1, context2 } = await setupMatchingPhase(browser);

    // Determine whose turn it is
    const currentTurn = await page1.evaluate(() => {
      return (window as any).game.gameState?.currentTurn;
    });
    const activePlayer = currentTurn === 0 ? page1 : page2;
    const observingPlayer = currentTurn === 0 ? page2 : page1;

    // Active player selects a plate
    await activePlayer.locator('.plate[data-index="0"]').click();

    // Wait for the opponent to receive the game state with opponentSelectedPlates
    await observingPlayer.waitForFunction(() => {
      const game = (window as any).game;
      return game.gameState?.opponentSelectedPlates && game.gameState.opponentSelectedPlates.length > 0;
    }, { timeout: 5000 });

    // Verify the opponent can see the selection with 'opponent-selected' class
    const hasOpponentSelectedClass = await observingPlayer.locator('.plate[data-index="0"]').evaluate(
      el => el.classList.contains('opponent-selected') || el.classList.contains('opponent-selected-first')
    );
    expect(hasOpponentSelectedClass).toBe(true);

    await context1.close();
    await context2.close();
  });

  test('should show both selections when player selects two plates', async ({ browser }) => {
    const { page1, page2, context1, context2 } = await setupMatchingPhase(browser);

    const currentTurn = await page1.evaluate(() => {
      return (window as any).game.gameState?.currentTurn;
    });
    const activePlayer = currentTurn === 0 ? page1 : page2;
    const observingPlayer = currentTurn === 0 ? page2 : page1;

    // Active player selects two plates
    await activePlayer.locator('.plate[data-index="0"]').click();
    await observingPlayer.waitForFunction(() => {
      const game = (window as any).game;
      return game.gameState?.opponentSelectedPlates?.length >= 1;
    }, { timeout: 5000 });

    await activePlayer.locator('.plate[data-index="2"]').click();
    await observingPlayer.waitForFunction(() => {
      const game = (window as any).game;
      return game.gameState?.opponentSelectedPlates?.length >= 2;
    }, { timeout: 5000 });

    // Verify observing player sees both selections
    const hasFirst = await observingPlayer.locator('.plate[data-index="0"]').evaluate(
      el => el.classList.contains('opponent-selected') || el.classList.contains('opponent-selected-first')
    );
    const hasSecond = await observingPlayer.locator('.plate[data-index="2"]').evaluate(
      el => el.classList.contains('opponent-selected') || el.classList.contains('opponent-selected-second')
    );
    expect(hasFirst).toBe(true);
    expect(hasSecond).toBe(true);

    await context1.close();
    await context2.close();
  });

  test('should remove opponent highlight when plate is deselected', async ({ browser }) => {
    const { page1, page2, context1, context2 } = await setupMatchingPhase(browser);

    const currentTurn = await page1.evaluate(() => {
      return (window as any).game.gameState?.currentTurn;
    });
    const activePlayer = currentTurn === 0 ? page1 : page2;
    const observingPlayer = currentTurn === 0 ? page2 : page1;

    // Active player selects a plate
    await activePlayer.locator('.plate[data-index="0"]').click();
    await observingPlayer.waitForFunction(() => {
      const game = (window as any).game;
      return game.gameState?.opponentSelectedPlates?.length >= 1;
    }, { timeout: 5000 });

    // Verify opponent sees selection
    let hasOpponentSelected = await observingPlayer.locator('.plate[data-index="0"]').evaluate(
      el => el.classList.contains('opponent-selected') || el.classList.contains('opponent-selected-first')
    );
    expect(hasOpponentSelected).toBe(true);

    // Active player deselects the plate (click again)
    await activePlayer.locator('.plate[data-index="0"]').click();
    await observingPlayer.waitForFunction(() => {
      const game = (window as any).game;
      return !game.gameState?.opponentSelectedPlates || game.gameState.opponentSelectedPlates.length === 0;
    }, { timeout: 5000 });

    // Verify opponent no longer sees selection
    hasOpponentSelected = await observingPlayer.locator('.plate[data-index="0"]').evaluate(
      el => el.classList.contains('opponent-selected') || el.classList.contains('opponent-selected-first')
    );
    expect(hasOpponentSelected).toBe(false);

    await context1.close();
    await context2.close();
  });

  test('should receive opponentSelectedPlates in game state', async ({ browser }) => {
    const { page1, page2, context1, context2 } = await setupMatchingPhase(browser);

    const currentTurn = await page1.evaluate(() => {
      return (window as any).game.gameState?.currentTurn;
    });
    const activePlayer = currentTurn === 0 ? page1 : page2;
    const observingPlayer = currentTurn === 0 ? page2 : page1;

    // Active player selects a plate
    await activePlayer.locator('.plate[data-index="0"]').click();

    // Wait for the state to be received
    await observingPlayer.waitForFunction(() => {
      const game = (window as any).game;
      return game.gameState?.opponentSelectedPlates?.length >= 1;
    }, { timeout: 5000 });

    // Check that observing player's game state has opponentSelectedPlates
    const opponentSelections = await observingPlayer.evaluate(() => {
      const game = (window as any).game;
      return game.gameState?.opponentSelectedPlates || [];
    });

    expect(opponentSelections).toContain(0);

    await context1.close();
    await context2.close();
  });
});
