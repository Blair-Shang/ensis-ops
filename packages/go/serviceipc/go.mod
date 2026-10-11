module ensis-ops/pkg/serviceipc

go 1.25.0

require github.com/Microsoft/go-winio v0.6.2

require ensis-ops/pkg/logutil v0.0.0

require golang.org/x/sys v0.47.0 // indirect

replace ensis-ops/pkg/logutil => ../logutil
