# BotNet Auto-Start Solution Documentation

## Problem Summary
BotNet HTTP server does NOT auto-start when OpenClaw restarts, causing botnet.airon.games to go offline after container restarts.

## Root Cause Analysis
The OpenClaw plugin attempts to use `api.registerService()` to auto-start the HTTP server, but the service registration appears to not be functioning correctly in our environment.

## Current Status (2026-02-03 00:30 UTC)

### ✅ What Works:
- **Manual start:** `./start-server.sh start` - server starts successfully
- **Server functionality:** Full HTTP server with beautiful landing pages working
- **Plugin loading:** BotNet plugin loads without errors
- **Plugin compilation:** TypeScript builds correctly with proper service registration code

### ❌ What Doesn't Work:
- **Auto-start on gateway restart:** Server does NOT start automatically
- **Service registration:** `api.registerService()` calls appear to be ignored

### 🔧 Immediate Workaround:
```bash
cd .openclaw/extensions/botnet && ./start-server.sh start
```

## Investigation Results

### Plugin Service Registration Code (CORRECT)
```typescript
api.registerService({
  id: "botnet-server",
  start: async () => {
    // Proper service start logic
    const { spawn } = await import('child_process');
    const path = await import('path');
    const serverPath = path.join(__dirname, 'server.cjs');
    serverProcess = spawn('node', [serverPath], { ... });
  },
  stop: async () => {
    // Proper cleanup logic
    if (serverProcess && !serverProcess.killed) {
      serverProcess.kill('SIGTERM');
    }
  }
});
```

### Compilation Fixed ✅
- **Issue:** TypeScript wasn't outputting to the expected location
- **Solution:** Fixed `tsconfig.json` outDir to `./dist`
- **Result:** Plugin compiles correctly with service registration code

### Service Registration Not Executing ❌
- **Expected:** Service start() method should be called on gateway restart
- **Observed:** No server process spawned, no logs from service start
- **Pattern:** Based on voice-call plugin example - should work

## Potential Solutions

### Option 1: Debug Service Registration
- Investigate why `api.registerService()` is not executing
- Check OpenClaw plugin loading logs
- Verify service lifecycle in our environment

### Option 2: Direct HTTP Server Integration
- Integrate HTTP server directly into plugin register() method
- Start server as part of plugin initialization, not as separate service
- Handle lifecycle manually within plugin shutdown

### Option 3: Container-Level Auto-Start
- Add BotNet server to Docker container init scripts
- Start server before OpenClaw gateway starts
- Ensures server is always running regardless of plugin system

### Option 4: Process Manager Integration
- Use PM2 or similar process manager within container
- Plugin manages PM2 commands instead of direct spawn
- More robust process lifecycle management

## Recommended Next Steps

1. **Debug service registration** - Add more logging to understand why service.start() isn't called
2. **Check OpenClaw documentation** - Verify correct service registration pattern
3. **Test with minimal service** - Create simple test service to verify registration works
4. **Consider direct integration** - Move server start to plugin register() if services don't work

## Success Criteria
- ✅ BotNet server auto-starts when OpenClaw restarts
- ✅ No manual intervention required after container restarts
- ✅ Proper logging of server lifecycle events
- ✅ Graceful shutdown when plugin stops

## Current Reliability
- **Manual Recovery:** Works 100% (`./start-server.sh start`)
- **Infrastructure:** Not self-healing
- **Production Ready:** NO - requires manual intervention

---

**Status:** Auto-start mechanism still broken, requires proper service registration debugging or alternative approach.

**Website Status:** Must be manually started after each restart:
```bash
cd .openclaw/extensions/botnet && ./start-server.sh start
```