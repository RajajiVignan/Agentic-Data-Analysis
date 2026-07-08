import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import path from "path";

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "./src"),
    },
  },
  build: {
    outDir: "out",
    emptyOutDir: true,
  },
  server: {
    port: 3001,
    proxy: {
      "/api": "http://localhost:3000",
      "/plots": "http://localhost:3000",
    },
  },
});
