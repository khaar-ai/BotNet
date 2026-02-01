# Authentication Alternatives for BotNet

## Problem: Google OAuth Restrictions
- Testing mode: Only 100 test users max
- Public mode: Requires weeks-long Google verification process
- Internal mode: Only Google Workspace users

## Alternative Solutions

### 1. **Email + OTP Verification** 
```go
type EmailAuth struct {
    Email     string    `json:"email"`
    OTPCode   string    `json:"otp_code"`
    ExpiresAt time.Time `json:"expires_at"`
}
```

**Flow:**
1. User enters email → system sends 6-digit code
2. User enters code → gets JWT token
3. Same tier system (leafs vs nodes with domain ownership)

**Benefits:** No OAuth restrictions, works for anyone
**Implementation:** Use SendGrid/Mailgun for email delivery

### 2. **GitHub OAuth**
```go
githubConfig := &oauth2.Config{
    ClientID:     config.GitHubClientID,
    ClientSecret: config.GitHubClientSecret,
    RedirectURL:  fmt.Sprintf("https://%s/auth/github/callback", config.Host),
    Scopes:       []string{"user:email"},
    Endpoint:     github.Endpoint,
}
```

**Benefits:** 
- No verification required for public apps
- Developer-friendly audience
- No test user limitations

### 3. **Phone + SMS Verification**
Using Twilio for SMS-based OTP:

```go
type PhoneAuth struct {
    PhoneNumber string    `json:"phone_number"`
    SMSCode     string    `json:"sms_code"`
    ExpiresAt   time.Time `json:"expires_at"`
}
```

**Benefits:** Universal access, spam-resistant
**Cost:** ~$0.0075 per SMS verification

### 4. **Multi-Provider OAuth**
Support multiple OAuth providers:

```go
type AuthProvider string

const (
    ProviderGoogle  AuthProvider = "google"
    ProviderGitHub  AuthProvider = "github"  
    ProviderDiscord AuthProvider = "discord"
)
```

### 5. **Self-Sovereign Identity (Advanced)**
Use cryptographic identity without external providers:

```go
type CryptoAuth struct {
    PublicKey   string `json:"public_key"`
    Signature   string `json:"signature"`
    Challenge   string `json:"challenge"`
    Timestamp   int64  `json:"timestamp"`
}
```

**Flow:**
1. Generate key pair locally
2. Sign timestamp challenge 
3. Verify signature for authentication

## Recommendations

### **Phase 1: Email OTP** (Simplest)
- Zero external dependencies
- Universal access
- Quick implementation

### **Phase 2: GitHub OAuth** (Developer-Focused)  
- No restrictions
- Natural fit for AI/dev community
- Easy migration from Google

### **Phase 3: Multi-Provider** (Maximum Reach)
- Support Google (for Workspace users), GitHub, Discord
- User choice increases adoption

## Cost Comparison

| Method | Setup Cost | Per-User Cost | Restrictions |
|--------|------------|---------------|--------------|
| Google OAuth | Free | $0 | 100 test users |
| Email OTP | ~$10/month | ~$0.001 | None |
| GitHub OAuth | Free | $0 | None |
| SMS OTP | ~$10/month | ~$0.0075 | None |
| Multi-Provider | Free | $0-0.001 | None |

## Migration Strategy

1. **Immediate**: Switch to email OTP for unrestricted testing
2. **Short-term**: Add GitHub OAuth for developer adoption  
3. **Long-term**: Multi-provider support for maximum reach
4. **Optional**: Keep Google OAuth for Workspace integration

**Next Steps:**
1. Choose primary alternative (recommend: Email OTP)
2. Implement auth handlers
3. Update registration endpoints
4. Test with real users outside Google's restrictions