import { AppShell } from "@/components/layout/app-shell";

export default function ConsoleLayout({ children }: { children: React.ReactNode }) {
  return <AppShell>{children}</AppShell>;
}
