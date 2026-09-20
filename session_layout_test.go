package main

import (
	"path/filepath"
	"testing"
)

func testPane(id string, tabs ...string) SessionSplitNode {
	active := ""
	if len(tabs) > 0 {
		active = tabs[len(tabs)-1]
	}
	return SessionSplitNode{Type: "pane", ID: id, Tabs: tabs, Active: active}
}

// 失效 path 被剔除、同 pane 内重复 path 去重、激活项回退到最后一个有效 tab。
func TestSanitizeSessionLayoutFiltersInvalidTabs(t *testing.T) {
	valid := map[string]struct{}{
		`C:\a\1.txt`: {},
		`C:\a\2.txt`: {},
	}
	layout := &SessionPreviewLayout{
		FocusedPaneID: "pane-2",
		Root: &SessionSplitNode{
			Type:      "split",
			ID:        "split-1",
			Direction: "row",
			Ratio:     0.5,
			Children: []SessionSplitNode{
				{
					Type:   "pane",
					ID:     "pane-1",
					Tabs:   []string{`C:\a\1.txt`, `C:\a\gone.txt`},
					Active: `C:\a\gone.txt`,
				},
				{
					Type:   "pane",
					ID:     "pane-2",
					Tabs:   []string{`C:\a\2.txt`, `C:\a\2.txt`},
					Active: `C:\a\2.txt`,
				},
			},
		},
	}

	clean, changed := sanitizeSessionLayout(layout, valid)
	if !changed {
		t.Fatal("剔除失效 tab 时应报告布局已修改")
	}
	if clean == nil || clean.Root == nil || clean.Root.Type != "split" {
		t.Fatalf("应保留 split 根节点，实际 %+v", clean)
	}

	first := clean.Root.Children[0]
	if len(first.Tabs) != 1 || first.Tabs[0] != `C:\a\1.txt` {
		t.Fatalf("应剔除失效 tab，实际 %+v", first.Tabs)
	}
	if first.Active != `C:\a\1.txt` {
		t.Fatalf("激活项应回退到最后一个有效 tab，实际 %q", first.Active)
	}

	second := clean.Root.Children[1]
	if len(second.Tabs) != 1 || second.Tabs[0] != `C:\a\2.txt` {
		t.Fatalf("应去除同 pane 内的重复 tab，实际 %+v", second.Tabs)
	}

	if clean.FocusedPaneID != "pane-2" {
		t.Fatalf("有效焦点 pane 应保留，实际 %q", clean.FocusedPaneID)
	}
}

// tab 全部失效的 pane 被回收，父 split 由兄弟节点替换。
func TestSanitizeSessionLayoutCollapsesEmptyPane(t *testing.T) {
	valid := map[string]struct{}{`C:\a\1.txt`: {}}
	layout := &SessionPreviewLayout{
		FocusedPaneID: "pane-2",
		Root: &SessionSplitNode{
			Type:      "split",
			ID:        "split-1",
			Direction: "row",
			Ratio:     0.5,
			Children: []SessionSplitNode{
				testPane("pane-1", `C:\a\1.txt`),
				testPane("pane-2", `C:\a\gone.txt`),
			},
		},
	}

	clean, changed := sanitizeSessionLayout(layout, valid)
	if !changed {
		t.Fatal("回收空 pane 时应报告布局已修改")
	}
	if clean.Root == nil || clean.Root.Type != "pane" || clean.Root.ID != "pane-1" {
		t.Fatalf("应退化为剩下的唯一 pane，实际 %+v", clean.Root)
	}
	if clean.FocusedPaneID != "pane-1" {
		t.Fatalf("失效焦点应回退到第一个 pane，实际 %q", clean.FocusedPaneID)
	}
}

// 非法方向 / 越界比例被修正，缺失的焦点 pane 回退到第一个 pane。
func TestSanitizeSessionLayoutClampsRatioAndFocused(t *testing.T) {
	valid := map[string]struct{}{
		`C:\a\1.txt`: {},
		`C:\a\2.txt`: {},
	}
	layout := &SessionPreviewLayout{
		FocusedPaneID: "pane-missing",
		Root: &SessionSplitNode{
			Type:      "split",
			ID:        "split-1",
			Direction: "diagonal",
			Ratio:     0.95,
			Children: []SessionSplitNode{
				testPane("pane-1", `C:\a\1.txt`),
				testPane("pane-2", `C:\a\2.txt`),
			},
		},
	}

	clean, changed := sanitizeSessionLayout(layout, valid)
	if !changed {
		t.Fatal("修正方向 / 比例时应报告布局已修改")
	}
	if clean.Root.Direction != "row" {
		t.Fatalf("非法方向应回退为 row，实际 %q", clean.Root.Direction)
	}
	if clean.Root.Ratio != 0.5 {
		t.Fatalf("越界比例应回退为 0.5，实际 %v", clean.Root.Ratio)
	}
	if clean.FocusedPaneID != "pane-1" {
		t.Fatalf("缺失焦点应回退到第一个 pane，实际 %q", clean.FocusedPaneID)
	}
}

