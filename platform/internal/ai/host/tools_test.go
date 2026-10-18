package host

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

type toolsStubRuntime struct {
	lastMethod string
	lastParams map[string]any
}

func (s *toolsStubRuntime) Call(_ context.Context, method string, params map[string]any) (json.RawMessage, error) {
	s.lastMethod = method
	s.lastParams = params
	if method != "tools.run" {
		return json.RawMessage(`{}`), nil
	}
	id, _ := params["id"].(string)
	if id == "text-diff" {
		return json.RawMessage(`{"rows":[{"kind":"equal","left":"a","right":"a"},{"kind":"change","left":"b","right":"x"}]}`), nil
	}
	return json.RawMessage(`{"output":"{\n  \"a\": 1\n}"}`), nil
}

func (s *toolsStubRuntime) KindOf(_ context.Context, _ string) (string, error) {
	return "", nil
}

func TestHostToolSpecsTools(t *testing.T) {
	specs := HostToolSpecs("tools")
	if len(specs) != 4 {
		t.Fatalf("tools specs=%d", len(specs))
	}
	for _, spec := range specs {
		if !IsToolsTool(spec.Name) {
			t.Fatalf("unexpected %s", spec.Name)
		}
		if spec.Risk != "read" || SpecServerID(spec) != ServerIDTools {
			t.Fatalf("spec=%+v", spec)
		}
	}
	sql := HostToolSpecs("mysql")
	for _, spec := range sql {
		if IsToolsTool(spec.Name) {
			t.Fatalf("sql tab exposed %s", spec.Name)
		}
	}
	open := HostToolSpecs("")
	var saw bool
	for _, spec := range open {
		if spec.Name == ToolFormatJSON {
			saw = true
		}
	}
	if !saw {
		t.Fatal("empty workspace missing tools_format_json")
	}
}

func TestCallToolsFormatJSON(t *testing.T) {
	rt := &toolsStubRuntime{}
	text, err := Call(context.Background(), rt, ToolFormatJSON, map[string]any{
		"input":     `{"a":1}`,
		"action":    "format",
		"indent":    float64(4),
		"profileId": "should-not-forward",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(text, `\"a\"`) || strings.Contains(text, `"truncated": true`) {
		t.Fatalf("result: %s", text)
	}
	if rt.lastMethod != "tools.run" {
		t.Fatalf("method=%s", rt.lastMethod)
	}
	if rt.lastParams["id"] != "json-format" || rt.lastParams["action"] != "format" || rt.lastParams["indent"] != 4 {
		t.Fatalf("params=%v", rt.lastParams)
	}
	if _, ok := rt.lastParams["profileId"]; ok {
		t.Fatal("profile forwarded")
	}
}

func TestCallToolsDiff(t *testing.T) {
	rt := &toolsStubRuntime{}
	text, err := CallTools(context.Background(), rt, ToolDiffText, map[string]any{
		"input": "a\nb",
		"right": "a\nx",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(text, `"changed": 1`) || !strings.Contains(text, `"added": 0`) {
		t.Fatalf("result: %s", text)
	}
	if rt.lastParams["id"] != "text-diff" || rt.lastParams["action"] != "compare" || rt.lastParams["right"] != "a\nx" {
		t.Fatalf("params=%v", rt.lastParams)
	}
}

func TestCallToolsRequiresInput(t *testing.T) {
	_, err := Call(context.Background(), &toolsStubRuntime{}, ToolFormatXML, map[string]any{})
	if err == nil {
		t.Fatal("expected input required")
	}
}
