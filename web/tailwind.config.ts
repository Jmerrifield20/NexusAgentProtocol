import type { Config } from "tailwindcss";

const config: Config = {
  content: [
    "./app/**/*.{js,ts,jsx,tsx,mdx}",
    "./components/**/*.{js,ts,jsx,tsx,mdx}",
  ],
  theme: {
    extend: {
      colors: {
        nexus: {
          50: "#f0f4ff",
          500: "#4f46e5",
          900: "#1e1b4b",
        },
      },
    },
  },
  plugins: [],
};

export default config;
