"use client";

import { X } from "lucide-react";
import { useState } from "react";
import { Sidebar } from "@/components/layout/sidebar";
import { Topbar } from "@/components/layout/topbar";
import { IconButton } from "@/components/ui/icon-button";
import { useUIStore } from "@/stores/ui-store";

export function AppShell({ children }: { children: React.ReactNode }) {
  const [mobileSidebarOpen, setMobileSidebarOpen] = useState(false);
  const collapsed = useUIStore((state) => state.sidebarCollapsed);
  const toggleSidebar = useUIStore((state) => state.toggleSidebar);
  const openNavigation = () => {
    toggleSidebar();
    setMobileSidebarOpen(true);
  };

  return (
    <div className="min-h-dvh bg-paper text-ink-primary">
      <div className="flex">
        <Sidebar collapsed={collapsed} />
        <div className="min-w-0 flex-1">
          <Topbar onToggleSidebar={openNavigation} />
          {children}
        </div>
      </div>
      {mobileSidebarOpen ? (
        <div
          aria-label="Mobile navigation"
          className="fixed inset-0 z-50 lg:hidden"
          data-testid="mobile-sidebar"
          role="dialog"
        >
          <button
            aria-label="Close mobile menu"
            className="absolute inset-0 bg-ink-primary/55"
            onClick={() => setMobileSidebarOpen(false)}
            type="button"
          />
          <div className="relative min-h-dvh w-[min(84vw,320px)] shadow-[0_24px_80px_rgba(15,20,25,0.25)]">
            <Sidebar variant="mobile" />
            <IconButton
              className="absolute right-3 top-3"
              icon={X}
              label="Close mobile menu"
              onClick={() => setMobileSidebarOpen(false)}
            />
          </div>
        </div>
      ) : null}
    </div>
  );
}
