// Package handler 把 tools.list / tools.run 分发到工具目录。
// 本机检查（localHostInfo 等）只给 AI「这台电脑」，不进工具页目录。
//
// platform 按 manifest namespace 去掉前缀后转发，本进程看到的方法名不带 tools. 前缀。
package handler

import (
	"context"
	"encoding/json"
	"fmt"

	"ensis-ops/pkg/serviceipc/envelope"
	"ensis-ops/services/tools-service/internal/local"
	"ensis-ops/services/tools-service/internal/tools"
)

const (
	// MethodList 返回工具目录。
	MethodList = "list"
	// MethodRun 执行一个工具。
	MethodRun = "run"
	// 本机检查只给 AI「这台电脑」，不进工具页目录。
	MethodLocalHostInfo  = "localHostInfo"
	MethodLocalWhich     = "localWhich"
	MethodLocalReadFile  = "localReadFile"
	MethodLocalWriteFile = "localWriteFile"
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
	case MethodLocalHostInfo:
		return d.localHostInfo(req)
	case MethodLocalWhich:
		return d.localWhich(ctx, req)
	case MethodLocalReadFile:
		return d.localReadFile(req)
	case MethodLocalWriteFile:
		return d.localWriteFile(req)
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

func (d *Dispatcher) localHostInfo(req Request) Response {
	info, err := local.Info()
	if err != nil {
		return envelope.Fail(req.ID, err.Error())
	}
	return envelope.OK(req.ID, info)
}

func (d *Dispatcher) localWhich(ctx context.Context, req Request) Response {
	var params struct {
		Name string `json:"name"`
	}
	if len(req.Params) > 0 && json.Unmarshal(req.Params, &params) != nil {
		return envelope.Fail(req.ID, "invalid params")
	}
	got, err := local.Which(ctx, params.Name)
	if err != nil {
		return envelope.Fail(req.ID, err.Error())
	}
	return envelope.OK(req.ID, got)
}

func (d *Dispatcher) localReadFile(req Request) Response {
	var params struct {
		Path string `json:"path"`
	}
	if len(req.Params) > 0 && json.Unmarshal(req.Params, &params) != nil {
		return envelope.Fail(req.ID, "invalid params")
	}
	got, err := local.ReadFile(params.Path)
	if err != nil {
		return envelope.Fail(req.ID, err.Error())
	}
	return envelope.OK(req.ID, got)
}

func (d *Dispatcher) localWriteFile(req Request) Response {
	var params struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if len(req.Params) > 0 && json.Unmarshal(req.Params, &params) != nil {
		return envelope.Fail(req.ID, "invalid params")
	}
	got, err := local.WriteFile(params.Path, params.Content)
	if err != nil {
		return envelope.Fail(req.ID, err.Error())
	}
	return envelope.OK(req.ID, got)
}
