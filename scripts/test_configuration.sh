#!/bin/bash

# Test script for Configuration Management
# Usage: ./test_configuration.sh [server_url] [client_id]

SERVER_URL=${1:-"http://localhost:8081"}
CLIENT_ID=${2:-"TEST-CP-001"}

echo "Testing Configuration Management..."
echo "================================="
echo "Server URL: $SERVER_URL"
echo "Client ID: $CLIENT_ID"

# Check if jq is available
if ! command -v jq &> /dev/null; then
    echo "Error: jq is required but not installed."
    echo "Please install jq: https://stedolan.github.io/jq/"
    exit 1
fi

# Check if curl is available
if ! command -v curl &> /dev/null; then
    echo "Error: curl is required but not installed."
    exit 1
fi

# Test 1: Get all configuration via REST API
echo -e "\n1. Getting all configuration for $CLIENT_ID..."
ALL_CONFIG=$(curl -s "${SERVER_URL}/api/v1/chargepoints/${CLIENT_ID}/configuration")
if [ $? -eq 0 ]; then
    echo "✓ Successfully retrieved configuration"
    echo "$ALL_CONFIG" | jq '.'

    # Check if response has success field
    SUCCESS=$(echo "$ALL_CONFIG" | jq -r '.success // false')
    if [ "$SUCCESS" = "true" ]; then
        echo "✓ Configuration retrieval succeeded"
        CONFIG_COUNT=$(echo "$ALL_CONFIG" | jq '.data.configuration | length')
        echo "✓ Found $CONFIG_COUNT configuration keys"

        # Check for essential keys
        HAS_HEARTBEAT=$(echo "$ALL_CONFIG" | jq '.data.configuration | has("HeartbeatInterval")')
        if [ "$HAS_HEARTBEAT" = "true" ]; then
            echo "✓ HeartbeatInterval key found"
        else
            echo "✗ HeartbeatInterval key missing"
        fi
    else
        echo "✗ Configuration retrieval failed"
    fi
else
    echo "✗ Failed to connect to server"
fi

# Test 2: Get specific configuration keys
echo -e "\n2. Getting specific configuration keys..."
SPECIFIC_CONFIG=$(curl -s "${SERVER_URL}/api/v1/chargepoints/${CLIENT_ID}/configuration?keys=HeartbeatInterval,MeterValueSampleInterval")
if [ $? -eq 0 ]; then
    echo "✓ Successfully retrieved specific keys"
    echo "$SPECIFIC_CONFIG" | jq '.'

    SUCCESS=$(echo "$SPECIFIC_CONFIG" | jq -r '.success // false')
    if [ "$SUCCESS" = "true" ]; then
        CONFIG_COUNT=$(echo "$SPECIFIC_CONFIG" | jq '.data.configuration | length')
        echo "✓ Found $CONFIG_COUNT specific configuration keys"
    fi
else
    echo "✗ Failed to retrieve specific keys"
fi

# Test 3: Change configuration value
echo -e "\n3. Changing HeartbeatInterval to 600..."
CHANGE_RESULT=$(curl -s -X PUT "${SERVER_URL}/api/v1/chargepoints/${CLIENT_ID}/configuration" \
  -H "Content-Type: application/json" \
  -d '{
    "key": "HeartbeatInterval",
    "value": "600"
  }')

if [ $? -eq 0 ]; then
    echo "✓ Configuration change request sent"
    echo "$CHANGE_RESULT" | jq '.'

    SUCCESS=$(echo "$CHANGE_RESULT" | jq -r '.success // false')
    STATUS=$(echo "$CHANGE_RESULT" | jq -r '.data.status // "unknown"')

    if [ "$SUCCESS" = "true" ]; then
        echo "✓ Configuration change request succeeded"
        echo "✓ Status: $STATUS"

        if [ "$STATUS" = "Accepted" ] || [ "$STATUS" = "RebootRequired" ]; then
            echo "✓ Configuration change was accepted"
        else
            echo "⚠ Configuration change status: $STATUS"
        fi
    else
        echo "✗ Configuration change failed"
    fi
else
    echo "✗ Failed to send configuration change request"
fi

# Test 4: Verify the change
echo -e "\n4. Verifying configuration change..."
VERIFY_CONFIG=$(curl -s "${SERVER_URL}/api/v1/chargepoints/${CLIENT_ID}/configuration?keys=HeartbeatInterval")
if [ $? -eq 0 ]; then
    echo "✓ Successfully retrieved HeartbeatInterval"
    echo "$VERIFY_CONFIG" | jq '.'

    SUCCESS=$(echo "$VERIFY_CONFIG" | jq -r '.success // false')
    if [ "$SUCCESS" = "true" ]; then
        HEARTBEAT_VALUE=$(echo "$VERIFY_CONFIG" | jq -r '.data.configuration.HeartbeatInterval.value // "unknown"')
        echo "✓ Current HeartbeatInterval value: $HEARTBEAT_VALUE"

        if [ "$HEARTBEAT_VALUE" = "600" ]; then
            echo "✓ Configuration change was successfully applied"
        else
            echo "⚠ Configuration value doesn't match expected (expected: 600, actual: $HEARTBEAT_VALUE)"
        fi
    fi
else
    echo "✗ Failed to verify configuration change"
fi

