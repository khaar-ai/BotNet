#!/usr/bin/env python3
"""
Mock BotNet Node Server for Testing Handshake Protocol
This simulates a real node that can receive callback verifications
"""

from flask import Flask, request, jsonify
import json
import time
from datetime import datetime

app = Flask(__name__)

# Store received handshake results
handshake_results = []

@app.route('/api/v1/handshake/result', methods=['POST'])
def receive_handshake_result():
    """Endpoint that receives handshake evaluation results from registry"""
    try:
        data = request.get_json()
        
        # Validate required fields
        required_fields = ['session_id', 'score', 'accepted', 'riddle_id']
        for field in required_fields:
            if field not in data:
                return jsonify({
                    "success": False,
                    "error": f"Missing required field: {field}"
                }), 400
        
        # Log the received result
        result = {
            "timestamp": datetime.now().isoformat(),
            "session_id": data['session_id'],
            "score": data['score'],
            "accepted": data['accepted'],
            "riddle_id": data['riddle_id'],
            "evaluator_id": data.get('evaluator_id', 'unknown'),
            "feedback": data.get('feedback', 'No feedback provided')
        }
        
        handshake_results.append(result)
        
        print(f"🤝 Handshake Result Received!")
        print(f"   Session: {result['session_id']}")
        print(f"   Score: {result['score']}")
        print(f"   Accepted: {result['accepted']}")
        print(f"   Feedback: {result['feedback']}")
        print()
        
        return jsonify({
            "success": True,
            "message": "Handshake result received successfully",
            "node_id": "test-mock-node",
            "timestamp": result['timestamp']
        })
        
    except Exception as e:
        print(f"❌ Error processing handshake result: {e}")
        return jsonify({
            "success": False,
            "error": str(e)
        }), 500

@app.route('/api/v1/info', methods=['GET'])
def node_info():
    """Provide information about this mock node"""
    return jsonify({
        "success": True,
        "data": {
            "node_id": "test-mock-node",
            "version": "1.0.0",
            "capabilities": ["handshake_testing", "callback_verification"],
            "status": "active",
            "handshake_results_received": len(handshake_results),
            "uptime": time.time()
        }
    })

@app.route('/api/v1/handshake/history', methods=['GET'])
def handshake_history():
    """Get history of received handshake results"""
    return jsonify({
        "success": True,
        "data": {
            "total_results": len(handshake_results),
            "results": handshake_results[-10:]  # Last 10 results
        }
    })

@app.route('/health', methods=['GET'])
def health_check():
    """Health check endpoint"""
    return jsonify({
        "status": "healthy",
        "timestamp": time.time(),
        "node_type": "mock_test_node"
    })

@app.route('/', methods=['GET'])
def root():
    """Root endpoint with node information"""
    return f"""
    <html>
    <head><title>Mock BotNet Test Node</title></head>
    <body style="font-family: monospace; background: #0f1419; color: #e6e6e6; padding: 20px;">
        <h1>🧪 Mock BotNet Test Node</h1>
        <p><strong>Status:</strong> Active and ready for handshake testing</p>
        <p><strong>Results Received:</strong> {len(handshake_results)}</p>
        <p><strong>Endpoints:</strong></p>
        <ul>
            <li><code>POST /api/v1/handshake/result</code> - Receive handshake results</li>
            <li><code>GET /api/v1/info</code> - Node information</li>
            <li><code>GET /api/v1/handshake/history</code> - Handshake history</li>
            <li><code>GET /health</code> - Health check</li>
        </ul>
        
        <h2>Recent Handshake Results:</h2>
        <pre>""" + json.dumps(handshake_results[-3:], indent=2) + """</pre>
    </body>
    </html>
    """

if __name__ == '__main__':
    print("🚀 Starting Mock BotNet Test Node Server...")
    print("   This simulates a real node that can receive handshake callbacks")
    print("   Listening on: http://localhost:8081")
    print("   Callback endpoint: http://localhost:8081/api/v1/handshake/result")
    print()
    
    app.run(host='0.0.0.0', port=8081, debug=True)