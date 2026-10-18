package tools

import (
	"context"
	"testing"
)

func TestPairHunkSkipsBlankAgainstText(t *testing.T) {
	rows := pairHunk([]string{"", "keep-this-line"}, []string{"<tag>brand-new</tag>", "keep-this-line"})
	if len(rows) != 3 || rows[2].Kind != "change" || rows[2].Left != "keep-this-line" {
		t.Fatalf("rows: %+v", rows)
	}
	kinds := rows[0].Kind + rows[1].Kind
	if kinds != "deleteinsert" && kinds != "insertdelete" {
		t.Fatalf("kinds: %+v", rows)
	}
}

func TestIgnoreSpaceKeepsOriginal(t *testing.T) {
	out, err := Run(context.Background(), RunRequest{
		ID:          "text-diff",
		Action:      ActionCompare,
		Input:       "a\n  b\nc",
		Right:       "a\nb\nc",
		IgnoreSpace: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Rows) != 3 || out.Rows[1].Kind != "equal" || out.Rows[1].Left != "  b" || out.Rows[1].Right != "b" {
		t.Fatalf("rows: %+v", out.Rows)
	}
}

func TestMarkMoves(t *testing.T) {
	rows := markMoves([]DiffRow{
		{Kind: "equal", Left: "keep", Right: "keep"},
		{Kind: "delete", Left: "moved-aaa"},
		{Kind: "delete", Left: "moved-bbb"},
		{Kind: "delete", Left: "moved-ccc"},
		{Kind: "insert", Right: "fresh-line"},
		{Kind: "insert", Right: "moved-aaa"},
		{Kind: "insert", Right: "moved-bbb"},
		{Kind: "insert", Right: "moved-ccc"},
	})
	for _, i := range []int{1, 2, 3, 5, 6, 7} {
		if !rows[i].Moved {
			t.Fatalf("row %d not moved: %+v", i, rows[i])
		}
	}
	if rows[4].Moved || rows[4].Kind != "insert" {
		t.Fatalf("fresh: %+v", rows[4])
	}
}
