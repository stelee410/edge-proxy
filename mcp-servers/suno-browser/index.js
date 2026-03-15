#!/usr/bin/env node

/**
 * Suno Browser MCP Server
 * 通过浏览器自动化方式使用 Suno AI 生成音乐
 * 使用 Playwright 实现，支持 :has-text() 等选择器，自动等待元素可操作
 */

import { chromium } from 'playwright';
import { getPlaylistSongIds, waitForNewSongs, getShareLinks } from './lib/playlist.js';
import { Server } from '@modelcontextprotocol/sdk/server/index.js';
import { StdioServerTransport } from '@modelcontextprotocol/sdk/server/stdio.js';
import {
  CallToolRequestSchema,
  ListToolsRequestSchema,
} from '@modelcontextprotocol/sdk/types.js';
import path from 'path';
import { fileURLToPath } from 'url';
import fs from 'fs';

const __dirname = path.dirname(fileURLToPath(import.meta.url));

// 创建 MCP Server
const server = new Server({
  name: 'suno-browser-mcp-server',
  version: '1.0.0',
}, {
  capabilities: { tools: {} }
});

// 浏览器上下文和页面
let context = null;
let page = null;
const USER_DATA_DIR = path.join(__dirname, '.chrome-data');

/**
 * 列出可用工具
 */
server.setRequestHandler(ListToolsRequestSchema, async () => {
  return {
    tools: [
      {
        name: 'suno_login',
        description: '打开 Suno 网页并显示浏览器窗口，供手动登录。首次使用必须先登录。',
        inputSchema: {
          type: 'object',
          properties: {},
        },
      },
      {
        name: 'suno_generate',
        description: '在 Suno 中生成音乐。需要先调用 suno_login 登录。',
        inputSchema: {
          type: 'object',
          properties: {
            prompt: {
              type: 'string',
              description: '音乐描述提示词，如「一首快乐的流行歌曲，节奏轻快」',
            },
            style: {
              type: 'string',
              description: '音乐风格（可选），如 pop、rock、electronic、jazz 等',
            },
            custom_mode: {
              type: 'boolean',
              description: '是否使用自定义模式（Custom Mode）',
              default: false,
            },
            lyrics: {
              type: 'string',
              description: '自定义歌词（仅在 custom_mode=true 时使用）',
            },
          },
          required: ['prompt'],
        },
      },
      {
        name: 'suno_get_share_link',
        description: '从当前页面获取歌曲分享链接。需要先生成音乐。',
        inputSchema: {
          type: 'object',
          properties: {},
        },
      },
      {
        name: 'suno_close',
        description: '关闭浏览器并清理资源',
        inputSchema: {
          type: 'object',
          properties: {},
        },
      },
      {
        name: 'suno_get_page_info',
        description: '获取当前页面信息（调试用）',
        inputSchema: {
          type: 'object',
          properties: {},
        },
      },
      {
        name: 'suno_inspect_page',
        description: '检查页面元素（用于调试选择器问题）',
        inputSchema: {
          type: 'object',
          properties: {},
        },
      },
    ],
  };
});

/**
 * 处理工具调用
 */
server.setRequestHandler(CallToolRequestSchema, async (request) => {
  const { name, arguments: args } = request.params;

  switch (name) {
    case 'suno_login':
      return await handleLogin();

    case 'suno_generate':
      return await handleGenerate(args);

    case 'suno_get_share_link':
      return await handleGetShareLink();

    case 'suno_close':
      return await handleClose();

    case 'suno_get_page_info':
      return await handleGetPageInfo();

    case 'suno_inspect_page':
      return await handleInspectPage();

    default:
      return {
        content: [{ type: 'text', text: `Unknown tool: ${name}` }],
        isError: true,
      };
  }
});

/**
 * 登录 Suno
 */
