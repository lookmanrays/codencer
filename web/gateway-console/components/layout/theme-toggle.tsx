"use client";

import { Moon, Sun } from "lucide-react";
import { useEffect } from "react";
import { IconButton } from "@/components/ui/icon-button";
import { useUIStore } from "@/stores/ui-store";

export function ThemeToggle() {
  const theme = useUIStore((state) => state.theme);
  const setTheme = useUIStore((state) => state.setTheme);
  const next = theme === "dark" ? "light" : "dark";

  useEffect(() => {
    document.documentElement.dataset.theme = theme;
  }, [theme]);

  return (
    <IconButton
      icon={theme === "dark" ? Sun : Moon}
      label={`Switch to ${next} theme`}
      onClick={() => setTheme(next)}
    />
  );
}
