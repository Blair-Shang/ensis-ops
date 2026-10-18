package tools

import "context"

const maxDiffLines = 20000

func init() {
	Register(Spec{
		ID:       "text-diff",
		Category: CategoryText,
		TitleKey: "tools.diff.title",
		DescKey:  "tools.diff.desc",
		Icon:     "git-compare",
		Order:    10,
		Kind:     KindDiff,
	}, runDiff)
}

func runDiff(ctx context.Context, req RunRequest) (RunResult, error) {
	if err := ctx.Err(); err != nil {
		return RunResult{}, err
	}
	if req.Action != ActionCompare {
		return RunResult{}, ErrBadAction
	}
	left := splitLines(req.Input)
	right := splitLines(req.Right)
	if len(left) == 0 && len(right) == 0 {
		return RunResult{}, ErrEmpty
	}
	if len(left) > maxDiffLines || len(right) > maxDiffLines {
		return RunResult{}, ErrTooLarge
	}
	return RunResult{Rows: pairDiff(myersDiff(left, right))}, nil
}

func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	lines := make([]string, 0, 16)
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] != '\n' {
			continue
		}
		lines = append(lines, trimCR(s[start:i]))
		start = i + 1
	}
	if start < len(s) {
		lines = append(lines, trimCR(s[start:]))
	}
	return lines
}

func trimCR(s string) string {
	if len(s) > 0 && s[len(s)-1] == '\r' {
		return s[:len(s)-1]
	}
	return s
}

type diffOp struct {
	kind string
	text string
}

// myersDiff 按行做 Myers 最短编辑脚本，返回从左到右的 equal / delete / insert。
func myersDiff(a, b []string) []diffOp {
	n, m := len(a), len(b)
	max := n + m
	if max == 0 {
		return nil
	}
	offset := max
	v := make([]int, 2*max+1)
	trace := make([][]int, 0, max+1)

	for d := 0; d <= max; d++ {
		for k := -d; k <= d; k += 2 {
			var x int
			if k == -d || (k != d && v[offset+k-1] < v[offset+k+1]) {
				x = v[offset+k+1]
			} else {
				x = v[offset+k-1] + 1
			}
			y := x - k
			for x < n && y < m && a[x] == b[y] {
				x++
				y++
			}
			v[offset+k] = x
			if x >= n && y >= m {
				trace = append(trace, append([]int(nil), v...))
				return backtrack(a, b, trace)
			}
		}
		trace = append(trace, append([]int(nil), v...))
	}
	return nil
}

func backtrack(a, b []string, trace [][]int) []diffOp {
	x, y := len(a), len(b)
	offset := len(a) + len(b)
	ops := make([]diffOp, 0)
	for d := len(trace) - 1; d > 0; d-- {
		prev := trace[d-1]
		k := x - y
		var prevK int
		if k == -d || (k != d && prev[offset+k-1] < prev[offset+k+1]) {
			prevK = k + 1
		} else {
			prevK = k - 1
		}
		prevX := prev[offset+prevK]
		prevY := prevX - prevK
		for x > prevX && y > prevY {
			ops = append(ops, diffOp{kind: "equal", text: a[x-1]})
			x--
			y--
		}
		if x == prevX {
			ops = append(ops, diffOp{kind: "insert", text: b[y-1]})
			y--
		} else {
			ops = append(ops, diffOp{kind: "delete", text: a[x-1]})
			x--
		}
		x, y = prevX, prevY
	}
	for x > 0 && y > 0 {
		ops = append(ops, diffOp{kind: "equal", text: a[x-1]})
		x--
		y--
	}
	for i, j := 0, len(ops)-1; i < j; i, j = i+1, j-1 {
		ops[i], ops[j] = ops[j], ops[i]
	}
	return ops
}

// pairDiff 把连续的删除和插入配成并排的 change 行。
func pairDiff(ops []diffOp) []DiffRow {
	rows := make([]DiffRow, 0, len(ops))
	for i := 0; i < len(ops); {
		if ops[i].kind == "equal" {
			rows = append(rows, DiffRow{Kind: "equal", Left: ops[i].text, Right: ops[i].text})
			i++
			continue
		}
		var dels, ins []string
		for i < len(ops) && ops[i].kind != "equal" {
			if ops[i].kind == "delete" {
				dels = append(dels, ops[i].text)
			} else {
				ins = append(ins, ops[i].text)
			}
			i++
		}
		n := len(dels)
		if len(ins) > n {
			n = len(ins)
		}
		for j := 0; j < n; j++ {
			row := DiffRow{Kind: "change"}
			if j < len(dels) {
				row.Left = dels[j]
			}
			if j < len(ins) {
				row.Right = ins[j]
			}
			switch {
			case j >= len(dels):
				row.Kind = "insert"
			case j >= len(ins):
				row.Kind = "delete"
			}
			rows = append(rows, row)
		}
	}
	return rows
}
