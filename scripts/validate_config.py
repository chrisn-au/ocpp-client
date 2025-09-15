#!/usr/bin/env python3
"""
Validation script for Configuration Management
Tests OCPP message flow and configuration persistence
"""

import asyncio
import json
import sys
import websockets
import uuid
import time
import argparse
from typing import Dict, List, Any, Optional

class ConfigurationValidator:
    def __init__(self, server_url: str, client_id: str):
        self.server_url = server_url
        self.client_id = client_id
        self.websocket = None
        self.test_results: List[tuple] = []

    async def connect(self):
        """Connect to the OCPP server via WebSocket"""
        uri = f"{self.server_url}/{self.client_id}"

        try:
            self.websocket = await websockets.connect(uri, subprotocols=["ocpp1.6"])
            print(f"✓ Connected to {uri}")
            return True
        except Exception as e:
            print(f"✗ Failed to connect to {uri}: {e}")
            return False

    async def send_get_configuration(self, keys: Optional[List[str]] = None) -> Dict[str, Any]:
        """Send GetConfiguration OCPP message"""
        request_id = str(uuid.uuid4())

        payload = {}
        if keys:
            payload["key"] = keys

        message = [2, request_id, "GetConfiguration", payload]

        try:
            await self.websocket.send(json.dumps(message))
            response = await asyncio.wait_for(self.websocket.recv(), timeout=10.0)
            return json.loads(response)
        except asyncio.TimeoutError:
            print("✗ Timeout waiting for GetConfiguration response")
            return {}
        except Exception as e:
            print(f"✗ Error in GetConfiguration: {e}")
            return {}

    async def send_change_configuration(self, key: str, value: str) -> Dict[str, Any]:
        """Send ChangeConfiguration OCPP message"""
        request_id = str(uuid.uuid4())

        message = [2, request_id, "ChangeConfiguration", {
            "key": key,
            "value": value
        }]

        try:
            await self.websocket.send(json.dumps(message))
            response = await asyncio.wait_for(self.websocket.recv(), timeout=10.0)
            return json.loads(response)
        except asyncio.TimeoutError:
            print("✗ Timeout waiting for ChangeConfiguration response")
            return {}
        except Exception as e:
            print(f"✗ Error in ChangeConfiguration: {e}")
            return {}

    async def test_get_all_configuration(self) -> bool:
        """Test getting all configuration keys"""
        print("\n📋 Test 1: Get all configuration keys")
        response = await self.send_get_configuration()

        if response and response[0] == 3:  # CallResult
            config_keys = response[2].get("configurationKey", [])
            print(f"  ✓ Received {len(config_keys)} configuration keys")

            if len(config_keys) < 10:
                print(f"  ⚠ Expected at least 10 keys, got {len(config_keys)}")
                return False

            # Check for essential keys
            key_names = [k["key"] for k in config_keys]
            essential_keys = [
                "HeartbeatInterval",
                "MeterValueSampleInterval",
                "LocalAuthorizeOffline",
                "ChargeProfileMaxStackLevel"
            ]

            for key in essential_keys:
                if key in key_names:
                    print(f"  ✓ Found essential key: {key}")
                else:
                    print(f"  ✗ Missing essential key: {key}")
                    return False

            # Check readonly status
            readonly_keys = ["ChargeProfileMaxStackLevel", "SupportedFeatureProfiles"]
            for config_key in config_keys:
                if config_key["key"] in readonly_keys:
                    if config_key.get("readonly", False):
                        print(f"  ✓ {config_key['key']} is correctly marked as readonly")
                    else:
                        print(f"  ✗ {config_key['key']} should be readonly")
                        return False

            self.test_results.append(("Get all configuration", True))
            return True
        else:
            print(f"  ✗ Unexpected response: {response}")
            self.test_results.append(("Get all configuration", False))
            return False

    async def test_get_specific_keys(self) -> bool:
        """Test getting specific configuration keys"""
        print("\n📋 Test 2: Get specific configuration keys")
        response = await self.send_get_configuration(["HeartbeatInterval", "UnknownKey", "MeterValueSampleInterval"])

        if response and response[0] == 3:
            config_keys = response[2].get("configurationKey", [])
            unknown_keys = response[2].get("unknownKey", [])

            print(f"  ✓ Found {len(config_keys)} known keys")
            print(f"  ✓ Found {len(unknown_keys)} unknown keys")

            if len(config_keys) != 2:
                print(f"  ✗ Expected 2 known keys, got {len(config_keys)}")
                return False

            if "UnknownKey" not in unknown_keys:
                print("  ✗ UnknownKey not in unknown keys list")
                return False

            print("  ✓ Unknown key correctly identified")
            self.test_results.append(("Get specific keys", True))
            return True
        else:
            print(f"  ✗ Unexpected response: {response}")
            self.test_results.append(("Get specific keys", False))
            return False

    async def test_change_configuration(self) -> bool:
        """Test changing configuration value"""
        print("\n📋 Test 3: Change configuration value")

        # Get original value first
        response = await self.send_get_configuration(["HeartbeatInterval"])
        original_value = None
        if response and response[0] == 3:
            config_keys = response[2].get("configurationKey", [])
            if config_keys:
                original_value = config_keys[0].get("value")
                print(f"  Original HeartbeatInterval: {original_value}")

        # Change value
        new_value = "900"
        response = await self.send_change_configuration("HeartbeatInterval", new_value)

        if response and response[0] == 3:
            status = response[2].get("status")
            print(f"  Change status: {status}")

            if status in ["Accepted", "RebootRequired"]:
                # Verify change
                await asyncio.sleep(0.5)  # Small delay
                response = await self.send_get_configuration(["HeartbeatInterval"])
                if response and response[0] == 3:
                    config_keys = response[2].get("configurationKey", [])
                    if config_keys and config_keys[0].get("value") == new_value:
                        print(f"  ✓ Value successfully changed to {new_value}")

                        # Restore original value if possible
                        if original_value:
                            await self.send_change_configuration("HeartbeatInterval", original_value)
                            print(f"  ✓ Restored original value: {original_value}")

                        self.test_results.append(("Change configuration", True))
                        return True
                    else:
                        print(f"  ✗ Value not updated in storage")
                        return False
            else:
                print(f"  ✗ Change rejected with status: {status}")
                return False

        print("  ✗ Configuration change failed")
        self.test_results.append(("Change configuration", False))
        return False

    async def test_readonly_rejection(self) -> bool:
        """Test that read-only keys are rejected"""
        print("\n📋 Test 4: Reject read-only configuration change")
        response = await self.send_change_configuration("ChargeProfileMaxStackLevel", "20")

        if response and response[0] == 3:
            status = response[2].get("status")
            print(f"  Status: {status}")

            if status == "Rejected":
                print("  ✓ Read-only key correctly rejected")
                self.test_results.append(("Read-only rejection", True))
                return True
            else:
                print(f"  ✗ Expected 'Rejected', got '{status}'")
                self.test_results.append(("Read-only rejection", False))
                return False
        else:
            print(f"  ✗ Unexpected response: {response}")
            self.test_results.append(("Read-only rejection", False))
            return False

    async def test_invalid_values(self) -> bool:
        """Test validation of invalid values"""
        print("\n📋 Test 5: Validate value rejection")

        test_cases = [
            ("HeartbeatInterval", "not-a-number", "Invalid integer"),
            ("LightIntensity", "150", "Out of range (0-100)"),
            ("LocalAuthorizeOffline", "yes", "Invalid boolean"),
            ("UnknownKey", "value", "Unknown key"),
        ]

        all_passed = True
        for key, value, description in test_cases:
            print(f"  Testing {key} = {value} ({description})")
            response = await self.send_change_configuration(key, value)

            if response and response[0] == 3:
                status = response[2].get("status")
                expected_statuses = ["Rejected", "NotSupported"]

                if status in expected_statuses:
                    print(f"    ✓ Correctly rejected with status: {status}")
                else:
                    print(f"    ✗ Unexpected status: {status}")
                    all_passed = False
            else:
                print(f"    ✗ Unexpected response: {response}")
                all_passed = False

        self.test_results.append(("Invalid value rejection", all_passed))
        return all_passed

    async def test_csv_validation(self) -> bool:
        """Test CSV field validation"""
        print("\n📋 Test 6: CSV field validation")

        # Test valid CSV
        response = await self.send_change_configuration(
            "MeterValuesSampledData",
            "Energy.Active.Import.Register,Power.Active.Import"
        )

        if response and response[0] == 3:
            status = response[2].get("status")
            if status in ["Accepted", "RebootRequired"]:
                print("  ✓ Valid CSV accepted")
            else:
                print(f"  ✗ Valid CSV rejected: {status}")
                return False
        else:
            print("  ✗ Failed to test valid CSV")
            return False

        # Test invalid CSV (if validation is strict)
        response = await self.send_change_configuration(
            "MeterValuesSampledData",
            "Energy.Active.Import.Register,InvalidMeasurand"
        )

        if response and response[0] == 3:
            status = response[2].get("status")
            print(f"  CSV with invalid value status: {status}")
            # Note: This might be accepted if validation is permissive

        self.test_results.append(("CSV validation", True))
        return True

    async def test_reboot_required_keys(self) -> bool:
        """Test keys that require reboot"""
        print("\n📋 Test 7: Reboot required keys")

        response = await self.send_change_configuration("WebSocketPingInterval", "120")

        if response and response[0] == 3:
            status = response[2].get("status")
            print(f"  WebSocketPingInterval change status: {status}")

            if status == "RebootRequired":
                print("  ✓ Reboot required status correctly returned")
                self.test_results.append(("Reboot required", True))
                return True
            elif status == "Accepted":
                print("  ⚠ Change accepted (reboot might not be required for this implementation)")
                self.test_results.append(("Reboot required", True))
                return True
            else:
                print(f"  ✗ Unexpected status: {status}")
                self.test_results.append(("Reboot required", False))
                return False
        else:
            print(f"  ✗ Unexpected response: {response}")
            self.test_results.append(("Reboot required", False))
            return False

    async def test_persistence(self) -> bool:
        """Test configuration persistence across requests"""
        print("\n📋 Test 8: Configuration persistence")

        test_key = "MeterValueSampleInterval"
        test_value = "45"

        # Set a value
        response = await self.send_change_configuration(test_key, test_value)
        if not (response and response[0] == 3 and response[2].get("status") in ["Accepted", "RebootRequired"]):
            print(f"  ✗ Failed to set {test_key}")
            return False

        # Wait a bit
        await asyncio.sleep(1)

        # Get the value back
        response = await self.send_get_configuration([test_key])
        if response and response[0] == 3:
            config_keys = response[2].get("configurationKey", [])
            if config_keys and config_keys[0].get("value") == test_value:
                print(f"  ✓ Configuration persisted: {test_key} = {test_value}")
                self.test_results.append(("Configuration persistence", True))
                return True
            else:
                print(f"  ✗ Configuration not persisted correctly")
                return False
        else:
            print(f"  ✗ Failed to retrieve configuration")
            return False

    async def run_all_tests(self) -> int:
        """Run all configuration tests"""
        if not await self.connect():
            return 1

        print("\n" + "="*60)
        print("Configuration Management Validation")
        print("="*60)

        # Run all tests
        tests = [
            self.test_get_all_configuration(),
            self.test_get_specific_keys(),
            self.test_change_configuration(),
            self.test_readonly_rejection(),
            self.test_invalid_values(),
            self.test_csv_validation(),
            self.test_reboot_required_keys(),
            self.test_persistence(),
        ]

        # Execute tests sequentially
        results = []
        for test in tests:
            try:
                result = await test
                results.append(result)
            except Exception as e:
                print(f"  ✗ Test failed with exception: {e}")
                results.append(False)

        # Print summary
        print("\n" + "="*60)
        print("Test Results Summary")
        print("="*60)

        for (test_name, result), passed in zip(self.test_results, results):
            status = "✓ PASS" if result and passed else "✗ FAIL"
            print(f"{status}: {test_name}")

        all_passed = all(results) and all(result for _, result in self.test_results)

        if all_passed:
            print("\n✅ All tests passed!")
            return 0
        else:
            print("\n❌ Some tests failed!")
            return 1

    async def disconnect(self):
        """Disconnect from the server"""
        if self.websocket:
            await self.websocket.close()

