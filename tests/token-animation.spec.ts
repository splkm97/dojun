import { test, expect, Browser, BrowserContext, Page } from '@playwright/test';

test.describe('Token Placement/Addition Animation', () => {
  test.describe.configure({ mode: 'serial', timeout: 120000 });

  // Helper to create two player sessions and reach placement phase
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

    // Wait for connection
    await page1.waitForSelector('.connection-status.connected');
    await page2.waitForSelector('.connection-status.connected');

    // Player 1 creates room
    await page1.fill('#nickname', 'Player1');
    await page1.click('button:has-text("방 만들기")');

    // Wait for room code
    await page1.waitForSelector('.room-code-display');
    const roomCode = await page1.locator('.room-code-display').textContent();

    // Player 2 joins
    await page2.fill('#nickname', 'Player2');
    await page2.fill('#room-code', roomCode!);
    await page2.click('button:has-text("참여")');

    // Wait for game to start (placement phase)
    await page1.waitForSelector('#game-screen', { state: 'visible' });
    await page2.waitForSelector('#game-screen', { state: 'visible' });

    return { page1, page2, context1, context2 };
  }

  test('should show animation on plate when current player places token', async ({ browser }) => {
    const { page1, page2, context1, context2 } = await setupTwoPlayers(browser);

    // Determine whose turn it is
    const currentTurn = await page1.evaluate(() => {
      return (window as any).game.gameState?.currentTurn;
    });
    const activePlayer = currentTurn === 0 ? page1 : page2;

    // Click on a plate to place token
    await activePlayer.locator('.plate[data-index="0"]').click();

    // Wait for state to update with lastActionPlate
    await activePlayer.waitForFunction(() => {
      const game = (window as any).game;
      return game.gameState?.lastActionPlate === 0;
    }, { timeout: 5000 });

    // Verify the plate has animation class
    const hasAnimationClass = await activePlayer.locator('.plate[data-index="0"]').evaluate(
      el => el.classList.contains('token-placed') || el.classList.contains('just-placed')
    );
    expect(hasAnimationClass).toBe(true);

    await context1.close();
    await context2.close();
  });

  test('should show animation on plate when opponent places token', async ({ browser }) => {
    const { page1, page2, context1, context2 } = await setupTwoPlayers(browser);

    const currentTurn = await page1.evaluate(() => {
      return (window as any).game.gameState?.currentTurn;
    });
    const activePlayer = currentTurn === 0 ? page1 : page2;
    const observingPlayer = currentTurn === 0 ? page2 : page1;

    // Active player places token
    await activePlayer.locator('.plate[data-index="0"]').click();

    // Wait for state to propagate to opponent
    await observingPlayer.waitForFunction(() => {
      const game = (window as any).game;
      return game.gameState?.lastActionPlate === 0;
    }, { timeout: 5000 });

    // Verify opponent sees the animation on the same plate
    const hasAnimationClass = await observingPlayer.locator('.plate[data-index="0"]').evaluate(
      el => el.classList.contains('token-placed') || el.classList.contains('just-placed')
    );
    expect(hasAnimationClass).toBe(true);

    await context1.close();
    await context2.close();
  });

  test('should receive lastActionPlate in game state after placement', async ({ browser }) => {
    const { page1, page2, context1, context2 } = await setupTwoPlayers(browser);

    const currentTurn = await page1.evaluate(() => {
      return (window as any).game.gameState?.currentTurn;
    });
    const activePlayer = currentTurn === 0 ? page1 : page2;

    // Place token on plate 2
    await activePlayer.locator('.plate[data-index="2"]').click();

    // Wait and verify lastActionPlate is set
    await activePlayer.waitForFunction(() => {
      const game = (window as any).game;
      return game.gameState?.lastActionPlate === 2;
    }, { timeout: 5000 });

    const lastActionPlate = await activePlayer.evaluate(() => {
      return (window as any).game.gameState?.lastActionPlate;
    });
    expect(lastActionPlate).toBe(2);

    await context1.close();
    await context2.close();
  });

  test('should update animation to new plate on subsequent placement', async ({ browser }) => {
    const { page1, page2, context1, context2 } = await setupTwoPlayers(browser);

    const currentTurn = await page1.evaluate(() => {
      return (window as any).game.gameState?.currentTurn;
    });
    const activePlayer = currentTurn === 0 ? page1 : page2;
    const nextPlayer = currentTurn === 0 ? page2 : page1;

    // First player places token on plate 0
    await activePlayer.locator('.plate[data-index="0"]').click();

    // Wait for state to update
    await activePlayer.waitForFunction(() => {
      const game = (window as any).game;
      return game.gameState?.lastActionPlate === 0;
    }, { timeout: 5000 });

    // Wait for turn to advance (1.5s + buffer)
    await activePlayer.waitForTimeout(2000);

    // Next player places on plate 1
    await nextPlayer.locator('.plate[data-index="1"]').click();

    // Wait for state to update with new lastActionPlate
    await nextPlayer.waitForFunction(() => {
      const game = (window as any).game;
      return game.gameState?.lastActionPlate === 1;
    }, { timeout: 5000 });

    // Verify plate 1 now has animation, not plate 0
    const plate1HasAnimation = await nextPlayer.locator('.plate[data-index="1"]').evaluate(
      el => el.classList.contains('just-placed')
    );
    expect(plate1HasAnimation).toBe(true);

    await context1.close();
    await context2.close();
  });

  test('should have visible pulse animation effect on placed plate', async ({ browser }) => {
    const { page1, page2, context1, context2 } = await setupTwoPlayers(browser);

    const currentTurn = await page1.evaluate(() => {
      return (window as any).game.gameState?.currentTurn;
    });
    const activePlayer = currentTurn === 0 ? page1 : page2;

    // Place token
    await activePlayer.locator('.plate[data-index="0"]').click();

    // Check that the plate cover has enhanced visual styling (box-shadow animation)
    const coverStyles = await activePlayer.locator('.plate[data-index="0"] .plate-cover').evaluate(el => {
      const computed = window.getComputedStyle(el);
      return {
        boxShadow: computed.boxShadow,
        borderColor: computed.borderColor
      };
    });

    // Should have a glowing effect (box-shadow should not be 'none')
    expect(coverStyles.boxShadow).not.toBe('none');

    await context1.close();
    await context2.close();
  });
});
