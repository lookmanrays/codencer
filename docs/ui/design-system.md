# Codencer UI Design System

Source of truth inspected for this pass:

- `codencer-web-main.zip` from the task attachment;
- live `https://codencer.dev` CSS and markup for confirmation.

The public Gateway Console uses the same visual language as the Codencer
website, adapted from marketing/documentation sections into an operational
console surface.

## Stack

- Next.js App Router
- React + TypeScript
- Tailwind CSS v4 token model
- Radix UI for behavior and accessibility
- Zod for runtime validation
- TanStack Query/Table for server state and tables
- Zustand for local UI state only
- React Hook Form + Zod resolver for forms

## Colors

Codencer is a warm paper/ink interface with a single orange accent.

| Token | Value | Use |
| --- | --- | --- |
| `--color-ink-primary` | `#0f1419` | primary text, dark sections |
| `--color-ink-secondary` | `#52525b` | secondary prose |
| `--color-ink-muted` | `#94949e` | metadata, captions |
| `--color-paper` | `#faf9f6` | page background |
| `--color-paper-tinted` | `#f2f1ed` | alternate surfaces |
| `--color-paper-strong` | `#ffffff` | raised surfaces |
| `--color-accent` | `#ff5a1f` | CTAs, kickers, focus |
| `--color-accent-hover` | `#e64a12` | primary hover |
| `--color-accent-tint-bg` | `#fff1e8` | highlighted cards |
| `--color-border` | `#e8e6e1` | warm separators |
| `--color-border-strong` | `#d4d4d8` | stronger field/table borders |
| `--color-code-bg` | `#0f1419` | terminal/code background |

Dark theme keeps the same semantic tokens and changes paper/ink values rather
than introducing a separate palette.

## Typography

- Sans: `"Helvetica Neue", Inter, system-ui, -apple-system, sans-serif`
- Mono: `"Courier New", "JetBrains Mono", ui-monospace, monospace`
- Headlines are bold, tight, and slightly compact.
- Kicker labels are mono-like in behavior: uppercase, `0.12em` tracking.
- Console metadata uses mono at `13px` with `0.04em` tracking.

## Spacing

Base rhythm follows the website:

- `xs`: 4px
- `sm`: 8px
- `md`: 16px
- `lg`: 32px
- `xl`: 64px
- `2xl`: 120px

Console pages use denser `md/lg` spacing inside cards and preserve wider
website spacing for page headers.

## Radius, Borders, Elevation

- Cards and controls use `--radius-card: 4px`.
- Borders are visible, warm, and usually `1px`.
- Strong borders use `1.5px` or accent left bars.
- Shadows are reserved for floating Radix surfaces such as dialogs, popovers,
  dropdowns, and tooltips.

## Motion

- Hover/focus transitions use `150ms ease`.
- Reduced motion disables long transitions.
- No decorative animation is used in the operational console.

## Components

- Buttons: solid accent primary, ink-outline secondary, quiet paper controls,
  danger outline for destructive placeholders.
- Cards: flat bordered surfaces, no nested card stacks.
- Fields: explicit labels, warm borders, visible focus rings.
- Code/terminal: dark ink background, mono text, copy button, no raw tokens.
- Status: text label plus tone; never color-only.

## Theme

Light and dark themes are token-based. The console uses Zustand only for local
theme preference and writes the selected theme to `document.documentElement`.
No auth tokens or server data are stored in Zustand.
