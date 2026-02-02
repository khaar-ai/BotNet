#!/bin/sh
# Certificate expiry monitoring script for BotNet Direct HTTPS mode

# Configuration
DOMAIN="${BOT_DOMAIN:-localhost}"
ALERT_EMAIL="${ALERT_EMAIL:-admin@localhost}"
WARNING_DAYS="${WARNING_DAYS:-14}"
CRITICAL_DAYS="${CRITICAL_DAYS:-7}"

# Colors for output
RED='\033[0;31m'
YELLOW='\033[1;33m'
GREEN='\033[0;32m'
NC='\033[0m' # No Color

# Function to check certificate expiry
check_cert_expiry() {
    local domain=$1
    local port=${2:-443}
    
    # Get certificate expiry date
    cert_expiry=$(echo | openssl s_client -servername "$domain" -connect "$domain:$port" 2>/dev/null | \
                  openssl x509 -noout -enddate 2>/dev/null | \
                  cut -d= -f2)
    
    if [ -z "$cert_expiry" ]; then
        echo -e "${RED}ERROR: Could not retrieve certificate for $domain:$port${NC}"
        return 1
    fi
    
    # Convert to epoch
    if date --version >/dev/null 2>&1; then
        # GNU date
        expiry_epoch=$(date -d "$cert_expiry" +%s)
        current_epoch=$(date +%s)
    else
        # BSD date (macOS/Alpine)
        expiry_epoch=$(date -j -f "%b %d %T %Y %Z" "$cert_expiry" +%s 2>/dev/null)
        current_epoch=$(date +%s)
    fi
    
    # Calculate days remaining
    days_remaining=$(( ($expiry_epoch - $current_epoch) / 86400 ))
    
    # Determine status
    if [ $days_remaining -lt $CRITICAL_DAYS ]; then
        status="CRITICAL"
        color=$RED
    elif [ $days_remaining -lt $WARNING_DAYS ]; then
        status="WARNING"
        color=$YELLOW
    else
        status="OK"
        color=$GREEN
    fi
    
    # Output status
    echo -e "${color}Certificate Status: $status${NC}"
    echo "Domain: $domain"
    echo "Expires: $cert_expiry"
    echo "Days remaining: $days_remaining"
    
    # Send alert if needed
    if [ "$status" != "OK" ] && [ -n "$ALERT_EMAIL" ]; then
        send_alert "$domain" "$status" "$days_remaining" "$cert_expiry"
    fi
    
    # Return status code
    case $status in
        OK) return 0 ;;
        WARNING) return 1 ;;
        CRITICAL) return 2 ;;
    esac
}

# Function to send alert (customize based on your notification method)
send_alert() {
    local domain=$1
    local status=$2
    local days=$3
    local expiry=$4
    
    # Log to stderr (will be captured by Docker logs)
    echo "ALERT: Certificate $status for $domain - expires in $days days ($expiry)" >&2
    
    # If you have mail configured, uncomment:
    # echo "Certificate $status for $domain - expires in $days days ($expiry)" | \
    #     mail -s "BotNet Certificate Alert: $status" "$ALERT_EMAIL"
    
    # Or use a webhook:
    # curl -X POST https://your-webhook-url \
    #     -H "Content-Type: application/json" \
    #     -d "{\"domain\":\"$domain\",\"status\":\"$status\",\"days\":$days,\"expiry\":\"$expiry\"}"
}

# Function to check Let's Encrypt certificates from filesystem
check_letsencrypt_cert() {
    local cert_path="/var/lib/botnet/letsencrypt/certificates/$DOMAIN.crt"
    
    if [ ! -f "$cert_path" ]; then
        cert_path="/etc/botnet/certs/fullchain.pem"
    fi
    
    if [ ! -f "$cert_path" ]; then
        echo -e "${YELLOW}No certificate file found at expected paths${NC}"
        return 0
    fi
    
    # Check certificate file directly
    cert_expiry=$(openssl x509 -enddate -noout -in "$cert_path" 2>/dev/null | cut -d= -f2)
    
    if [ -z "$cert_expiry" ]; then
        echo -e "${RED}ERROR: Could not read certificate from $cert_path${NC}"
        return 1
    fi
    
    # Same expiry calculation as above
    if date --version >/dev/null 2>&1; then
        expiry_epoch=$(date -d "$cert_expiry" +%s)
        current_epoch=$(date +%s)
    else
        expiry_epoch=$(date -j -f "%b %d %T %Y %Z" "$cert_expiry" +%s 2>/dev/null)
        current_epoch=$(date +%s)
    fi
    
    days_remaining=$(( ($expiry_epoch - $current_epoch) / 86400 ))
    
    echo "Certificate file check:"
    echo "Path: $cert_path"
    echo "Expires: $cert_expiry"
    echo "Days remaining: $days_remaining"
}

# Main monitoring logic
echo "=== BotNet Certificate Expiry Check ==="
echo "Time: $(date)"
echo "Domain: $DOMAIN"
echo ""

# Check the running service certificate
echo "Checking live certificate..."
check_cert_expiry "$DOMAIN" 443

echo ""

# Also check filesystem certificates
echo "Checking certificate files..."
check_letsencrypt_cert

echo ""
echo "=== Check complete ===