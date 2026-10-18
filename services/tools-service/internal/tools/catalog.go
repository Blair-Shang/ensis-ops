package tools

import (
	"context"
	"errors"
	"sort"
	"sync"
)

const (
	// CategoryFormat 是格式化类工具的分类 id。
	CategoryFormat = "format"
	// CategoryText 是文本类工具的分类 id。
	CategoryText = "text"
	// KindFormat 表示前端使用共用格式化页。
	KindFormat = "format"
	// KindDiff 表示前端使用文本对比页。
	KindDiff = "diff"
	// ActionFormat 按缩进重排输入。
	ActionFormat = "format"
	// ActionMinify 去掉可省略空白。
	ActionMinify = "minify"
	// ActionCompare 按行对比左右两段文本。
	ActionCompare = "compare"

	// maxInputBytes 限制单次处理体积，避免大文本占住工具进程。
	maxInputBytes = 8 << 20
)

var (
	// ErrEmpty 表示输入为空。
	ErrEmpty = errors.New("empty")
	// ErrTooLarge 表示输入超过上限。
	ErrTooLarge = errors.New("too_large")
	// ErrInvalid 表示输入无法解析。
	ErrInvalid = errors.New("invalid")
	// ErrNotSOAP 表示文档不是 SOAP Envelope。
	ErrNotSOAP = errors.New("not_soap")
	// ErrUnknown 表示目录中没有该工具。
	ErrUnknown = errors.New("unknown_tool")
	// ErrBadAction 表示 action 不被该工具接受。
	ErrBadAction = errors.New("bad_action")
)

// Spec 是工具目录项。文案键由前端 i18n 翻译，本包不写死语言。
type Spec struct {
	ID       string `json:"id"`
	Category string `json:"category"`
	TitleKey string `json:"titleKey"`
	DescKey  string `json:"descKey"`
	Icon     string `json:"icon"`
	Order    int    `json:"order"`
	Kind     string `json:"kind"`
}

// CategoryGroup 是同一分类下的工具。
type CategoryGroup struct {
	ID       string `json:"id"`
	TitleKey string `json:"titleKey"`
	Order    int    `json:"order"`
	Tools    []Spec `json:"tools"`
}

// ListResult 是 tools.list 的结果。
type ListResult struct {
	Categories []CategoryGroup `json:"categories"`
}

// RunRequest 是 tools.run 的入参。Right 和 IgnoreSpace 只给文本对比使用。
type RunRequest struct {
	ID          string `json:"id"`
	Action      string `json:"action"`
	Input       string `json:"input"`
	Right       string `json:"right"`
	Indent      int    `json:"indent"`
	IgnoreSpace bool   `json:"ignoreSpace"`
}

// DiffRow 是并排对比的一行。Moved 表示这行在另一侧原样出现，是移动而不是改写。
type DiffRow struct {
	Kind  string `json:"kind"`
	Left  string `json:"left"`
	Right string `json:"right"`
	Moved bool   `json:"moved,omitempty"`
}

// RunResult 是处理结果。格式化填 Output，对比填 Rows。
type RunResult struct {
	Output string    `json:"output,omitempty"`
	Rows   []DiffRow `json:"rows,omitempty"`
}

// Runner 执行一个已登记工具。
type Runner func(ctx context.Context, req RunRequest) (RunResult, error)

type entry struct {
	spec Spec
	run  Runner
}

var (
	mu    sync.RWMutex
	items []entry
)

type categoryMeta struct {
	titleKey string
	order    int
}

func knownCategory(id string) categoryMeta {
	switch id {
	case CategoryFormat:
		return categoryMeta{titleKey: "tools.category.format", order: 10}
	case CategoryText:
		return categoryMeta{titleKey: "tools.category.text", order: 20}
	default:
		return categoryMeta{titleKey: "tools.category." + id, order: 1000}
	}
}

// Register 把工具登记进目录。同 id 已存在时保留先登记的一项。
func Register(spec Spec, run Runner) {
	if spec.ID == "" || spec.Category == "" || run == nil {
		return
	}
	mu.Lock()
	defer mu.Unlock()
	for _, item := range items {
		if item.spec.ID == spec.ID {
			return
		}
	}
	items = append(items, entry{spec: spec, run: run})
}

// List 返回按分类与 order 排好的目录，不执行工具。
func List() ListResult {
	mu.RLock()
	defer mu.RUnlock()

	groups := make(map[string]*CategoryGroup)
	order := make([]string, 0)
	for _, item := range items {
		group, ok := groups[item.spec.Category]
		if !ok {
			meta := knownCategory(item.spec.Category)
			group = &CategoryGroup{
				ID:       item.spec.Category,
				TitleKey: meta.titleKey,
				Order:    meta.order,
			}
			groups[item.spec.Category] = group
			order = append(order, item.spec.Category)
		}
		group.Tools = append(group.Tools, item.spec)
	}

	out := make([]CategoryGroup, 0, len(order))
	for _, id := range order {
		group := groups[id]
		sort.SliceStable(group.Tools, func(i, j int) bool {
			return group.Tools[i].Order < group.Tools[j].Order
		})
		out = append(out, *group)
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Order < out[j].Order
	})
	return ListResult{Categories: out}
}

// Run 按 id 执行已登记工具。未知 id、超限与非法 action 直接返回，不调用 Runner。
func Run(ctx context.Context, req RunRequest) (RunResult, error) {
	if len(req.Input)+len(req.Right) > maxInputBytes {
		return RunResult{}, ErrTooLarge
	}
	if req.Action == "" {
		req.Action = ActionFormat
	}
	switch req.Action {
	case ActionFormat, ActionMinify, ActionCompare:
	default:
		return RunResult{}, ErrBadAction
	}

	mu.RLock()
	var run Runner
	for _, item := range items {
		if item.spec.ID == req.ID {
			run = item.run
			break
		}
	}
	mu.RUnlock()
	if run == nil {
		return RunResult{}, ErrUnknown
	}
	return run(ctx, req)
}

// clampIndent 把缩进空格数限制在 1 到 8。未传时用 2。
func clampIndent(indent int) int {
	if indent <= 0 {
		return 2
	}
	if indent > 8 {
		return 8
	}
	return indent
}