async function handleLogin() {
  try {
    if (context) {
      await context.close();
      context = null;
      page = null;
    }

    console.error('[Suno] 正在启动浏览器...');

    context = await launchContext(false); // 登录时始终有头模式，让用户看到浏览器

    // 使用已有页面或创建新页面
    const pages = context.pages();
    page = pages.length > 0 ? pages[0] : await context.newPage();

    console.error('[Suno] 正在打开 Suno 网站...');

    await page.goto('https://suno.ai', {
      waitUntil: 'networkidle',
      timeout: 30000,
    });

    console.error('[Suno] 页面已加载，等待用户手动登录...');
    console.error('[Suno] 浏览器窗口已打开，请手动完成登录。');

    await page.waitForTimeout(3000);

    const isLoggedIn = await checkLoginStatus();

    if (isLoggedIn) {
      console.error('[Suno] 检测到已登录状态');
      return {
        content: [{
          type: 'text',
          text: JSON.stringify({
            success: true,
            message: '✅ 已登录 Suno！可以直接调用 suno_generate 生成音乐。',
            status: 'logged_in',
          }, null, 2),
        }],
      };
    }

    return {
      content: [{
        type: 'text',
        text: JSON.stringify({
          success: true,
          message: '🌐 浏览器已打开，请手动完成登录操作。\n\n' +
                   '登录步骤：\n' +
                   '1. 点击页面上的「登录」或「Sign In」按钮\n' +
                   '2. 选择登录方式（Google、邮箱等）\n' +
                   '3. 完成登录后，浏览器窗口将保持打开状态\n' +
                   '4. 然后可以调用 suno_generate 开始生成音乐',
          status: 'waiting_for_login',
          current_url: page.url(),
        }, null, 2),
      }],
    };
  } catch (error) {
    console.error('[Suno] Login error:', error);
    return {
      content: [{
        type: 'text',
        text: JSON.stringify({
          success: false,
          error: error.message,
        }, null, 2),
      }],
      isError: true,
    };
  }
}

/**
 * 生成音乐
 * 打开 Suno 创建页、填写提示词、点击 Create 按钮
 */
async function handleGenerate(args) {
  try {
    const prompt = args.prompt || '';

    // 若无浏览器会话，自动启动
    // 有持久化登录数据 → 无头模式（静默后台运行）
    // 没有登录数据 → 有头模式（让用户看到浏览器完成登录）
    if (!context || !page) {
      const loginData = hasLoginData();
      const headless = loginData;
      console.error(`[Suno] 无浏览器会话，自动启动（headless=${headless}，loginData=${loginData}）...`);
      try {
        context = await launchContext(headless);
        const pages = context.pages();
        page = pages.length > 0 ? pages[0] : await context.newPage();
        await page.goto('https://suno.ai', { waitUntil: 'networkidle', timeout: 30000 });
        await page.waitForTimeout(2000);
      } catch (e) {
        console.error('[Suno] 自动启动浏览器失败:', e.message);
        return {
          content: [{ type: 'text', text: '[NOTIFY]Suno 浏览器启动失败，请稍后再试' }],
          isError: true,
        };
      }
    }

    console.error(`[Suno] 正在处理: 打开创建页并填写提示词 "${prompt}"`);

    const isLoggedIn = await checkLoginStatus();
    if (!isLoggedIn) {
      return {
        content: [{
          type: 'text',
          text: '[NOTIFY]Suno 未登录，请在浏览器中完成登录后再试',
        }],
        isError: true,
      };
    }

    console.error('[Suno] 导航到创建页面...');
    await page.goto('https://suno.ai/create', {
      waitUntil: 'networkidle',
      timeout: 30000,
    });

    await page.waitForTimeout(2000);

    const inputResult = await fillPromptInput(prompt);
    if (!inputResult.success) {
      console.error(`[Suno] ${inputResult.message}`);
      return {
        content: [{
          type: 'text',
          text: JSON.stringify({
            success: false,
            error: inputResult.message,
            debug: inputResult.debug,
          }, null, 2),
        }],
        isError: true,
      };
    }

    await page.waitForTimeout(500);

    // 获取点击 Create 前的歌单 ID 列表
    const { set: initialSet } = await getPlaylistSongIds(page);
    console.error(`[Suno] 当前歌单歌曲数: ${initialSet.size}`);

    const createResult = await clickCreateButton();
    if (!createResult.success) {
      console.error(`[Suno] ${createResult.message}`);
      return {
        content: [{
          type: 'text',
          text: JSON.stringify({
            success: false,
            error: createResult.message,
            debug: createResult.debug,
          }, null, 2),
        }],
        isError: true,
      };
    }

    // 每 5 秒检测歌单是否新增 2 首歌，最多检测 12 次（60 秒）
    const newSongIds = await waitForNewSongs(page, initialSet, {
      pollInterval: 5000,
      maxPolls: 12,
      onProgress: (info) => {
        if (info.done) {
          console.error(`[Suno] 检测到 ${info.newCount} 首新歌`);
        } else {
          console.error(`[Suno] 第 ${info.pollIndex + 1}/${info.maxPolls} 次检测，新歌数: ${info.newCount}，继续等待...`);
        }
      },
    });
    let shareLinks = [];

    if (newSongIds.length >= 2) {
      shareLinks = getShareLinks(newSongIds.slice(0, 2));
      for (const link of shareLinks) {
        console.error(`[Suno] 分享链接: ${link}`);
      }
    } else {
      console.error(`[Suno] 超时：未检测到 2 首新歌（检测到 ${newSongIds.length} 首）`);
    }

    return {
      content: [{
        type: 'text',
        text: JSON.stringify({
          success: true,
          message: shareLinks.length >= 2
            ? '✅ 音乐生成完成，已获取分享链接'
            : '⏳ 已点击 Create，音乐可能仍在生成中',
          prompt,
          share_links: shareLinks,
          current_url: page.url(),
        }, null, 2),
      }],
    };
  } catch (error) {
    console.error('[Suno] Generate error:', error);
    return {
      content: [{
        type: 'text',
        text: JSON.stringify({
          success: false,
          error: error.message,
          stack: error.stack,
        }, null, 2),
      }],
      isError: true,
    };
  }
}

