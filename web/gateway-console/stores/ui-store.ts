"use client";

import { create } from "zustand";

type ThemePreference = "light" | "dark";

type UIState = {
  sidebarCollapsed: boolean;
  theme: ThemePreference;
  toggleSidebar: () => void;
  setSidebarCollapsed: (collapsed: boolean) => void;
  setTheme: (theme: ThemePreference) => void;
};

export const useUIStore = create<UIState>((set) => ({
  sidebarCollapsed: false,
  theme: "light",
  toggleSidebar: () =>
    set((state) => ({ sidebarCollapsed: !state.sidebarCollapsed })),
  setSidebarCollapsed: (sidebarCollapsed) => set({ sidebarCollapsed }),
  setTheme: (theme) => set({ theme }),
}));
