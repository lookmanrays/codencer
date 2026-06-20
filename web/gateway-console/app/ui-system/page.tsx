"use client";

import { MoreHorizontal } from "lucide-react";
import { RelayProfileCard } from "@/components/console/relay-profile-card";
import { RelayProfileForm } from "@/components/console/relay-profile-form";
import { MachineConnectorTable } from "@/components/console/machine-connector-table";
import { ProjectLocationsTable } from "@/components/console/project-locations-table";
import { ActivationCommandPanel } from "@/components/console/activation-command-panel";
import { AuditEventTimeline } from "@/components/console/audit-event-timeline";
import {
  DemoModeNotice,
  OfficialGatewayNotice,
  SelfHostModeNotice,
} from "@/components/console/mode-notices";
import { ThemeToggle } from "@/components/layout/theme-toggle";
import { PageShell } from "@/components/layout/page-shell";
import { Alert } from "@/components/ui/alert";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogTitle,
  AlertDialogTrigger,
} from "@/components/ui/alert-dialog";
import { Badge, StatusBadge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Checkbox } from "@/components/ui/checkbox";
import { CommandBlock } from "@/components/ui/code-block";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Field } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover";
import { Progress } from "@/components/ui/progress";
import { RadioGroup, RadioGroupItem } from "@/components/ui/radio-group";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Separator } from "@/components/ui/separator";
import { Switch } from "@/components/ui/switch";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Textarea } from "@/components/ui/textarea";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import { demoSnapshot } from "@/api/demo-data";

const colors = [
  "ink-primary",
  "ink-secondary",
  "ink-muted",
  "paper",
  "paper-tinted",
  "paper-strong",
  "accent",
  "accent-hover",
  "accent-tint-bg",
  "info",
  "info-soft",
  "success",
  "success-soft",
  "warning",
  "warning-soft",
  "danger",
  "danger-soft",
  "border",
  "border-strong",
  "code-bg",
  "code-fg",
];

const typeRows = [
  ["wordmark", "text-wordmark", "CODENCER"],
  ["h1", "text-h1", "Gateway Console"],
  ["h2", "text-h2", "Operational surface"],
  ["h3", "text-h3", "Relay profile"],
  ["body-lg", "text-body-lg", "Local-first bridge with Gateway routing."],
  ["body", "text-body", "Default readable console copy."],
  ["body-sm", "text-body-sm", "Dense metadata and helper text."],
  ["mono", "font-mono text-mono tracking-[0.04em]", "relay_profile_id=default"],
] as const;

