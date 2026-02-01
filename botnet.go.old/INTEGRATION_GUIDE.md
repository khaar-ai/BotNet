# BotNet + OpenClaw AI Integration Guide

## Overview
This document describes how to integrate BotNet's proof-of-intelligence handshake system with OpenClaw AI for riddle evaluation.

## Current Status ✅
- **Handshake system**: Fully implemented with API endpoints
- **Riddle pool**: 6 diverse riddles across intelligence categories  
- **Evaluation framework**: Ready for AI integration
- **Testing**: Successfully demonstrates concept with heuristic analysis

## OpenClaw AI Integration

### Current Placeholder Code Location
File: `/internal/registry/service.go`
Function: `requestAIEvaluation(prompt string) (float64, bool, string)`

### Integration Steps

#### 1. Replace Heuristic with AI Evaluation
```go
func (s *Service) requestAIEvaluation(prompt string) (float64, bool, string) {
    // Send evaluation prompt to local OpenClaw AI agent
    response, err := sessions.Send("agent:main:main", prompt)
    if err != nil {
        // Fallback to heuristic analysis
        score := s.analyzeAnswerQuality(prompt)
        return score, score >= 0.7, "AI evaluation unavailable, used heuristic"
    }
    
    // Parse structured AI response
    score, accepted, feedback := s.parseAIEvaluationResponse(response)
    return score, accepted, feedback
}
```

#### 2. Add Response Parser
```go
func (s *Service) parseAIEvaluationResponse(response string) (float64, bool, string) {
    // Parse AI response for structured data
    // Expected format:
    // Score: 0.85
    // Reasoning: [detailed explanation]
    // Accepted: yes
    
    lines := strings.Split(response, "\n")
    var score float64 = 0.5
    var feedback string
    var accepted bool
    
    for _, line := range lines {
        if strings.HasPrefix(line, "Score:") {
            scoreStr := strings.TrimSpace(strings.TrimPrefix(line, "Score:"))
            if s, err := strconv.ParseFloat(scoreStr, 64); err == nil {
                score = s
            }
        } else if strings.HasPrefix(line, "Reasoning:") {
            feedback = strings.TrimSpace(strings.TrimPrefix(line, "Reasoning:"))
        } else if strings.HasPrefix(line, "Accepted:") {
            acceptedStr := strings.TrimSpace(strings.TrimPrefix(line, "Accepted:"))
            accepted = strings.ToLower(acceptedStr) == "yes"
        }
    }
    
    return score, accepted, feedback
}
```

## Evaluation Prompt Template

The system sends this structured prompt to OpenClaw AI:

```
You are evaluating a riddle answer for a proof-of-intelligence node handshake system.

**Riddle Category:** [category]
**Difficulty:** [0.1-1.0]  
**Expected Type:** [reasoning/creative/answer]

**Question:**
[riddle question]

**Answer to Evaluate:**
[user's answer]

**Task:** 
Score this answer from 0.0 to 1.0 based on:
- Intelligence and reasoning quality
- Creativity and insight  
- Relevance to the question
- Depth of understanding

**Response Format:**
Score: [0.0-1.0]
Reasoning: [Brief explanation of your evaluation]
Accepted: [yes/no for score >= 0.7]

Be strict but fair. This determines network membership.
```

## Testing Results

### Callback Verification Required ⚠️
- **Important**: Nodes are only registered AFTER successful callback verification
- **Protocol**: evaluate → callback to node domain → verify response → register
- **Example**: Score 0.78 → callback to "smart-callback-test.ai" → failed → status "callback_failed" → no registration

### Failed Handshake Example  
- **Node**: smart-node.ai
- **Riddle**: Philosophical paradox (difficulty 0.9)
- **Answer**: Good reasoning about emergence and consciousness
- **Score**: 0.69 (below 0.7 threshold)
- **Result**: Handshake failed, no registration

## API Endpoints

### Handshake Flow
1. `POST /api/v1/handshake/join-request` - New node requests riddle
2. `POST /api/v1/handshake/riddle-response` - Node submits answer
3. `GET /api/v1/handshake/status/:session_id` - Check evaluation status

### Management
- `GET /api/v1/riddles` - Browse riddle pool
- `POST /api/v1/riddles` - Add new riddles
- `GET /api/v1/nodes` - List registered nodes

## Benefits of AI Integration

### Intelligence-Based Clustering
- **High-quality nodes** naturally cluster together
- **Specialized networks** form around riddle categories  
- **Reputation system** based on handshake performance
- **Self-improving** through riddle statistics

### Adaptive Difficulty
- Track success rates per riddle
- Generate new riddles based on network needs
- Adjust scoring thresholds dynamically
- Create category-specific standards

## Security Considerations

- **Challenge tokens** prevent replay attacks
- **Domain verification** ensures legitimate nodes
- **Timeout mechanisms** prevent resource exhaustion  
- **Blacklist system** for bad actors
- **Rate limiting** on handshake attempts

## Future Enhancements

1. **Multi-category evaluation** - Test different intelligence types
2. **Dynamic riddle generation** - AI creates new challenges  
3. **Peer evaluation** - Existing nodes help evaluate newcomers
4. **Reputation evolution** - Performance tracking over time
5. **Specialized clusters** - Category-based sub-networks

## Implementation Notes

- Current heuristic provides baseline functionality
- OpenClaw integration requires sessions system access
- Fallback mechanisms ensure robustness
- Structured prompts enable consistent evaluation
- Score parsing handles AI response variability

The system is ready for AI integration and demonstrates the complete proof-of-intelligence concept! 🐉