// gameserver：GX-4 本地对局原型的本机权威服务端（一进程一局）。
//
// 用法示例：
//
//	gameserver --addr 127.0.0.1:8787 --seed 20261018 --dealer 0 --dump /tmp/gx4-game.json
//
// 一局结束（和牌或流局）后：写完整对局记录（--dump，含隐藏信息与终局全揭示，供复现与规则核对）、
// 关闭全部连接、退出。种子可由 --seed 显式给定以精确复现（0＝按当前时间随机并打印）。
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"talgent-work/game-x/server/internal/engine"
	"talgent-work/game-x/server/internal/table"
	"talgent-work/game-x/server/internal/transport"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	addr := flag.String("addr", "127.0.0.1:8787", "监听地址")
	path := flag.String("path", "/ws", "WebSocket 端点路径")
	seed := flag.Int64("seed", 0, "洗牌种子（0＝按当前时间随机；复现请显式传值）")
	dealer := flag.Int("dealer", 0, "庄家座位 0..3")
	dump := flag.String("dump", "", "一局结束时写出完整对局记录（JSON）的文件路径；空＝不写")
	flag.Parse()

	if *seed == 0 {
		*seed = time.Now().UnixNano()
	}
	log.Printf("GX-4 本地对局服务端启动：addr=%s path=%s seed=%d dealer=%d", *addr, *path, *seed, *dealer)
	if *dump != "" {
		log.Printf("一局结束后将写出完整对局记录：%s", *dump)
	}

	tb := table.New(table.Config{Seed: *seed, Dealer: *dealer})

	mux := http.NewServeMux()
	mux.Handle(*path, transport.Handler(tb))

	srv := &http.Server{Addr: *addr, Handler: mux}

	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.ListenAndServe() }()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-serveErr:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("监听失败：%v", err)
		}
		return
	case <-sig:
		log.Printf("收到停止信号，关闭")
		tb.Close()
	case <-tb.Done():
		log.Printf("对局结束，收尾")
	}

	// 终局收尾（确定性顺序，全部在进程退出前完成）：
	// 先写完整对局记录，再报结果，最后关服务、停串行环。
	if *dump != "" {
		if err := writeDump(*dump, tb); err != nil {
			log.Printf("完整对局记录写出失败：%v", err)
		} else {
			log.Printf("完整对局记录已写出：%s", *dump)
		}
	}
	if r := tb.Result(); r != nil {
		if r.Exhaustive {
			log.Printf("对局结束：流局")
		} else {
			log.Printf("对局结束：座位 %d 和牌（%s，放铳座位 %d）", r.Winner, winDesc(r.Source), r.Feeder)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	_ = srv.Shutdown(ctx)
	cancel()
	tb.Close()
}

func writeDump(path string, tb *table.Table) error {
	b, err := json.MarshalIndent(tb.Dump(), "", "  ")
	if err != nil {
		return err
	}
	// 原子写：先写同目录临时文件，再 rename 落盘。
	// 消费方不会在「文件已存在但内容未写完」的窗口里读到半截 JSON（GX-5 实机验证 E1）。
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func winDesc(s engine.WinSource) string {
	switch s {
	case engine.WinSelfDrawn:
		return "自摸"
	case engine.WinByDiscard:
		return "点和"
	case engine.WinByRobbedKong:
		return "抢杠和"
	}
	return "未知"
}
