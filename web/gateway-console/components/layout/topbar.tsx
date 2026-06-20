"use client";

import { Menu } from "lucide-react";
import { authNav, consoleNav } from "@/components/layout/nav";
import { ThemeToggle } from "@/components/layout/theme-toggle";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { IconButton } from "@/components/ui/icon-button";
import { Avatar, AvatarFallback } from "@/components/ui/avatar";

export function Topbar({
  onToggleSidebar,
  workspaceName = "Personal Gateway Workspace",
}: {
  onToggleSidebar: () => void;
  workspaceName?: string;
}) {
  return (
    <header className="sticky top-0 z-30 border-b border-border bg-paper/95 backdrop-blur">
      <div className="flex min-h-[64px] items-center justify-between gap-md px-md">
        <div className="flex min-w-0 items-center gap-sm">
          <IconButton
            icon={Menu}
            label="Toggle sidebar"
            onClick={onToggleSidebar}
          />
          <div className="min-w-0">
            <p className="m-0 truncate font-semibold">{workspaceName}</p>
            <p className="m-0 font-mono text-mono tracking-[0.04em] text-ink-muted">
              public/self-host console
            </p>
          </div>
        </div>
        <div className="flex items-center gap-sm">
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button data-testid="nav-menu" variant="quiet">
                Routes
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end">
              {[...consoleNav, ...authNav].map((item) => (
                <DropdownMenuItem asChild key={item.href}>
                  <a href={item.href}>{item.label}</a>
                </DropdownMenuItem>
              ))}
            </DropdownMenuContent>
          </DropdownMenu>
          <ThemeToggle />
          <Avatar>
            <AvatarFallback>CO</AvatarFallback>
          </Avatar>
        </div>
      </div>
    </header>
  );
}
