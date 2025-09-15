# OCPP Server - Product Requirements Document

## Executive Summary

A multi-tenant, white-label OCPP 1.6 server designed as the core backend for Charge Station Management Systems (CSMS). Built for scalability with microservices architecture, the server maintains authoritative session state while publishing real-time events via MQTT for integration with external billing, energy management, and smart charging services.

## 1. Product Vision & Goals

### Primary Objectives
- **CSMS Backend**: Core OCPP protocol handling and charge point management
- **Multi-tenant Architecture**: White-label ready with tenant-based isolation
- **Microservices Ready**: Event-driven architecture with MQTT pub/sub
- **Scalable Foundation**: Built to handle enterprise-scale charging networks

### Success Metrics
- Support 10,000+ concurrent charge points per instance
- Sub-second OCPP response times
- 99.9% uptime for critical charging operations
- Zero data leakage between tenants

## 2. Core Architecture

### 2.1 System Components
```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Charge Point  │────│   OCPP Server   │────│  External       │
│                 │    │                 │    │  Services       │
│                 │    │                 │    │                 │
└─────────────────┘    └─────────────────┘    └─────────────────┘
                                │
                                │
                       ┌─────────────────┐
                       │   MQTT Broker   │
                       │                 │
                       └─────────────────┘
```

### 2.2 Technology Stack
- **Language**: Go 1.16+
- **OCPP Library**: ocpp-go (local fork)
- **Transport**: Redis (existing) + MQTT Broker
- **Database**: MongoDB (tenant-isolated collections)
- **Authentication**: JWT with tenant claims
- **Messaging**: MQTT for pub/sub, Redis for internal queuing

### 2.3 Multi-tenancy Model
- **Tenant Isolation**: Separate MongoDB databases per tenant
- **Connection Routing**: Charge point ID prefixes for tenant identification
- **API Isolation**: Tenant-scoped endpoints with JWT validation
- **Resource Limits**: Per-tenant quotas and rate limiting

## 3. Functional Requirements

### 3.1 OCPP Protocol Support

#### Core Profile (Required)
- **Boot Notification**: Charge point registration and acceptance
- **Heartbeat**: Connection monitoring and timestamp sync
- **Status Notification**: Connector and charge point status updates
- **Start/Stop Transaction**: Session lifecycle management
- **Meter Values**: Real-time energy consumption reporting
- **Data Transfer**: Vendor-specific extensions

#### Smart Charging Profile (Phase 2)
- **Set Charging Profile**: Power limits and scheduling
- **Clear Charging Profile**: Profile management
- **Get Composite Schedule**: Current charging plan retrieval

#### Remote Trigger Profile (Phase 2)
- **Remote Start Transaction**: External session initiation
- **Remote Stop Transaction**: External session termination
- **Reset**: Charge point restart commands

### 3.2 Session State Management

The OCPP server maintains authoritative state for:
- **Active Sessions**: Transaction ID, start time, energy consumed
- **Charge Point Status**: Availability, connector states, last seen
- **Tenant Configuration**: Pricing rules, operational settings
- **Historical Data**: Transaction logs, meter readings, events

### 3.3 Event Publishing (MQTT)

#### Real-time Session Events
```json
Topic: ocpp/events/{tenant_id}/{charge_point_id}/session
{
  "event_type": "session_started|session_stopped|meter_value",
  "timestamp": "2025-09-15T01:51:49Z",
  "session_id": "12345",
  "charge_point_id": "CP-001",
  "tenant_id": "tenant-123",
  "data": {
    "connector_id": 1,
    "energy_wh": 15000,
    "duration_seconds": 3600
  }
}
```

#### Status Updates
```json
Topic: ocpp/status/{tenant_id}/{charge_point_id}
{
  "charge_point_id": "CP-001",
  "status": "Available|Occupied|Faulted",
  "connectors": [
    {
      "id": 1,
      "status": "Available",
      "last_update": "2025-09-15T01:51:49Z"
    }
  ]
}
```

