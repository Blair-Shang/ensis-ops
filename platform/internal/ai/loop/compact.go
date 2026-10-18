package loop

import (
	"strings"
	"unicode/utf8"

	"ensis-ops/platform/internal/store"
)

const (
	// historyKeepTurns 是摘要之后仍按原文保留的轮数。一轮从用户消息算到下一条用户消息之前。
	historyKeepTurns = 8
	// historyFreshTurns 是靠近当前问题、尽量保持完整的轮数。再往前的原文会收短。
	historyFreshTurns = 2
	// historyBudgetRunes 是保留原文的体积上限。条数没超、单条结果很大时也会往前收。
	historyBudgetRunes = 16000
	historyClipRunes   = 4000
	historyThinRunes   = 480
	historyLineRunes   = 80
	historyGoalCap     = 4
	historyRuleCap     = 4
	historyConcludeCap = 4
)

// compactHistory 把更早的轮次收成摘要，留下最近若干轮原文。
// 工具结果不进摘要，也不再整段送回模型。会话库里的原文保持不动。
func compactHistory(history []store.AIMessage) (note string, kept []store.AIMessage) {
	turns := dialogueTurns(history)
	if len(turns) == 0 {
		return "", nil
	}
	var dropped []dialogueTurn
	for len(turns) > historyFreshTurns && (len(turns) > historyKeepTurns || dialogueRunes(turns) > historyBudgetRunes) {
		dropped = append(dropped, turns[0])
		turns = turns[1:]
	}
	kept = flattenTurns(thinTurns(turns))
	if len(dropped) == 0 {
		return "", kept
	}
	return formatHistoryNote(dropped), kept
}

type dialogueTurn struct {
	msgs []store.AIMessage
}

func dialogueTurns(history []store.AIMessage) []dialogueTurn {
	var turns []dialogueTurn
	for _, m := range history {
		role := m.MessageRole
		if role != MessageRoleUser && role != MessageRoleAssistant && role != MessageRoleSystem {
			continue
		}
		if strings.TrimSpace(m.MessageContent) == "" {
			continue
		}
		if role == MessageRoleUser || len(turns) == 0 {
			turns = append(turns, dialogueTurn{})
		}
		i := len(turns) - 1
		turns[i].msgs = append(turns[i].msgs, m)
	}
	return turns
}

func flattenTurns(turns []dialogueTurn) []store.AIMessage {
	n := 0
	for _, t := range turns {
		n += len(t.msgs)
	}
	out := make([]store.AIMessage, 0, n)
	for _, t := range turns {
		out = append(out, t.msgs...)
	}
	return out
}

func thinTurns(turns []dialogueTurn) []dialogueTurn {
	freshAt := len(turns) - historyFreshTurns
	if freshAt < 0 {
		freshAt = 0
	}
	out := make([]dialogueTurn, len(turns))
	for i, turn := range turns {
		max := historyClipRunes
		if i < freshAt {
			max = historyThinRunes
		}
		msgs := make([]store.AIMessage, len(turn.msgs))
		for j, m := range turn.msgs {
			msgs[j] = m
			if m.MessageRole != MessageRoleAssistant {
				continue
			}
			msgs[j].MessageContent = clipRunes(strings.TrimSpace(m.MessageContent), max)
		}
		out[i] = dialogueTurn{msgs: msgs}
	}
	return out
}

func dialogueRunes(turns []dialogueTurn) int {
	n := 0
	thinned := thinTurns(turns)
	for _, t := range thinned {
		for _, m := range t.msgs {
			n += utf8.RuneCountInString(visibleText(m.MessageContent))
		}
	}
	return n
}

func formatHistoryNote(dropped []dialogueTurn) string {
	var goals, rules, conclusions []string
	var unfinished string
	for _, turn := range dropped {
		var userLine, assistantLine string
		for _, m := range turn.msgs {
			line := clipRunes(firstLine(visibleText(m.MessageContent)), historyLineRunes)
			if line == "" {
				continue
			}
			switch m.MessageRole {
			case MessageRoleUser:
				userLine = line
				goals = appendUnique(goals, line, historyGoalCap)
				if stickyRule(line) {
					rules = appendUnique(rules, line, historyRuleCap)
				}
			case MessageRoleAssistant:
				assistantLine = line
				conclusions = appendRecent(conclusions, line, historyConcludeCap)
			}
		}
		if userLine != "" && assistantLine == "" {
			unfinished = userLine
		} else if assistantLine != "" {
			unfinished = ""
		}
	}
	var b strings.Builder
	b.WriteString("【更早会话摘要 · 共 ")
	b.WriteString(itoa(len(dropped)))
	b.WriteString(" 轮】\n")
	writeNoteLine(&b, "目标", goals)
	writeNoteLine(&b, "约定", rules)
	writeNoteLine(&b, "结论", conclusions)
	if unfinished != "" {
		b.WriteString("未了：")
		b.WriteString(unfinished)
		b.WriteByte('\n')
	}
	if len(goals) == 0 && len(rules) == 0 && len(conclusions) == 0 && unfinished == "" {
		b.WriteString("更早的对话已省略。\n")
	}
	return strings.TrimSpace(b.String())
}

func writeNoteLine(b *strings.Builder, label string, items []string) {
	if len(items) == 0 {
		return
	}
	b.WriteString(label)
	b.WriteString("：")
	b.WriteString(strings.Join(items, "；"))
	b.WriteByte('\n')
}

func stickyRule(s string) bool {
	for _, k := range []string{"不要", "禁止", "必须", "只能", "务必", "别改", "别动"} {
		if strings.Contains(s, k) {
			return true
		}
	}
	return false
}

func visibleText(s string) string {
	return strings.TrimSpace(stripAllMarkers(s))
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexAny(s, "\r\n"); i >= 0 {
		s = s[:i]
	}
	return strings.TrimSpace(s)
}

func clipRunes(s string, max int) string {
	if max <= 0 || utf8.RuneCountInString(s) <= max {
		return s
	}
	out := make([]rune, 0, max)
	for _, r := range s {
		if len(out) == max {
			break
		}
		out = append(out, r)
	}
	return string(out) + "…"
}

func appendUnique(dst []string, item string, cap int) []string {
	item = strings.TrimSpace(item)
	if item == "" || cap <= 0 {
		return dst
	}
	for _, x := range dst {
		if x == item {
			return dst
		}
	}
	if len(dst) >= cap {
		return dst
	}
	return append(dst, item)
}

func appendRecent(dst []string, item string, cap int) []string {
	item = strings.TrimSpace(item)
	if item == "" || cap <= 0 {
		return dst
	}
	for _, x := range dst {
		if x == item {
			return dst
		}
	}
	dst = append(dst, item)
	if len(dst) > cap {
		dst = dst[len(dst)-cap:]
	}
	return dst
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [12]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
