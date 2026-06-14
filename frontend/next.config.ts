import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  // Static export: no SSR, no API routes, no server components
  // All pages are pre-rendered to HTML at build time
  output: "export",
  // Output directory for static export
  distDir: "out",
  // Disable image optimization for static output
  images: {
    unoptimized: true,
  },
};

export default nextConfig;
