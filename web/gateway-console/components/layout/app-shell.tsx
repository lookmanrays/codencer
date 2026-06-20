"use client";

import { Sidebar } from "@/components/layout/sidebar";
import { Topbar } from "@/components/layout/topbar";
import { useUIStore } from "@/stores/ui-store";

export function AppShell({ children }: { children: React.ReactNode }) {
  const collapsed = useUIStore((state) => state.sidebarCollapsed);
  const toggleSidebar = useUIStore((state) => state.toggleSidebar);
  return (
    <div className="min-h-dvh bg-paper text-ink-primary">
      <div className="flex">
        <Sidebar collapsed={collapsed} />
        <div className="min-w-0 flex-1">
          <Topbar onToggleSidebar={toggleSidebar} />
          {children}
        </div>
      </div>
    </div>
  );
}
