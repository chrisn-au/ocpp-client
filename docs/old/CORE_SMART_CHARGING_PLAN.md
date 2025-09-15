# OCPP Core + Smart Charging Implementation Plan

## Objective
Implement complete OCPP 1.6 Core and Smart Charging profiles with intelligent responses and REST API trigger mechanisms for server-initiated commands.

## Current Foundation
- ✅ Redis distributed state working
- ✅ Basic OCPP handlers (Boot, Heartbeat, Status, Start/Stop Transaction)
- ✅ Business state persistence in Redis
- ✅ REST API framework in place

## Implementation Strategy
Break down into small, manageable chunks that build upon each other. Each chunk should be completable in 2-4 hours.

---

## Phase 1: Complete Core Profile (OCPP 1.6)

### Chunk 1.1: Enhanced Authorization & Data Transfer
**Goal**: Complete authorization handling and vendor-specific extensions
**Time**: 2-3 hours

#### File: `/Users/chrishome/development/home/mcp-access/csms/ocpp-server/handlers/core_enhanced.go`

**Add support for:**
1. **Authorize Request**: Validate ID tags
2. **Data Transfer Request**: Vendor-specific data exchange
3. **Enhanced ID Tag validation** with configurable policies

**Implementation:**
```go
func (s *Server) handleAuthorize(clientID, requestId string, req *core.AuthorizeRequest) {
    // Check ID tag against authorization database/service
    // Support local cache + external validation
    // Return appropriate IdTagInfo status
}

func (s *Server) handleDataTransfer(clientID, requestId string, req *core.DataTransferRequest) {
    // Route vendor-specific data to appropriate handlers
    // Support configurable vendor extensions
    // Log and forward to external services if needed
}
```

**Business State Updates:**
- Store ID tag authorization cache
- Track data transfer requests/responses
- Vendor-specific configuration storage

### Chunk 1.2: Configuration Management
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

**Add Handlers:**
```go
func (s *Server) handleGetConfiguration(clientID, requestId string, req *core.GetConfigurationRequest)
func (s *Server) handleChangeConfiguration(clientID, requestId string, req *core.ChangeConfigurationRequest)
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

**Implementation:**
```go
func (s *Server) handleMeterValues(clientID, requestId string, req *core.MeterValuesRequest) {
    // Store meter values with timestamp
    // Update transaction energy totals
    // Trigger alerts for anomalies (configurable thresholds)
    // Support multiple measurands per reading
}

type MeterValueProcessor struct {
    businessState *ocppj.RedisBusinessState
    alertManager  *AlertManager
}
```

---

## Phase 2: Server-Initiated Commands (REST API Triggers)

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
    pendingCommands map[string]*PendingCommand // Track command status
}

type RemoteStartRequest struct {
    ConnectorID int    `json:"connectorId"`
    IDTag       string `json:"idTag"`
    ChargingProfile *ChargingProfile `json:"chargingProfile,omitempty"`
}

func (rcm *RemoteCommandManager) RemoteStartTransaction(clientID string, req *RemoteStartRequest) (*CommandResult, error)
func (rcm *RemoteCommandManager) RemoteStopTransaction(clientID string, transactionID int) (*CommandResult, error)
```

**Command Tracking:**
- Store pending commands in Redis with timeout
- Track command status (pending/accepted/rejected/failed)
- Provide status endpoints for monitoring

### Chunk 2.2: Reset & Diagnostics Commands
**Goal**: System control and diagnostics via REST API
**Time**: 2-3 hours

**REST Endpoints:**
```
POST /api/v1/chargepoints/{clientID}/reset
POST /api/v1/chargepoints/{clientID}/get-diagnostics
POST /api/v1/chargepoints/{clientID}/update-firmware
```

**Implementation:**
```go
func (s *Server) handleResetRequest(clientID, requestId string, req *core.ResetRequest)
func (s *Server) handleGetDiagnosticsRequest(clientID, requestId string, req *firmware.GetDiagnosticsRequest)
```

