# Gateway Console Visual Evidence

The public Gateway Console keeps reproducible screenshot evidence local by
default. Screenshots are review artifacts, not product marketing screenshots.

## Command

```bash
cd web/gateway-console
npm run visual:evidence
```

The command launches Chromium through Playwright using the existing local Next.js
server on `127.0.0.1:19575` and writes artifacts under:

```text
reports/gateway-console-screenshots/YYYY-MM-DD-HHMM/
```

## Captured Matrix

Every run captures full-page PNGs for:

- `/ui-system`
- `/console`
- `/console/relays`
- `/console/connectors`
- `/console/projects`
- `/console/activation`
- `/console/audit`
- `/console/settings`
- `/device`
- `/oauth/authorize`

Each route is captured in:

- desktop `1440x1000`, light;
- desktop `1440x1000`, dark;
- mobile `390x844`, light;
- mobile `390x844`, dark.

## Interaction States

The visual evidence run also captures:

- Relay profile form focused;
- Relay profile form validation errors;
- device approval validation error;
- OAuth approve/deny controls and denied state;
- UI system dialog open;
- UI system dropdown open;
- UI system select open;
- UI system popover open;
- activation command copy affordance focused;
- mobile navigation opened;
- theme toggle light and dark states.

## Review Rules

Review the generated `index.md` and PNGs for:

- broken responsive layout;
- unreadable dark theme;
- card/table overflow;
- weak focus states;
- inconsistent badges/forms;
- command block readability;
- unclear mock/demo notices;
- token or absolute path leakage.

Fix only low-risk UI issues in this public repository. Do not add billing,
team/admin, support console, production provider login, managed runners, private
Cloud UI, or new backend product APIs as part of visual review.

## Known Limits

The console is mock-backed by default unless
`NEXT_PUBLIC_CODENCER_CONSOLE_MOCKS=false` is configured. Live Gateway API
coverage is limited to the read-only paths actually wired in the UI data client.
Private managed Codencer Cloud features are intentionally out of scope.
