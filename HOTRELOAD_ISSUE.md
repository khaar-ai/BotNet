# 🐛 OpenClaw Plugin Hot Reload Issue

**Date Discovered:** 2026-02-03  
**Status:** CONFIRMED BUG  
**Impact:** HIGH - Affects all plugin HTTP server development

## Problem Description

OpenClaw's `gateway restart` (SIGUSR1) **fails to properly reload plugin HTTP server code** despite successfully recompiling and restarting services.

## Evidence

**Test Case:** BotNet color scheme change (green → red)
1. ✅ Modified CSS colors in `src/http-server.ts`
2. ✅ `npm run build` compiled changes to `dist/src/http-server.js`  
3. ❌ `gateway restart` continued serving old green colors
4. ✅ `kill -TERM <pid>` (full process restart) loaded new red colors

**Conclusion:** HTTP request handlers use cached/old modules despite service restart.

## Root Cause Analysis

**OpenClaw Service Lifecycle:**
- ✅ `service.stop()` called correctly
- ✅ `service.start()` called correctly  
- ❌ **Module cache not cleared** for HTTP handler code
- ❌ Old modules continue serving responses

**Hypothesis:** Node.js module cache (`require.cache`) persists across `api.registerService()` restart cycles.

## Development Impact

**Current Workflow (BROKEN):**
```bash
npm run build && gateway restart  # ❌ Changes not loaded
```

**Required Workaround:**  
```bash
npm run build && <full process restart>  # ✅ Works
```

**Docker Environment:**
```bash
npm run build && docker restart <container>
```

## Technical Investigation Needed

1. **Module Cache Behavior:** How does OpenClaw handle `require.cache` during service restarts?
2. **HTTP Handler Lifecycle:** Are HTTP request handlers cached differently than other plugin code?
3. **Manual Cache Clearing:** Can `delete require.cache[...]` be used in service restart?
4. **Alternative Patterns:** Dynamic module loading that bypasses cache?

## Future Work

**Priority:** HIGH  
**Scope:** Affects all OpenClaw plugins using HTTP servers (not just BotNet)  
**Ideal Fix:** OpenClaw should clear relevant module cache during service restart  
**Current Status:** Documented limitation requiring process restart workaround

---

**NOTE:** This issue was initially misdiagnosed as "working correctly" due to the in-process nature of plugin services. Actual testing proved the hot reload limitation is real and reproducible.