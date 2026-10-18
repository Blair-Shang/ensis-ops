module ensis-ops/services/tools-service

go 1.25.0

require (
	ensis-ops/pkg/logutil v0.0.0
	ensis-ops/pkg/serviceipc v0.0.0
)

require (
	github.com/Microsoft/go-winio v0.6.2 // indirect
	golang.org/x/sys v0.47.0 // indirect
)

replace ensis-ops/pkg/logutil => ../../packages/go/logutil

replace ensis-ops/pkg/serviceipc => ../../packages/go/serviceipc
