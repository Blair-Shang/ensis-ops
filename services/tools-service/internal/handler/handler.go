// Package handler 把 tools.list / tools.run 分发到工具目录。
//
// platform 按 manifest namespace 去掉前缀后转发，本进程看到的方法名是 list 与 run。
package handler

import (
	"context"
	"encoding/json"
	"fmt"

	"ensis-ops/pkg/serviceipc/envelope"
	"ensis-ops/services/tools-service/internal/tools"
)

const (
	// MethodList 返回工具目录。
	MethodList = "list"
	// MethodRun 执行一个工具。
	MethodRun = "run"
)

// Request 是 IPC 请求帧。
type Request = envelope.Request

// Response 是 IPC 响应帧。
type Response = envelope.Response

// Dispatcher 只做方法分发，不持有连接或凭据。
type Dispatcher struct{}

// New 创建 Dispatcher。
func New() *Dispatcher {
	return &Dispatcher{}
}

// HandleFrame 解析请求并返回响应 JSON 字节。
func (d *Dispatcher) HandleFrame(ctx context.Context, raw []byte) []byte {
	var req Request
	if err := json.Unmarshal(raw, &req); err != nil {
		return envelope.Marshal(envelope.Fail("", fmt.Sprintf("invalid request json: %v", err)))
	}
	return envelope.Marshal(envelope.WithRequest(req, d.dispatch(ctx, req)))
}

func (d *Dispatcher) dispatch(ctx context.Context, req Request) Response {
	switch req.Method {
	case MethodList:
		return envelope.OK(req.ID, tools.List())
	case MethodRun:
		return d.run(ctx, req)
	default:
		return envelope.Fail(req.ID, "method not found: "+req.Method)
	}
}

func (d *Dispatcher) run(ctx context.Context, req Request) Response {
	var params tools.RunRequest
	if len(req.Params) > 0 {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return envelope.Fail(req.ID, "invalid params: "+err.Error())
		}
	}
	if params.ID == "" {
		return envelope.Fail(req.ID, "id required")
	}
	result, err := tools.Run(ctx, params)
	if err != nil {
		return envelope.Fail(req.ID, err.Error())
	}
	return envelope.OK(req.ID, result)
}