/**
 * 获取分享链接
 */
async function handleGetShareLink() {
  try {
    if (!context || !page) {
      return {
        content: [{
          type: 'text',
          text: JSON.stringify({ error: '浏览器未打开' }, null, 2),
        }],
        isError: true,
      };
    }

    console.error('[Suno] 正在获取分享链接...');

    const currentUrl = page.url();
    let shareUrl = '';

    if (currentUrl.includes('suno.ai/song') || currentUrl.includes('suno.ai/listen')) {
      shareUrl = currentUrl;
    }

    if (!shareUrl) {
      const linkSelectors = [
        'a[href*="suno.ai/song"]',
        'a[href*="suno.ai/listen"]',
        '[data-share-url]',
        '[class*="share"] a[href]',
      ];

      for (const selector of linkSelectors) {
        try {
          const links = await page.locator(selector).all();
          for (const link of links) {
            let href = await link.getAttribute('href') || await link.getAttribute('data-share-url');
            if (href) {
              if (href.startsWith('/')) {
                href = 'https://suno.ai' + href;
              }
              if (href.includes('suno.ai')) {
                shareUrl = href;
                break;
              }
            }
          }
          if (shareUrl) break;
        } catch (e) {
          console.error('[Suno] 获取链接时出错:', e.message);
        }
      }
    }

    if (!shareUrl) {
      console.error('[Suno] 尝试点击分享按钮...');
      const shareButton = page.getByRole('button', { name: /share|分享|link/i }).first();
      try {
        await shareButton.click({ timeout: 3000 });
        await page.waitForTimeout(1000);
      } catch (e) {
        const buttons = await page.locator('button, a[role="button"]').all();
        for (const btn of buttons) {
          try {
            const text = await btn.textContent();
            const ariaLabel = await btn.getAttribute('aria-label') || '';
            if ((text || ariaLabel).toLowerCase().match(/share|分享|link/)) {
              await btn.click();
              await page.waitForTimeout(1000);
              break;
            }
          } catch {}
        }
      }

      const newUrl = page.url();
      if (newUrl !== currentUrl && newUrl.includes('suno.ai')) {
        shareUrl = newUrl;
      }
    }

    if (shareUrl) {
      console.error(`[Suno] 获取到分享链接: ${shareUrl}`);
      return {
        content: [{
          type: 'text',
          text: JSON.stringify({
            success: true,
            share_url: shareUrl,
            message: '🔗 分享链接已获取，点击即可播放',
          }, null, 2),
        }],
      };
    }

    return {
      content: [{
        type: 'text',
        text: JSON.stringify({
          success: false,
          message: '⚠️ 未自动获取到分享链接，请手动在浏览器中点击分享按钮并复制链接。',
          current_url: currentUrl,
        }, null, 2),
      }],
      isError: true,
    };
  } catch (error) {
    console.error('[Suno] Get share link error:', error);
    return {
      content: [{
        type: 'text',
        text: JSON.stringify({
          success: false,
          error: error.message,
        }, null, 2),
      }],
      isError: true,
    };
  }
}

