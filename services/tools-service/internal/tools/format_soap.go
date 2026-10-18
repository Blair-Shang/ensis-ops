package tools

import (
	"context"
	"encoding/xml"
	"io"
	"strings"
)

func init() {
	Register(Spec{
		ID:       "soap-format",
		Category: CategoryFormat,
		TitleKey: "tools.soap.title",
		DescKey:  "tools.soap.desc",
		Icon:     "layers",
		Order:    30,
		Kind:     KindFormat,
	}, runSOAP)
}

func runSOAP(ctx context.Context, req RunRequest) (RunResult, error) {
	if err := ctx.Err(); err != nil {
		return RunResult{}, err
	}
	input := trimDoc(req.Input)
	if input == "" {
		return RunResult{}, ErrEmpty
	}
	if err := requireSOAP(input); err != nil {
		return RunResult{}, err
	}
	out, err := renderXML(input, req.Action, clampIndent(req.Indent))
	if err != nil {
		return RunResult{}, err
	}
	return RunResult{Output: out}, nil
}

// requireSOAP 确认文档可解析，且含 SOAP 1.1 或 1.2 的 Envelope。
func requireSOAP(input string) error {
	dec := xml.NewDecoder(strings.NewReader(input))
	found := false
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			off := int(dec.InputOffset())
			if off > 0 {
				off--
			}
			return atOffset(input, preferFault(input, off))
		}
		start, ok := tok.(xml.StartElement)
		if !ok || start.Name.Local != "Envelope" {
			continue
		}
		if isSOAPNamespace(start.Name.Space) {
			found = true
		}
	}
	if !found {
		return ErrNotSOAP
	}
	return nil
}

func isSOAPNamespace(space string) bool {
	switch strings.TrimSpace(space) {
	case "http://schemas.xmlsoap.org/soap/envelope/",
		"http://www.w3.org/2003/05/soap-envelope":
		return true
	default:
		return false
	}
}
