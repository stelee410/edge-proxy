#!/usr/bin/env node

/**
 * 测试 Suno MCP Server 是否正常工作
 */

import { Client } from '@modelcontextprotocol/sdk/client/index.js';
import { StdioClientTransport } from '@modelcontextprotocol/sdk/client/stdio.js';

async function test() {
  console.error('Starting test client...\n');

  const transport = new StdioClientTransport({
    command: 'node',
    args: ['index.js'],
    env: {
      NODE_ENV: 'production',
    },
  });

  const client = new Client({
    name: 'test-client',
    version: '1.0.0',
  }, {
    capabilities: {},
  });

  try {
    console.error('Connecting to Suno MCP Server...');
    await client.connect(transport);
    console.error('✓ Connected\n');

    // 测试列出工具
    console.error('Listing tools...');
    const toolsResult = await client.listTools();
    console.error(`✓ Found ${toolsResult.tools.length} tools:`);
    for (const tool of toolsResult.tools) {
      console.error(`  - ${tool.name}: ${tool.description}`);
    }
    console.error('');

    // 测试登录工具
    console.error('Testing suno_login tool...');
    const loginResult = await client.callTool({
      name: 'suno_login',
      arguments: {},
    });
    console.error('✓ suno_login result:');
    console.error(loginResult.content[0]?.text);
    console.error('');

  } catch (error) {
    console.error('✗ Error:', error.message);
    console.error(error);
    process.exit(1);
  }
}

test().catch(console.error);
