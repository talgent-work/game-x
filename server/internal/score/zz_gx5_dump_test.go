package score

// GX-5 验证：对真实 --dump JSON 的 result 用 score 包重算（RoundWind=1 东风圈）。
import (
	"encoding/json"
	"os"
	"testing"

	"talgent-work/game-x/server/internal/engine"
)

func TestGX5ScoreFromDump(t *testing.T) {
	for _, path := range []string{
		"/workspace/gx5-final/seed-1067-selfdraw.json",
		"/workspace/gx5-final/seed-6-selfdraw.json",
	} {
		b, err := os.ReadFile(path)
		if err != nil {
			t.Skipf("读不到 %s：%v", path, err)
		}
		var d struct {
			Seed   int           `json:"seed"`
			Result engine.Result `json:"result"`
			Score  struct {
				Best struct {
					Fans []struct {
						Kind  string `json:"kind"`
						Count int    `json:"count"`
						Value int    `json:"value"`
					} `json:"fans"`
					Total int `json:"total"`
				} `json:"best"`
			} `json:"score"`
		}
		if err := json.Unmarshal(b, &d); err != nil {
			t.Fatalf("%s 解析失败：%v", path, err)
		}
		s := Score(&d.Result, Context{RoundWind: 1})
		t.Logf("种子 %d：重算 total=%d", d.Seed, s.Best.Total)
		for _, f := range s.Best.Fans {
			t.Logf("  重算 fan: %s ×%d = %d", f.Kind, f.Count, f.Value)
		}
		for _, f := range d.Score.Best.Fans {
			t.Logf("  dump  fan: %s ×%d = %d", f.Kind, f.Count, f.Value)
		}
		t.Logf("  dump total=%d", d.Score.Best.Total)
	}
}
