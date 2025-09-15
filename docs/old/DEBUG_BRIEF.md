# Debug Brief: OCPP Redis Transport Bidirectional Communication Issue

**File Location**: `/Users/chrishome/development/home/mcp-access/csms/DEBUG_BRIEF.md`

## Status: ✅ PARTIALLY WORKING - One Direction Only

### What's Working
- ✅ Client → WebSocket Server → Redis OCPP Server (inbound messages working)
- ✅ Redis OCPP Server is receiving and processing OCPP requests
- ✅ Docker containers are running and connected
- ✅ Redis connectivity is established

### What's NOT Working
- ❌ Redis OCPP Server → WebSocket Server → Client (outbound responses not reaching client)
- ❌ OCPP responses/confirmations not making the return journey

## Architecture Overview
```
[Classic OCPP Client]
    ↕️ WebSocket
[WebSocket Server]
    ↕️ Redis Pub/Sub
[Redis OCPP Server] ← YOU ARE HERE (receiving but not responding back)
```

## Problem Hypothesis
The Redis OCPP Server is likely:
1. **Receiving OCPP requests successfully** (proven working)
2. **Processing requests and creating responses**
3. **Publishing responses to Redis**
4. **But responses aren't reaching the WebSocket Server or Client**

## Key Investigation Areas

### 1. Response Publishing Pattern
- **Check**: How does Redis OCPP Server publish responses back to Redis?
- **Look at**: `server.SendResponse()` implementation in our transport layer
- **Verify**: Redis pub/sub channel naming for responses vs requests

### 2. WebSocket Server Subscription Pattern
- **Check**: Is WebSocket Server subscribed to the correct Redis channels for responses?
- **Look at**: WebSocket Server Redis subscriber configuration
- **Verify**: Channel naming consistency between publisher and subscriber

### 3. Message Flow Debugging
- **Check**: Redis MONITOR to see actual pub/sub traffic
- **Look at**: Both request and response Redis messages
- **Verify**: Message format and routing

### 4. Transport Implementation Details
- **Check**: Redis transport `Write()` method implementation
- **Look at**: Channel routing logic in transport layer
- **Verify**: Client ID mapping and response routing

## Debugging Commands

### Monitor Redis Traffic
```bash
# Monitor all Redis activity
docker exec ocpp-redis redis-cli MONITOR

# Check pub/sub channels
docker exec ocpp-redis redis-cli PUBSUB CHANNELS "*"

# Check active subscribers
docker exec ocpp-redis redis-cli PUBSUB NUMSUB
```

### Check Container Logs
```bash
# Redis OCPP Server logs
docker logs redis-ocpp-server -f

# WebSocket Server logs
docker logs ocpp-ws-server -f

# Client logs
docker logs ocpp-client -f
```

### Test Message Flow
```bash
# Manually publish test message to Redis
docker exec ocpp-redis redis-cli PUBLISH "ocpp:response:TEST-CP-001" '{"test":"message"}'
```

## Files to Investigate

### Primary Suspects
1. **`transport/redis/redis.go`** - Redis transport implementation
2. **`ocppj/server.go:SendResponse()`** - Response sending logic
3. **WebSocket Server Redis subscriber code** - Response listening
4. **Channel naming conventions** - Request vs response routing

### Investigation Priority
1. **HIGH**: Trace a single request/response cycle end-to-end
2. **HIGH**: Verify Redis pub/sub channel names for responses
3. **MEDIUM**: Check WebSocket Server response forwarding logic
4. **LOW**: Verify OCPP message formatting

## Success Criteria
- Client receives OCPP responses (BootNotification confirmation, Heartbeat confirmation)
- Bidirectional OCPP communication working end-to-end
- Full request-response cycle complete through Redis transport

## Expected Root Cause
Most likely one of:
1. **Channel Naming Mismatch**: Response published to different channel than WebSocket Server expects
2. **Missing Subscription**: WebSocket Server not listening for response messages
3. **Response Routing**: Redis transport not routing responses back correctly
4. **Message Format**: Response format incompatible with WebSocket Server expectations

## Context
This is a **groundbreaking integration** - we've successfully created the first distributed OCPP implementation using Redis transport while maintaining backward compatibility. The fact that inbound works proves the core architecture is sound. This is just a routing/pub-sub configuration issue.

**Priority**: HIGH - This completes the proof-of-concept for distributed OCPP architecture.

## Specific Investigation Steps

### Step 1: Verify Redis Transport Write Implementation
**File**: `ocpp-go/transport/redis/redis.go`
**Focus**: Look at how `Write()` method publishes messages
**Question**: Does it use the correct channel naming for responses?

### Step 2: Trace SendResponse in OCPPJ Server
**File**: `ocpp-go/ocppj/server.go` around line 200-300
**Focus**: Our new `SendResponse()` implementation with transport
**Question**: Is `s.transport.Write(clientID, jsonMessage)` being called?

### Step 3: Check WebSocket Server Redis Subscription
**File**: `ws-server/` (Node.js WebSocket server)
**Focus**: Redis subscriber setup and channel patterns
**Question**: Is it subscribed to response channels?

### Step 4: Live Debugging Session
1. **Start containers**: `docker-compose up`
2. **Monitor Redis**: `docker exec ocpp-redis redis-cli MONITOR`
3. **Check logs**: `docker logs redis-ocpp-server -f` in another terminal
4. **Send test message** from client
5. **Observe**: Request appears in Redis ✅ Response appears in Redis ❓

### Step 5: Manual Response Test
```bash
# Test if WebSocket Server can receive manual Redis message
docker exec ocpp-redis redis-cli PUBLISH "ocpp:response:TEST-CP-001" '{"messageTypeId":3,"uniqueId":"test123","payload":{"status":"Accepted"}}'
```

## Channel Naming Investigation
**Critical Question**: What are the exact Redis channel names being used?

**Expected Pattern**:
- Requests: `ocpp:request:CLIENT_ID`
- Responses: `ocpp:response:CLIENT_ID`

**Verification**:
1. Check Redis transport channel construction
2. Verify WebSocket Server subscription patterns
3. Confirm client ID consistency across the pipeline

## Logging Strategy
Enable maximum verbosity:
- Redis OCPP Server: Debug level logging
- WebSocket Server: Debug level logging
- Client: Debug level logging
- Redis MONITOR: Real-time command monitoring

## Expected Timeline
- **15 minutes**: Initial investigation and log analysis
- **15 minutes**: Redis transport code review
- **15 minutes**: Channel naming verification and manual testing
- **15 minutes**: Fix implementation and verification

**Total**: ~1 hour to diagnose and resolve

This represents completing the final piece of a revolutionary distributed OCPP architecture. The hard work is done - this is just connecting the pipes correctly.