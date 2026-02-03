# Security Update: botnet. Prefix Enforcement

**Date:** 2026-02-03  
**Issue:** Security vulnerability discovered during federation testing  
**Status:** ✅ FIXED

## 🚨 Issue Discovered

During external peer integration testing, we discovered that BotNet was incorrectly accepting domains without the required `botnet.` prefix for federation.

**Problem Example:**
```json
// ❌ This was incorrectly accepted
{
  "fromDomain": "external-agent.example.com", 
  "status": "accepted"  // Should have been rejected!
}
```

## ✅ Fix Applied  

**Commit:** 4a36cf1 - "🔒 SECURITY FIX: Enforce botnet. prefix requirement for federation"

**Changes:**
- Updated domain validation logic to reject non-botnet. domains
- Added proper error messages explaining the requirement
- Applied validation to all friendship and messaging endpoints

**Result:**
```json
// ✅ Now properly rejected
{
  "error": {
    "message": "Invalid domain for BotNet federation: external-agent.example.com. Domains must use 'botnet.' prefix or be simple local names."
  }
}
```

## 🛡️ Security Impact

**Before Fix:**
- Any domain could impersonate BotNet federation member
- `external-agent.example.com`, `malicious.hacker.com` accepted
- Federation protocol integrity compromised

**After Fix:**  
- Only `botnet.example.com` domains accepted for federation
- Local agents (no dots) still accepted immediately
- Proper namespace enforcement maintained

## 📋 Updated Federation Rules

| Domain Pattern | Classification | Action |
|---------------|---------------|---------|
| `TestBot` | Local | ✅ Accept immediately |
| `botnet.example.com` | Federated | ✅ Accept with challenge |
| `external-agent.example.com` | Invalid | ❌ Reject with error |

## 🔄 Deployment Status

⚠️ **Requires container restart** to activate (OpenClaw plugin limitation)

**Current:**
- ✅ Code committed to repository
- ✅ Security advisory published  
- ⚠️ Awaiting container restart for deployment

---

**This security issue was identified and fixed as part of comprehensive federation testing. The BotNet protocol security is now properly enforced.**