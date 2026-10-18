package tools

import (
	"context"
	"strings"
)

const maxDiffLines = 20000

const (
	// similarMin 低于这个相似度的两行不当成同一次修改。
	similarMin = 0.4
	// minMoveLines 连续相同的行数达到这个值，才标成移动，避免括号和空行误报。
	minMoveLines = 3
	// maxPairCells 一段里删除行和插入行的乘积超过这个数时，改用线性配对。
	maxPairCells = 2500
	// similarScanRunes 相似度只看每行前这么多个字符。
	similarScanRunes = 2000
)

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
	return RunResult{Rows: markMoves(pairDiff(myersDiff(left, right, req.IgnoreSpace)))}, nil
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
	kind  string
	left  string
	right string
}

func lineKey(s string, ignoreSpace bool) string {
	if !ignoreSpace {
		return s
	}
	return strings.Join(strings.Fields(s), " ")
}

// myersDiff 按行做 Myers 最短编辑脚本，返回从左到右的 equal / delete / insert。
// ignoreSpace 为真时，比较前先折掉空白，但结果里仍保留原文。
func myersDiff(a, b []string, ignoreSpace bool) []diffOp {
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
			for x < n && y < m && lineKey(a[x], ignoreSpace) == lineKey(b[y], ignoreSpace) {
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
			ops = append(ops, diffOp{kind: "equal", left: a[x-1], right: b[y-1]})
			x--
			y--
		}
		if x == prevX {
			ops = append(ops, diffOp{kind: "insert", right: b[y-1]})
			y--
		} else {
			ops = append(ops, diffOp{kind: "delete", left: a[x-1]})
			x--
		}
		x, y = prevX, prevY
	}
	for x > 0 && y > 0 {
		ops = append(ops, diffOp{kind: "equal", left: a[x-1], right: b[y-1]})
		x--
		y--
	}
	for i, j := 0, len(ops)-1; i < j; i, j = i+1, j-1 {
		ops[i], ops[j] = ops[j], ops[i]
	}
	return ops
}

// pairDiff 把一段里的删除和插入配成并排行。
// 只有内容足够像，或恰好一行换成一行时，才标成修改；空行不和正文配在一起。
func pairDiff(ops []diffOp) []DiffRow {
	rows := make([]DiffRow, 0, len(ops))
	for i := 0; i < len(ops); {
		if ops[i].kind == "equal" {
			rows = append(rows, DiffRow{Kind: "equal", Left: ops[i].left, Right: ops[i].right})
			i++
			continue
		}
		var dels, ins []string
		for i < len(ops) && ops[i].kind != "equal" {
			if ops[i].kind == "delete" {
				dels = append(dels, ops[i].left)
			} else {
				ins = append(ins, ops[i].right)
			}
			i++
		}
		rows = append(rows, pairHunk(dels, ins)...)
	}
	return rows
}

func pairHunk(dels, ins []string) []DiffRow {
	nd, ni := len(dels), len(ins)
	if nd == 1 && ni == 1 && strings.TrimSpace(dels[0]) != "" && strings.TrimSpace(ins[0]) != "" {
		return []DiffRow{{Kind: "change", Left: dels[0], Right: ins[0]}}
	}
	if nd*ni > maxPairCells {
		return pairHunkGreedy(dels, ins)
	}
	return pairHunkSearch(dels, ins)
}

func pairHunkGreedy(dels, ins []string) []DiffRow {
	rows := make([]DiffRow, 0, len(dels)+len(ins))
	i, j := 0, 0
	for i < len(dels) && j < len(ins) {
		if strings.TrimSpace(dels[i]) != "" && strings.TrimSpace(ins[j]) != "" &&
			(lineSimilar(dels[i], ins[j]) >= similarMin || (len(dels)-i == 1 && len(ins)-j == 1)) {
			rows = append(rows, DiffRow{Kind: "change", Left: dels[i], Right: ins[j]})
			i++
			j++
			continue
		}
		if j+1 < len(ins) && lineSimilar(dels[i], ins[j+1]) >= similarMin {
			rows = append(rows, DiffRow{Kind: "insert", Right: ins[j]})
			j++
			continue
		}
		if i+1 < len(dels) && lineSimilar(dels[i+1], ins[j]) >= similarMin {
			rows = append(rows, DiffRow{Kind: "delete", Left: dels[i]})
			i++
			continue
		}
		if len(dels)-i > len(ins)-j {
			rows = append(rows, DiffRow{Kind: "delete", Left: dels[i]})
			i++
		} else {
			rows = append(rows, DiffRow{Kind: "insert", Right: ins[j]})
			j++
		}
	}
	for ; i < len(dels); i++ {
		rows = append(rows, DiffRow{Kind: "delete", Left: dels[i]})
	}
	for ; j < len(ins); j++ {
		rows = append(rows, DiffRow{Kind: "insert", Right: ins[j]})
	}
	return rows
}