/**
 * 获取页面信息（调试用）
 */
async function handleGetPageInfo() {
  try {
    if (!context || !page) {
      return {
        content: [{
          type: 'text',
          text: JSON.stringify({ error: '浏览器未打开' }, null, 2),
        }],
      };
    }

    const url = page.url();
    const title = await page.title();

    return {
      content: [{
        type: 'text',
        text: JSON.stringify({
          url,
          title,
        }, null, 2),
      }],
    };
  } catch (error) {
    return {
      content: [{
        type: 'text',
        text: JSON.stringify({ error: error.message }, null, 2),
      }],
    };
  }
}

/**
 * 检查页面元素（用于调试）
 */
async function handleInspectPage() {
  try {
    if (!context || !page) {
      return {
        content: [{
          type: 'text',
          text: JSON.stringify({ error: '浏览器未打开' }, null, 2),
        }],
        isError: true,
      };
    }

    console.error('[Suno] 正在检查页面元素...');

    const info = await page.evaluate(() => {
      const url = window.location.href;
      const title = document.title;

      const textareas = [];
      document.querySelectorAll('textarea').forEach((el, idx) => {
        const style = window.getComputedStyle(el);
        const rect = el.getBoundingClientRect();
        textareas.push({
          index: idx,
          placeholder: el.placeholder || '',
          class: el.className,
          id: el.id,
          value: el.value?.substring(0, 100) || '',
          visible: style.display !== 'none' && style.visibility !== 'hidden' && rect.width > 0,
        });
      });

      const contentEditables = [];
      document.querySelectorAll('[contenteditable="true"]').forEach((el, idx) => {
        const style = window.getComputedStyle(el);
        const rect = el.getBoundingClientRect();
        contentEditables.push({
          index: idx,
          placeholder: el.getAttribute('placeholder') || '',
          class: el.className,
          id: el.id,
          text: el.textContent?.substring(0, 100) || '',
          visible: style.display !== 'none' && style.visibility !== 'hidden' && rect.width > 0,
        });
      });

      const buttons = [];
      document.querySelectorAll('button, [role="button"]').forEach((el, idx) => {
        const style = window.getComputedStyle(el);
        const rect = el.getBoundingClientRect();
        if (style.display !== 'none' && style.visibility !== 'hidden' && rect.width > 0 && rect.height > 0) {
          buttons.push({
            index: idx,
            text: el.textContent?.substring(0, 50) || '',
            ariaLabel: el.getAttribute('aria-label') || '',
            class: el.className,
            id: el.id,
            tag: el.tagName.toLowerCase(),
          });
        }
      });

      const textInputs = [];
      document.querySelectorAll('input[type="text"], input:not([type])').forEach((el, idx) => {
        const style = window.getComputedStyle(el);
        const rect = el.getBoundingClientRect();
        if (style.display !== 'none' && style.visibility !== 'hidden' && rect.width > 0) {
          textInputs.push({
            index: idx,
            placeholder: el.placeholder || '',
            class: el.className,
            id: el.id,
            value: el.value?.substring(0, 50) || '',
          });
        }
      });

      return {
        url,
        title,
        textareas: textareas.filter(t => t.visible),
        contentEditables: contentEditables.filter(c => c.visible),
        buttons: buttons.slice(0, 20),
        textInputs,
      };
    });

    console.error('[Suno] 页面检查完成');

    return {
      content: [{
        type: 'text',
        text: JSON.stringify({
          success: true,
          url: info.url,
          title: info.title,
          textareas_found: info.textareas.length,
          content_editables_found: info.contentEditables.length,
          buttons_found: info.buttons.length,
          text_inputs_found: info.textInputs.length,
          visible_textareas: info.textareas,
          visible_content_editables: info.contentEditables,
          visible_buttons: info.buttons,
          visible_text_inputs: info.textInputs,
        }, null, 2),
      }],
    };
  } catch (error) {
    console.error('[Suno] Inspect page error:', error);
    return {
      content: [{
        type: 'text',
        text: JSON.stringify({
          success: false,
          error: error.message,
        }, null, 2),
      }],
      isError: true,
    };
  }
}

