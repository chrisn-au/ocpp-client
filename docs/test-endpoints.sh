#!/bin/bash

# OCPP Server REST API Test Script
# Interactive menu for testing endpoints documented in end-point.md

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
CYAN='\033[0;36m'
MAGENTA='\033[0;35m'
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

# Function to display main menu
show_main_menu() {
    clear
    echo -e "${GREEN}╔══════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${GREEN}║                OCPP Server API Test Menu                     ║${NC}"
    echo -e "${GREEN}╚══════════════════════════════════════════════════════════════╝${NC}"
    echo -e "${CYAN}Base URL: $BASE_URL${NC}"
    echo -e "${CYAN}Client ID: $CLIENT_ID | ID Tag: $ID_TAG${NC}\n"

    echo -e "${YELLOW}🚀 QUICK ACCESS (Most Frequently Used):${NC}"
    echo -e "  ${MAGENTA}1)${NC} Remote Start Transaction"
    echo -e "  ${MAGENTA}2)${NC} Remote Stop Transaction"
    echo -e "  ${MAGENTA}3)${NC} Check Connected Clients"
    echo -e "  ${MAGENTA}4)${NC} Get Active Transactions"
    echo ""

    echo -e "${YELLOW}📋 FULL TEST SUITES:${NC}"
    echo -e "  ${BLUE}5)${NC} Health & System Information"
    echo -e "  ${BLUE}6)${NC} Charge Point Management"
    echo -e "  ${BLUE}7)${NC} Transaction Information"
    echo -e "  ${BLUE}8)${NC} Configuration Management"
    echo -e "  ${BLUE}9)${NC} Live Configuration Management"
    echo -e "  ${BLUE}10)${NC} Error Testing"
    echo ""

    echo -e "${YELLOW}🔧 UTILITIES:${NC}"
    echo -e "  ${GREEN}11)${NC} Run All Tests (Full Suite)"
    echo -e "  ${GREEN}12)${NC} Update Configuration"
    echo -e "  ${GREEN}0)${NC} Exit"
    echo ""
    echo -n "Select an option: "
}

# Function for remote start transaction
test_remote_start() {
    print_section "🚀 REMOTE START TRANSACTION"

    REMOTE_START_DATA='{
      "clientId": "'$CLIENT_ID'",
      "connectorId": '$CONNECTOR_ID',
      "idTag": "'$ID_TAG'"
    }'

    make_request "POST" "/api/v1/transactions/remote-start" "$REMOTE_START_DATA" "Remote start transaction"

    echo -e "\n${YELLOW}Press Enter to return to menu...${NC}"
    read
}

# Function for remote stop transaction
test_remote_stop() {
    print_section "🛑 REMOTE STOP TRANSACTION"

    REMOTE_STOP_DATA='{
      "clientId": "'$CLIENT_ID'",
      "transactionId": '$TRANSACTION_ID'
    }'

    make_request "POST" "/api/v1/transactions/remote-stop" "$REMOTE_STOP_DATA" "Remote stop transaction"

    echo -e "\n${YELLOW}Press Enter to return to menu...${NC}"
    read
}

# Function for connected clients
test_connected_clients() {
    print_section "📡 CONNECTED CLIENTS"
    make_request "GET" "/clients" "" "Get connected clients"

    echo -e "\n${YELLOW}Press Enter to return to menu...${NC}"
    read
}

# Function for active transactions
test_active_transactions() {
    print_section "📊 ACTIVE TRANSACTIONS"
    make_request "GET" "/api/v1/transactions" "" "Get all transactions"

    echo -e "\n${YELLOW}Press Enter to return to menu...${NC}"
    read
}

# Function for health & system tests
test_health_system() {
    print_section "🏥 HEALTH & SYSTEM INFORMATION"
    make_request "GET" "/health" "" "Health check"
    make_request "GET" "/clients" "" "Get connected clients"

    echo -e "\n${YELLOW}Press Enter to return to menu...${NC}"
    read
}

# Function for charge point tests
test_charge_points() {
    print_section "🔌 CHARGE POINT MANAGEMENT"
    make_request "GET" "/api/v1/chargepoints" "" "Get all charge points"
    make_request "GET" "/api/v1/chargepoints/$CLIENT_ID" "" "Get specific charge point"
    make_request "GET" "/api/v1/chargepoints/$CLIENT_ID/connectors" "" "Get all connectors for charge point"
    make_request "GET" "/api/v1/chargepoints/$CLIENT_ID/connectors/$CONNECTOR_ID" "" "Get specific connector"
    make_request "GET" "/api/v1/chargepoints/$CLIENT_ID/status" "" "Get charge point online status"

    echo -e "\n${YELLOW}Press Enter to return to menu...${NC}"
    read
}

# Function for transaction tests
test_transactions() {
    print_section "📊 TRANSACTION INFORMATION"
    make_request "GET" "/api/v1/transactions" "" "Get all transactions"
    make_request "GET" "/api/v1/transactions?clientId=$CLIENT_ID" "" "Get transactions filtered by client"
    make_request "GET" "/api/v1/transactions/$TRANSACTION_ID" "" "Get specific transaction"

    echo -e "\n${YELLOW}Press Enter to return to menu...${NC}"
    read
}

