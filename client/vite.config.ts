import { defineConfig } from "vite";

// 渲染层构建：base "./" 以支持 file:// 加载（打包后 Electron 直接载入 dist/index.html）。
export default defineConfig({
  base: "./",
  build: {
    outDir: "dist",
    emptyOutDir: true,
  },
});