### Chunk 2.3: Configuration Updates via API
**Goal**: REST endpoints to modify charge point configuration
**Time**: 2 hours

**REST Endpoints:**
```
GET /api/v1/chargepoints/{clientID}/configuration
PUT /api/v1/chargepoints/{clientID}/configuration
POST /api/v1/chargepoints/{clientID}/configuration/reset
```

---

## Phase 3: Smart Charging Profile Implementation

### Chunk 3.1: Charging Profile Data Model
**Goal**: Define and store charging profiles with Redis persistence
**Time**: 2-3 hours

#### File: `/Users/chrishome/development/home/mcp-access/csms/ocpp-server/models/charging_profile.go`

**Data Structures:**
```go
type ChargingProfile struct {
    ChargingProfileID     int                    `json:"chargingProfileId"`
    TransactionID         *int                   `json:"transactionId,omitempty"`
    StackLevel           int                    `json:"stackLevel"`
    ChargingProfilePurpose ChargingProfilePurpose `json:"chargingProfilePurpose"`
    ChargingProfileKind   ChargingProfileKind    `json:"chargingProfileKind"`
    RecurrencyKind       *RecurrencyKind        `json:"recurrencyKind,omitempty"`
    ValidFrom            *time.Time             `json:"validFrom,omitempty"`
    ValidTo              *time.Time             `json:"validTo,omitempty"`
    ChargingSchedule     ChargingSchedule       `json:"chargingSchedule"`
}

type ChargingSchedule struct {
    Duration                 *int                     `json:"duration,omitempty"`
    StartSchedule           *time.Time               `json:"startSchedule,omitempty"`
    ChargingRateUnit        ChargingRateUnit         `json:"chargingRateUnit"`
    ChargingSchedulePeriod  []ChargingSchedulePeriod `json:"chargingSchedulePeriod"`
    MinChargingRate         *float64                 `json:"minChargingRate,omitempty"`
}
```

**Redis Storage:**
- Profile storage per charge point
- Stack-level management
- Profile validation and conflict resolution

### Chunk 3.2: SetChargingProfile Implementation
**Goal**: Handle SetChargingProfile requests with validation
**Time**: 3-4 hours

#### File: `/Users/chrishome/development/home/mcp-access/csms/ocpp-server/handlers/smart_charging.go`

**Features:**
1. **Profile Validation**: Stack levels, time ranges, power limits
2. **Conflict Resolution**: Handle overlapping profiles
3. **Active Profile Calculation**: Determine current power limit
4. **Profile Storage**: Persist in Redis with proper indexing

**Implementation:**
```go
func (s *Server) handleSetChargingProfile(clientID, requestId string, req *smartcharging.SetChargingProfileRequest) {
    // Validate charging profile
    // Check for conflicts with existing profiles
    // Store in Redis with proper indexing
    // Calculate and cache active profile for connector
    // Return appropriate status
}

type ChargingProfileManager struct {
    businessState *ocppj.RedisBusinessState
}

func (cpm *ChargingProfileManager) SetProfile(clientID string, connectorID int, profile *ChargingProfile) error
func (cpm *ChargingProfileManager) GetActiveProfile(clientID string, connectorID int, timestamp time.Time) *ChargingProfile
func (cpm *ChargingProfileManager) ClearProfiles(clientID string, criteria *ClearCriteria) int
```

### Chunk 3.3: GetCompositeSchedule & ClearChargingProfile
**Goal**: Complete smart charging profile management
**Time**: 2-3 hours

**Implementation:**
```go
func (s *Server) handleGetCompositeSchedule(clientID, requestId string, req *smartcharging.GetCompositeScheduleRequest) {
    // Calculate composite schedule from all active profiles
    // Handle stack level priorities
    // Return schedule periods with power limits
}

func (s *Server) handleClearChargingProfile(clientID, requestId string, req *smartcharging.ClearChargingProfileRequest) {
    // Clear profiles based on criteria (ID, connector, purpose, stack level)
    // Update active profile cache
    // Return number of cleared profiles
}
```

