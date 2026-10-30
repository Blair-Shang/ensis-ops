package loop

import (
	"encoding/json"
	"fmt"
	"strings"

	"ensis-ops/platform/internal/ai/host"
)

// ToolScope 是这一轮用户带进来的一条可执行连接。
// 工作区页签和拖入的页签各算一条；同一会话不重复。
type ToolScope struct {
	TabID        string
	Label        string
	ModuleID     string
	Family       string
	ProfileID    string
	SessionID    string
	Database     string
	Schema       string
	Collection   string
	Cwd          string
	DialectRules string
}

// collectScopes 从工作区和 tab/connection 附件收集可执行作用域。
// 没连上的 SSH（没有 sessionId）不进入列表，避免把命令绑到一条不存在的会话。
func collectScopes(ws *ContextWorkspace, attachments []ContextAttachment) []ToolScope {
	var out []ToolScope
	if ws != nil {
		if sc, ok := scopeFromWorkspace(ws); ok {
			out = append(out, sc)
		}
	}
	for _, a := range attachments {
		sc, ok := scopeFromAttachment(a)
		if !ok {
			continue
		}
		if i := indexSameScope(out, sc); i >= 0 {
			out[i] = richerScope(out[i], sc)
			continue
		}
		out = append(out, sc)
	}
	return out
}

// toolModuleIDs 决定这一轮暴露哪些工具族。
// 工作区模块始终算上（未连接的 SSH 页签仍只给 ssh_*），附件里的可执行作用域再并进来。
func toolModuleIDs(ws *ContextWorkspace, scopes []ToolScope) []string {
	seen := map[string]struct{}{}
	var ids []string
	add := func(moduleID string) {
		fam := host.ModuleFamily(moduleID)
		if fam == "" {
			return
		}
		if _, ok := seen[fam]; ok {
			return
		}
		seen[fam] = struct{}{}
		ids = append(ids, moduleID)
	}
	if ws != nil {
		add(ws.ModuleID)
	}
	for _, sc := range scopes {
		add(sc.ModuleID)
	}
	return ids
}

func scopeFromWorkspace(ws *ContextWorkspace) (ToolScope, bool) {
	fam := host.ModuleFamily(ws.ModuleID)
	if fam == "" {
		return ToolScope{}, false
	}
	sc := ToolScope{
		TabID:        ws.TabID,
		Label:        ws.Title,
		ModuleID:     ws.ModuleID,
		Family:       fam,
		ProfileID:    ws.ProfileID,
		SessionID:    ws.SessionID,
		Database:     ws.Database,
		Schema:       ws.Schema,
		Collection:   ws.Collection,
		Cwd:          ws.Cwd,
		DialectRules: ws.DialectRules,
	}
	if !executableScope(sc) {
		return ToolScope{}, false
	}
	return sc, true
}

func scopeFromAttachment(a ContextAttachment) (ToolScope, bool) {
	if a.Kind != "tab" && a.Kind != "connection" {
		return ToolScope{}, false
	}
	moduleID := payloadString(a.Payload, "moduleId")
	fam := host.ModuleFamily(moduleID)
	if fam == "" {
		return ToolScope{}, false
	}
	sc := ToolScope{
		TabID:        payloadString(a.Payload, "tabId"),
		Label:        strings.TrimSpace(a.Label),
		ModuleID:     moduleID,
		Family:       fam,
		ProfileID:    payloadString(a.Payload, "profileId"),
		SessionID:    payloadString(a.Payload, "sessionId"),
		Database:     payloadString(a.Payload, "database"),
		Schema:       payloadString(a.Payload, "schema"),
		Collection:   payloadString(a.Payload, "collection"),
		Cwd:          firstNonEmpty(payloadString(a.Payload, "cwd"), payloadString(a.Payload, "path")),
		DialectRules: payloadString(a.Payload, "dialectRules"),
	}
	if fam == "sql" {
		tmp := ContextWorkspace{ModuleID: moduleID, Database: sc.Database, Schema: sc.Schema}
		fillSQLCatalogScope(&tmp)
		sc.Database = tmp.Database
		sc.Schema = tmp.Schema
	}
	if !executableScope(sc) {
		return ToolScope{}, false
	}
	return sc, true
}

func executableScope(sc ToolScope) bool {
	switch sc.Family {
	case "local", "tools":
		return true
	case "ssh":
		return sc.SessionID != ""
	default:
		return sc.SessionID != "" || sc.ProfileID != ""
	}
}