// pairHunkSearch 在一段删除和插入里找不交叉的最佳配对。
func pairHunkSearch(dels, ins []string) []DiffRow {
	nd, ni := len(dels), len(ins)
	const none = -1.0
	dp := make([][]float64, nd+1)
	from := make([][]byte, nd+1)
	for i := range dp {
		dp[i] = make([]float64, ni+1)
		from[i] = make([]byte, ni+1)
		for j := range dp[i] {
			dp[i][j] = none
		}
	}
	dp[0][0] = 0
	for i := 0; i <= nd; i++ {
		for j := 0; j <= ni; j++ {
			if dp[i][j] < 0 {
				continue
			}
			if i < nd && dp[i][j] >= dp[i+1][j] {
				dp[i+1][j] = dp[i][j]
				from[i+1][j] = 1
			}
			if j < ni && dp[i][j] >= dp[i][j+1] {
				dp[i][j+1] = dp[i][j]
				from[i][j+1] = 2
			}
			if i < nd && j < ni {
				sim := lineSimilar(dels[i], ins[j])
				if sim >= similarMin && dp[i][j]+sim >= dp[i+1][j+1] {
					dp[i+1][j+1] = dp[i][j] + sim
					from[i+1][j+1] = 3
				}
			}
		}
	}
	rev := make([]DiffRow, 0, nd+ni)
	i, j := nd, ni
	for i > 0 || j > 0 {
		switch from[i][j] {
		case 3:
			rev = append(rev, DiffRow{Kind: "change", Left: dels[i-1], Right: ins[j-1]})
			i--
			j--
		case 2:
			rev = append(rev, DiffRow{Kind: "insert", Right: ins[j-1]})
			j--
		default:
			if i == 0 {
				rev = append(rev, DiffRow{Kind: "insert", Right: ins[j-1]})
				j--
				continue
			}
			rev = append(rev, DiffRow{Kind: "delete", Left: dels[i-1]})
			i--
		}
	}
	for a, b := 0, len(rev)-1; a < b; a, b = a+1, b-1 {
		rev[a], rev[b] = rev[b], rev[a]
	}
	return rev
}

func lineSimilar(a, b string) float64 {
	if a == b {
		return 1
	}
	if strings.TrimSpace(a) == "" || strings.TrimSpace(b) == "" {
		return 0
	}
	ar := []rune(a)
	br := []rune(b)
	if len(ar) > similarScanRunes {
		ar = ar[:similarScanRunes]
	}
	if len(br) > similarScanRunes {
		br = br[:similarScanRunes]
	}
	if len(ar) < 2 || len(br) < 2 {
		return runeOverlap(ar, br)
	}
	left := bigramCounts(ar)
	right := bigramCounts(br)
	inter := 0
	for gram, n := range left {
		m := right[gram]
		if m == 0 {
			continue
		}
		if n < m {
			inter += n
		} else {
			inter += m
		}
	}
	return float64(2*inter) / float64(len(ar)-1+len(br)-1)
}

func runeOverlap(a, b []rune) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	seen := make(map[rune]int, len(a))
	for _, r := range a {
		seen[r]++
	}
	same := 0
	for _, r := range b {
		if seen[r] == 0 {
			continue
		}
		same++
		seen[r]--
	}
	return float64(2*same) / float64(len(a)+len(b))
}

func bigramCounts(r []rune) map[string]int {
	out := make(map[string]int, len(r))
	for i := 0; i < len(r)-1; i++ {
		out[string(r[i:i+2])]++
	}
	return out
}

// markMoves 把两边内容相同、且连续够长的删除和插入标成移动。
func markMoves(rows []DiffRow) []DiffRow {
	type span struct{ start, n int }
	var dels, ins []span
	for i := 0; i < len(rows); {
		switch rows[i].Kind {
		case "delete":
			start := i
			for i < len(rows) && rows[i].Kind == "delete" {
				i++
			}
			if i-start >= minMoveLines {
				dels = append(dels, span{start, i - start})
			}
		case "insert":
			start := i
			for i < len(rows) && rows[i].Kind == "insert" {
				i++
			}
			if i-start >= minMoveLines {
				ins = append(ins, span{start, i - start})
			}
		default:
			i++
		}
	}
	usedDel := map[int]bool{}
	usedIns := map[int]bool{}
	for _, d := range dels {
		block := rowTexts(rows, d.start, d.n, true)
		for ii, in := range ins {
			if usedIns[ii] || usedDel[d.start] {
				continue
			}
			hay := rowTexts(rows, in.start, in.n, false)
			if at := findSub(hay, block); at >= 0 && substantial(block) {
				for k := 0; k < len(block); k++ {
					rows[d.start+k].Moved = true
					rows[in.start+at+k].Moved = true
				}
				usedDel[d.start] = true
				usedIns[ii] = true
				continue
			}
			if at := findSub(block, hay); at >= 0 && substantial(hay) {
				for k := 0; k < len(hay); k++ {
					rows[d.start+at+k].Moved = true
					rows[in.start+k].Moved = true
				}
				usedDel[d.start] = true
				usedIns[ii] = true
			}
		}
	}
	return rows
}

func rowTexts(rows []DiffRow, start, n int, left bool) []string {
	out := make([]string, n)
	for i := 0; i < n; i++ {
		if left {
			out[i] = rows[start+i].Left
		} else {
			out[i] = rows[start+i].Right
		}
	}
	return out
}

func substantial(lines []string) bool {
	for _, line := range lines {
		if len(strings.TrimSpace(line)) >= 8 {
			return true
		}
	}
	return false
}

func findSub(hay, needle []string) int {
	if len(needle) < minMoveLines || len(needle) > len(hay) {
		return -1
	}
	for i := 0; i+len(needle) <= len(hay); i++ {
		ok := true
		for k := range needle {
			if hay[i+k] != needle[k] {
				ok = false
				break
			}
		}
		if ok {
			return i
		}
	}
	return -1
}
