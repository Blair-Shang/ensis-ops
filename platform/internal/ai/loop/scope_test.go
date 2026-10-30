package loop

import (
	"encoding/json"
	"strings"
	"testing"

	"ensis-ops/platform/internal/ai/host"
)

func TestNormalizeContext_sshAndMySQLScopes(t *testing.T) {
	n := NormalizeContext(&ContextDraft{
		Workspace: &ContextWorkspace{
			TabID:     "tab-ssh",
			ModuleID:  "ssh",
			ProfileID: "p-ssh",
			SessionID: "s-ssh",
			Title:     "web-1",
			Cwd:       "/opt/app",
		},
		Attachments: []ContextAttachment{{
			ID:    "tab:db",
			Kind:  "tab",
			Label: "orders",
			Payload: map[string]any{
				"tabId":        "tab-db",
				"moduleId":     "mysql",
				"profileId":    "p-db",
				"sessionId":    "s-db",
				"database":     "app",
				"dialectRules": "mysql: use the current database",
			},
		}},
	})
	if len(n.Scopes) != 2 {
		t.Fatalf("scopes=%d", len(n.Scopes))
	}
	if !strings.Contains(n.PromptBlock, "[Scopes]") {
		t.Fatalf("missing scopes: %q", n.PromptBlock)
	}
	if !strings.Contains(n.PromptBlock, "session=s-ssh") || !strings.Contains(n.PromptBlock, "session=s-db") {
		t.Fatalf("sessions: %q", n.PromptBlock)
	}
	if strings.Contains(n.PromptBlock, "Do not call sql_*") || strings.Contains(n.PromptBlock, "[Workspace · SSH]") {
		t.Fatalf("exclusive guide leaked: %q", n.PromptBlock)
	}
	if !strings.Contains(n.PromptBlock, "mysql: use the current database") {
		t.Fatalf("dialect missing: %q", n.PromptBlock)
	}
	mods := toolModuleIDs(n.Workspace, n.Scopes)
	specs := host.HostToolSpecsForModules(mods)
	var ssh, sql, redis bool
	for _, spec := range specs {
		if host.IsSSHTool(spec.Name) {
			ssh = true
		}
		if host.IsSQLTool(spec.Name) {
			sql = true
		}
		if host.IsRedisTool(spec.Name) {
			redis = true
		}
	}
	if !ssh || !sql || redis {
		t.Fatalf("tools ssh=%v sql=%v redis=%v", ssh, sql, redis)
	}
}

func TestBindToolArgs_familyKeepsItsOwnSession(t *testing.T) {
	n := NormalizeContext(&ContextDraft{
		Workspace: &ContextWorkspace{
			ModuleID:  "ssh",
			ProfileID: "p-ssh",
			SessionID: "s-ssh",
			Title:     "web-1",
			Cwd:       "/opt/app",
		},
		Attachments: []ContextAttachment{{
			ID:    "tab:db",
			Kind:  "tab",
			Label: "orders",
			Payload: map[string]any{
				"tabId":     "tab-db",
				"moduleId":  "mysql",
				"profileId": "p-db",
				"sessionId": "s-db",
				"database":  "app",
			},
		}},
	})
	sqlRaw, err := mergeWorkspaceArgs(`{"sql":"SELECT 1"}`, n, host.ToolRunReadonly)
	if err != nil {
		t.Fatal(err)
	}
	sqlArgs := decodeArgs(t, sqlRaw)
	if sqlArgs["sessionId"] != "s-db" || sqlArgs["profileId"] != "p-db" || sqlArgs["schema"] != "app" || sqlArgs["scope"] != "orders" {
		t.Fatalf("sql bound wrong: %v", sqlArgs)
	}
	if sqlArgs["cwd"] != nil {
		t.Fatalf("ssh cwd leaked into sql: %v", sqlArgs)
	}
	sshRaw, err := mergeWorkspaceArgs(`{"command":"uptime"}`, n, host.ToolSSHExec)
	if err != nil {
		t.Fatal(err)
	}
	sshArgs := decodeArgs(t, sshRaw)
	if sshArgs["sessionId"] != "s-ssh" || sshArgs["path"] != "/opt/app" || sshArgs["scope"] != "web-1" {
		t.Fatalf("ssh bound wrong: %v", sshArgs)
	}
}

func TestBindToolArgs_severalSSHRequireSession(t *testing.T) {
	n := NormalizeContext(&ContextDraft{
		Workspace: &ContextWorkspace{
			TabID:     "tab-a",
			ModuleID:  "ssh",
			ProfileID: "p-a",
			SessionID: "s-a",
			Title:     "a",
		},
		Attachments: []ContextAttachment{{
			ID:    "tab:b",
			Kind:  "tab",
			Label: "b",
			Payload: map[string]any{
				"tabId":     "tab-b",
				"moduleId":  "ssh",
				"profileId": "p-b",
				"sessionId": "s-b",
			},
		}},
	})
	if _, err := mergeWorkspaceArgs(`{"command":"uptime"}`, n, host.ToolSSHExec); err == nil {
		t.Fatal("expected sessionId required")
	}
	raw, err := mergeWorkspaceArgs(`{"command":"uptime","sessionId":"s-b"}`, n, host.ToolSSHExec)
	if err != nil {
		t.Fatal(err)
	}
	args := decodeArgs(t, raw)
	if args["sessionId"] != "s-b" || args["profileId"] != "p-b" || args["scope"] != "b" {
		t.Fatalf("%v", args)
	}
	if _, err := mergeWorkspaceArgs(`{"command":"uptime","sessionId":"other"}`, n, host.ToolSSHExec); err == nil {
		t.Fatal("expected unknown session rejected")
	}
}

func TestFormatToolArgsSummary_putsScopeFirst(t *testing.T) {
	got := FormatToolArgsSummary(`{"command":"uptime","sessionId":"s1","profileId":"p1","scope":"web-1"}`)
	if !strings.HasPrefix(got, "web-1 · ") {
		t.Fatalf("%q", got)
	}
	if strings.Contains(got, "s1") || strings.Contains(got, "p1") {
		t.Fatalf("ids leaked: %q", got)
	}
	if !strings.Contains(got, "uptime") {
		t.Fatalf("%q", got)
	}
}

func TestCollectScopes_dedupActiveTab(t *testing.T) {
	n := NormalizeContext(&ContextDraft{
		Workspace: &ContextWorkspace{
			TabID:     "tab-ssh",
			ModuleID:  "ssh",
			ProfileID: "p-ssh",
			SessionID: "s-ssh",
			Title:     "web-1",
		},
		Attachments: []ContextAttachment{{
			ID:    "tab:tab-ssh",
			Kind:  "tab",
			Label: "web-1",
			Payload: map[string]any{
				"tabId":     "tab-ssh",
				"moduleId":  "ssh",
				"profileId": "p-ssh",
				"sessionId": "s-ssh",
			},
		}},
	})
	if len(n.Scopes) != 1 {
		t.Fatalf("scopes=%d", len(n.Scopes))
	}
	if !strings.Contains(n.PromptBlock, "[Workspace · SSH]") {
		t.Fatalf("single ssh should keep its guide: %q", n.PromptBlock)
	}
}

func decodeArgs(t *testing.T, raw json.RawMessage) map[string]any {
	t.Helper()
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		t.Fatal(err)
	}
	return obj
}
