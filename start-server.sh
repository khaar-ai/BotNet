#!/bin/bash

# Start BotNet Dragon Server
# Usage: ./start-botnet.sh [stop|restart|status]

PID_FILE="/tmp/botnet-server.pid"
LOG_FILE="/tmp/botnet-server.log"

case "${1:-start}" in
  start)
    if [ -f "$PID_FILE" ]; then
      if kill -0 $(cat "$PID_FILE") 2>/dev/null; then
        echo "🐉 BotNet server already running (PID: $(cat $PID_FILE))"
        exit 1
      else
        rm -f "$PID_FILE"
      fi
    fi
    
    echo "🐉 Starting BotNet server..."
    nohup node botnet-server.js > "$LOG_FILE" 2>&1 &
    echo $! > "$PID_FILE"
    echo "🐉 BotNet server started (PID: $(cat $PID_FILE))"
    echo "🐉 Logs: tail -f $LOG_FILE"
    ;;
    
  stop)
    if [ -f "$PID_FILE" ]; then
      PID=$(cat "$PID_FILE")
      echo "🐉 Stopping BotNet server (PID: $PID)..."
      kill "$PID" 2>/dev/null
      rm -f "$PID_FILE"
      echo "🐉 BotNet server stopped"
    else
      echo "🐉 BotNet server not running"
    fi
    ;;
    
  restart)
    $0 stop
    sleep 2
    $0 start
    ;;
    
  status)
    if [ -f "$PID_FILE" ]; then
      PID=$(cat "$PID_FILE")
      if kill -0 "$PID" 2>/dev/null; then
        echo "🐉 BotNet server running (PID: $PID)"
        curl -s http://localhost:8080/status | head -1
      else
        echo "🐉 BotNet server not running (stale PID file)"
        rm -f "$PID_FILE"
      fi
    else
      echo "🐉 BotNet server not running"
    fi
    ;;
    
  logs)
    if [ -f "$LOG_FILE" ]; then
      tail -f "$LOG_FILE"
    else
      echo "🐉 No log file found"
    fi
    ;;
    
  *)
    echo "Usage: $0 {start|stop|restart|status|logs}"
    exit 1
    ;;
esac