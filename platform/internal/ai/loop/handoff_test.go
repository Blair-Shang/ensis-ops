package loop

import "testing"

func TestOwesAction(t *testing.T) {
	cases := []struct {
		name    string
		content string
		finish  string
		want    bool
	}{
		{
			name:    "model ended on its own",
			content: "执行 Tomcat 重启。先关闭 + 启动，下一轮等待启动完成 + 验证。",
			finish:  "stop",
			want:    false,
		},
		{
			name:    "announced work but chose to stop",
			content: "<think>Alright, executing now.</think>\n\n跑两表的扫描：",
			finish:  "end_turn",
			want:    false,
		},
		{
			name:    "finished report",
			content: "修复完成，全部验证通过。需要的话可以再看日志。",
			want:    false,
		},
		{
			name:    "asks the user to choose",
			content: "要修就告诉我修哪种。",
			want:    false,
		},
		{
			name:    "empty",
			content: "  ",
			want:    false,
		},
		{
			name:    "thinking only",
			content: "<think>先规划查询，还没调用。</think>",
			finish:  "stop",
			want:    true,
		},
		{
			name:    "output cut off",
			content: "窄列里 SOAP 最多的是 valueStr",
			finish:  "length",
			want:    true,
		},
		{
			name:    "cut off even if the fragment looks finished",
			content: "用方案 A 还是方案 B？",
			finish:  "max_tokens",
			want:    true,
		},
		{
			name:    "context window filled",
			content: "扫描进行到一半",
			finish:  "model_context_window_exceeded",
			want:    true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := owesAction(tc.content, tc.finish); got != tc.want {
				t.Fatalf("owesAction(%q, %q) = %v, want %v", tc.content, tc.finish, got, tc.want)
			}
		})
	}
}
