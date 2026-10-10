package pricing

import (
	"testing"
)

// 联网拉取真实社区价格表，验证解析与缓存（失败时跳过而非报错，避免无网环境卡住 CI）
func TestUpdate(t *testing.T) {
	tbl := Load(t.TempDir())

	count, source, err := tbl.Update()
	if err != nil {
		t.Skipf("网络不可用，跳过: %v", err)
	}
	t.Logf("从 %s 获取到 %d 个模型价格", source, count)
	if count < 100 {
		t.Errorf("社区价格表模型数异常偏少: %d", count)
	}

	// 常见模型必须能查到
	for _, model := range []string{"claude-sonnet-4-5", "gpt-4o"} {
		p, ok := tbl.Lookup(model)
		if !ok {
			t.Errorf("查不到 %s 的价格", model)
			continue
		}
		t.Logf("  %s: 输入 $%.2f 输出 $%.2f / 1M", model, p.Input, p.Output)
	}

	// 带日期后缀的模型名也应命中
	if _, ok := tbl.Lookup("claude-sonnet-4-5-20250929"); !ok {
		t.Error("去日期后缀匹配失效")
	}

	// 裸名应命中官方渠道前缀条目：kimi-k3 → moonshot/kimi-k3
	p, ok := tbl.Lookup("kimi-k3")
	if !ok {
		t.Fatal("kimi-k3 裸名未命中任何价格")
	}
	t.Logf("  kimi-k3: 输入 $%.2f 输出 $%.2f / 1M", p.Input, p.Output)
	if p.Input == 0.79 {
		t.Error("命中了 openrouter 中转价，应优先 moonshot 官方价")
	}
}

func TestLookupPriority(t *testing.T) {
	tbl := Load(t.TempDir())

	// 内置可查
	if _, ok := tbl.Lookup("claude-opus-5-5"); !ok {
		t.Fatal("内置价格缺失")
	}

	// 自定义覆盖内置
	if err := tbl.Set("claude-opus-5-5", Price{Input: 99, Output: 99}); err != nil {
		t.Fatal(err)
	}
	p, _ := tbl.Lookup("claude-opus-5-5")
	if p.Input != 99 {
		t.Errorf("自定义价格未生效，input=%v", p.Input)
	}

	// 删除后恢复内置
	if err := tbl.Delete("claude-opus-5-5"); err != nil {
		t.Fatal(err)
	}
	p, _ = tbl.Lookup("claude-opus-5-5")
	if p.Input == 99 {
		t.Error("删除自定义后应恢复内置价格")
	}
}
