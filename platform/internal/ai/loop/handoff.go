package loop

import (
	"regexp"
	"strings"
)

// maxHandoffNudges 是一次 run 里因输出未完成而续写的次数。只续一次，避免停不下来。
const maxHandoffNudges = 1

const (
	// finishReasonLength 表示输出额度用尽，工具调用可能还没写出来。
	finishReasonLength = "length"
	// finishReasonMaxTokens 是部分厂商对同一截断的写法。
	finishReasonMaxTokens = "max_tokens"
	// finishReasonWindow 表示回复撑满了上下文窗口，应视为未写完。
	finishReasonWindow = "model_context_window_exceeded"
)

// owesAction 判断这次无工具调用的回复是不是还没写完。
// 用户意图由模型在本轮决定：发出 tool_calls 就继续，以 stop / end_turn 结束就停。
// 这里不读正文用词，只处理两种结构信号：输出被截断，或只有思考、没有给用户的正文。
func owesAction(content, finishReason string) bool {
	if outputCutOff(finishReason) {
		return true
	}
	visible := strings.TrimSpace(stripThink(content))
	return visible == "" && strings.TrimSpace(content) != ""
}

func outputCutOff(reason string) bool {
	switch strings.ToLower(strings.TrimSpace(reason)) {
	case finishReasonLength, finishReasonMaxTokens, finishReasonWindow:
		return true
	default:
		return false
	}
}

var thinkBlock = regexp.MustCompile(`(?is)<\s*(?:think|thinking)\s*>[\s\S]*?(?:<\s*/\s*(?:think|thinking)\s*>|$)`)

func stripThink(content string) string {
	return thinkBlock.ReplaceAllString(content, "")
}