/**
 * 关闭浏览器
 */
async function handleClose() {
  try {
    if (context) {
      await context.close();
      context = null;
      page = null;
      console.error('[Suno] 浏览器已关闭');
    }

    return {
      content: [{
        type: 'text',
        text: JSON.stringify({ success: true, message: '浏览器已关闭' }, null, 2),
      }],
    };
  } catch (error) {
    console.error('[Suno] Close error:', error);
    return {
      content: [{
        type: 'text',
        text: JSON.stringify({ error: error.message }, null, 2),
      }],
    };
  }
}

// ========== 辅助函数 ==========

/**
 * 填写提示词输入框
 * 通过稳定的 "Song Description" 文案定位 textarea，避免依赖动态 placeholder
 */
async function fillPromptInput(prompt) {
  try {
    console.error('[Suno] 正在查找提示词输入框（Song Description 区域）...');

    const strategies = [
      () => page.locator('.useResizer-resizable-container').filter({ hasText: 'Song Description' }).locator('textarea').first(),
      () => page.locator('div:has-text("Song Description")').locator('textarea').first(),
      () => page.locator('div').filter({ hasText: 'Song Description' }).locator('textarea').first(),
    ];

    for (const getLocator of strategies) {
      try {
        const locator = getLocator();
        await locator.waitFor({ state: 'visible', timeout: 5000 });
        if (await locator.isVisible()) {
          console.error('[Suno] 找到 Song Description 输入框');
          await locator.click();
          await locator.fill('');
          await locator.fill(prompt);
          await page.waitForTimeout(500);

          const inputValue = await locator.evaluate((el) => el.value || el.textContent || '');
          if (String(inputValue).includes(prompt.substring(0, Math.min(10, prompt.length)))) {
            return { success: true, message: '已填写提示词' };
          }
        }
      } catch (e) {
        console.error(`[Suno] 定位策略失败:`, e.message);
      }
    }

    return {
      success: false,
      message: '无法找到 Song Description 输入框',
      debug: '请确认页面已加载到 suno.ai/create 且已登录',
    };
  } catch (error) {
    return {
      success: false,
      message: '无法找到或填写输入框',
      debug: error.message,
    };
  }
}

/**
 * 填写风格输入框
 */
async function fillStyleInput(style) {
  try {
    console.error('[Suno] 正在查找风格输入框...');

    const styleSelectors = [
      'input[placeholder*="style of music"]',
      'input[placeholder*="Style"]',
      'input[placeholder*="style"]',
      'input[placeholder*="风格"]',
      '[class*="style"] input',
      '[class*="Style"] input',
      'div[class*="style"] [contenteditable="true"]',
    ];

    for (const selector of styleSelectors) {
      try {
        const locator = page.locator(selector).first();
        if (await locator.isVisible({ timeout: 1000 })) {
          console.error(`[Suno] 找到风格输入框: ${selector}`);
          await locator.click({ clickCount: 3 });
          await page.keyboard.type(style, { delay: 30 });
          return true;
        }
      } catch (e) {
        console.error(`[Suno] 风格选择器 ${selector}:`, e.message);
      }
    }

    console.error('[Suno] 未找到风格输入框');
    return false;
  } catch (error) {
    console.error('[Suno] 填写风格时出错:', error);
    return false;
  }
}

/**
 * 填写歌词输入框
 */