# Function for configuration tests
test_configuration() {
    print_section "⚙️ CONFIGURATION MANAGEMENT"
    make_request "GET" "/api/v1/chargepoints/$CLIENT_ID/configuration" "" "Get all stored configuration"
    make_request "GET" "/api/v1/chargepoints/$CLIENT_ID/configuration?keys=MeterValueSampleInterval,HeartbeatInterval" "" "Get specific configuration keys"

    CHANGE_CONFIG_DATA='{
      "key": "MeterValueSampleInterval",
      "value": "30"
    }'
    make_request "PUT" "/api/v1/chargepoints/$CLIENT_ID/configuration" "$CHANGE_CONFIG_DATA" "Change stored configuration"
    make_request "GET" "/api/v1/chargepoints/$CLIENT_ID/configuration/export" "" "Export all configuration"

    echo -e "\n${YELLOW}Press Enter to return to menu...${NC}"
    read
}

# Function for live configuration tests
test_live_configuration() {
    print_section "📡 LIVE CONFIGURATION MANAGEMENT"
    make_request "GET" "/api/v1/chargepoints/$CLIENT_ID/configuration/live" "" "Get all live configuration"
    make_request "GET" "/api/v1/chargepoints/$CLIENT_ID/configuration/live?keys=MeterValueSampleInterval,HeartbeatInterval" "" "Get specific live configuration keys"

    CHANGE_LIVE_CONFIG_DATA='{
      "key": "MeterValueSampleInterval",
      "value": "60"
    }'
    make_request "PUT" "/api/v1/chargepoints/$CLIENT_ID/configuration/live" "$CHANGE_LIVE_CONFIG_DATA" "Change live configuration"

    echo -e "\n${YELLOW}Press Enter to return to menu...${NC}"
    read
}

# Function for error tests
test_errors() {
    print_section "❌ ERROR TESTING"
    make_request "GET" "/api/v1/chargepoints/NONEXISTENT-CP" "" "Test 404 - Nonexistent charge point"
    make_request "GET" "/api/v1/chargepoints/$CLIENT_ID/connectors/999" "" "Test 400 - Invalid connector ID"
    make_request "GET" "/api/v1/transactions/999999" "" "Test 404 - Nonexistent transaction"

    INVALID_REMOTE_START='{
      "clientId": "'$CLIENT_ID'"
    }'
    make_request "POST" "/api/v1/transactions/remote-start" "$INVALID_REMOTE_START" "Test 400 - Invalid remote start (missing idTag)"

    INVALID_REMOTE_STOP='{
      "transactionId": 0
    }'
    make_request "POST" "/api/v1/transactions/remote-stop" "$INVALID_REMOTE_STOP" "Test 400 - Invalid remote stop (invalid transaction ID)"

    INVALID_CONFIG='{
      "key": "TestKey"
    }'
    make_request "PUT" "/api/v1/chargepoints/$CLIENT_ID/configuration" "$INVALID_CONFIG" "Test 400 - Invalid configuration change (missing value)"

    echo -e "\n${YELLOW}Press Enter to return to menu...${NC}"
    read
}

# Function to run all tests
run_all_tests() {
    echo -e "${GREEN}🏃 Running Full Test Suite...${NC}\n"
    test_health_system
    test_charge_points
    test_transactions
    test_configuration
    test_live_configuration
    test_errors
}

# Function to update configuration
update_config() {
    clear
    echo -e "${CYAN}⚙️ Update Test Configuration${NC}\n"

    echo -e "Current settings:"
    echo -e "Base URL: ${YELLOW}$BASE_URL${NC}"
    echo -e "Client ID: ${YELLOW}$CLIENT_ID${NC}"
    echo -e "Connector ID: ${YELLOW}$CONNECTOR_ID${NC}"
    echo -e "Transaction ID: ${YELLOW}$TRANSACTION_ID${NC}"
    echo -e "ID Tag: ${YELLOW}$ID_TAG${NC}\n"

    echo -n "New Base URL (or press Enter to keep current): "
    read new_url
    if [ ! -z "$new_url" ]; then
        BASE_URL="$new_url"
    fi

    echo -n "New Client ID (or press Enter to keep current): "
    read new_client
    if [ ! -z "$new_client" ]; then
        CLIENT_ID="$new_client"
    fi

    echo -n "New Transaction ID (or press Enter to keep current): "
    read new_transaction
    if [ ! -z "$new_transaction" ]; then
        TRANSACTION_ID="$new_transaction"
    fi

    echo -n "New ID Tag (or press Enter to keep current): "
    read new_tag
    if [ ! -z "$new_tag" ]; then
        ID_TAG="$new_tag"
    fi

    echo -e "\n${GREEN}Configuration updated!${NC}"
    echo -e "\n${YELLOW}Press Enter to return to menu...${NC}"
    read
}

# Main menu loop
while true; do
    show_main_menu
    read choice

    case $choice in
        1) test_remote_start ;;
        2) test_remote_stop ;;
        3) test_connected_clients ;;
        4) test_active_transactions ;;
        5) test_health_system ;;
        6) test_charge_points ;;
        7) test_transactions ;;
        8) test_configuration ;;
        9) test_live_configuration ;;
        10) test_errors ;;
        11) run_all_tests ;;
        12) update_config ;;
        0)
            echo -e "\n${GREEN}Thank you for using the OCPP API Test Menu!${NC}"
            echo -e "${YELLOW}Notes:${NC}"
            echo -e "- Some endpoints may return 404/503 errors if the charge point is not connected"
            echo -e "- Live configuration endpoints require the charge point to be online"
            echo -e "- Transaction-related endpoints depend on actual transaction state"
            exit 0
            ;;
        *)
            echo -e "${RED}Invalid option. Please try again.${NC}"
            sleep 1
            ;;
    esac
done