func indexSameScope(list []ToolScope, sc ToolScope) int {
	for i := range list {
		if sameScope(list[i], sc) {
			return i
		}
	}
	return -1
}

func sameScope(a, b ToolScope) bool {
	if a.Family != b.Family {
		return false
	}
	if a.TabID != "" && a.TabID == b.TabID {
		return true
	}
	if a.SessionID != "" && a.SessionID == b.SessionID {
		return true
	}
	if a.ProfileID != "" && a.ProfileID == b.ProfileID && (a.SessionID == "" || b.SessionID == "") {
		return true
	}
	return false
}

func richerScope(a, b ToolScope) ToolScope {
	out := a
	if b.SessionID != "" {
		out.SessionID = b.SessionID
	}
	if out.Label == "" {
		out.Label = b.Label
	}
	if out.TabID == "" {
		out.TabID = b.TabID
	}
	if out.ProfileID == "" {
		out.ProfileID = b.ProfileID
	}
	if out.ModuleID == "" {
		out.ModuleID = b.ModuleID
	}
	if out.Database == "" {
		out.Database = b.Database
	}
	if out.Schema == "" {
		out.Schema = b.Schema
	}
	if out.Collection == "" {
		out.Collection = b.Collection
	}
	if out.Cwd == "" {
		out.Cwd = b.Cwd
	}
	if out.DialectRules == "" {
		out.DialectRules = b.DialectRules
	}
	return out
}

func scopeFamilyPresent(scopes []ToolScope, family string) bool {
	for _, sc := range scopes {
		if sc.Family == family {
			return true
		}
	}
	return false
}

func scopesOf(scopes []ToolScope, family string) []ToolScope {
	var out []ToolScope
	for _, sc := range scopes {
		if sc.Family == family {
			out = append(out, sc)
		}
	}
	return out
}

func payloadString(p map[string]any, key string) string {
	if p == nil {
		return ""
	}
	s, _ := p[key].(string)
	return strings.TrimSpace(s)
}

func scopeChoices(scopes []ToolScope) string {
	parts := make([]string, 0, len(scopes))
	for _, sc := range scopes {
		name := sc.Label
		if name == "" {
			name = sc.ModuleID
		}
		parts = append(parts, fmt.Sprintf("%s session=%s", name, dash(sc.SessionID)))
	}
	return strings.Join(parts, "; ")
}

// bindToolArgs 把官方工具绑到对应作用域。
// 同一族只有一条时补齐 sessionId；有多条时必须由模型写明，禁止落到当前页签。
func bindToolArgs(raw string, scopes []ToolScope, toolName string) (json.RawMessage, error) {
	obj := parseArgObject(raw)
	family := host.ToolFamily(toolName)
	matched := scopesOf(scopes, family)
	sessionID := strings.TrimSpace(stringArg(obj, "sessionId"))
	profileID := strings.TrimSpace(stringArg(obj, "profileId"))
	if sessionID != "" || profileID != "" {
		hit, err := matchExplicitScope(matched, sessionID, profileID)
		if err != nil {
			return nil, err
		}
		if hit != nil {
			applyScope(obj, *hit, family)
		}
		return marshalArgs(obj)
	}
	switch len(matched) {
	case 0:
		return marshalArgs(obj)
	case 1:
		applyScope(obj, matched[0], family)
		return marshalArgs(obj)
	default:
		return nil, fmt.Errorf("%s needs sessionId when several %s connections are attached (%s)", toolName, family, scopeChoices(matched))
	}
}

func matchExplicitScope(matched []ToolScope, sessionID, profileID string) (*ToolScope, error) {
	if len(matched) == 0 {
		return nil, nil
	}
	if sessionID != "" {
		var hits []ToolScope
		for _, sc := range matched {
			if sc.SessionID == sessionID {
				hits = append(hits, sc)
			}
		}
		if len(hits) == 1 {
			return &hits[0], nil
		}
		if len(hits) == 0 {
			return nil, fmt.Errorf("sessionId %s is not an attached %s connection (%s)", sessionID, matched[0].Family, scopeChoices(matched))
		}
	}
	if profileID != "" {
		var hits []ToolScope
		for _, sc := range matched {
			if sc.ProfileID == profileID {
				hits = append(hits, sc)
			}
		}
		if len(hits) == 1 {
			return &hits[0], nil
		}
		if len(hits) > 1 {
			return nil, fmt.Errorf("profileId %s matches several connections; pass sessionId (%s)", profileID, scopeChoices(hits))
		}
		return nil, fmt.Errorf("profileId %s is not an attached connection (%s)", profileID, scopeChoices(matched))
	}
	return nil, fmt.Errorf("sessionId %s is ambiguous (%s)", sessionID, scopeChoices(matched))
}

