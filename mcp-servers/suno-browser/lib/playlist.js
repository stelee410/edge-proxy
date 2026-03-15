/**
 * 播放列表相关工具函数
 * 供 test-fill-prompt.js 和 index.js (MCP) 共用
 */

/**
 * 获取播放列表中所有歌曲的 ID（按 DOM 顺序，新歌通常在顶部）
 * @param {import('playwright').Page} page
 * @returns {Promise<{ ids: string[], set: Set<string> }>}
 */
export async function getPlaylistSongIds(page) {
  try {
    const ids = await page.evaluate(() => {
      const anchors = document.querySelectorAll('[data-testid="clip-row"] a[href*="/song/"]');
      const seen = new Set();
      const ordered = [];
      anchors.forEach((a) => {
        const href = a.getAttribute('href') || '';
        const match = href.match(/\/song\/([a-f0-9-]+)/i);
        if (match && !seen.has(match[1])) {
          seen.add(match[1]);
          ordered.push(match[1]);
        }
      });
      return ordered;
    });
    return { ids, set: new Set(ids) };
  } catch {
    return { ids: [], set: new Set() };
  }
}

/**
 * 轮询等待歌单新增 2 首歌
 * @param {import('playwright').Page} page
 * @param {Set<string>} initialSet - 点击 Create 前的歌曲 ID 集合
 * @param {{ pollInterval?: number, maxPolls?: number, onProgress?: (info: { pollIndex: number, maxPolls: number, newCount: number, done?: boolean }) => void }} opts
 * @returns {Promise<string[]>} 新增的歌曲 ID 列表（按列表顺序，新的在前）
 */
export async function waitForNewSongs(page, initialSet, opts = {}) {
  const { pollInterval = 5000, maxPolls = 12, onProgress } = opts;
  for (let i = 0; i < maxPolls; i++) {
    await page.waitForTimeout(pollInterval);
    const { ids } = await getPlaylistSongIds(page);
    const newIds = ids.filter((id) => !initialSet.has(id));
    if (newIds.length >= 2) {
      onProgress?.({ pollIndex: i, maxPolls, newCount: newIds.length, done: true });
      return newIds;
    }
    if (i < maxPolls - 1) {
      onProgress?.({ pollIndex: i, maxPolls, newCount: newIds.length });
    }
  }
  const { ids } = await getPlaylistSongIds(page);
  return ids.filter((id) => !initialSet.has(id));
}

/**
 * 根据歌曲 ID 生成分享链接
 * @param {string[]} songIds - 歌曲 ID 列表
 * @returns {string[]} 分享链接列表
 */
export function getShareLinks(songIds) {
  return songIds.map((id) => `https://suno.ai/song/${id}`);
}
