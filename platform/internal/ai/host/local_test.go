package host

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

type localStubRuntime struct {
	lastMethod string
}

func (s *localStubRuntime) Call(_ context.Context, method string, _ map[string]any) (json.RawMessage, error) {
	s.lastMethod = method
	return json.RawMessage(`{"found":false,"name":"java"}`), nil
}

func (s *localStubRuntime) KindOf(context.Context, string) (string, error) {
	return "", nil
}

func TestHostToolSpecsLocalIsExplicit(t *testing.T) {
	local := HostToolSpecs("local")
	if len(local) != 4 {
		t.Fatalf("local specs=%d", len(local))
	}
	var sawWrite bool
	for _, spec := range local {
		if !IsLocalTool(spec.Name) {
			t.Fatalf("non-local tool %s", spec.Name)
		}
		if spec.Name == "local_write_file" {
			sawWrite = spec.Risk == "write"
		}
		if spec.Risk == "dangerous" {
			t.Fatalf("local tool %s is dangerous", spec.Name)
		}
	}
	if !sawWrite {
		t.Fatal("local_write_file must require confirm")
	}
	for _, spec := range HostToolSpecs("") {
		if IsLocalTool(spec.Name) {
			t.Fatalf("empty workspace leaked %s", spec.Name)
		}
	}
	if len(HostToolSpecs("")) != 26 {
		t.Fatalf("empty specs=%d", len(HostToolSpecs("")))
	}
	for _, spec := range HostToolSpecs("mysql") {
		if IsLocalTool(spec.Name) {
			t.Fatalf("mysql leaked %s", spec.Name)
		}
	}
}

func TestCallLocalWhich(t *testing.T) {
	rt := &localStubRuntime{}
	text, err := Call(context.Background(), rt, "local_which", map[string]any{"name": "java"})
	if err != nil {
		t.Fatal(err)
	}
	if rt.lastMethod != "tools.localWhich" {
		t.Fatalf("method=%s", rt.lastMethod)
	}
	if !strings.Contains(text, `"found":false`) {
		t.Fatalf("result: %s", text)
	}
}
