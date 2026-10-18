// 构建服务端二进制到 client/resources/server/：
//   gameserver-host(.exe)  —— 本机平台（开发运行用）
//   gameserver-win.exe     —— Windows/amd64 交叉编译（打包与 Windows 运行用）
import { execFileSync } from "node:child_process";
import { mkdirSync } from "node:fs";
import path from "node:path";

const clientDir = path.resolve(import.meta.dirname, "..");
const repoRoot = path.resolve(clientDir, "..");
const serverDir = path.join(repoRoot, "server");
const outDir = path.join(clientDir, "resources", "server");
mkdirSync(outDir, { recursive: true });

function build(out, env) {
  execFileSync("go", ["build", "-o", path.join(outDir, out), "./cmd/gameserver"], {
    cwd: serverDir,
    stdio: "inherit",
    env: { ...process.env, ...env },
  });
}

const isWinHost = process.platform === "win32";
build(isWinHost ? "gameserver-host.exe" : "gameserver-host", {});
build("gameserver-win.exe", { GOOS: "windows", GOARCH: "amd64" });
console.log(`服务端二进制已产出：${outDir}`);