export default function UISystemPage() {
  return (
    <PageShell
      breadcrumbs={[{ label: "UI System" }]}
      description="Live reference for tokens, layout rules, generic primitives, console modules, and accessibility behavior."
      kicker="Design system"
      title="Codencer Gateway Console UI"
    >
      <div className="grid min-w-0 max-w-full gap-xl">
        <section className="grid min-w-0 max-w-full gap-md">
          <h2 className="m-0 text-h2 font-bold">Tokens</h2>
          <div className="grid min-w-0 gap-md sm:grid-cols-2 lg:grid-cols-4">
            {colors.map((name) => (
              <Card key={name}>
                <div
                  className="aspect-[16/10]"
                  style={{ background: `var(--color-${name})` }}
                />
                <CardContent>
                  <p className="m-0 font-mono text-mono tracking-[0.04em]">
                    --color-{name}
                  </p>
                </CardContent>
              </Card>
            ))}
          </div>
          <Card>
            <CardContent>
              {typeRows.map(([name, className, sample]) => (
                <div
                  className="grid min-w-0 grid-cols-[180px_minmax(0,1fr)] items-baseline gap-md border-t border-border py-md first:border-t-0 max-md:grid-cols-1"
                  key={name}
                >
                  <span className="min-w-0 break-words font-mono text-mono text-ink-muted">
                    --text-{name}
                  </span>
                  <span
                    className={`${className} min-w-0 break-words leading-tight`}
                  >
                    {sample}
                  </span>
                </div>
              ))}
            </CardContent>
          </Card>
        </section>

        <section className="grid min-w-0 max-w-full gap-md">
          <h2 className="m-0 text-h2 font-bold">Layout grids</h2>
          <div className="grid min-w-0 gap-md md:grid-cols-2 xl:grid-cols-3">
            {[1, 2, 3].map((item) => (
              <Card key={item}>
                <CardContent>
                  <p className="m-0 font-mono text-mono">
                    dashboard grid {item}
                  </p>
                  <p className="mb-0 mt-sm text-body-sm text-ink-secondary">
                    Cards keep 4px radius, 1px warm borders, and clear density.
                  </p>
                </CardContent>
              </Card>
            ))}
          </div>
        </section>

        <section className="grid min-w-0 max-w-full gap-md">
          <h2 className="m-0 text-h2 font-bold">Primitives</h2>
          <Card>
            <CardHeader>
              <CardTitle>Buttons and status</CardTitle>
            </CardHeader>
            <CardContent className="flex flex-wrap items-center gap-sm">
              <Button>Primary</Button>
              <Button variant="secondary">Secondary</Button>
              <Button variant="quiet">Quiet</Button>
              <Button variant="danger">Danger</Button>
              <Badge variant="brand">brand</Badge>
              <Badge variant="info">info</Badge>
              <Badge variant="warning">warning</Badge>
              <Badge variant="danger">danger</Badge>
              <StatusBadge status="available" />
              <StatusBadge status="unavailable" />
              <ThemeToggle />
            </CardContent>
          </Card>
          <Card>
            <CardHeader>
              <CardTitle>Semantic alerts</CardTitle>
            </CardHeader>
            <CardContent className="grid min-w-0 gap-sm md:grid-cols-2">
              <Alert title="Neutral">Default operational note.</Alert>
              <Alert title="Info" tone="info">
                Non-blocking context for Gateway operators.
              </Alert>
              <Alert title="Success" tone="success">
                Verification or mutation completed.
              </Alert>
              <Alert title="Warning" tone="warning">
                Requires operator attention, but is not a brand accent.
              </Alert>
              <Alert title="Danger" tone="danger">
                A live API operation failed or is unsafe.
              </Alert>
              <Alert title="Brand" tone="brand">
                Primary Codencer path or Gateway MCP callout.
              </Alert>
            </CardContent>
          </Card>
          <Card>
            <CardHeader>
              <CardTitle>Forms</CardTitle>
            </CardHeader>
            <CardContent className="grid min-w-0 gap-md md:grid-cols-2">
              <Field id="demo-input" label="Input">
                <Input
                  id="demo-input"
                  placeholder="https://relay.example.com"
                />
              </Field>
              <Field id="demo-textarea" label="Textarea">
                <Textarea id="demo-textarea" placeholder="Operator note" />
              </Field>
              <Field id="demo-select" label="Select">
                <Select defaultValue="default">
                  <SelectTrigger id="demo-select">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="default">Default Relay</SelectItem>
                    <SelectItem value="personal">Personal Relay</SelectItem>
                  </SelectContent>
                </Select>
              </Field>
              <div className="flex min-w-0 flex-wrap items-center gap-md">
                <Checkbox id="demo-checkbox" defaultChecked />
                <label htmlFor="demo-checkbox">Explicit project sharing</label>
                <Switch defaultChecked />
              </div>
              <RadioGroup
                className="flex min-w-0 flex-wrap gap-md"
                defaultValue="gateway"
              >
                <label className="flex items-center gap-sm">
                  <RadioGroupItem value="gateway" /> Gateway
                </label>
                <label className="flex items-center gap-sm">
                  <RadioGroupItem value="relay" /> Direct Relay
                </label>
              </RadioGroup>
            </CardContent>
          </Card>
          <Card>
            <CardHeader>
              <CardTitle>Radix behavior</CardTitle>
            </CardHeader>
            <CardContent className="flex flex-wrap gap-sm">
              <Dialog>
                <DialogTrigger asChild>
                  <Button data-testid="dialog-open" variant="quiet">
                    Open dialog
                  </Button>
                </DialogTrigger>
                <DialogContent>
                  <DialogTitle className="m-0 text-h3 font-bold">
                    Gateway dialog
                  </DialogTitle>
                  <DialogDescription className="mt-sm text-body text-ink-secondary">
                    Focus is trapped and Escape closes the dialog.
                  </DialogDescription>
                </DialogContent>
              </Dialog>
              <AlertDialog>
                <AlertDialogTrigger asChild>
                  <Button variant="danger">Confirm destructive action</Button>
                </AlertDialogTrigger>
                <AlertDialogContent>
                  <AlertDialogTitle className="m-0 text-h3 font-bold">
                    Remove profile?
                  </AlertDialogTitle>
                  <AlertDialogDescription className="mt-sm text-body text-ink-secondary">
                    Destructive actions require confirmation.
                  </AlertDialogDescription>
                  <AlertDialogFooter>
                    <AlertDialogCancel>Cancel</AlertDialogCancel>
                    <AlertDialogAction>Remove</AlertDialogAction>
                  </AlertDialogFooter>
                </AlertDialogContent>
              </AlertDialog>
              <DropdownMenu>
                <DropdownMenuTrigger asChild>
                  <Button data-testid="dropdown-open" variant="quiet">
                    <MoreHorizontal className="h-4 w-4" /> Menu
                  </Button>
                </DropdownMenuTrigger>
                <DropdownMenuContent>
                  <DropdownMenuItem>Copy command</DropdownMenuItem>
                  <DropdownMenuItem>Open docs</DropdownMenuItem>
                </DropdownMenuContent>
              </DropdownMenu>
              <Popover>
                <PopoverTrigger asChild>
                  <Button variant="quiet">Popover</Button>
                </PopoverTrigger>
                <PopoverContent>
                  Relay tokens resolve server-side.
                </PopoverContent>
              </Popover>
              <Tooltip>
                <TooltipTrigger asChild>
                  <Button variant="quiet">Tooltip</Button>
                </TooltipTrigger>
                <TooltipContent>Visible focus ring is required.</TooltipContent>
              </Tooltip>
            </CardContent>
          </Card>
          <Tabs className="min-w-0 max-w-full" defaultValue="one">
            <TabsList>
              <TabsTrigger value="one">Terminal</TabsTrigger>
              <TabsTrigger value="two">Progress</TabsTrigger>
            </TabsList>
            <TabsContent value="one">
              <CommandBlock command="codencer setup mcp --client codex --endpoint http://127.0.0.1:19090/mcp --json" />
            </TabsContent>
            <TabsContent value="two">
              <Progress value={66} />
            </TabsContent>
          </Tabs>
        </section>

        <section className="grid min-w-0 max-w-full gap-md">
          <h2 className="m-0 text-h2 font-bold">Console modules</h2>
          <div className="grid min-w-0 gap-md lg:grid-cols-3">
            <DemoModeNotice />
            <OfficialGatewayNotice />
            <SelfHostModeNotice />
          </div>
          <RelayProfileCard relay={demoSnapshot.relays[0]!} />
          <RelayProfileForm />
          <MachineConnectorTable
            connectors={demoSnapshot.connectors}
            machines={demoSnapshot.machines}
          />
          <ProjectLocationsTable projects={demoSnapshot.projects} />
          <ActivationCommandPanel commands={demoSnapshot.activationCommands} />
          <AuditEventTimeline events={demoSnapshot.auditEvents} />
          <Alert title="No color-only status">
            Status text and badge tone are both rendered so color is not the
            only communication channel.
          </Alert>
          <Separator />
        </section>
      </div>
    </PageShell>
  );
}
