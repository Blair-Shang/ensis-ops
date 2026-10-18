package tools

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// ParseError 是带行列的解析失败。行、列都从 1 计。
type ParseError struct {
	Line   int
	Column int
}

func (e *ParseError) Error() string {
	if e.Column > 0 {
		return fmt.Sprintf("invalid: %d:%d", e.Line, e.Column)
	}
	if e.Line > 0 {
		return fmt.Sprintf("invalid: %d", e.Line)
	}
	return ErrInvalid.Error()
}

func (e *ParseError) Unwrap() error { return ErrInvalid }

// atOffset 把字节偏移换成 1 起算的行和列。列按 UTF-16 计，与编辑器一致。
func atOffset(src string, offset int) *ParseError {
	if offset < 0 {
		offset = 0
	}
	if offset > len(src) {
		offset = len(src)
	}
	line, col := 1, 1
	for i := 0; i < offset && i < len(src); {
		if src[i] == '\n' {
			line++
			col = 1
			i++
			continue
		}
		r, size := utf8.DecodeRuneInString(src[i:])
		if size <= 0 {
			size = 1
		}
		if r > 0xFFFF {
			col += 2
		} else {
			col++
		}
		i += size
	}
	return &ParseError{Line: line, Column: col}
}

// preferFault 在失败点落在下一行的下一个记号时，退回上一行最后一个非空白字符。
func preferFault(src string, index int) int {
	if index < 0 || index >= len(src) || !jsonTokenStart(src[index]) {
		return index
	}
	prev := index - 1
	for prev >= 0 && isJSONSpace(src[prev]) {
		prev--
	}
	if prev < 0 || !strings.Contains(src[prev+1:index], "\n") {
		return index
	}
	return prev
}

func jsonTokenStart(b byte) bool {
	switch b {
	case '"', '{', '[', '}', ']', '-', 't', 'f', 'n':
		return true
	default:
		return b >= '0' && b <= '9'
	}
}

func isJSONSpace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r'
}