async function fillLyricsInput(lyrics) {
  try {
    console.error('[Suno] 正在查找歌词输入框...');

    const lyricsSelectors = [
      'textarea[placeholder*="lyrics"]',
      'textarea[placeholder*="Lyrics"]',
      'textarea[placeholder*="歌词"]',
      '[class*="lyrics"] textarea',
      '[class*="Lyrics"] textarea',
      'div[class*="lyrics"] textarea',
    ];

    for (const selector of lyricsSelectors) {
      try {
        const locator = page.locator(selector).first();
        if (await locator.isVisible({ timeout: 1000 })) {
          console.error(`[Suno] 找到歌词输入框: ${selector}`);
          await locator.click({ clickCount: 3 });
          await page.keyboard.type(lyrics, { delay: 30 });
          return true;
        }
      } catch (e) {
        console.error(`[Suno] 歌词选择器 ${selector}:`, e.message);
      }
    }

    console.error('[Suno] 未找到歌词输入框');
    return false;
  } catch (error) {
    console.error('[Suno] 填写歌词时出错:', error);
    return false;
  }
}

/**
 * 点击 Create 按钮
 * 优先使用稳定的 aria-label="Create song"
 */
async function clickCreateButton() {
  try {
    console.error('[Suno] 正在查找 Create 按钮...');

    const strategies = [
      () => page.getByRole('button', { name: 'Create song' }),
      () => page.getByRole('button', { name: 'Create' }),
      () => page.locator('button[aria-label="Create song"]'),
      () => page.getByRole('button', { name: /create|generate|生成/i }).first(),
    ];

    for (const getLocator of strategies) {
      try {
        const locator = getLocator();
        await locator.waitFor({ state: 'visible', timeout: 3000 });
        if (await locator.isVisible()) {
          console.error('[Suno] 找到 Create 按钮，正在点击');
          await locator.click();
          await page.waitForTimeout(500);
          return { success: true };
        }
      } catch (e) {
        console.error('[Suno] Create 按钮策略失败:', e.message);
      }
    }

    return {
      success: false,
      message: '无法找到 Create 按钮',
      debug: '请确认页面在 suno.ai/create 且已填写提示词',
    };
  } catch (error) {
    return {
      success: false,
      message: '无法点击 Create 按钮',
      debug: error.message,
    };
  }
}

/**
 * 检查页面是否有错误提示
 */
async function checkForErrors() {
  try {
    const errorLocator = page.locator('[class*="error"], [class*="Error"], .error, .alert, [role="alert"]').first();
    return await errorLocator.isVisible({ timeout: 1000 });
  } catch {
    return false;
  }
}

/**
 * 检查生成状态是否完成
 */
async function checkGenerationStatus() {
  try {
    const url = page.url();
    if (url.includes('suno.ai/song') || url.includes('suno.ai/listen')) {
      return 'completed';
    }
    const completedIndicator = page.locator('[class*="complete"], [class*="done"], audio, [data-status="completed"]').first();
    if (await completedIndicator.isVisible({ timeout: 500 })) {
      return 'completed';
    }
    return 'generating';
  } catch {
    return 'generating';
  }
}

/**
 * 获取错误消息
 */
async function getErrorMessage() {
  try {
    const errorSelectors = [
      '[class*="error"]',
      '[class*="Error"]',
      '.error',
      '.alert',
      '[role="alert"]',
    ];

    for (const selector of errorSelectors) {
      try {
        const locator = page.locator(selector).first();
        if (await locator.isVisible({ timeout: 500 })) {
          const text = await locator.textContent();
          if (text && text.length > 0 && text.length < 500) {
            return text.trim();
          }
        }
      } catch (e) {
        console.error(`[Suno] getErrorMessage ${selector}:`, e.message);
      }
    }
    return '';
  } catch {
    return '';
  }
}

/**
 * 检查登录状态
 */
