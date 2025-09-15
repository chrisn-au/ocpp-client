#!/usr/bin/env python3
"""
Simulates meter value reporting from a charge point
"""

import asyncio
import json
import random
import sys
import time
import uuid
import websockets
from datetime import datetime

class MeterValueSimulator:
    def __init__(self, server_url, client_id):
        self.server_url = server_url
        self.client_id = client_id
        self.websocket = None
        self.transaction_id = None

    async def connect(self):
        uri = f"{self.server_url}/{self.client_id}"
        self.websocket = await websockets.connect(uri, subprotocols=["ocpp1.6"])
        print(f"✓ Connected to {uri}")

    async def send_boot_notification(self):
        request_id = str(uuid.uuid4())
        message = [2, request_id, "BootNotification", {
            "chargePointModel": "Simulator",
            "chargePointVendor": "Test"
        }]

        await self.websocket.send(json.dumps(message))
        response = await self.websocket.recv()
        print(f"✓ Boot notification accepted")

    async def start_transaction(self):
        request_id = str(uuid.uuid4())
        message = [2, request_id, "StartTransaction", {
            "connectorId": 1,
            "idTag": "TEST-TAG",
            "meterStart": 1000,
            "timestamp": datetime.utcnow().isoformat() + "Z"
        }]

        await self.websocket.send(json.dumps(message))
        response = await self.websocket.recv()
        response_data = json.loads(response)

        if response_data[0] == 3:
            self.transaction_id = response_data[2]["transactionId"]
            print(f"✓ Transaction started: ID={self.transaction_id}")
            return True
        return False

    async def send_meter_values(self, energy_wh, power_w, current_a=None, voltage_v=None, temperature_c=None):
        request_id = str(uuid.uuid4())

        sampled_values = [
            {
                "value": str(energy_wh),
                "measurand": "Energy.Active.Import.Register",
                "unit": "Wh"
            },
            {
                "value": str(power_w),
                "measurand": "Power.Active.Import",
                "unit": "W"
            }
        ]

        if current_a:
            sampled_values.append({
                "value": str(current_a),
                "measurand": "Current.Import",
                "unit": "A"
            })

        if voltage_v:
            sampled_values.append({
                "value": str(voltage_v),
                "measurand": "Voltage",
                "unit": "V"
            })

        if temperature_c:
            sampled_values.append({
                "value": str(temperature_c),
                "measurand": "Temperature",
                "unit": "Celsius"
            })

        message_data = {
            "connectorId": 1,
            "meterValue": [{
                "timestamp": datetime.utcnow().isoformat() + "Z",
                "sampledValue": sampled_values
            }]
        }

        if self.transaction_id:
            message_data["transactionId"] = self.transaction_id

        message = [2, request_id, "MeterValues", message_data]

        await self.websocket.send(json.dumps(message))
        response = await self.websocket.recv()

        print(f"  Sent: Energy={energy_wh}Wh, Power={power_w}W", end="")
        if current_a:
            print(f", Current={current_a}A", end="")
        if voltage_v:
            print(f", Voltage={voltage_v}V", end="")
        if temperature_c:
            print(f", Temp={temperature_c}°C", end="")
        print()

    async def stop_transaction(self, meter_stop):
        if not self.transaction_id:
            return

        request_id = str(uuid.uuid4())
        message = [2, request_id, "StopTransaction", {
            "transactionId": self.transaction_id,
            "meterStop": meter_stop,
            "timestamp": datetime.utcnow().isoformat() + "Z"
        }]

        await self.websocket.send(json.dumps(message))
        response = await self.websocket.recv()
        print(f"✓ Transaction stopped at {meter_stop}Wh")

    async def simulate_charging_session(self, duration_seconds=60, interval_seconds=10):
        print(f"\n📊 Starting charging session simulation ({duration_seconds}s)")
        print("=" * 50)

        # Initial values
        energy_wh = 1000
        base_power = 7400  # 7.4kW
        voltage = 230

        # Start transaction
        await self.start_transaction()

        # Send meter values periodically
        start_time = time.time()
        while (time.time() - start_time) < duration_seconds:
            # Simulate realistic variations
            power_variation = random.uniform(-500, 500)
            power_w = base_power + power_variation

            # Calculate energy increment (power * time_interval / 3600)
            energy_increment = (power_w * interval_seconds) / 3600
            energy_wh += energy_increment

            # Calculate current from power and voltage
            current_a = power_w / voltage

            # Simulate temperature
            temperature_c = 25 + random.uniform(-5, 10)

            # Add voltage variation
            voltage_v = voltage + random.uniform(-5, 5)

            await self.send_meter_values(
                int(energy_wh),
                int(power_w),
                round(current_a, 1),
                round(voltage_v, 1),
                round(temperature_c, 1)
            )

            await asyncio.sleep(interval_seconds)

        # Stop transaction
        await self.stop_transaction(int(energy_wh))

        print("=" * 50)
        print(f"✓ Charging session complete. Total energy: {int(energy_wh - 1000)}Wh")

    async def run_tests(self):
        await self.connect()
        await self.send_boot_notification()

        print("\n📋 Test 1: Send single meter value")
        await self.send_meter_values(5000, 3700, 16, 230, 25)

        print("\n📋 Test 2: Simulate charging session")
        await self.simulate_charging_session(duration_seconds=30, interval_seconds=5)

        print("\n📋 Test 3: Send high power alert")
        await self.send_meter_values(10000, 55000, 240, 230)  # 55kW - should trigger alert

        print("\n✅ All tests completed!")

    async def disconnect(self):
        if self.websocket:
            await self.websocket.close()

async def main():
    server_url = sys.argv[1] if len(sys.argv) > 1 else "ws://localhost:8080"
    client_id = sys.argv[2] if len(sys.argv) > 2 else "TEST-CP-METER"

    simulator = MeterValueSimulator(server_url, client_id)

    try:
        await simulator.run_tests()
    finally:
        await simulator.disconnect()

if __name__ == "__main__":
    asyncio.run(main())