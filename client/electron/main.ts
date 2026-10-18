// GX-4：Electron 主进程——窗口 + 拉起本机 Go 权威服务端（localhost）。
// 不含任何对局逻辑；对局数据一律经 WebSocket 直达渲染层。
import { app, BrowserWindow } from "electron";
import { spawn, type ChildProcess } from "node:child_process";
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const PORT = Number(process.env.GX_PORT ?? "8787");

let serverProc: ChildProcess | null = null;

function serverExecutable(): string {
  if (app.isPackaged) {
    const exe = process.platform === "win32" ? "gameserver.exe" : "gameserver";
    return path.join(process.resourcesPath, "server", exe);
  }
  const name = process.platform === "win32" ? "gameserver-win.exe" : "gameserver-host";
  return path.join(__dirname, "..", "resources", "server", name);
}

function startServer(): void {
  const exe = serverExecutable();
  if (!fs.existsSync(exe)) {
    console.error(`找不到服务端可执行文件：${exe}（请先运行 npm run build:server）`);
    return;
  }
  const dumpPath = path.join(app.getPath("userData"), "last-game.json");
  const args = ["--addr", `127.0.0.1:${PORT}`, "--dump", dumpPath];
  if (process.env.GX_SEED) args.push("--seed", process.env.GX_SEED);
  serverProc = spawn(exe, args, { stdio: ["ignore", "pipe", "pipe"] });
  serverProc.stdout?.on("data", (d) => process.stdout.write(`[gameserver] ${d}`));
  serverProc.stderr?.on("data", (d) => process.stdout.write(`[gameserver] ${d}`));
  serverProc.on("exit", (code) => {
    console.log(`[gameserver] 退出（code=${code}）；完整对局记录：${dumpPath}`);
    serverProc = null;
  });
}

function stopServer(): void {
  if (serverProc) {
    serverProc.kill();
    serverProc = null;
  }
}

app.whenReady().then(async () => {
  startServer();
  const win = new BrowserWindow({
    width: 1200,
    height: 780,
    title: "Game X · 本地对局原型（GX-4）",
    webPreferences: {
      preload: path.join(__dirname, "preload.cjs"),
      contextIsolation: true,
      nodeIntegration: false,
      sandbox: true,
    },
  });
  await win.loadFile(path.join(__dirname, "..", "dist", "index.html"), {
    query: { port: String(PORT) },
  });
});

app.on("window-all-closed", () => {
  stopServer();
  app.quit();
});
app.on("before-quit", stopServer);