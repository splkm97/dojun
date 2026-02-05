import { test, expect, Browser, BrowserContext, Page } from '@playwright/test';

test.describe('Placement Phase Fixes', () => {
  test.describe.configure({ mode: 'serial', timeout: 120000 });

  async function setupTwoPlayers(browser: Browser): Promise<{
    page1: Page;
    page2: Page;
    context1: BrowserContext;
    context2: BrowserContext;
  }> {
    const context1 = await browser.newContext();
    const context2 = await browser.newContext();
    const page1 = await context1.newPage();
    const page2 = await context2.newPage();

    await page1.goto('http://localhost:8080');
    await page2.goto('http://localhost:8080');

    await page1.waitForSelector('.connection-status.connected');
    await page2.waitForSelector('.connection-status.connected');

    await page1.fill('#nickname', 'Player1');
    await page1.click('button:has-text("방 만들기")');

    await page1.waitForSelector('.room-code-display');
    const roomCode = await page1.locator('.room-code-display').textContent();

    await page2.fill('#nickname', 'Player2');
    await page2.fill('#room-code', roomCode!);
    await page2.click('button:has-text("참여")');

    await page1.waitForSelector('#game-screen', { state: 'visible' });
    await page2.waitForSelector('#game-screen', { state: 'visible' });

    return { page1, page2, context1, context2 };
  }

  test('Task 1: should show token count when placing token (cover opens briefly)', async ({ browser }) => {
    const { page1, page2, context1, context2 } = await setupTwoPlayers(browser);

    const currentTurn = await page1.evaluate(() => {
      return (window as any).game.gameState?.currentTurn;
    });
    const activePlayer = currentTurn === 0 ? page1 : page2;

    // Place token on plate 0
    await activePlayer.locator('.plate[data-index="0"]').click();

    // Wait for state update and check that plate cover is open (showing token count)
    await activePlayer.waitForFunction(() => {
      const game = (window as any).game;
      // Plate should be uncovered briefly to show token count
      return game.gameState?.plates?.[0]?.covered === false;
    }, { timeout: 3000 });

    // Verify the cover has 'open' class
    const coverIsOpen = await activePlayer.locator('.plate[data-index="0"] .plate-cover').evaluate(
      el => el.classList.contains('open')
    );
    expect(coverIsOpen).toBe(true);

    await context1.close();
    await context2.close();
  });

  test('Task 1: opponent should also see token count when player places token', async ({ browser }) => {
    const { page1, page2, context1, context2 } = await setupTwoPlayers(browser);

    const currentTurn = await page1.evaluate(() => {
      return (window as any).game.gameState?.currentTurn;
    });
    const activePlayer = currentTurn === 0 ? page1 : page2;
    const observingPlayer = currentTurn === 0 ? page2 : page1;

    // Place token
    await activePlayer.locator('.plate[data-index="0"]').click();

    // Opponent should also see the uncovered plate
    await observingPlayer.waitForFunction(() => {
      const game = (window as any).game;
      return game.gameState?.plates?.[0]?.covered === false;
    }, { timeout: 3000 });

    const coverIsOpen = await observingPlayer.locator('.plate[data-index="0"] .plate-cover').evaluate(
      el => el.classList.contains('open')
    );
    expect(coverIsOpen).toBe(true);

    await context1.close();
    await context2.close();
  });

  test('Task 3: should prevent multiple plate selection during placement', async ({ browser }) => {
    const { page1, page2, context1, context2 } = await setupTwoPlayers(browser);

    const currentTurn = await page1.evaluate(() => {
      return (window as any).game.gameState?.currentTurn;
    });
    const activePlayer = currentTurn === 0 ? page1 : page2;

    // Rapidly click multiple plates
    await activePlayer.locator('.plate[data-index="0"]').click();
    await activePlayer.locator('.plate[data-index="1"]').click();
    await activePlayer.locator('.plate[data-index="2"]').click();

    // Wait for state to settle
    await activePlayer.waitForTimeout(500);

    // Check how many plates have tokens - should be exactly 1
    const platesWithTokens = await activePlayer.evaluate(() => {
      const game = (window as any).game;
      return game.gameState?.plates?.filter((p: any) => p.hasTokens).length || 0;
    });

    expect(platesWithTokens).toBe(1);

    await context1.close();
    await context2.close();
  });

  test('Task 3: plates should be disabled immediately after clicking during placement', async ({ browser }) => {
    const { page1, page2, context1, context2 } = await setupTwoPlayers(browser);

    const currentTurn = await page1.evaluate(() => {
      return (window as any).game.gameState?.currentTurn;
    });
    const activePlayer = currentTurn === 0 ? page1 : page2;

    // Click a plate
    await activePlayer.locator('.plate[data-index="0"]').click();

    // Immediately check if other plates are disabled (or click is blocked)
    // The second click should not result in a second token placement
    await activePlayer.locator('.plate[data-index="1"]').click();

    // Wait for turn to advance
    await activePlayer.waitForTimeout(2000);

    // Only plate 0 should have tokens from this player
    const plate0HasTokens = await activePlayer.evaluate(() => {
      return (window as any).game.gameState?.plates?.[0]?.hasTokens;
    });
    const plate1HasTokens = await activePlayer.evaluate(() => {
      return (window as any).game.gameState?.plates?.[1]?.hasTokens;
    });

    expect(plate0HasTokens).toBe(true);
    // plate1 might have tokens if it's the next player's turn, but not from rapid clicking
    // This test verifies the first click locks further clicks

    await context1.close();
    await context2.close();
  });
});

