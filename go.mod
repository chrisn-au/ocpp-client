module ocpp-server

go 1.16

require (
	github.com/go-redis/redis/v8 v8.11.5
	github.com/gorilla/mux v1.8.1
	github.com/lorenzodonini/ocpp-go v0.0.0-00010101000000-000000000000
	github.com/stretchr/testify v1.8.0
)

replace github.com/lorenzodonini/ocpp-go => ../ocpp-go