### Chunk 3.4: Smart Charging REST API
**Goal**: REST endpoints for charging profile management
**Time**: 2-3 hours

**REST Endpoints:**
```
GET    /api/v1/chargepoints/{clientID}/charging-profiles
POST   /api/v1/chargepoints/{clientID}/charging-profiles
DELETE /api/v1/chargepoints/{clientID}/charging-profiles/{profileID}
GET    /api/v1/chargepoints/{clientID}/connectors/{connectorID}/composite-schedule
POST   /api/v1/chargepoints/{clientID}/connectors/{connectorID}/set-charging-profile
```

**Features:**
- Create/update/delete charging profiles via API
- Get composite schedules for specific time ranges
- Bulk operations for multiple charge points

---

## Phase 4: Advanced Features & Integration

### Chunk 4.1: Real-time Power Management
**Goal**: Active power limit enforcement based on profiles
**Time**: 3-4 hours

**Features:**
1. **Real-time Monitoring**: Track actual vs. scheduled power
2. **Dynamic Adjustments**: Modify profiles based on grid conditions
3. **Load Balancing**: Distribute available power across connectors
4. **Alerts**: Notify when limits are exceeded

### Chunk 4.2: Reservation System
**Goal**: Implement ReserveNow/CancelReservation
**Time**: 2-3 hours

**Implementation:**
```go
func (s *Server) handleReserveNow(clientID, requestId string, req *reservation.ReserveNowRequest)
func (s *Server) handleCancelReservation(clientID, requestId string, req *reservation.CancelReservationRequest)
```

### Chunk 4.3: Enhanced Transaction Features
**Goal**: Transaction validation, timeout handling, cost calculation
**Time**: 3-4 hours

**Features:**
1. **Transaction Validation**: Energy limits, time limits, cost limits
2. **Automatic Timeout**: Stop transactions after configured duration
3. **Cost Calculation**: Real-time pricing based on energy and time
4. **Session Analytics**: Energy efficiency, charging curves

---

## Implementation Guidelines

### Development Approach
1. **Test-Driven**: Create tests for each handler before implementation
2. **Incremental**: Each chunk should be fully functional before moving to next
3. **Backwards Compatible**: Don't break existing functionality
4. **Configuration-Driven**: Make features configurable per charge point

### Error Handling Strategy
```go
// Consistent error response pattern
func (s *Server) sendOCPPError(clientID, requestId, errorCode, description string) {
    if err := s.ocppServer.SendError(clientID, requestId, errorCode, description, nil); err != nil {
        log.Printf("Error sending OCPP error response: %v", err)
    }
}
```

### Testing Strategy
- **Unit Tests**: Test individual handlers with mock Redis
- **Integration Tests**: Test full OCPP message flow
- **Load Tests**: Verify performance with multiple clients
- **Compliance Tests**: Validate against OCPP 1.6 specification

### Dependencies to Add
```go
// Add to go.mod
require (
    github.com/lorenzodonini/ocpp-go/ocpp1.6/smartcharging v0.16.0
    github.com/lorenzodonini/ocpp-go/ocpp1.6/reservation v0.16.0
    github.com/lorenzodonini/ocpp-go/ocpp1.6/firmware v0.16.0
)
```

## Execution Timeline

**Phase 1** (Core Profile): 6-9 hours
**Phase 2** (REST Commands): 7-10 hours
**Phase 3** (Smart Charging): 9-13 hours
**Phase 4** (Advanced): 8-12 hours

**Total**: ~30-44 hours (1-2 weeks full-time development)

## Success Criteria

✅ **Core Profile**: All OCPP 1.6 Core messages handled intelligently
✅ **Smart Charging**: Full charging profile lifecycle management
✅ **REST API**: Complete server command interface via HTTP
✅ **Real-time**: Live power management and monitoring
✅ **Production Ready**: Error handling, logging, configuration management

Each chunk builds upon previous work and can be validated independently before proceeding to the next chunk.