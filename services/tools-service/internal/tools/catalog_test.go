package tools

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestListGroupsFormatters(t *testing.T) {
	got := List()
	if len(got.Categories) == 0 || got.Categories[0].ID != CategoryFormat {
		t.Fatalf("categories: %+v", got.Categories)
	}
	ids := make([]string, 0, 3)
	for _, tool := range got.Categories[0].Tools {
		ids = append(ids, tool.ID)
	}
	want := []string{"json-format", "xml-format", "soap-format"}
	if strings.Join(ids, ",") != strings.Join(want, ",") {
		t.Fatalf("tools: %v", ids)
	}
}

func TestRunJSON(t *testing.T) {
	out, err := Run(context.Background(), RunRequest{
		ID:     "json-format",
		Action: ActionFormat,
		Input:  `{"a":1}`,
		Indent: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.Output != "{\n  \"a\": 1\n}" {
		t.Fatalf("output: %q", out.Output)
	}

	mini, err := Run(context.Background(), RunRequest{
		ID:     "json-format",
		Action: ActionMinify,
		Input:  "{\n  \"a\": 1\n}",
	})
	if err != nil {
		t.Fatal(err)
	}
	if mini.Output != `{"a":1}` {
		t.Fatalf("minify: %q", mini.Output)
	}
}

func TestRunXML(t *testing.T) {
	out, err := Run(context.Background(), RunRequest{
		ID:     "xml-format",
		Action: ActionFormat,
		Input:  `<root><a>hi</a></root>`,
	})
	if err != nil {
		t.Fatal(err)
	}
	want := "<root>\n  <a>\n    hi\n  </a>\n</root>"
	if out.Output != want {
		t.Fatalf("output:\n%s", out.Output)
	}
}

func TestRunSOAP(t *testing.T) {
	in := `<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/"><soap:Body><Get><Id>1</Id></Get></soap:Body></soap:Envelope>`
	out, err := Run(context.Background(), RunRequest{ID: "soap-format", Input: in, Indent: 2})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.Output, "soap:Envelope") || !strings.Contains(out.Output, "\n") {
		t.Fatalf("output: %s", out.Output)
	}

	_, err = Run(context.Background(), RunRequest{ID: "soap-format", Input: `<root/>`})
	if !errors.Is(err, ErrNotSOAP) {
		t.Fatalf("err: %v", err)
	}
}

func TestRunDiff(t *testing.T) {
	out, err := Run(context.Background(), RunRequest{
		ID:     "text-diff",
		Action: ActionCompare,
		Input:  "a\nb\nc",
		Right:  "a\nx\nc",
	})
	if err != nil {
		t.Fatal(err)
	}
	got := make([]string, 0, len(out.Rows))
	for _, row := range out.Rows {
		got = append(got, row.Kind+":"+row.Left+"/"+row.Right)
	}
	want := []string{"equal:a/a", "change:b/x", "equal:c/c"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("rows: %v", got)
	}

	ins, err := Run(context.Background(), RunRequest{
		ID:     "text-diff",
		Action: ActionCompare,
		Input:  "a",
		Right:  "a\nb",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(ins.Rows) != 2 || ins.Rows[1].Kind != "insert" || ins.Rows[1].Right != "b" {
		t.Fatalf("insert: %+v", ins.Rows)
	}

	if _, err := Run(context.Background(), RunRequest{ID: "text-diff", Action: ActionCompare}); !errors.Is(err, ErrEmpty) {
		t.Fatalf("empty: %v", err)
	}
}

func TestRunGuards(t *testing.T) {
	_, err := Run(context.Background(), RunRequest{ID: "missing", Input: "{}"})
	if !errors.Is(err, ErrUnknown) {
		t.Fatalf("unknown: %v", err)
	}
	_, err = Run(context.Background(), RunRequest{ID: "json-format", Input: strings.Repeat("a", maxInputBytes+1)})
	if !errors.Is(err, ErrTooLarge) {
		t.Fatalf("size: %v", err)
	}
	_, err = Run(context.Background(), RunRequest{ID: "json-format", Action: "drop", Input: "{}"})
	if !errors.Is(err, ErrBadAction) {
		t.Fatalf("action: %v", err)
	}
	if _, err := Run(context.Background(), RunRequest{ID: "json-format", Input: "  "}); !errors.Is(err, ErrEmpty) {
		t.Fatalf("empty: %v", err)
	}
}

func TestInvalidPointsAtLine(t *testing.T) {
	_, err := Run(context.Background(), RunRequest{
		ID:     "json-format",
		Action: ActionFormat,
		Input:  "{\n  \"a\": ,\n}",
	})
	var loc *ParseError
	if !errors.As(err, &loc) || !errors.Is(err, ErrInvalid) || loc.Line != 2 || loc.Column != 8 {
		t.Fatalf("json: %v", err)
	}

	_, err = Run(context.Background(), RunRequest{
		ID:     "json-format",
		Action: ActionFormat,
		Input:  "{\n  \"sku\"\n  \"name\": 1\n}",
	})
	if !errors.As(err, &loc) || loc.Line != 2 || loc.Column != 7 {
		t.Fatalf("next line: %v", err)
	}

	_, err = Run(context.Background(), RunRequest{
		ID:     "json-format",
		Action: ActionFormat,
		Input:  "{\n  \"sku\": 1,\n}",
	})
	if !errors.As(err, &loc) || loc.Line != 2 || loc.Column != 11 {
		t.Fatalf("trailing comma: %v", err)
	}

	_, err = Run(context.Background(), RunRequest{
		ID:     "xml-format",
		Action: ActionFormat,
		Input:  "<root>\n<child",
	})
	if !errors.As(err, &loc) || loc.Line != 2 {
		t.Fatalf("xml: %v", err)
	}

	_, err = Run(context.Background(), RunRequest{
		ID:     "xml-format",
		Action: ActionFormat,
		Input:  "<root><child></root>",
	})
	if !errors.As(err, &loc) || loc.Line != 1 || loc.Column != 14 {
		t.Fatalf("mismatch: %v", err)
	}

	_, err = Run(context.Background(), RunRequest{
		ID:     "soap-format",
		Action: ActionFormat,
		Input:  `<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/"><bad>`,
	})
	if !errors.As(err, &loc) || loc.Line != 1 || loc.Column <= 1 {
		t.Fatalf("soap column: %v", err)
	}
}
