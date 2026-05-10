import type { Config } from "tailwindcss";

const config: Config = {
  content: [
    "./app/**/*.{js,ts,jsx,tsx,mdx}",
    "./components/**/*.{js,ts,jsx,tsx,mdx}",
  ],
  darkMode: "class",
  theme: {
    extend: {
      colors: {
        ink: "#172026",
        forest: "#2A6F63",
        jade: "#2A9D8F",
        cream: "#F8F7F3",
        paper: "#F7F9F8",
        coral: "#E76F51",
        amber: "#F4A261",
        leaf: "#6A994E",
      },
      boxShadow: {
        soft: "0 16px 48px rgb(23 32 38 / 0.10)",
      },
    },
  },
  plugins: [],
};

export default config;
