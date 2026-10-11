module ensis-ops/services/clickhouse-service

go 1.25.0

require (
	github.com/ClickHouse/clickhouse-go/v2 v2.40.1
	golang.org/x/text v0.40.0
	ensis-ops/pkg/common v0.0.0
	ensis-ops/pkg/logutil v0.0.0
	ensis-ops/pkg/netproxy v0.0.0-00010101000000-000000000000
	ensis-ops/pkg/serviceipc v0.0.0
	ensis-ops/pkg/sqllsp v0.0.0
	ensis-ops/pkg/tunnel v0.0.0-00010101000000-000000000000
)

require (
	github.com/ClickHouse/ch-go v0.67.0 // indirect
	github.com/Microsoft/go-winio v0.6.2 // indirect
	github.com/andybalholm/brotli v1.2.0 // indirect
	github.com/go-faster/city v1.0.1 // indirect
	github.com/go-faster/errors v0.7.1 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/klauspost/compress v1.18.0 // indirect
	github.com/paulmach/orb v0.11.1 // indirect
	github.com/pierrec/lz4/v4 v4.1.22 // indirect
	github.com/segmentio/asm v1.2.0 // indirect
	github.com/shopspring/decimal v1.4.0 // indirect
	go.opentelemetry.io/otel v1.37.0 // indirect
	go.opentelemetry.io/otel/trace v1.37.0 // indirect
	golang.org/x/crypto v0.54.0 // indirect
	golang.org/x/net v0.56.0 // indirect
	golang.org/x/sys v0.47.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace ensis-ops/pkg/common => ../../packages/go/common

replace ensis-ops/pkg/logutil => ../../packages/go/logutil

replace ensis-ops/pkg/netproxy => ../../packages/go/netproxy

replace ensis-ops/pkg/serviceipc => ../../packages/go/serviceipc

replace ensis-ops/pkg/sqllsp => ../../packages/go/sqllsp

replace ensis-ops/pkg/tunnel => ../../packages/go/tunnel
