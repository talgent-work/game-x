// GX-4：最小 preload。只暴露运行时版本；对局数据一律经 WebSocket（渲染层直连本机服务端）。
const { contextBridge } = require("electron");

contextBridge.exposeInMainWorld("gxInfo", {
  electron: process.versions.electron,
  chrome: process.versions.chrome,
  node: process.versions.node,
});