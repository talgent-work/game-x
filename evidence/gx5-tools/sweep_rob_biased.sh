#!/bin/bash
# GX-5 抢杠和搜索：偏置驱动（优先碰/杠/补杠/和牌）
for s in $(seq 100001 104000); do
  node /workspace/gx5-tools/run_game_biased.mjs $s 8797 /workspace/gx5-evidence-rob2 >/dev/null 2>&1
done
