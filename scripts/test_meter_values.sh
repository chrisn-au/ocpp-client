#!/bin/bash

# Test script for Meter Values functionality
# Usage: ./test_meter_values.sh [server_url] [client_id]

SERVER_URL=${1:-"http://localhost:8081"}
CLIENT_ID=${2:-"TEST-CP-001"}

echo "Testing Meter Values functionality..."
echo "===================================="

# Test 1: Get latest meter values
echo -e "\n1. Getting latest meter values for $CLIENT_ID..."
LATEST=$(curl -s "${SERVER_URL}/api/v1/chargepoints/${CLIENT_ID}/meter-values/latest?connectorId=1")
echo "$LATEST" | jq '.' || echo "$LATEST"

# Test 2: Get historical meter values
echo -e "\n2. Getting historical meter values..."
HISTORICAL=$(curl -s "${SERVER_URL}/api/v1/chargepoints/${CLIENT_ID}/meter-values?connectorId=1&limit=10")
echo "$HISTORICAL" | jq '.' || echo "$HISTORICAL"

# Test 3: Get aggregated values (hourly)
echo -e "\n3. Getting hourly aggregated values..."
HOURLY=$(curl -s "${SERVER_URL}/api/v1/chargepoints/${CLIENT_ID}/meter-values/aggregate?period=hour&connectorId=1")
echo "$HOURLY" | jq '.' || echo "$HOURLY"

# Test 4: Get aggregated values (daily)
echo -e "\n4. Getting daily aggregated values..."
DAILY=$(curl -s "${SERVER_URL}/api/v1/chargepoints/${CLIENT_ID}/meter-values/aggregate?period=day&connectorId=1")
echo "$DAILY" | jq '.' || echo "$DAILY"

# Test 5: Get alert thresholds
echo -e "\n5. Getting configured alert thresholds..."
THRESHOLDS=$(curl -s "${SERVER_URL}/api/v1/alerts/thresholds")
echo "$THRESHOLDS" | jq '.' || echo "$THRESHOLDS"

# Test 6: Check meter value configuration
echo -e "\n6. Checking meter value configuration..."
CONFIG=$(curl -s "${SERVER_URL}/api/v1/chargepoints/${CLIENT_ID}/configuration?keys=MeterValuesSampledData,MeterValueSampleInterval")
echo "$CONFIG" | jq '.' || echo "$CONFIG"

echo -e "\nMeter Values testing complete!"
echo "===================================="

# Summary
echo -e "\nTest Summary:"
echo "- Latest values retrieved: $(echo "$LATEST" | jq -r '.success 2>/dev/null || echo "unknown"')"
echo "- Historical values available: $(echo "$HISTORICAL" | jq -r '.data | length 2>/dev/null || echo "unknown"')"
echo "- Aggregation working: $(echo "$HOURLY" | jq -r '.success 2>/dev/null || echo "unknown"')"