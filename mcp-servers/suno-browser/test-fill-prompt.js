#!/usr/bin/env node

/**
 * 独立测试：打开 Suno、填写提示词、点击 Create 按钮
 * 用法：npm run test:fill-prompt
 * 前置：需已登录 Suno（或先运行 suno_login 手动登录一次）
 */

import { chromium } from 'playwright';
import path from 'path';
import { fileURLToPath } from 'url';
import { getPlaylistSongIds, waitForNewSongs, getShareLinks } from './lib/playlist.js';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const USER_DATA_DIR = path.join(__dirname, '.chrome-data');
const TEST_PROMPT = '一首适合冥想的电音歌曲，节奏缓慢，氛围神秘，适合冥想时听';

async function testFillPrompt() {
  console.log('[Test] 启动浏览器...');
  const context = await chromium.launchPersistentContext(USER_DATA_DIR, {
    headless: false,
    viewport: null,
    args: ['--start-maximized', '--disable-blink-features=AutomationControlled'],
    userAgent: 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36',
    ignoreDefaultArgs: ['--enable-automation'],
  });

  const pages = context.pages();
  const page = pages.length > 0 ? pages[0] : await context.newPage();

  try {
    console.log('[Test] 导航到 suno.ai/create...');
    await page.goto('https://suno.ai/create', {
      waitUntil: 'networkidle',
      timeout: 30000,
    });

    await page.waitForTimeout(3000);

    console.log('[Test] 查找 Song Description 输入框...');
    const strategies = [
      () => page.locator('.useResizer-resizable-container').filter({ hasText: 'Song Description' }).locator('textarea').first(),
      () => page.locator('div:has-text("Song Description")').locator('textarea').first(),
      () => page.locator('div').filter({ hasText: 'Song Description' }).locator('textarea').first(),
    ];

    let filled = false;
    for (const getLocator of strategies) {
      try {
        const locator = getLocator();
        await locator.waitFor({ state: 'visible', timeout: 5000 });
        if (await locator.isVisible()) {
          console.log('[Test] 找到输入框，正在填写:', TEST_PROMPT);
          await locator.click();
          await locator.fill('');
          await locator.fill(TEST_PROMPT);
          await page.waitForTimeout(500);
          filled = true;
          break;
        }
      } catch (e) {
        console.log('[Test] 策略失败:', e.message);
      }
    }

    if (filled) {
      await page.waitForTimeout(500);

      // 点击 Create 前获取歌单快照
      const { set: initialSet } = await getPlaylistSongIds(page);
      console.log(`[Test] 当前歌单歌曲数: ${initialSet.size}`);

      console.log('[Test] 查找 Create 按钮并点击...');
      const createStrategies = [
        () => page.getByRole('button', { name: 'Create song' }),
        () => page.getByRole('button', { name: 'Create' }),
      ];
      let clicked = false;
      for (const getLocator of createStrategies) {
        try {
          const btn = getLocator();
          await btn.waitFor({ state: 'visible', timeout: 3000 });
          if (await btn.isVisible()) {
            await btn.click();
            clicked = true;
            console.log('[Test] ✅ 已点击 Create');
            break;
          }
        } catch (e) {
          console.log('[Test] Create 按钮策略:', e.message);
        }
      }

      if (clicked) {
        console.log('[Test] 开始轮询歌单（每5秒，最多12次）...');
        const newSongIds = await waitForNewSongs(page, initialSet, {
          pollInterval: 5000,
          maxPolls: 12,
          onProgress: (info) => {
            if (info.done) {
              console.log(`[Test] 检测到 ${info.newCount} 首新歌`);
            } else {
              console.log(`[Test] 第 ${info.pollIndex + 1}/${info.maxPolls} 次检测，新歌数: ${info.newCount}，继续等待...`);
            }
          },
        });
        if (newSongIds.length >= 2) {
          const shareLinks = getShareLinks(newSongIds.slice(0, 2));
          console.log('[Test] 分享链接:');
          shareLinks.forEach((link, i) => console.log(`[Test]   ${i + 1}. ${link}`));
        } else {
          console.log(`[Test] 超时：仅检测到 ${newSongIds.length} 首新歌`);
        }
      } else {
        console.log('[Test] ⚠️ 未找到 Create 按钮');
      }
    } else {
      console.log('[Test] ❌ 未找到 Song Description 输入框');
      console.log('[Test] 请确认已登录 Suno，且页面已加载到 suno.ai/create');
    }
    console.log('[Test] 浏览器保持打开，手动关闭即可结束');
  } finally {
    // 不关闭浏览器，方便观察结果
    console.log('[Test] 测试完成');
  }
}

testFillPrompt().catch((err) => {
  console.error('[Test] 错误:', err);
  process.exit(1);
});
