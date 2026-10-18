// preload 以 CommonJS 手写（electron/preload.cjs），复制进 dist-electron/ 供主进程引用。
import { copyFileSync, mkdirSync } from "node:fs";
import path from "node:path";

const clientDir = path.resolve(import.meta.dirname, "..");
mkdirSync(path.join(clientDir, "dist-electron"), { recursive: true });
copyFileSync(path.join(clientDir, "electron", "preload.cjs"), path.join(clientDir, "dist-electron", "preload.cjs"));
console.log("preload.cjs → dist-electron/");