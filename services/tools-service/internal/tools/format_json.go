package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
)

func init() {
	Register(Spec{
		ID:       "json-format",
		Category: CategoryFormat,
		TitleKey: "tools.json.title",
		DescKey:  "tools.json.desc",
		Icon:     "braces",
		Order:    10,
		Kind:     KindFormat,
	}, runJSON)
}

func runJSON(ctx context.Context, req RunRequest) (RunResult, error) {
	if err := ctx.Err(); err != nil {
		return RunResult{}, err
	}
	if req.Action != ActionFormat && req.Action != ActionMinify {
		return RunResult{}, ErrBadAction
	}
	raw := []byte(trimDoc(req.Input))
	if len(raw) == 0 {
		return RunResult{}, ErrEmpty
	}
	var (
		out string
		err error
	)
	if req.Action == ActionMinify {
		out, err = minifyJSON(raw)
	} else {
		out, err = formatJSON(raw, clampIndent(req.Indent))
	}
	if err != nil {
		return RunResult{}, err
	}
	return RunResult{Output: out}, nil
}

func formatJSON(raw []byte, spaces int) (string, error) {
	var buf bytes.Buffer
	if err := json.Indent(&buf, raw, "", strings.Repeat(" ", spaces)); err != nil {
		return "", invalidJSON(raw, err)
	}
	return buf.String(), nil
}

func minifyJSON(raw []byte) (string, error) {
	var buf bytes.Buffer
	if err := json.Compact(&buf, raw); err != nil {
		return "", invalidJSON(raw, err)
	}
	return buf.String(), nil
}

// invalidJSON 把 encoding/json 的偏移换成行和列。Offset 是已读字节数，出错点在它前一个字节。
func invalidJSON(raw []byte, err error) error {
	var syn *json.SyntaxError
	if errors.As(err, &syn) {
		off := int(syn.Offset)
		if off > 0 {
			off--
		}
		src := string(raw)
		return atOffset(src, preferFault(src, off))
	}
	return ErrInvalid
}
