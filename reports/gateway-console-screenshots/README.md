# Gateway Console Screenshot Evidence

This directory is the local output target for public Gateway Console visual
evidence.

Generate a run from the app package:

```bash
cd web/gateway-console
npm run visual:evidence
```

The command writes timestamped artifacts under:

```text
reports/gateway-console-screenshots/YYYY-MM-DD-HHMM/
```

Each run contains:

- full-page route screenshots for desktop/mobile and light/dark;
- interaction state screenshots;
- `index.md` with an inventory;
- `visual-review.md` with review notes and known limitations.

PNG artifacts are ignored by default to avoid repository bloat. Commit only the
tooling and small curated screenshots when there is a specific review reason.
