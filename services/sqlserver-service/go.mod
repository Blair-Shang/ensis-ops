module ensis-ops/services/sqlserver-service

go 1.25.7

require (
	github.com/microsoft/go-mssqldb v1.10.0
	ensis-ops/pkg/common v0.0.0
	ensis-ops/pkg/logutil v0.0.0
	ensis-ops/pkg/netproxy v0.0.0
	ensis-ops/pkg/serviceipc v0.0.0
	ensis-ops/pkg/sqllsp v0.0.0
	ensis-ops/pkg/tunnel v0.0.0
)

require (
	github.com/Microsoft/go-winio v0.6.2 // indirect
	github.com/golang-sql/civil v0.0.0-20220223132316-b832511892a9 // indirect
	github.com/golang-sql/sqlexp v0.1.0 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/shopspring/decimal v1.4.0 // indirect
	golang.org/x/crypto v0.54.0 // indirect
	golang.org/x/net v0.56.0 // indirect
	golang.org/x/sys v0.47.0 // indirect
	golang.org/x/text v0.40.0 // indirect
)

replace ensis-ops/pkg/common => ../../packages/go/common

replace ensis-ops/pkg/logutil => ../../packages/go/logutil

replace ensis-ops/pkg/netproxy => ../../packages/go/netproxy

replace ensis-ops/pkg/serviceipc => ../../packages/go/serviceipc

replace ensis-ops/pkg/sqllsp => ../../packages/go/sqllsp

replace ensis-ops/pkg/tunnel => ../../packages/go/tunnel
