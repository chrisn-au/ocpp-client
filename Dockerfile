FROM golang:1.16-alpine AS builder

WORKDIR /app

# Copy the local ocpp-go library
COPY ocpp-go ./ocpp-go

# Copy ocpp-server files
COPY ocpp-server/go.mod ocpp-server/go.sum ./

# Copy the rest of the ocpp-server application
COPY ocpp-server/ ./

# Fix the module replacement path to match Docker context
RUN sed -i 's|=> ../ocpp-go|=> ./ocpp-go|g' go.mod

# Download dependencies and build
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main .

FROM alpine:latest

RUN apk --no-cache add ca-certificates
WORKDIR /root/

COPY --from=builder /app/main .

EXPOSE 8081

CMD ["./main"]