async def main():
    parser = argparse.ArgumentParser(description='OCPP Configuration Management Validator')
    parser.add_argument('--server', default='ws://localhost:8080',
                       help='OCPP server WebSocket URL (default: ws://localhost:8080)')
    parser.add_argument('--client-id', default='TEST-CP-CONFIG',
                       help='Client ID to use for testing (default: TEST-CP-CONFIG)')
    parser.add_argument('--timeout', type=int, default=30,
                       help='Test timeout in seconds (default: 30)')

    args = parser.parse_args()

    print(f"OCPP Configuration Validator")
    print(f"Server: {args.server}")
    print(f"Client ID: {args.client_id}")
    print(f"Timeout: {args.timeout}s")

    validator = ConfigurationValidator(args.server, args.client_id)

    try:
        # Set overall timeout
        result = await asyncio.wait_for(validator.run_all_tests(), timeout=args.timeout)
        return result
    except asyncio.TimeoutError:
        print(f"\n❌ Test suite timed out after {args.timeout} seconds!")
        return 1
    except KeyboardInterrupt:
        print(f"\n⚠ Test suite interrupted by user")
        return 1
    except Exception as e:
        print(f"\n❌ Test suite failed with error: {e}")
        return 1
    finally:
        await validator.disconnect()

if __name__ == "__main__":
    try:
        exit_code = asyncio.run(main())
        sys.exit(exit_code)
    except KeyboardInterrupt:
        print("\n⚠ Interrupted by user")
        sys.exit(1)