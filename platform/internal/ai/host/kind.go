package host

import (
	"fmt"
	"strings"
)

// IsSQLModule 判断 moduleId / connection kind 是否走官方 sql_*。
func IsSQLModule(kind string) bool {
	k := strings.ToLower(strings.TrimSpace(kind))
	switch k {
	case "vastbase", "postgres", "postgresql", "kingbase", "mysql", "mariadb",
		"sqlite", "dameng", "oracle", "clickhouse", "sqlserver", "mssql":
		return true
	default:
		return false
	}
}

// IsSSHModule 判断 moduleId / connection kind 是否走官方 ssh_*。
func IsSSHModule(kind string) bool {
	return strings.ToLower(strings.TrimSpace(kind)) == "ssh"
}

// IsRedisModule 判断 moduleId / connection kind 是否走官方 redis_*。
func IsRedisModule(kind string) bool {
	return strings.ToLower(strings.TrimSpace(kind)) == "redis"
}

// IsMongoModule 判断 moduleId / connection kind 是否走官方 mongo_*。
func IsMongoModule(kind string) bool {
	return strings.ToLower(strings.TrimSpace(kind)) == "mongodb"
}

// ModuleFamily 把页签模块收成工具族。空串表示不是官方连接族。
func ModuleFamily(moduleID string) string {
	switch {
	case IsSSHModule(moduleID):
		return "ssh"
	case IsRedisModule(moduleID):
		return "redis"
	case IsMongoModule(moduleID):
		return "mongo"
	case IsSQLModule(moduleID):
		return "sql"
	case IsLocalModule(moduleID):
		return "local"
	case IsToolsModule(moduleID):
		return "tools"
	default:
		return ""
	}
}

// ToolFamily 把官方工具名收成工具族。MCP 或未知名称返回空串。
func ToolFamily(name string) string {
	switch {
	case IsSQLTool(name):
		return "sql"
	case IsSSHTool(name):
		return "ssh"
	case IsRedisTool(name):
		return "redis"
	case IsMongoTool(name):
		return "mongo"
	case IsLocalTool(name):
		return "local"
	case IsToolsTool(name):
		return "tools"
	default:
		return ""
	}
}

// HostToolSpecs 按当前 workspace 模块挑选官方工具。
// SSH / Redis / Mongo / SQL / 工具页签只暴露本族；其它（含空）各族都给，避免无页签时丢能力。
func HostToolSpecs(moduleID string) []ToolSpec {
	if IsSSHModule(moduleID) {
		return SSHToolSpecs()
	}
	if IsRedisModule(moduleID) {
		return RedisToolSpecs()
	}
	if IsMongoModule(moduleID) {
		return MongoToolSpecs()
	}
	if IsSQLModule(moduleID) {
		return SQLToolSpecs()
	}
	if IsToolsModule(moduleID) {
		return ToolsToolSpecs()
	}
	// 「这台电脑」要用户在输入框里点明。空白页签不带本机工具。
	if IsLocalModule(moduleID) {
		return LocalToolSpecs()
	}
	out := append([]ToolSpec{}, SQLToolSpecs()...)
	out = append(out, SSHToolSpecs()...)
	out = append(out, RedisToolSpecs()...)
	out = append(out, MongoToolSpecs()...)
	return append(out, ToolsToolSpecs()...)
}

// HostToolSpecsForModules 按这一轮带入的模块并集挑选工具。
// 空列表与无页签相同。同一族只收一次，顺序跟模块列表一致。
func HostToolSpecsForModules(moduleIDs []string) []ToolSpec {
	if len(moduleIDs) == 0 {
		return HostToolSpecs("")
	}
	seen := make(map[string]struct{}, len(moduleIDs))
	ids := make([]string, 0, len(moduleIDs))
	for _, id := range moduleIDs {
		fam := ModuleFamily(id)
		if fam == "" {
			continue
		}
		if _, ok := seen[fam]; ok {
			continue
		}
		seen[fam] = struct{}{}
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return HostToolSpecs("")
	}
	if len(ids) == 1 {
		return HostToolSpecs(ids[0])
	}
	var out []ToolSpec
	have := make(map[string]struct{})
	for _, id := range ids {
		for _, spec := range HostToolSpecs(id) {
			if _, ok := have[spec.Name]; ok {
				continue
			}
			have[spec.Name] = struct{}{}
			out = append(out, spec)
		}
	}
	return out
}

// SpecServerID 返回工具所属官方 server_id。
func SpecServerID(spec ToolSpec) string {
	if strings.TrimSpace(spec.ServerID) != "" {
		return spec.ServerID
	}
	if IsSSHTool(spec.Name) {
		return ServerIDSSH
	}
	if IsRedisTool(spec.Name) {
		return ServerIDRedis
	}
	if IsMongoTool(spec.Name) {
		return ServerIDMongo
	}
	if IsToolsTool(spec.Name) {
		return ServerIDTools
	}
	if IsLocalTool(spec.Name) {
		return ServerIDLocal
	}
	return ServerIDSQL
}

// UsesDatabaseAsSchema 表示 catalog 的 schema 槽位就是 database 名（无独立 schema 层）。
// MySQL / MariaDB / ClickHouse 如此；PostgreSQL 族仍用 public。
func UsesDatabaseAsSchema(kind string) bool {
	ns, err := NamespaceForKind(kind)
	if err != nil {
		k := strings.ToLower(strings.TrimSpace(kind))
		return k == "mysql" || k == "mariadb" || k == "clickhouse"
	}
	return ns == "mysql" || ns == "clickhouse"
}

// NamespaceForKind 将连接 kind / moduleId 映射为 Bridge namespace。
//
// 未识别的 kind 原样返回（已是 namespace 时）；空串报错。
func NamespaceForKind(kind string) (string, error) {
	k := strings.ToLower(strings.TrimSpace(kind))
	if k == "" {
		return "", fmt.Errorf("sql host: moduleId or connection kind required (open a database tab)")
	}
	switch k {
	case "postgresql":
		return "postgres", nil
	case "mariadb":
		return "mysql", nil
	case "mssql":
		return "sqlserver", nil
	case "vastbase", "postgres", "kingbase", "mysql", "sqlite",
		"dameng", "oracle", "clickhouse", "sqlserver":
		return k, nil
	default:
		return k, nil
	}
}
