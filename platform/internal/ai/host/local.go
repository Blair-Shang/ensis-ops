package host

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

const ServerIDLocal = "host_local"

func IsLocalModule(moduleID string) bool {
	return strings.EqualFold(strings.TrimSpace(moduleID), "local")
}

func IsLocalTool(name string) bool {
	switch name {
	case "local_host_info", "local_which", "local_read_file", "local_write_file":
		return true
	default:
		return false
	}
}

func LocalToolSpecs() []ToolSpec {
	return []ToolSpec{
		{
			Name:        "local_host_info",
			Description: "Read this computer's OS, architecture, CPU count, hostname, and home directory. Does not change the machine.",
			ServerID:    ServerIDLocal,
			Risk:        "read",
			Parameters:  objectSchema(map[string]any{}, nil),
		},
		{
			Name:        "local_which",
			Description: "Check whether one program is on PATH and read its -version output. name is a single token such as java or git, not a path and not a shell command.",
			ServerID:    ServerIDLocal,
			Risk:        "read",
			Parameters: objectSchema(map[string]any{
				"name": map[string]any{"type": "string", "description": "Program name, e.g. java"},
			}, []string{"name"}),
		},
		{
			Name:        "local_read_file",
			Description: "Read a text file under the user home directory. path may be absolute under home, or relative to home.",
			ServerID:    ServerIDLocal,
			Risk:        "read",
			Parameters: objectSchema(map[string]any{
				"path": map[string]any{"type": "string", "description": "File under the user home directory"},
			}, []string{"path"}),
		},
		{
			Name:        "local_write_file",
			Description: "Create or overwrite a text file under the user home directory. The app shows a confirm card before this runs. Parent directory must already exist.",
			ServerID:    ServerIDLocal,
			Risk:        "write",
			Parameters: objectSchema(map[string]any{
				"path":    map[string]any{"type": "string", "description": "File under the user home directory"},
				"content": map[string]any{"type": "string", "description": "Text to write"},
			}, []string{"path", "content"}),
		},
	}
}

func CallLocal(ctx context.Context, rt Runtime, name string, args map[string]any) (string, error) {
	if rt == nil {
		return "", fmt.Errorf("ai runtime is nil")
	}
	if args == nil {
		args = map[string]any{}
	}
	var method string
	params := map[string]any{}
	switch name {
	case "local_host_info":
		method = "tools.localHostInfo"
	case "local_which":
		method = "tools.localWhich"
		nameArg, _ := args["name"].(string)
		params["name"] = nameArg
	case "local_read_file":
		method = "tools.localReadFile"
		path, _ := args["path"].(string)
		params["path"] = path
	case "local_write_file":
		method = "tools.localWriteFile"
		path, _ := args["path"].(string)
		content, _ := args["content"].(string)
		params["path"] = path
		params["content"] = content
	default:
		return "", fmt.Errorf("unknown local tool %q", name)
	}
	var out map[string]any
	if err := invokeJSON(ctx, rt, method, params, &out); err != nil {
		return "", err
	}
	raw, err := json.Marshal(out)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}