// 结构损坏（缺 root / 子节点不足 / 未知类型）时整体丢弃，前端退化为单 pane。
func TestNormalizeSessionLayoutPathsRejectsBrokenStructure(t *testing.T) {
	if got := normalizeSessionLayoutPaths(nil); got != nil {
		t.Fatalf("nil 布局应返回 nil，实际 %+v", got)
	}
	if got := normalizeSessionLayoutPaths(&SessionPreviewLayout{}); got != nil {
		t.Fatalf("缺少 root 应返回 nil，实际 %+v", got)
	}
	if got := normalizeSessionLayoutPaths(&SessionPreviewLayout{
		Root: &SessionSplitNode{Type: "split", ID: "split-1"},
	}); got != nil {
		t.Fatalf("split 子节点不足应返回 nil，实际 %+v", got)
	}
	if got := normalizeSessionLayoutPaths(&SessionPreviewLayout{
		Root: &SessionSplitNode{Type: "unknown"},
	}); got != nil {
		t.Fatalf("未知节点类型应返回 nil，实际 %+v", got)
	}
}

// 布局中的 tab 路径与 OpenTabs 口径一致，统一绝对化。
func TestNormalizeSessionLayoutPathsAbsolutizesTabs(t *testing.T) {
	abs, err := filepath.Abs("a/1.txt")
	if err != nil {
		t.Fatalf("filepath.Abs 失败：%v", err)
	}

	clean := normalizeSessionLayoutPaths(&SessionPreviewLayout{
		FocusedPaneID: " pane-1 ",
		Root: &SessionSplitNode{
			Type:   "pane",
			ID:     "pane-1",
			Tabs:   []string{"a/1.txt"},
			Active: "a/1.txt",
		},
	})
	if clean == nil || clean.Root == nil {
		t.Fatal("合法布局不应被丢弃")
	}
	if clean.Root.Tabs[0] != abs {
		t.Fatalf("tab 路径应绝对化，期望 %q，实际 %q", abs, clean.Root.Tabs[0])
	}
	if clean.Root.Active != abs {
		t.Fatalf("激活路径应绝对化，期望 %q，实际 %q", abs, clean.Root.Active)
	}
	if clean.FocusedPaneID != "pane-1" {
		t.Fatalf("焦点 pane id 应去除首尾空白，实际 %q", clean.FocusedPaneID)
	}
}

// 布局参与会话相等性判断：布局不同视为会话已变化（用于触发持久化）。
func TestWindowSessionsEqualComparesLayout(t *testing.T) {
	base := WindowSession{
		Mode:       "file",
		OpenTabs:   []SessionTab{{Path: `C:\a\1.txt`}},
		ActivePath: `C:\a\1.txt`,
		Layout: &SessionPreviewLayout{
			FocusedPaneID: "pane-1",
			Root:          &SessionSplitNode{Type: "pane", ID: "pane-1", Tabs: []string{`C:\a\1.txt`}},
		},
	}
	identical := base
	identical.Layout = &SessionPreviewLayout{
		FocusedPaneID: "pane-1",
		Root:          &SessionSplitNode{Type: "pane", ID: "pane-1", Tabs: []string{`C:\a\1.txt`}},
	}
	if !windowSessionsEqual(base, identical) {
		t.Fatal("布局内容相同的会话应判定为相等")
	}

	split := base
	split.Layout = &SessionPreviewLayout{
		FocusedPaneID: "pane-1",
		Root: &SessionSplitNode{
			Type:      "split",
			ID:        "split-1",
			Direction: "row",
			Ratio:     0.5,
			Children: []SessionSplitNode{
				testPane("pane-1", `C:\a\1.txt`),
				testPane("pane-2", `C:\a\2.txt`),
			},
		},
	}
	if windowSessionsEqual(base, split) {
		t.Fatal("布局不同的会话不应判定为相等")
	}
}
