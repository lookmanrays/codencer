"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import {
  CodencerLockup,
  CodencerMark,
} from "@/components/brand/codencer-brand";
import { authNav, consoleNav } from "@/components/layout/nav";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/cn";

export function Sidebar({
  collapsed = false,
  variant = "desktop",
}: {
  collapsed?: boolean;
  variant?: "desktop" | "mobile";
}) {
  const pathname = usePathname();
  return (
    <aside
      className={cn(
        "min-h-dvh border-r border-border bg-paper-strong",
        variant === "desktop" ? "hidden lg:block" : "block",
        collapsed ? "w-[76px]" : "w-[260px]",
      )}
    >
      <div className="sticky top-0 flex min-h-dvh flex-col">
        <div className="border-b border-border p-md">
          <Link
            aria-label="Codencer Gateway Console"
            className={cn(
              "flex min-w-0 items-center text-ink-primary no-underline",
              collapsed ? "justify-center" : "justify-start",
            )}
            href="/console"
          >
            {!collapsed ? (
              <span className="grid min-w-0 gap-xs">
                <CodencerLockup />
                <span className="font-mono text-mono tracking-[0.04em] text-ink-muted">
                  Gateway Console
                </span>
              </span>
            ) : (
              <CodencerMark className="h-[22px] w-[58px]" />
            )}
          </Link>
        </div>
        <nav aria-label="Console" className="flex-1 p-sm">
          <NavGroup
            collapsed={collapsed}
            items={consoleNav}
            pathname={pathname}
          />
          <div className="my-md border-t border-border" />
          <NavGroup collapsed={collapsed} items={authNav} pathname={pathname} />
        </nav>
        <div className="border-t border-border p-md">
          <Button asChild className="w-full" size="sm" variant="quiet">
            <a
              href="https://github.com/lookmanrays/codencer"
              rel="noreferrer"
              target="_blank"
            >
              {collapsed ? "GH" : "GitHub"}
            </a>
          </Button>
        </div>
      </div>
    </aside>
  );
}

function NavGroup({
  collapsed,
  items,
  pathname,
}: {
  collapsed: boolean;
  items: readonly {
    href: string;
    label: string;
    icon: React.ComponentType<{ className?: string }>;
  }[];
  pathname: string;
}) {
  return (
    <ul className="m-0 grid list-none gap-xs p-0">
      {items.map((item) => {
        const active =
          item.href === "/console"
            ? pathname === item.href
            : pathname.startsWith(item.href);
        const Icon = item.icon;
        return (
          <li key={item.href}>
            <Link
              className={cn(
                "flex min-h-10 items-center gap-sm rounded-[var(--radius-card)] border-l-2 border-transparent px-3 py-2 text-body-sm text-ink-secondary no-underline transition-colors hover:bg-paper-tinted hover:text-ink-primary",
                active &&
                  "border-accent bg-paper-tinted text-ink-primary shadow-[inset_0_0_0_1px_var(--color-border)]",
                collapsed && "justify-center px-2",
              )}
              href={item.href}
            >
              <Icon className="h-4 w-4" />
              {!collapsed ? <span>{item.label}</span> : null}
            </Link>
          </li>
        );
      })}
    </ul>
  );
}
