package loop

import (
	"strings"
	"testing"
	"unicode/utf8"

	"ensis-ops/platform/internal/store"
)

func TestCompactHistory_keepsShortDialogue(t *testing.T) {
	history := []store.AIMessage{
		{MessageRole: MessageRoleUser, MessageContent: "看一下慢查询"},
		{MessageRole: MessageRoleAssistant, MessageContent: "orders 表有一条全表扫描"},
	}
	note, kept := compactHistory(history)
	if note != "" {
		t.Fatalf("note: %s", note)
	}
	if len(kept) != 2 || kept[1].MessageContent != "orders 表有一条全表扫描" {
		t.Fatalf("kept: %+v", kept)
	}
}

func TestCompactHistory_summarizesEarlierTurns(t *testing.T) {
	history := make([]store.AIMessage, 0, historyKeepTurns*2+4)
	history = append(history,
		store.AIMessage{MessageRole: MessageRoleUser, MessageContent: "查清订单库为什么变慢"},
		store.AIMessage{MessageRole: MessageRoleAssistant, MessageContent: "先看慢查询"},
		store.AIMessage{MessageRole: MessageRoleUser, MessageContent: "不要改生产库，只查"},
		store.AIMessage{MessageRole: MessageRoleAssistant, MessageContent: "只做只读查询"},
	)
	for i := 0; i < historyKeepTurns; i++ {
		history = append(history,
			store.AIMessage{MessageRole: MessageRoleUser, MessageContent: "继续看第 " + itoa(i) + " 张表"},
			store.AIMessage{MessageRole: MessageRoleAssistant, MessageContent: "这张表正常"},
		)
	}
	history = append(history, store.AIMessage{
		MessageRole:    MessageRoleTool,
		MessageContent: "SECRET_TOOL_DUMP " + strings.Repeat("x", 5000),
	})
	note, kept := compactHistory(history)
	if !strings.Contains(note, "查清订单库为什么变慢") || !strings.Contains(note, "不要改生产库") {
		t.Fatalf("note: %s", note)
	}
	if !strings.Contains(note, "约定") || !strings.Contains(note, "结论") {
		t.Fatalf("note sections: %s", note)
	}
	if strings.Contains(note, "SECRET_TOOL_DUMP") {
		t.Fatal("tool body leaked into summary")
	}
	joined := joinContents(kept)
	if strings.Contains(joined, "查清订单库为什么变慢") || strings.Contains(joined, "SECRET_TOOL_DUMP") {
		t.Fatalf("kept leaked: %s", joined)
	}
	if !strings.Contains(joined, "继续看第 "+itoa(historyKeepTurns-1)) {
		t.Fatalf("latest turn missing: %s", joined)
	}
}

func TestCompactHistory_dropsWhenVolumeExceedsBudget(t *testing.T) {
	history := []store.AIMessage{
		{MessageRole: MessageRoleUser, MessageContent: "最初要核对磁盘"},
		{MessageRole: MessageRoleAssistant, MessageContent: "磁盘还够"},
		{MessageRole: MessageRoleUser, MessageContent: strings.Repeat("查", historyBudgetRunes)},
		{MessageRole: MessageRoleAssistant, MessageContent: "查询结果很大"},
		{MessageRole: MessageRoleUser, MessageContent: "现在只要结论"},
	}
	note, kept := compactHistory(history)
	if !strings.Contains(note, "最初要核对磁盘") {
		t.Fatalf("note: %s", note)
	}
	joined := joinContents(kept)
	if strings.Contains(joined, "最初要核对磁盘") {
		t.Fatal("early turn still verbatim")
	}
	if !strings.Contains(joined, "现在只要结论") {
		t.Fatalf("current ask missing: %s", joined)
	}
	if utf8.RuneCountInString(joined) < historyBudgetRunes {
		t.Fatal("fresh oversized turn should stay")
	}
}

func TestAssembleMessages_prefixesSummary(t *testing.T) {
	history := []store.AIMessage{
		{MessageRole: MessageRoleUser, MessageContent: "查清订单库为什么变慢"},
		{MessageRole: MessageRoleAssistant, MessageContent: "先看慢查询"},
	}
	for i := 0; i < historyKeepTurns; i++ {
		history = append(history,
			store.AIMessage{MessageRole: MessageRoleUser, MessageContent: "下一张表 " + itoa(i)},
			store.AIMessage{MessageRole: MessageRoleAssistant, MessageContent: "正常"},
		)
	}
	msgs := AssembleMessages(history, NormalizedContext{}, "")
	var saw bool
	for _, m := range msgs {
		text, _ := m.Content.(string)
		if m.Role == MessageRoleSystem && strings.Contains(text, "【更早会话摘要") {
			saw = true
		}
	}
	if !saw {
		t.Fatal("summary was not injected")
	}
}

func joinContents(msgs []store.AIMessage) string {
	var b strings.Builder
	for _, m := range msgs {
		b.WriteString(m.MessageContent)
		b.WriteByte('\n')
	}
	return b.String()
}
