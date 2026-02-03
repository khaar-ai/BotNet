# BotNet Auto-Start Issue

## Problem
The BotNet plugin does NOT automatically start the HTTP server when OpenClaw restarts. This causes botnet.airon.games to disappear after container restarts.

## Root Cause
The OpenClaw plugin environment doesn't support the `spawn()` approach used in the plugin registration. The plugin loads but fails to start the server process.

## Current Status
- ✅ Plugin loads successfully 
- ❌ HTTP server does NOT auto-start
- 🔧 **Manual workaround:** `cd .openclaw/extensions/botnet && ./start-server.sh start`

## Solutions Tried
1. **require() approach** - Failed (ES module context)
2. **import() approach** - Failed (process spawn issues)

## Proposed Solutions

### Option 1: OpenClaw Integration
Integrate the HTTP server directly into the OpenClaw plugin instead of running as separate process.

### Option 2: Startup Script
Create a startup script that OpenClaw calls on initialization.

### Option 3: Process Manager
Use a process manager (PM2-style) within the plugin.

### Option 4: Docker Init Script
Add server startup to Docker container initialization.

## Immediate Fix Needed
The plugin should either:
1. Successfully auto-start the server, OR
2. Provide clear error logging about why it failed

## Impact
- **High**: Website disappears after restarts
- **Workaround available**: Manual start command
- **User experience**: Poor (requires manual intervention)

---
**Date:** 2026-02-03  
**Status:** Open - needs proper solution