### 3.4 Command Interface (MQTT)

External services can send commands via MQTT:

```json
Topic: ocpp/commands/{tenant_id}/{charge_point_id}
{
  "command_id": "cmd-789",
  "command_type": "remote_start|remote_stop|set_charging_profile",
  "timestamp": "2025-09-15T01:51:49Z",
  "parameters": {
    "connector_id": 1,
    "id_tag": "RFID-12345"
  }
}
```

### 3.5 REST API

#### Tenant Management
- `POST /api/v1/tenants` - Create new tenant
- `GET /api/v1/tenants/{tenant_id}` - Tenant details
- `PUT /api/v1/tenants/{tenant_id}/config` - Update configuration

#### Charge Point Management
- `GET /api/v1/chargepoints` - List tenant's charge points
- `GET /api/v1/chargepoints/{cp_id}` - Charge point details
- `POST /api/v1/chargepoints/{cp_id}/commands` - Send OCPP commands

#### Session Management
- `GET /api/v1/sessions` - Active and historical sessions
- `GET /api/v1/sessions/{session_id}` - Session details
- `POST /api/v1/sessions/{session_id}/stop` - Force stop session

#### Health & Monitoring
- `GET /health` - Server health status
- `GET /metrics` - Prometheus-compatible metrics

## 4. Non-Functional Requirements

### 4.1 Performance
- **Concurrent Connections**: 10,000+ charge points per instance
- **Message Throughput**: 100,000+ OCPP messages per minute
- **Response Time**: <500ms for OCPP protocol responses
- **Session State Persistence**: <100ms write latency to MongoDB

### 4.2 Scalability
- **Horizontal Scaling**: Stateless application design
- **Database Sharding**: MongoDB tenant databases can be distributed
- **Load Balancing**: Redis and MQTT clustering support
- **Queue Processing**: Configurable worker pools per tenant

### 4.3 Reliability
- **Uptime**: 99.9% availability SLA
- **Message Delivery**: At-least-once delivery for critical events
- **Data Consistency**: ACID transactions for session state changes
- **Graceful Degradation**: Queue bypass mode for direct processing

### 4.4 Security
- **Authentication**: JWT tokens with tenant claims
- **Authorization**: Role-based access control (tenant admin, operator, viewer)
- **Data Encryption**: TLS for all external communications
- **Tenant Isolation**: Database-level separation
- **Audit Logging**: All API calls and OCPP messages logged

## 5. Integration Architecture

### 5.1 External Service Integration

The OCPP server publishes events that drive external microservices:

#### Billing Service
- Subscribes to: `ocpp/events/+/+/session`
- Calculates pricing based on energy consumption and time
- Publishes billing events for payment processing

#### Energy Management Service
- Subscribes to: `ocpp/events/+/+/meter_value`
- Monitors grid load and capacity
- Publishes charging profile commands for load balancing

#### CRM/Customer Service
- Subscribes to: `ocpp/status/+/+`
- Tracks charge point availability and utilization
- Manages customer notifications and support

#### Mobile App Backend
- Uses REST API for real-time session monitoring
- Sends remote start/stop commands via MQTT

### 5.2 MQTT Topic Structure

```
ocpp/
├── events/{tenant_id}/{charge_point_id}/
│   ├── session          # Session lifecycle events
│   ├── meter_value      # Energy consumption updates
│   └── fault            # Error and fault notifications
├── status/{tenant_id}/{charge_point_id}    # Availability updates
├── commands/{tenant_id}/{charge_point_id}  # External commands
└── responses/{tenant_id}/{charge_point_id} # Command acknowledgments
```

## 6. Data Model

### 6.1 MongoDB Collections (per tenant)

