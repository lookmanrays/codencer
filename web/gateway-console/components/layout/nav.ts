import {
  Activity,
  Boxes,
  Cable,
  LayoutDashboard,
  ListChecks,
  MonitorCog,
  Rocket,
  Settings,
  ShieldCheck,
} from "lucide-react";

export const consoleNav = [
  { href: "/console", label: "Dashboard", icon: LayoutDashboard },
  { href: "/console/relays", label: "Relays", icon: Cable },
  { href: "/console/connectors", label: "Machines", icon: MonitorCog },
  { href: "/console/projects", label: "Projects", icon: Boxes },
  { href: "/console/activation", label: "Activation", icon: Rocket },
  { href: "/console/audit", label: "Audit", icon: Activity },
  { href: "/console/settings", label: "Settings", icon: Settings },
  { href: "/ui-system", label: "UI System", icon: ListChecks },
] as const;

export const authNav = [
  { href: "/device", label: "Device approval", icon: ShieldCheck },
  { href: "/oauth/authorize", label: "OAuth consent", icon: ShieldCheck },
] as const;
