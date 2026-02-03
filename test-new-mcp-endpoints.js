#!/usr/bin/env node

// Test script for the new MCP endpoints

import http from 'http';

// Configuration
const HOST = 'localhost';
const PORT = 8080;
const MCP_PATH = '/mcp';

// Helper function to make MCP requests
function makeMCPRequest(method, params = {}, id = 1) {
  return new Promise((resolve, reject) => {
    const data = JSON.stringify({
      jsonrpc: "2.0",
      method: method,
      params: params,
      id: id
    });

    const options = {
      hostname: HOST,
      port: PORT,
      path: MCP_PATH,
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'Content-Length': data.length
      }
    };

    const req = http.request(options, (res) => {
      let body = '';

      res.on('data', (chunk) => {
        body += chunk;
      });

      res.on('end', () => {
        try {
          const response = JSON.parse(body);
          resolve(response);
        } catch (e) {
          reject(new Error(`Failed to parse response: ${body}`));
        }
      });
    });

    req.on('error', (e) => {
      reject(e);
    });

    req.write(data);
    req.end();
  });
}

// Test the new endpoints
async function testNewEndpoints() {
  console.log('🧪 Testing new MCP endpoints...\n');

  // Test 1: botnet.health
  console.log('1️⃣ Testing botnet.health...');
  try {
    const healthResponse = await makeMCPRequest('botnet.health');
    console.log('✅ Health response:', JSON.stringify(healthResponse, null, 2));
  } catch (error) {
    console.log('❌ Health test failed:', error.message);
  }
  console.log('\n---\n');

  // Test 2: botnet.ping (existing endpoint for comparison)
  console.log('2️⃣ Testing botnet.ping (existing endpoint)...');
  try {
    const pingResponse = await makeMCPRequest('botnet.ping');
    console.log('✅ Ping response:', JSON.stringify(pingResponse, null, 2));
  } catch (error) {
    console.log('❌ Ping test failed:', error.message);
  }
  console.log('\n---\n');

  // Test 3: botnet.message.check (should require auth)
  console.log('3️⃣ Testing botnet.message.check (no auth - should fail)...');
  try {
    const msgCheckResponse = await makeMCPRequest('botnet.message.check');
    console.log('Response:', JSON.stringify(msgCheckResponse, null, 2));
  } catch (error) {
    console.log('❌ Message check test failed:', error.message);
  }
  console.log('\n---\n');

  // Test 4: tools/list to see new tools
  console.log('4️⃣ Testing tools/list to see new tools...');
  try {
    const toolsResponse = await makeMCPRequest('tools/list');
    console.log('✅ Tools response (showing new tools):', JSON.stringify(toolsResponse, null, 2));
  } catch (error) {
    console.log('❌ Tools list test failed:', error.message);
  }
}

// Run the tests
testNewEndpoints().catch(console.error);