#### charge_points
```json
{
  "_id": "CP-001",
  "tenant_id": "tenant-123",
  "model": "AC-22kW-Type2",
  "vendor": "Example Vendor",
  "serial_number": "SN123456",
  "connectors": [
    {
      "id": 1,
      "type": "Type2",
      "max_power": 22000,
      "status": "Available"
    }
  ],
  "configuration": {
    "heartbeat_interval": 300,
    "meter_values_interval": 60
  },
  "last_seen": "2025-09-15T01:51:49Z",
  "created_at": "2025-09-15T01:51:49Z"
}
```

#### sessions
```json
{
  "_id": "session-12345",
  "tenant_id": "tenant-123",
  "charge_point_id": "CP-001",
  "connector_id": 1,
  "transaction_id": 789,
  "id_tag": "RFID-12345",
  "start_time": "2025-09-15T01:51:49Z",
  "stop_time": null,
  "start_meter_value": 1000,
  "stop_meter_value": null,
  "meter_values": [
    {
      "timestamp": "2025-09-15T02:51:49Z",
      "value": 15000,
      "unit": "Wh"
    }
  ],
  "status": "active|completed|faulted"
}
```

#### tenants (global collection)
```json
{
  "_id": "tenant-123",
  "name": "City Charging Network",
  "database_name": "ocpp_tenant_123",
  "configuration": {
    "max_charge_points": 1000,
    "api_rate_limit": 10000,
    "queue_enabled": true
  },
  "created_at": "2025-09-15T01:51:49Z",
  "status": "active"
}
```

## 7. Configuration Management

### 7.1 Queue Processing
- **Queue Enabled**: Toggle between queue-based and direct processing
- **Worker Pool Size**: Configurable per tenant
- **Message Retention**: MQTT message persistence settings

### 7.2 Tenant Settings
- **Rate Limiting**: API and OCPP message limits
- **Feature Flags**: Enable/disable smart charging, remote triggers
- **Charging Profiles**: Default power limits and schedules

### 7.3 OCPP Protocol Settings
- **Supported Profiles**: Core, Smart Charging, Remote Trigger
- **Message Timeout**: Configurable per message type
- **Retry Logic**: Failed message handling

## 8. Deployment Architecture

### 8.1 Container Structure
```yaml
services:
  ocpp-server:
    - Multi-tenant OCPP protocol handler
    - REST API server
    - MQTT publisher/subscriber

  mongodb:
    - Primary data store
    - Tenant database isolation

  mqtt-broker:
    - Message pub/sub hub
    - External service integration

  redis:
    - Internal message queuing
    - Session caching
```

### 8.2 Environment Configuration
- **Development**: Single tenant, queue bypass enabled
- **Staging**: Multi-tenant, full MQTT integration
- **Production**: Clustered deployment, high availability

## 9. Success Criteria & Metrics

### 9.1 Technical Metrics
- **Protocol Compliance**: 100% OCPP 1.6 Core profile compliance
- **Message Processing**: <500ms average response time
- **Tenant Isolation**: Zero cross-tenant data leakage
- **Event Publishing**: <100ms MQTT publish latency

### 9.2 Business Metrics
- **Charge Point Capacity**: Support 10,000+ concurrent connections
- **Tenant Scaling**: Onboard new white-label customers in <24 hours
- **Service Integration**: External services receive real-time session data
- **Operational Efficiency**: 99.9% session completion rate

## 10. Implementation Phases

### Phase 1: Core Foundation (Current → 4 weeks)
- Multi-tenant MongoDB setup
- Enhanced OCPP message handling
- Basic MQTT event publishing
- Tenant-scoped REST API

### Phase 2: Advanced Features (4-8 weeks)
- Smart Charging profile support
- Remote trigger commands
- Queue processing architecture
- External service command interface

### Phase 3: Scale & Polish (8-12 weeks)
- Performance optimization
- High availability deployment
- Monitoring and alerting
- White-label customer onboarding tools

---

*This PRD serves as the foundation for building a production-ready, multi-tenant OCPP server that can scale to enterprise charging networks while maintaining clean separation of concerns through event-driven architecture.*