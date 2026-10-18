package tools

import (
	"context"
	"strings"
)

func init() {
	Register(Spec{
		ID:       "xml-format",
		Category: CategoryFormat,
		TitleKey: "tools.xml.title",
		DescKey:  "tools.xml.desc",
		Icon:     "file-code",
		Order:    20,
		Kind:     KindFormat,
	}, runXML)
}

func runXML(ctx context.Context, req RunRequest) (RunResult, error) {
	if err := ctx.Err(); err != nil {
		return RunResult{}, err
	}
	out, err := renderXML(trimDoc(req.Input), req.Action, clampIndent(req.Indent))
	if err != nil {
		return RunResult{}, err
	}
	return RunResult{Output: out}, nil
}

type xmlTok struct {
	kind string
	raw  string
	at   int
}

// renderXML 只调整标签之间的空白，不改写命名空间、前缀和属性顺序。
func renderXML(input, action string, spaces int) (string, error) {
	if input == "" {
		return "", ErrEmpty
	}
	tokens, err := scanXML(input)
	if err != nil {
		return "", err
	}
	if err := checkXMLBalance(input, tokens); err != nil {
		return "", err
	}
	if action == ActionMinify {
		return minifyXML(tokens), nil
	}
	return layoutXML(tokens, strings.Repeat(" ", spaces)), nil
}

func scanXML(src string) ([]xmlTok, error) {
	out := make([]xmlTok, 0, 16)
	i := 0
	for i < len(src) {
		if src[i] != '<' {
			j := strings.IndexByte(src[i:], '<')
			if j < 0 {
				out = append(out, xmlTok{kind: "text", raw: src[i:]})
				break
			}
			out = append(out, xmlTok{kind: "text", raw: src[i : i+j]})
			i += j
			continue
		}
		end, kind, err := consumeMarkup(src, i)
		if err != nil {
			return nil, atOffset(src, i)
		}
		out = append(out, xmlTok{kind: kind, raw: src[i:end], at: i})
		i = end
	}
	return out, nil
}

func consumeMarkup(src string, i int) (int, string, error) {
	rest := src[i:]
	switch {
	case strings.HasPrefix(rest, "<!--"):
		j := strings.Index(rest[4:], "-->")
		if j < 0 {
			return 0, "", ErrInvalid
		}
		return i + 4 + j + 3, "comment", nil
	case strings.HasPrefix(rest, "<![CDATA["):
		j := strings.Index(rest[9:], "]]>")
		if j < 0 {
			return 0, "", ErrInvalid
		}
		return i + 9 + j + 3, "cdata", nil
	case strings.HasPrefix(rest, "<?"):
		j := strings.Index(rest[2:], "?>")
		if j < 0 {
			return 0, "", ErrInvalid
		}
		return i + 2 + j + 2, "pi", nil
	}

	quote := byte(0)
	for j := i + 1; j < len(src); j++ {
		c := src[j]
		if quote != 0 {
			if c == quote {
				quote = 0
			}
			continue
		}
		if c == '"' || c == '\'' {
			quote = c
			continue
		}
		if c == '>' {
			return j + 1, markupKind(src[i : j+1]), nil
		}
	}
	return 0, "", ErrInvalid
}

type xmlOpen struct {
	name string
	at   int
}

// checkXMLBalance 要求开始标签和结束标签按名称配对。失败点落在不匹配的标签上。
func checkXMLBalance(src string, tokens []xmlTok) error {
	stack := make([]xmlOpen, 0, 8)
	for _, tok := range tokens {
		switch tok.kind {
		case "start":
			stack = append(stack, xmlOpen{name: xmlTagName(tok.raw), at: tok.at})
		case "end":
			name := xmlTagName(tok.raw)
			if len(stack) == 0 || stack[len(stack)-1].name != name {
				return atOffset(src, tok.at)
			}
			stack = stack[:len(stack)-1]
		}
	}
	if len(stack) == 0 {
		return nil
	}
	return atOffset(src, stack[len(stack)-1].at)
}

func xmlTagName(raw string) string {
	s := strings.TrimSpace(raw)
	s = strings.TrimPrefix(s, "</")
	s = strings.TrimPrefix(s, "<")
	s = strings.TrimSuffix(s, "/>")
	s = strings.TrimSuffix(s, ">")
	s = strings.TrimSpace(s)
	if i := strings.IndexAny(s, " \t\r\n"); i >= 0 {
		s = s[:i]
	}
	return s
}

func markupKind(raw string) string {
	switch {
	case strings.HasPrefix(raw, "</"):
		return "end"
	case strings.HasSuffix(strings.TrimSpace(raw), "/>"):
		return "empty"
	default:
		return "start"
	}
}

func layoutXML(tokens []xmlTok, indent string) string {
	var b strings.Builder
	depth := 0
	started := false
	for _, tok := range tokens {
		if tok.kind == "text" {
			text := strings.TrimSpace(tok.raw)
			if text == "" {
				continue
			}
			writeLine(&b, &started, indent, depth, text)
			continue
		}
		if tok.kind == "end" && depth > 0 {
			depth--
		}
		writeLine(&b, &started, indent, depth, tok.raw)
		if tok.kind == "start" {
			depth++
		}
	}
	return b.String()
}

func writeLine(b *strings.Builder, started *bool, indent string, depth int, text string) {
	if *started {
		b.WriteByte('\n')
	}
	*started = true
	for i := 0; i < depth; i++ {
		b.WriteString(indent)
	}
	b.WriteString(text)
}

func minifyXML(tokens []xmlTok) string {
	var b strings.Builder
	for _, tok := range tokens {
		if tok.kind == "text" {
			text := strings.TrimSpace(tok.raw)
			if text == "" {
				continue
			}
			b.WriteString(text)
			continue
		}
		b.WriteString(strings.TrimSpace(tok.raw))
	}
	return b.String()
}

func trimDoc(input string) string {
	return strings.TrimSpace(strings.TrimPrefix(input, "\uFEFF"))
}
