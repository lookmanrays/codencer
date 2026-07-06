# Gateway Console Component Inventory

## Generic UI Primitives

Implemented under `web/gateway-console/components/ui`:

- `Button`
- `IconButton`
- `Card`
- `Input`
- `Textarea`
- `Label`
- `Field`
- `FormMessage`
- `Select`
- `Checkbox`
- `RadioGroup`
- `Switch`
- `Badge`
- `StatusBadge`
- `Alert`
- `Tabs`
- `Dialog`
- `AlertDialog`
- `DropdownMenu`
- `Tooltip`
- `Popover`
- `Separator`
- `Progress`
- `Toast`
- `CodeBlock`
- `CommandBlock`
- `CopyButton`
- `SecretField`
- `EmptyState`
- `Skeleton`
- `DataTable`
- `KeyValueList`
- `StatCard`
- `Timeline`

## Layout Components

Implemented under `web/gateway-console/components/layout`:

- `AppShell`
- `Sidebar`
- `Topbar`
- `Breadcrumbs`
- `PageHeader`
- `PageShell`
- `Kicker`
- `ThemeToggle`

## Console Components

Implemented under `web/gateway-console/components/console`:

- `WorkspaceSummaryCard`
- `RelayProfileCard`
- `RelayProfileForm`
- `RelayHealthBadge`
- `MachineConnectorTable`
- `ConnectorStatusBadge`
- `ProjectLocationsTable`
- `ActivationCommandPanel`
- `AuditEventTimeline`
- `OAuthConsentPanel`
- `DeviceApprovalPanel`
- `McpEndpointCard`
- `SelfHostModeNotice`
- `OfficialGatewayNotice`
- `DemoModeNotice`

## Data And State Layers

- Zod schemas: domain-specific files under `web/gateway-console/schemas/`
- API data layer: domain-specific modules under `web/gateway-console/api/`
- Server state: TanStack Query
- Local UI state: Zustand in `stores/ui-store.ts`

Live mode is the default. Demo mode is explicit through
`NEXT_PUBLIC_CODENCER_CONSOLE_MODE=demo`.
