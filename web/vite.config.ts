import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";
import path from "node:path";

export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "./src"),
    },
  },
  build: {
    outDir: "dist",
    // emptyOutDir is false so the tracked `dist/.gitkeep` survives builds.
    // Without it, //go:embed all:dist would fail on a clean checkout that
    // has not yet run `npm run build`.
    emptyOutDir: false,
  },
  server: {
    port: 5173,
  },
});