async function checkLoginStatus() {
  try {
    console.error('[Suno] 检查登录状态...');

    const url = page.url();
    console.error(`[Suno] 当前 URL: ${url}`);

    if (url.includes('suno.ai/create') || url.includes('suno.ai/library')) {
      console.error('[Suno] URL 显示已登录');
      return true;
    }

    const loginSelectors = [
      'button:has-text("Sign in")',
      'button:has-text("Login")',
      'button:has-text("登录")',
      'a[href*="login"]',
      'a[href*="signin"]',
    ];

    for (const selector of loginSelectors) {
      try {
        const locator = page.locator(selector).first();
        if (await locator.isVisible({ timeout: 1000 })) {
          console.error('[Suno] 发现登录按钮，未登录');
          return false;
        }
      } catch (e) {
        console.error(`[Suno] 登录检查 ${selector}:`, e.message);
      }
    }

    const userSelectors = [
      '[class*="avatar"]',
      '[class*="user-menu"]',
      '[class*="user"] button',
      'button[aria-label*="user"]',
      'button[aria-label*="profile"]',
      'img[alt*="avatar"]',
      'img[alt*="Avatar"]',
    ];

    for (const selector of userSelectors) {
      try {
        const locator = page.locator(selector).first();
        if (await locator.isVisible({ timeout: 1000 })) {
          console.error('[Suno] 发现用户元素，已登录');
          return true;
        }
      } catch (e) {
        console.error(`[Suno] 用户元素 ${selector}:`, e.message);
      }
    }

    console.error('[Suno] 尝试访问创建页面验证登录状态...');
    try {
      await page.goto('https://suno.ai/create', {
        waitUntil: 'domcontentloaded',
        timeout: 10000,
      });
      await page.waitForTimeout(2000);

      const newUrl = page.url();
      console.error(`[Suno] 访问后 URL: ${newUrl}`);

      if (newUrl.includes('suno.ai/create')) {
        console.error('[Suno] 成功访问创建页面，已登录');
        return true;
      }

      if (newUrl.includes('login') || newUrl.includes('signin')) {
        console.error('[Suno] 被重定向到登录页，未登录');
        return false;
      }
    } catch (e) {
      console.error('[Suno] 访问创建页失败:', e.message);
    }

    console.error('[Suno] 无法确定登录状态，假设已登录');
    return true;
  } catch (error) {
    console.error('[Suno] 检查登录状态时出错:', error);
    return false;
  }
}

/**
 * 切换到自定义模式
 */
async function switchToCustomMode() {
  console.error('[Suno] 切换到 Custom Mode...');

  const customModeSelectors = [
    'button:has-text("Custom")',
    'button:has-text("自定义")',
    '[class*="custom"] button',
    '[class*="Custom"] button',
  ];

  for (const selector of customModeSelectors) {
    try {
      const locator = page.locator(selector).first();
      if (await locator.isVisible({ timeout: 1000 })) {
        await locator.click();
        await page.waitForTimeout(500);
        return;
      }
    } catch (e) {
      console.error(`[Suno] Custom Mode ${selector}:`, e.message);
    }
  }
}

/**
 * 判断是否已有持久化登录数据（.chrome-data 目录非空）
 */
function hasLoginData() {
  try {
    if (!fs.existsSync(USER_DATA_DIR)) return false;
    const entries = fs.readdirSync(USER_DATA_DIR);
    return entries.length > 0;
  } catch {
    return false;
  }
}

/**
 * 启动浏览器上下文
 * @param {boolean} headless - 是否无头模式
 */
async function launchContext(headless = false) {
  return chromium.launchPersistentContext(USER_DATA_DIR, {
    headless,
    viewport: null,
    args: [
      '--start-maximized',
      '--disable-blink-features=AutomationControlled',
      '--no-sandbox',
      '--disable-setuid-sandbox',
    ],
    userAgent: 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36',
    ignoreDefaultArgs: ['--enable-automation'],
  });
}

/**
 * 启动服务器
 */
async function main() {
  const transport = new StdioServerTransport();
  await server.connect(transport);
  console.error('[Suno] Browser MCP Server running on stdio (Playwright)');
}

main().catch((error) => {
  console.error('[Suno] Fatal error:', error);
  process.exit(1);
});

process.on('SIGINT', async () => {
  console.error('[Suno] 收到中断信号，正在关闭...');
  if (context) await context.close();
  process.exit(0);
});

process.on('SIGTERM', async () => {
  console.error('[Suno] 收到终止信号，正在关闭...');
  if (context) await context.close();
  process.exit(0);
});
