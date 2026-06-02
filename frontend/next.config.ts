import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  turbopack: {
    root: process.cwd(),
  },
};

module.exports = {
  allowedDevOrigins: ['192.168.0.112'],
}

export default nextConfig;
