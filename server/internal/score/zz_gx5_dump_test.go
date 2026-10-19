package score

// GX-5 验证：对真实 --dump JSON 的 result 用 score 包重算。
// 注：evidence/gx5/final/ 的 dump 生成于验证基线 talgent/GX-4@8f71ecd（GX-3 v1.0 口径）；
// R2/R3 修复（GX-3 v1.1）后，本测试断言「重算值符合 v1.1 口径」——dump 内嵌 score 字段
// 保留基线口径作历史对照，不与重算值比对。RoundWind=1（东风圈，与原型一致）。
import (
	"encoding/json"
	"os"
	"testing"

	"talgent-work/game-x/server/internal/engine"
)

func TestGX5ScoreFromDump(t *testing.T) {
	// v1.1 口径期望：种子 6/1067 自摸不计全求人；93 全求人不计单钓将。
	expect := map[int]int{6: 6, 1067: 4, 93: 8}
	for _, path := range []string{
		"../../../evidence/gx5/final/seed-6-selfdraw.json",
		"../../../evidence/gx5/final/seed-1067-selfdraw.json",
		"../../../evidence/gx5/final/seed-93-discard.json",
	} {
		b, err := os.ReadFile(path)
		if err != nil {
			t.Skipf("读不到 %s：%v（非仓库内运行）", path, err)
		}
		var d struct {
			Seed   int           `json:"seed"`
			Result engine.Result `json:"result"`
		}
		if err := json.Unmarshal(b, &d); err != nil {
			t.Fatalf("%s 解析失败：%v", path, err)
		}
		s := Score(&d.Result, Context{RoundWind: 1})
		if s.Exhaustive {
			t.Fatalf("种子 %d 意外流局", d.Seed)
		}
		got := map[string]int{}
		for _, f := range s.Best.Fans {
			got[string(f.Kind)] = f.Count
		}
		t.Logf("种子 %d：v1.1 重算 total=%d fans=%v", d.Seed, s.Best.Total, got)
		if want := expect[d.Seed]; s.Best.Total != want {
			t.Errorf("种子 %d：v1.1 口径总番 %d ≠ 期望 %d（fans=%v）", d.Seed, s.Best.Total, want, got)
		}
	}
	// R2：全求人不得与自摸共现；R3：全求人不计单钓将（v1.1）。
	{
		b, _ := os.ReadFile("../../../evidence/gx5/final/seed-6-selfdraw.json")
		var d struct {
			Result engine.Result `json:"result"`
		}
		if json.Unmarshal(b, &d) == nil {
			s := Score(&d.Result, Context{RoundWind: 1})
			for _, f := range s.Best.Fans {
				if f.Kind == MeldedHand {
					t.Errorf("种子 6 自摸不应计全求人（v1.1）")
				}
			}
		}
	}
}