func applyScope(obj map[string]any, sc ToolScope, family string) {
	setMissing(obj, "profileId", sc.ProfileID)
	setMissing(obj, "sessionId", sc.SessionID)
	setMissing(obj, "moduleId", sc.ModuleID)
	if sc.Label != "" {
		obj["scope"] = sc.Label
	}
	switch family {
	case "sql":
		setMissing(obj, "database", sc.Database)
		setMissing(obj, "schema", sc.Schema)
	case "redis":
		setMissing(obj, "database", sc.Database)
	case "mongo":
		setMissing(obj, "database", sc.Database)
		setMissing(obj, "collection", sc.Collection)
	case "ssh":
		if sc.Cwd != "" {
			setMissing(obj, "cwd", sc.Cwd)
			setMissing(obj, "path", sc.Cwd)
		}
	}
}

func setMissing(obj map[string]any, key, value string) {
	if strings.TrimSpace(value) == "" {
		return
	}
	if cur := strings.TrimSpace(stringArg(obj, key)); cur != "" {
		return
	}
	obj[key] = value
}

func parseArgObject(raw string) map[string]any {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		raw = "{}"
	}
	var obj map[string]any
	if err := json.Unmarshal([]byte(raw), &obj); err != nil || obj == nil {
		return map[string]any{}
	}
	return obj
}

func stringArg(obj map[string]any, key string) string {
	s, _ := obj[key].(string)
	return s
}

func marshalArgs(obj map[string]any) (json.RawMessage, error) {
	b, err := json.Marshal(obj)
	if err != nil {
		return nil, err
	}
	return b, nil
}

func formatScopesBlock(scopes []ToolScope) string {
	if len(scopes) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(workspaceScopesPrompt)
	b.WriteByte('\n')
	seenRules := map[string]struct{}{}
	for _, sc := range scopes {
		b.WriteString(formatScopeLine(sc))
		b.WriteByte('\n')
		rules := strings.TrimSpace(sc.DialectRules)
		if rules == "" {
			continue
		}
		if _, ok := seenRules[rules]; ok {
			continue
		}
		seenRules[rules] = struct{}{}
		b.WriteString(rules)
		b.WriteByte('\n')
	}
	return b.String()
}

func formatScopeLine(sc ToolScope) string {
	var b strings.Builder
	fmt.Fprintf(&b, "- %s %q session=%s profile=%s", sc.Family, sc.Label, dash(sc.SessionID), dash(sc.ProfileID))
	if sc.Database != "" {
		fmt.Fprintf(&b, " database=%s", sc.Database)
	}
	if sc.Schema != "" {
		fmt.Fprintf(&b, " schema=%s", sc.Schema)
	}
	if sc.Collection != "" {
		fmt.Fprintf(&b, " collection=%s", sc.Collection)
	}
	if sc.Cwd != "" {
		fmt.Fprintf(&b, " cwd=%s", sc.Cwd)
	}
	return b.String()
}

func attachmentFoldedIntoScope(a ContextAttachment) bool {
	_, ok := scopeFromAttachment(a)
	return ok
}

// FormatToolArgsSummary 给面板的一行摘要：连接名在前，会话号不占这一行。
func FormatToolArgsSummary(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	var obj map[string]any
	if err := json.Unmarshal([]byte(raw), &obj); err != nil || obj == nil {
		return clipToolSummary(raw)
	}
	label := strings.TrimSpace(stringArg(obj, "scope"))
	delete(obj, "scope")
	delete(obj, "sessionId")
	delete(obj, "profileId")
	delete(obj, "moduleId")
	b, err := json.Marshal(obj)
	text := raw
	if err == nil {
		text = string(b)
	}
	if label != "" {
		text = label + " · " + text
	}
	return clipToolSummary(text)
}

func clipToolSummary(s string) string {
	cut := truncateUTF8(s, 200)
	if cut != s {
		return cut + "…"
	}
	return cut
}
