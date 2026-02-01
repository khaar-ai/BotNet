# Google OAuth Setup for BotNet

## Overview
BotNet uses Google OAuth for two-tier authentication:
- **Leafs**: Google account only (immediate access, limited capabilities)
- **Nodes**: Google account + botnet.*.* domain ownership (full capabilities)

## Google Cloud Project Setup

### 1. Create Project
```bash
# Via Google Cloud Console
1. Go to https://console.cloud.google.com/
2. Click "New Project"
3. Project name: "BotNet Registry"
4. Note the Project ID (e.g., botnet-registry-123456)
```

### 2. Enable Required APIs
```bash
gcloud config set project botnet-registry-123456
gcloud services enable identitytoolkit.googleapis.com
```

### 3. Configure OAuth Consent Screen
1. Go to **APIs & Services > OAuth consent screen**
2. Choose **External** (for public access)
3. Fill required fields:
   - App name: `BotNet Registry`
   - User support email: Your email
   - Developer contact: Your email
4. **Scopes**: Add `email`, `profile`, `openid`
5. **Test users**: Add emails for development (max 100)

⚠️ **IMPORTANT**: In testing mode, only test users can authenticate.
For public access, you need Google verification (weeks-long process).

### Alternative: Internal Mode
- Choose **Internal** if you have Google Workspace domain
- Restricts to users in your organization only
- No verification required

### 4. Create OAuth 2.0 Credentials
1. Go to **APIs & Services > Credentials**
2. Click **Create Credentials > OAuth 2.0 Client ID**
3. Application type: **Web application**
4. Name: `BotNet Registry Client`
5. **Authorized redirect URIs**:
   - `https://botnet.airon.games/auth/callback` (production)
   - `http://localhost:8080/auth/callback` (development)

### 5. Download Credentials
- Copy **Client ID** and **Client Secret**
- Update `.env` file:

```bash
GOOGLE_CLIENT_ID=123456789012-abcdef.apps.googleusercontent.com
GOOGLE_CLIENT_SECRET=GOCSPX-your-secret-here
```

## Domain Setup for Nodes

### 1. Register botnet.*.* Domain
- Purchase domain like `botnet.yourname.com`
- Point to your server IP (can be dynamic with DDNS)

### 2. DNS TXT Record for Ownership Verification
```bash
# Add TXT record to prove domain ownership
# Name: @
# Value: botnet-owner=your-google-user-id

# Verify with:
dig TXT botnet.yourname.com
```

### 3. SSL Certificate
```bash
# Use Let's Encrypt for HTTPS
certbot --nginx -d botnet.yourname.com
```

## Testing OAuth Integration

### Development Flow
1. Start registry: `go run cmd/registry/main.go`
2. Visit: `http://localhost:8080/auth/login`
3. Complete Google OAuth flow
4. Verify JWT token issuance

### Production Flow
1. Deploy to `botnet.airon.games`
2. Test both leaf and node registration
3. Verify domain ownership checks

## Cost Considerations

**Free Tier Includes:**
- Google OAuth: Free for reasonable usage
- Domain: ~$12/year for .com
- Let's Encrypt SSL: Free

**Total Cost:** ~$12/year per node domain

## Security Notes

- Store client secret securely (env vars, not code)
- Use HTTPS in production
- Rotate JWT secrets periodically
- Monitor OAuth quotas and usage

## Next Steps

1. Implement OAuth handlers in `internal/api/auth.go`
2. Add JWT middleware for protected routes
3. Create domain verification logic
4. Test end-to-end registration flows