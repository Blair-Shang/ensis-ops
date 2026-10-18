package host

import (
	"context"
	"fmt"
	"strings"
)

const (
	// ServerIDTools 是官方 tools_* 工具的 invocation server_id。
	ServerIDTools = "host_tools"

	// maxToolsObserveBytes 是交回模型的观察上限，留在 Loop 截断之前，避免把 JSON 从中间切断。
	maxToolsObserveBytes = 14 * 1024
)

// 官方本机工具名（须符合 ^[a-zA-Z0-9_-]+$）。处理仍在 tools-service。
const (
	ToolFormatJSON = "tools_format_json"
	ToolFormatXML  = "tools_format_xml"
	ToolFormatSOAP = "tools_format_soap"
	ToolDiffText   = "tools_diff"
)

const (
	toolsIDJSON = "json-format"
	toolsIDXML  = "xml-format"
	toolsIDSOAP = "soap-format"
	toolsIDDiff = "text-diff"
)

// IsToolsModule 判断当前页签是不是工具工作台。
func IsToolsModule(kind string) bool {
	return strings.ToLower(strings.TrimSpace(kind)) == "tools"
}

// IsToolsTool 判断名称是否为官方 tools_*。
func IsToolsTool(name string) bool {
	switch name {
	case ToolFormatJSON, ToolFormatXML, ToolFormatSOAP, ToolDiffText:
		return true
	default:
		return false
	}
}

// ToolsToolSpecs 返回格式化与文本对比。只读，不改连接、不写盘。
func ToolsToolSpecs() []ToolSpec {
	text := map[string]any{
		"input": map[string]any{"type": "string", "description": "Text to process. Pass it in the call; editor contents are not injected."},
		"action": map[string]any{
			"type":        "string",
			"enum":        []string{"format", "minify"},
			"description": "format pretty-prints; minify collapses to one line. Default format.",
		},
		"indent": map[string]any{"type": "integer", "description": "Spaces when formatting. Use 2 or 4."},
	}
	return []ToolSpec{
		{
			Name:        ToolFormatJSON,
			Description: "Format or minify JSON. Syntax errors include a line and column. Does not change connections or files.",
			Parameters:  objectSchema(text, []string{"input"}),
			Risk:        "read",
			ServerID:    ServerIDTools,
		},
		{
			Name:        ToolFormatXML,
			Description: "Format or minify XML and check that start and end tags pair. Does not change connections or files.",
			Parameters:  objectSchema(text, []string{"input"}),
			Risk:        "read",
			ServerID:    ServerIDTools,
		},
		{
			Name:        ToolFormatSOAP,
			Description: "Format or minify a SOAP 1.1 or 1.2 Envelope. Rejects documents that are not an Envelope, and reports the line and column.",
			Parameters:  objectSchema(text, []string{"input"}),
			Risk:        "read",
			ServerID:    ServerIDTools,
		},
		{
			Name:        ToolDiffText,
			Description: "Compare two texts line by line. input is the left side, right is the right side. Returns equal, insert, delete, and change rows plus counts.",
			Parameters: objectSchema(map[string]any{
				"input": map[string]any{"type": "string", "description": "Left side"},
				"right": map[string]any{"type": "string", "description": "Right side"},
			}, nil),
			Risk:     "read",
			ServerID: ServerIDTools,
		},
	}
}

// CallTools 执行官方 tools_*：只转发到 tools.run，不在本进程解析文本。
func CallTools(ctx context.Context, rt Runtime, name string, args map[string]any) (string, error) {
	if args == nil {
		args = map[string]any{}
	}
	switch name {
	case ToolFormatJSON:
		return toolsFormat(ctx, rt, toolsIDJSON, args)
	case ToolFormatXML:
		return toolsFormat(ctx, rt, toolsIDXML, args)
	case ToolFormatSOAP:
		return toolsFormat(ctx, rt, toolsIDSOAP, args)
	case ToolDiffText:
		return toolsDiff(ctx, rt, args)
	default:
		return "", fmt.Errorf("unknown host tool: %s", name)
	}
}

func toolsFormat(ctx context.Context, rt Runtime, id string, args map[string]any) (string, error) {
	input, _ := args["input"].(string)
	if strings.TrimSpace(input) == "" {
		return "", fmt.Errorf("input required")
	}
	params := map[string]any{
		"id":     id,
		"action": toolsFormatAction(args["action"]),
		"input":  input,
		"indent": toolsIndent(args["indent"]),
	}
	var result struct {
		Output string `json:"output"`
	}
	if err := invokeJSON(ctx, rt, "tools.run", params, &result); err != nil {
		return "", err
	}
	output, truncated := truncateBytesKeepUTF8(result.Output, maxToolsObserveBytes)
	return indentJSON(map[string]any{
		"output":    output,
		"truncated": truncated,
	})
}

func toolsDiff(ctx context.Context, rt Runtime, args map[string]any) (string, error) {
	input, _ := args["input"].(string)
	right, _ := args["right"].(string)
	if strings.TrimSpace(input) == "" && strings.TrimSpace(right) == "" {
		return "", fmt.Errorf("input required")
	}
	params := map[string]any{
		"id":     toolsIDDiff,
		"action": "compare",
		"input":  input,
		"right":  right,
	}
	var result struct {
		Rows []toolsDiffRow `json:"rows"`
	}
	if err := invokeJSON(ctx, rt, "tools.run", params, &result); err != nil {
		return "", err
	}
	return presentToolsDiff(result.Rows)
}

type toolsDiffRow struct {
	Kind  string `json:"kind"`
	Left  string `json:"left"`
	Right string `json:"right"`
	Moved bool   `json:"moved,omitempty"`
}

func presentToolsDiff(rows []toolsDiffRow) (string, error) {
	added, removed, changed, moved, equal := 0, 0, 0, 0, 0
	for _, row := range rows {
		if row.Moved {
			if row.Kind == "delete" {
				moved++
			}
			continue
		}
		switch row.Kind {
		case "insert":
			added++
		case "delete":
			removed++
		case "change":
			changed++
		default:
			equal++
		}
	}
	view := rows
	truncated := false
	for {
		text, err := indentJSON(map[string]any{
			"added":     added,
			"removed":   removed,
			"changed":   changed,
			"moved":     moved,
			"equal":     equal,
			"rows":      view,
			"truncated": truncated,
		})
		if err != nil {
			return "", err
		}
		if len(text) <= maxToolsObserveBytes || len(view) == 0 {
			return text, nil
		}
		next := len(view) / 2
		if next >= len(view) {
			next = len(view) - 1
		}
		if next < 0 {
			next = 0
		}
		view = view[:next]
		truncated = true
	}
}

func toolsFormatAction(v any) string {
	action, _ := v.(string)
	switch strings.ToLower(strings.TrimSpace(action)) {
	case "", "format":
		return "format"
	case "minify":
		return "minify"
	default:
		return strings.TrimSpace(action)
	}
}

func toolsIndent(v any) int {
	n := int(intFromAny(v))
	if n <= 0 {
		return 2
	}
	if n > 8 {
		return 8
	}
	return n
}
