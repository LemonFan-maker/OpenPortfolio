"use client";

import { Moon, Sun } from "lucide-react";
import { useEffect, useState } from "react";

export function ThemeToggle() {
  const [dark, setDark] = useState(false);

  useEffect(() => {
    const initial = window.localStorage.getItem("theme") === "dark";
    setDark(initial);
    document.documentElement.classList.toggle("dark", initial);
  }, []);

  function toggle() {
    const next = !dark;
    setDark(next);
    window.localStorage.setItem("theme", next ? "dark" : "light");
    document.documentElement.classList.toggle("dark", next);
  }

  return (
    <button className="btn btn-secondary h-10 w-10 px-0" onClick={toggle} title="切换主题">
      {dark ? <Sun size={18} /> : <Moon size={18} />}
    </button>
  );
}
