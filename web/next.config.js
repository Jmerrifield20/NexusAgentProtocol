/** @type {import('next').NextConfig} */
const nextConfig = {
  env: {
    NEXT_PUBLIC_REGISTRY_URL: process.env.NEXT_PUBLIC_REGISTRY_URL || "http://localhost:8080",
  },
};

module.exports = nextConfig;