# Test 5: Try to change read-only configuration
echo -e "\n5. Attempting to change read-only key (should fail)..."
READONLY_RESULT=$(curl -s -X PUT "${SERVER_URL}/api/v1/chargepoints/${CLIENT_ID}/configuration" \
  -H "Content-Type: application/json" \
  -d '{
    "key": "ChargeProfileMaxStackLevel",
    "value": "20"
  }')

if [ $? -eq 0 ]; then
    echo "✓ Read-only change request sent"
    echo "$READONLY_RESULT" | jq '.'

    SUCCESS=$(echo "$READONLY_RESULT" | jq -r '.success // false')
    STATUS=$(echo "$READONLY_RESULT" | jq -r '.data.status // "unknown"')

    if [ "$SUCCESS" = "true" ]; then
        if [ "$STATUS" = "Rejected" ]; then
            echo "✓ Read-only key correctly rejected"
        else
            echo "⚠ Read-only key was not rejected (status: $STATUS)"
        fi
    else
        echo "✗ Read-only change request failed"
    fi
else
    echo "✗ Failed to send read-only change request"
fi

# Test 6: Try invalid value
echo -e "\n6. Attempting to set invalid value (should fail)..."
INVALID_RESULT=$(curl -s -X PUT "${SERVER_URL}/api/v1/chargepoints/${CLIENT_ID}/configuration" \
  -H "Content-Type: application/json" \
  -d '{
    "key": "HeartbeatInterval",
    "value": "not-a-number"
  }')

if [ $? -eq 0 ]; then
    echo "✓ Invalid value request sent"
    echo "$INVALID_RESULT" | jq '.'

    SUCCESS=$(echo "$INVALID_RESULT" | jq -r '.success // false')
    STATUS=$(echo "$INVALID_RESULT" | jq -r '.data.status // "unknown"')

    if [ "$SUCCESS" = "true" ]; then
        if [ "$STATUS" = "Rejected" ]; then
            echo "✓ Invalid value correctly rejected"
        else
            echo "⚠ Invalid value was not rejected (status: $STATUS)"
        fi
    fi
else
    echo "✗ Failed to send invalid value request"
fi

# Test 7: Check charger status
echo -e "\n7. Checking charger status..."
STATUS_RESULT=$(curl -s "${SERVER_URL}/api/v1/chargepoints/${CLIENT_ID}/status")
if [ $? -eq 0 ]; then
    echo "✓ Successfully retrieved charger status"
    echo "$STATUS_RESULT" | jq '.'

    IS_ONLINE=$(echo "$STATUS_RESULT" | jq -r '.data.online // false')
    echo "✓ Charger online status: $IS_ONLINE"
else
    echo "✗ Failed to retrieve charger status"
fi

# Test 8: Test live configuration (if charger is online)
echo -e "\n8. Testing live configuration query..."
LIVE_CONFIG_RESULT=$(curl -s "${SERVER_URL}/api/v1/chargepoints/${CLIENT_ID}/configuration/live")
if [ $? -eq 0 ]; then
    echo "✓ Live configuration request sent"
    echo "$LIVE_CONFIG_RESULT" | jq '.'

    SUCCESS=$(echo "$LIVE_CONFIG_RESULT" | jq -r '.success // false')
    if [ "$SUCCESS" = "true" ]; then
        echo "✓ Live configuration request accepted"
    else
        echo "⚠ Live configuration request failed (charger may be offline)"
    fi
else
    echo "✗ Failed to send live configuration request"
fi

# Test 9: Export all configuration
echo -e "\n9. Exporting complete configuration..."
EXPORT_CONFIG=$(curl -s "${SERVER_URL}/api/v1/chargepoints/${CLIENT_ID}/configuration/export")
if [ $? -eq 0 ]; then
    echo "✓ Successfully exported configuration"
    echo "$EXPORT_CONFIG" | jq '.'

    SUCCESS=$(echo "$EXPORT_CONFIG" | jq -r '.success // false')
    if [ "$SUCCESS" = "true" ]; then
        EXPORT_COUNT=$(echo "$EXPORT_CONFIG" | jq '.data | length')
        echo "✓ Exported $EXPORT_COUNT configuration keys"
    fi
else
    echo "✗ Failed to export configuration"
fi

echo -e "\nConfiguration testing complete!"
echo "================================="

# Summary
echo -e "\nTest Summary:"
ALL_SUCCESS=$(echo "$ALL_CONFIG" | jq -r '.success // false')
CHANGE_SUCCESS=$(echo "$CHANGE_RESULT" | jq -r '.success // false')
READONLY_SUCCESS=$(echo "$READONLY_RESULT" | jq -r '.success // false')
STATUS_SUCCESS=$(echo "$STATUS_RESULT" | jq -r '.success // false')
EXPORT_SUCCESS=$(echo "$EXPORT_CONFIG" | jq -r '.success // false')

echo "- All configuration retrieved: $ALL_SUCCESS"
echo "- Configuration changed: $CHANGE_SUCCESS"
echo "- Read-only rejection: $READONLY_SUCCESS"
echo "- Charger status checked: $STATUS_SUCCESS"
echo "- Configuration exported: $EXPORT_SUCCESS"

# Exit with error if any critical test failed
if [ "$ALL_SUCCESS" != "true" ] || [ "$CHANGE_SUCCESS" != "true" ] || [ "$EXPORT_SUCCESS" != "true" ]; then
    echo -e "\n❌ Some critical tests failed!"
    exit 1
else
    echo -e "\n✅ All critical tests passed!"
    exit 0
fi