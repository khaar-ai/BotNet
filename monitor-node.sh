#!/bin/bash
#
# Monitor BotNet node and catch signals that cause it to exit
#

cd "$(dirname "$0")"

echo "🔍 Starting BotNet node with monitoring..."
echo "📝 Logging to monitor.log"

while true; do
    echo "$(date): Starting BotNet node..." | tee -a monitor.log
    
    # Start node and capture its PID
    ./node &
    NODE_PID=$!
    
    echo "$(date): Node started with PID $NODE_PID" | tee -a monitor.log
    
    # Wait for the process to exit
    wait $NODE_PID
    EXIT_CODE=$?
    
    echo "$(date): Node exited with code $EXIT_CODE" | tee -a monitor.log
    
    # Check what happened
    if [ $EXIT_CODE -eq 0 ]; then
        echo "$(date): Clean exit (probably received SIGTERM/SIGINT)" | tee -a monitor.log
    elif [ $EXIT_CODE -eq 130 ]; then
        echo "$(date): Received SIGINT (Ctrl+C)" | tee -a monitor.log
    elif [ $EXIT_CODE -eq 143 ]; then
        echo "$(date): Received SIGTERM" | tee -a monitor.log
    elif [ $EXIT_CODE -eq 137 ]; then
        echo "$(date): Killed by SIGKILL (OOM or force kill)" | tee -a monitor.log
    else
        echo "$(date): Unexpected exit code: $EXIT_CODE" | tee -a monitor.log
    fi
    
    echo "$(date): Waiting 5 seconds before restart..." | tee -a monitor.log
    sleep 5
done