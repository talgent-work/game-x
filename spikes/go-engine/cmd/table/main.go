// 与逻辑层分离的调用方（本机"服务端进程"的雏形）：
// 它不渲染任何界面，只驱动 engine 并打印各座位的可见视图——
// 将来的权威服务端（Go）与本地原型里的"本机服务端"都是同一形状的调用方。
package main

import (
	"encoding/json"
	"fmt"

	"talgent-work/game-x/spikes/go-engine/engine"
)

func main() {
	fmt.Println("== GX-1 最小实证 4a：Go 纯逻辑层 + 分离的调用方（无界面、无网络依赖） ==")
	tb := engine.NewTable(20261017)

	fmt.Println("\n-- 发牌后，从各座位视角读快照 --")
	for s := 0; s < engine.Seats; s++ {
		snap := tb.ViewFor(s)
		fmt.Printf("座位 %d 视角：手牌 %d 张 %v ｜ 牌墙余 %d ｜ 轮到 %d\n",
			snap.Seat, len(snap.Hand), snap.Hand, snap.WallCount, snap.Turn)
	}

	tile, err := tb.Draw()
	if err != nil {
		panic(err)
	}
	fmt.Printf("\n-- 座位 0 摸入 %v 并打出 --\n", tile)
	if err := tb.Discard(tile); err != nil {
		panic(err)
	}

	snap := tb.ViewFor(1)
	fmt.Printf("座位 1 视角（座位 0 打牌后）：手牌 %d 张；公开牌河[座位 0] %v；牌墙余 %d；轮到 %d\n",
		len(snap.Hand), snap.Discards[0], snap.WallCount, snap.Turn)
	fmt.Println("（注意：座位 1 的快照里没有任何别家手牌——隐藏信息留在逻辑层内部）")

	payload, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		panic(err)
	}
	fmt.Printf("\n-- 座位 1 快照的 JSON 载荷（将来经 WebSocket 发给该客户端的内容形状）--\n%s\n", payload)
}
