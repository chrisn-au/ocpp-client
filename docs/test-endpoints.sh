#!/bin/bash

# OCPP Server REST API Test Script
# Tests all non-legacy endpoints documented in end-point.md

# Configuration
BASE_URL="http://localhost:8083"
CLIENT_ID="TEST-CP-001"
CONNECTOR_ID="1"
TRANSACTION_ID="190963"
ID_TAG="TEST-USER-001"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Function to print section headers
print_section() {
    echo -e "\n${BLUE}===============================================${NC}"
    echo -e "${BLUE}$1${NC}"
    echo -e "${BLUE}===============================================${NC}\n"
}

# Function to print test descriptions
print_test() {
    echo -e "${YELLOW}Testing: $1${NC}"
}

# Function to make HTTP requests with pretty output
make_request() {
    local method=$1
    local endpoint=$2
    local data=$3
    local description=$4

    print_test "$description"
    echo -e "${BLUE}$method $endpoint${NC}"

    if [ "$method" = "GET" ]; then
        response=$(curl -s -w "HTTPSTATUS:%{http_code}" \
                        -H "Content-Type: application/json" \
                        "$BASE_URL$endpoint")
        http_code=$(echo "$response" | tr -d '\n' | sed -e 's/.*HTTPSTATUS://')
        body=$(echo "$response" | sed -e 's/HTTPSTATUS:.*//g')
        echo "$body" | jq . 2>/dev/null || echo "$body"
        echo -e "\n${GREEN}HTTP Status: $http_code${NC}"
    else
        response=$(curl -s -w "HTTPSTATUS:%{http_code}" \
                        -X "$method" \
                        -H "Content-Type: application/json" \
                        -d "$data" \
                        "$BASE_URL$endpoint")
        http_code=$(echo "$response" | tr -d '\n' | sed -e 's/.*HTTPSTATUS://')
        body=$(echo "$response" | sed -e 's/HTTPSTATUS:.*//g')
        echo "$body" | jq . 2>/dev/null || echo "$body"
        echo -e "\n${GREEN}HTTP Status: $http_code${NC}"
    fi

    echo -e "\n${GREEN}---${NC}\n"
}

# Check if jq is installed
if ! command -v jq &> /dev/null; then
    echo -e "${RED}Error: jq is required but not installed. Please install jq first.${NC}"
    echo "On macOS: brew install jq"
    echo "On Ubuntu/Debian: sudo apt-get install jq"
    exit 1
fi

# Check if curl is installed
if ! command -v curl &> /dev/null; then
    echo -e "${RED}Error: curl is required but not installed.${NC}"
    exit 1
fi

echo -e "${GREEN}OCPP Server REST API Test Suite${NC}"
echo -e "${GREEN}Base URL: $BASE_URL${NC}"
echo -e "${GREEN}Test Client ID: $CLIENT_ID${NC}"
echo -e "${GREEN}Test ID Tag: $ID_TAG${NC}\n"

# =============================================================================
# HEALTH AND SYSTEM INFORMATION
# =============================================================================

print_section "HEALTH AND SYSTEM INFORMATION"

make_request "GET" "/health" "" "Health check"

make_request "GET" "/clients" "" "Get connected clients"

# =============================================================================
# CHARGE POINT INFORMATION
# =============================================================================

print_section "CHARGE POINT INFORMATION"

make_request "GET" "/chargepoints" "" "Get all charge points"

make_request "GET" "/chargepoints/$CLIENT_ID" "" "Get specific charge point"

make_request "GET" "/chargepoints/$CLIENT_ID/connectors" "" "Get all connectors for charge point"

make_request "GET" "/chargepoints/$CLIENT_ID/connectors/$CONNECTOR_ID" "" "Get specific connector"

make_request "GET" "/api/v1/chargepoints/$CLIENT_ID/status" "" "Get charge point online status"

# =============================================================================
# TRANSACTION INFORMATION
# =============================================================================

print_section "TRANSACTION INFORMATION"

make_request "GET" "/transactions" "" "Get all transactions"

make_request "GET" "/transactions?clientId=$CLIENT_ID" "" "Get transactions filtered by client"

make_request "GET" "/transactions/$TRANSACTION_ID" "" "Get specific transaction"

# =============================================================================
# REMOTE TRANSACTION CONTROL (NEW API)
# =============================================================================

print_section "REMOTE TRANSACTION CONTROL (NEW API)"

