# Gateway Console Layout And Composition Rules

## App Shell

- Sidebar width: `260px` expanded, `76px` collapsed.
- Topbar height: `64px`.
- Page content max width: `1200px`.
- Page padding: `32px` desktop, `20px` mobile.
- Mobile and narrow viewports use the topbar route menu; the sidebar is hidden.

## Page Rules

- Every console route uses `AppShell`.
- Every page uses `PageShell`.
- Every page starts with `PageHeader`.
- Page headers include a kicker, title, and plain operational description.
- Data pages use `AsyncState` with loading, error, empty, and populated states.
- Destructive actions require `AlertDialog`.
- Copyable commands use `CommandBlock` and `CopyButton`.
- Secrets use `SecretField` and are never rendered raw.

## Grid System

- Dashboard summary: responsive 1/2/4 stat grid.
- Page sections: `grid gap-lg`.
- Data + detail split: `xl:grid-cols-[1fr_1.2fr]`.
- Tables are full-width inside bordered surfaces and horizontally scroll on
  small screens.
- Forms stay inside one card or panel; they do not float as nested cards.

## Empty, Loading, Error

- Loading uses `Skeleton` and must be visually stable.
- Empty states use dashed bordered panels with one action when useful.
- Errors use `Alert` with text and tone.

## Accessibility

- Radix components provide keyboard behavior for dialogs, menus, select,
  popovers, tabs, tooltips, switches, checkboxes, radio groups, and alerts.
- Focus rings are visible and accent colored.
- Statuses include text labels.
- Dialogs trap focus.
- Forms use explicit labels and error messages.
