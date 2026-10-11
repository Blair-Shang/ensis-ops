module ensis-ops/services/ftp-service

go 1.25.0

require (
	github.com/jlaffaye/ftp v0.2.0
	ensis-ops/pkg/logutil v0.0.0
	ensis-ops/pkg/netproxy v0.0.0
	ensis-ops/pkg/serviceipc v0.0.0
	ensis-ops/pkg/tunnel v0.0.0-00010101000000-000000000000
)

require (
	github.com/Microsoft/go-winio v0.6.2 // indirect
	github.com/hashicorp/errwrap v1.0.0 // indirect
	github.com/hashicorp/go-multierror v1.1.1 // indirect
	github.com/stretchr/testify v1.11.1 // indirect
	golang.org/x/crypto v0.54.0 // indirect
	golang.org/x/net v0.56.0 // indirect
	golang.org/x/sys v0.47.0 // indirect
)

replace ensis-ops/pkg/buildinfo => ../../packages/go/buildinfo

replace ensis-ops/pkg/logutil => ../../packages/go/logutil

replace ensis-ops/pkg/netproxy => ../../packages/go/netproxy

replace ensis-ops/pkg/serviceipc => ../../packages/go/serviceipc

replace ensis-ops/pkg/tunnel => ../../packages/go/tunnel