# Remote Start Transaction
REMOTE_START_DATA='{
  "clientId": "'$CLIENT_ID'",
  "connectorId": '$CONNECTOR_ID',
  "idTag": "'$ID_TAG'"
}'

make_request "POST" "/api/v1/transactions/remote-start" "$REMOTE_START_DATA" "Remote start transaction (new API)"

# Remote Stop Transaction
REMOTE_STOP_DATA='{
  "clientId": "'$CLIENT_ID'",
  "transactionId": '$TRANSACTION_ID'
}'

make_request "POST" "/api/v1/transactions/remote-stop" "$REMOTE_STOP_DATA" "Remote stop transaction (new API)"

# Remote Stop without clientId (auto-detect)
REMOTE_STOP_AUTO_DATA='{
  "transactionId": '$TRANSACTION_ID'
}'

make_request "POST" "/api/v1/transactions/remote-stop" "$REMOTE_STOP_AUTO_DATA" "Remote stop transaction (auto-detect client)"

# =============================================================================
# CONFIGURATION MANAGEMENT
# =============================================================================

print_section "CONFIGURATION MANAGEMENT"

make_request "GET" "/api/v1/chargepoints/$CLIENT_ID/configuration" "" "Get all stored configuration"

make_request "GET" "/api/v1/chargepoints/$CLIENT_ID/configuration?keys=MeterValueSampleInterval,HeartbeatInterval" "" "Get specific configuration keys"

# Change Configuration
CHANGE_CONFIG_DATA='{
  "key": "MeterValueSampleInterval",
  "value": "30"
}'

make_request "PUT" "/api/v1/chargepoints/$CLIENT_ID/configuration" "$CHANGE_CONFIG_DATA" "Change stored configuration"

make_request "GET" "/api/v1/chargepoints/$CLIENT_ID/configuration/export" "" "Export all configuration"

# =============================================================================
# LIVE CONFIGURATION MANAGEMENT
# =============================================================================

print_section "LIVE CONFIGURATION MANAGEMENT"

make_request "GET" "/api/v1/chargepoints/$CLIENT_ID/configuration/live" "" "Get all live configuration"

make_request "GET" "/api/v1/chargepoints/$CLIENT_ID/configuration/live?keys=MeterValueSampleInterval,HeartbeatInterval" "" "Get specific live configuration keys"

# Change Live Configuration
CHANGE_LIVE_CONFIG_DATA='{
  "key": "MeterValueSampleInterval",
  "value": "60"
}'

make_request "PUT" "/api/v1/chargepoints/$CLIENT_ID/configuration/live" "$CHANGE_LIVE_CONFIG_DATA" "Change live configuration"

# =============================================================================
# ERROR TESTING
# =============================================================================

print_section "ERROR TESTING"

make_request "GET" "/chargepoints/NONEXISTENT-CP" "" "Test 404 - Nonexistent charge point"

make_request "GET" "/chargepoints/$CLIENT_ID/connectors/999" "" "Test 400 - Invalid connector ID"

make_request "GET" "/transactions/999999" "" "Test 404 - Nonexistent transaction"

# Invalid Remote Start (missing required fields)
INVALID_REMOTE_START='{
  "clientId": "'$CLIENT_ID'"
}'

make_request "POST" "/api/v1/transactions/remote-start" "$INVALID_REMOTE_START" "Test 400 - Invalid remote start (missing idTag)"

# Invalid Remote Stop (invalid transaction ID)
INVALID_REMOTE_STOP='{
  "transactionId": 0
}'

make_request "POST" "/api/v1/transactions/remote-stop" "$INVALID_REMOTE_STOP" "Test 400 - Invalid remote stop (invalid transaction ID)"

# Invalid Configuration Change (missing fields)
INVALID_CONFIG='{
  "key": "TestKey"
}'

make_request "PUT" "/api/v1/chargepoints/$CLIENT_ID/configuration" "$INVALID_CONFIG" "Test 400 - Invalid configuration change (missing value)"

echo -e "\n${GREEN}===============================================${NC}"
echo -e "${GREEN}Test Suite Complete!${NC}"
echo -e "${GREEN}===============================================${NC}\n"

echo -e "${YELLOW}Notes:${NC}"
echo -e "- Some endpoints may return 404/503 errors if the charge point is not connected"
echo -e "- Live configuration endpoints require the charge point to be online"
echo -e "- Transaction-related endpoints depend on actual transaction state"
echo -e "- Modify CLIENT_ID, TRANSACTION_ID, and ID_TAG variables as needed"
echo -e "- Update BASE_URL if the server is running on a different port\n"