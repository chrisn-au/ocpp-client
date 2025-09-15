# OCPP Core + Smart Charging Implementation Plan (Revised)

## Objective
Implement complete OCPP 1.6 Core and Smart Charging profiles with intelligent responses and REST API trigger mechanisms for server-initiated commands.

## Implementation Strategy
Reorganized into logical progression, starting with configuration and core features, then smart charging, then advanced features.

---

## Phase 1: Core Configuration & Meter Values

### Chunk 1.1: Configuration Management
**Goal**: Complete GetConfiguration/ChangeConfiguration handling
**Time**: 2-3 hours

#### File: `/Users/chrishome/development/home/mcp-access/csms/ocpp-server/config/manager.go`

**Configuration Categories:**
1. **Core Configuration Keys** (OCPP standard)
2. **Vendor-Specific Keys**
3. **Smart Charging Keys**

**Implementation:**
```go
type ConfigurationManager struct {
    businessState *ocppj.RedisBusinessState
    defaultConfig map[string]ConfigValue
}

type ConfigValue struct {
    Value     string
    ReadOnly  bool
    Validator func(string) error
}

func (cm *ConfigurationManager) GetConfiguration(clientID string, keys []string) []*core.KeyValue
func (cm *ConfigurationManager) ChangeConfiguration(clientID, key, value string) core.ConfigurationStatus
```

### Chunk 1.2: Configuration Updates via API
**Goal**: REST endpoints to modify charge point configuration
**Time**: 2 hours

**REST Endpoints:**
```
GET /api/v1/chargepoints/{clientID}/configuration
PUT /api/v1/chargepoints/{clientID}/configuration
POST /api/v1/chargepoints/{clientID}/configuration/reset
```

### Chunk 1.3: Meter Values & Sampling
**Goal**: Enhanced meter value collection with configurable intervals
**Time**: 2-3 hours

#### File: `/Users/chrishome/development/home/mcp-access/csms/ocpp-server/handlers/meter_values.go`

**Features:**
1. **Configurable sampling intervals** per charge point
2. **Multiple measurands** (Energy, Power, Current, Voltage, etc.)
3. **Historical storage** in Redis with TTL
4. **Real-time aggregation** for reporting

---

## Phase 2: Server-Initiated Transaction Commands

### Chunk 2.1: Remote Transaction Control
**Goal**: REST endpoints to trigger RemoteStartTransaction/RemoteStopTransaction
**Time**: 3-4 hours

#### File: `/Users/chrishome/development/home/mcp-access/csms/ocpp-server/commands/remote_control.go`

**REST Endpoints:**
```
POST /api/v1/chargepoints/{clientID}/remote-start
POST /api/v1/chargepoints/{clientID}/remote-stop
POST /api/v1/chargepoints/{clientID}/unlock-connector
```

**Implementation:**
```go
type RemoteCommandManager struct {
    ocppServer    *ocppj.Server
    businessState *ocppj.RedisBusinessState
    pendingCommands map[string]*PendingCommand
}

type RemoteStartRequest struct {
    ConnectorID int    `json:"connectorId"`
    IDTag       string `json:"idTag"`
    ChargingProfile *ChargingProfile `json:"chargingProfile,omitempty"`
}
```

---

## Phase 3: Smart Charging Profile Implementation

### Chunk 3.1: Charging Profile Data Model
**Goal**: Define and store charging profiles with Redis persistence
**Time**: 2-3 hours

#### File: `/Users/chrishome/development/home/mcp-access/csms/ocpp-server/models/charging_profile.go`

### Chunk 3.2: SetChargingProfile Implementation
**Goal**: Handle SetChargingProfile requests with validation
**Time**: 3-4 hours

#### File: `/Users/chrishome/development/home/mcp-access/csms/ocpp-server/handlers/smart_charging.go`

### Chunk 3.3: GetCompositeSchedule & ClearChargingProfile
**Goal**: Complete smart charging profile management
**Time**: 2-3 hours

### Chunk 3.4: Smart Charging REST API
**Goal**: REST endpoints for charging profile management
**Time**: 2-3 hours

**REST Endpoints:**
```
GET    /api/v1/chargepoints/{clientID}/charging-profiles
POST   /api/v1/chargepoints/{clientID}/charging-profiles
DELETE /api/v1/chargepoints/{clientID}/charging-profiles/{profileID}
GET    /api/v1/chargepoints/{clientID}/connectors/{connectorID}/composite-schedule
```

---

## Phase 4: Advanced Features & Integration

### Chunk 4.1: Real-time Power Management
**Goal**: Active power limit enforcement based on profiles
**Time**: 3-4 hours

### Chunk 4.2: Reservation System
**Goal**: Implement ReserveNow/CancelReservation
**Time**: 2-3 hours

### Chunk 4.3: Enhanced Transaction Features
**Goal**: Transaction validation, timeout handling, cost calculation
**Time**: 3-4 hours

---

## Phase 5: Authorization & System Control

### Chunk 5.1: Enhanced Authorization & Data Transfer
**Goal**: Complete authorization handling and vendor-specific extensions
**Time**: 2-3 hours
**(Previously Chunk 1.1)**

#### File: `/Users/chrishome/development/home/mcp-access/csms/ocpp-server/handlers/core_enhanced.go`

**Add support for:**
1. **Authorize Request**: Validate ID tags
2. **Data Transfer Request**: Vendor-specific data exchange
3. **Enhanced ID Tag validation** with configurable policies

### Chunk 5.2: Reset & Diagnostics Commands
**Goal**: System control and diagnostics via REST API
**Time**: 2-3 hours
**(Previously Chunk 2.2)**

**REST Endpoints:**
```
POST /api/v1/chargepoints/{clientID}/reset
POST /api/v1/chargepoints/{clientID}/get-diagnostics
POST /api/v1/chargepoints/{clientID}/update-firmware
```

---

## Revised Execution Timeline

**Phase 1** (Core Configuration): 6-8 hours
- Configuration management and API
- Meter value collection

**Phase 2** (Transaction Control): 3-4 hours
- Remote start/stop commands

**Phase 3** (Smart Charging): 10-13 hours
- Full charging profile lifecycle

**Phase 4** (Advanced): 8-11 hours
- Power management, reservations, enhanced transactions

**Phase 5** (Auth & System): 4-6 hours
- Authorization, data transfer, reset, diagnostics

**Total**: ~31-42 hours

## Rationale for Reorganization

1. **Configuration First**: Essential foundation for all other features
2. **Transaction Control Early**: Critical for basic operations
3. **Smart Charging Core**: Main value proposition gets priority
4. **Advanced Features**: Build on established foundation
5. **Auth & System Last**: Can operate without these initially, add when needed

This order allows for:
- Faster time to core functionality
- Progressive complexity increase
- Each phase delivers working features
- Authorization can be simplified initially (accept all) then enhanced later