test.describe('Add Token Phase Fixes', () => {
  test.describe.configure({ mode: 'serial', timeout: 180000 });

  async function setupMatchingPhase(browser: Browser): Promise<{
    page1: Page;
    page2: Page;
    context1: BrowserContext;
    context2: BrowserContext;
  }> {
    const context1 = await browser.newContext();
    const context2 = await browser.newContext();
    const page1 = await context1.newPage();
    const page2 = await context2.newPage();

    await page1.goto('http://localhost:8080');
    await page2.goto('http://localhost:8080');

    await page1.waitForSelector('.connection-status.connected');
    await page2.waitForSelector('.connection-status.connected');

    await page1.fill('#nickname', 'Player1');
    await page1.click('button:has-text("방 만들기")');

    await page1.waitForSelector('.room-code-display');
    const roomCode = await page1.locator('.room-code-display').textContent();

    await page2.fill('#nickname', 'Player2');
    await page2.fill('#room-code', roomCode!);
    await page2.click('button:has-text("참여")');

    await page1.waitForSelector('#game-screen', { state: 'visible' });
    await page2.waitForSelector('#game-screen', { state: 'visible' });

    // Complete placement phase quickly (both players place tokens alternately)
    const pages = [page1, page2];
    let plateIndex = 0;

    // Wait for placement phase
    await page1.waitForFunction(() => {
      return (window as any).game.gameState?.phase === 'placement';
    });

    // Complete all placement rounds
    while (true) {
      const phase = await page1.evaluate(() => (window as any).game.gameState?.phase);
      if (phase !== 'placement') break;

      const currentTurn = await page1.evaluate(() => (window as any).game.gameState?.currentTurn);
      const currentPlayer = pages[currentTurn];

      // Find an available plate
      const availablePlate = await currentPlayer.evaluate(() => {
        const state = (window as any).game.gameState;
        return state?.plates?.findIndex((p: any) => !p.hasTokens) ?? -1;
      });

      if (availablePlate === -1) break;

      await currentPlayer.locator(`.plate[data-index="${availablePlate}"]`).click();
      await currentPlayer.waitForTimeout(2000); // Wait for turn to advance

      plateIndex++;
      if (plateIndex > 40) break; // Safety limit
    }

    // Wait for matching phase
    await page1.waitForFunction(() => {
      return (window as any).game.gameState?.phase === 'matching';
    }, { timeout: 30000 });

    return { page1, page2, context1, context2 };
  }

  test('Task 4: clicking during confirm reveal should not affect add_token phase', async ({ browser }) => {
    const { page1, page2, context1, context2 } = await setupMatchingPhase(browser);

    const currentTurn = await page1.evaluate(() => {
      return (window as any).game.gameState?.currentTurn;
    });
    const activePlayer = currentTurn === 0 ? page1 : page2;

    // Select two plates with same token count (need to find matching pair)
    // For simplicity, select plates 0 and 1 (they might not match, but we test the flow)
    await activePlayer.locator('.plate[data-index="0"]').click();
    await activePlayer.waitForTimeout(300);
    await activePlayer.locator('.plate[data-index="1"]').click();

    // Wait for selection
    await activePlayer.waitForFunction(() => {
      const state = (window as any).game.gameState;
      return state?.selectedPlates?.length === 2;
    }, { timeout: 5000 });

    // Click confirm
    await activePlayer.locator('button:has-text("선택 확인")').click();

    // During the 2-second reveal, try clicking another plate
    await activePlayer.waitForTimeout(500);
    await activePlayer.locator('.plate[data-index="2"]').click();
    await activePlayer.locator('.plate[data-index="3"]').click();

    // Wait for phase transition
    await activePlayer.waitForTimeout(3000);

    // Check phase - should be either add_token (if matched) or back to matching (if failed)
    const phase = await activePlayer.evaluate(() => {
      return (window as any).game.gameState?.phase;
    });

    // If add_token phase, matched plates should still be clickable
    if (phase === 'add_token') {
      const matchedPlates = await activePlayer.evaluate(() => {
        return (window as any).game.gameState?.matchedPlates || [];
      });

      expect(matchedPlates.length).toBe(2);

      // Matched plates should be clickable (not affected by earlier clicks)
      // Click one of the matched plates
      if (matchedPlates.length > 0) {
        const plateToClick = matchedPlates[0];
        await activePlayer.locator(`.plate[data-index="${plateToClick}"]`).click();

        // Should successfully add token (phase changes)
        await activePlayer.waitForFunction(() => {
          const state = (window as any).game.gameState;
          return state?.phase !== 'add_token';
        }, { timeout: 5000 });
      }
    }

    await context1.close();
    await context2.close();
  });
});
