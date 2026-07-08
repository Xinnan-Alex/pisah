# Calm Utility Visual Redesign Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Restyle the Pisah web UI to calm slate soft utilities — DM Sans, monochrome ink CTAs, light/dark via `prefers-color-scheme` — per `docs/superpowers/specs/2026-07-08-calm-utility-visual-redesign-design.md`.

**Architecture:** Keep CSS variables as the single source of truth in `web/static/app.css`. Replace light tokens, add a dark `@media (prefers-color-scheme: dark)` block that reassigns the same names, then remap components that hardcode JetBrains Mono / orange / green / cream. Update `layout.html` font + theme-color. Sweep templates and `app.js` avatar colors so nothing bypasses the new palette.

**Tech Stack:** Go-embedded templates, plain CSS, Google Fonts (DM Sans), Alpine/htmx unchanged.

**Spec:** `docs/superpowers/specs/2026-07-08-calm-utility-visual-redesign-design.md`

---

## File map

| File | Responsibility |
|------|----------------|
| `web/templates/layout.html` | DM Sans link, `theme-color` |
| `web/static/app.css` | Tokens (light+dark), base type, all shared component remaps, new utility classes (`alert-error`, `alert-info`, `section-label`, `amount`, hero sizing) |
| `web/static/app.js` | Avatar color palette → slate/monochrome-friendly set |
| Owner templates (`signin`, `signup`, `capture`, `settings`, `setup`, `share`, `review`, `track`, `scanning`) | Drop inline JetBrains / orange / green / cream / peach hex; use vars or new classes |
| Friend templates (`landing`, `pick`, `share`, `pay`, `done`) | Same |
| Partials (`summary_strip`, `track_participants`) | Same |

No new packages. No Go logic changes. `make test` should still pass (DB-free suite).

**Verification style:** This is visual CSS work — use `rg` leftover scans + `make test` instead of inventing CSS unit tests.

---

### Task 1: Font + theme-color in layout

**Files:**
- Modify: `web/templates/layout.html`

- [ ] **Step 1: Swap font stylesheet and theme-color**

Replace the JetBrains Mono block and warm theme-color:

```html
<meta name="theme-color" content="#F1F5F9">
<title>{{if .Title}}{{.Title}}{{else}}Pisah · split the bill{{end}}</title>
<link rel="preconnect" href="https://fonts.googleapis.com">
<link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
<link href="https://fonts.googleapis.com/css2?family=DM+Sans:opsz,wght@9..40,400;9..40,500;9..40,600;9..40,700&display=swap" rel="stylesheet">
<link rel="stylesheet" href="/static/app.css?v={{.Version}}">
```

- [ ] **Step 2: Confirm no JetBrains reference remains in layout**

Run: `rg -n "JetBrains|FBEFE8" web/templates/layout.html`
Expected: no matches

- [ ] **Step 3: Commit**

```bash
git add web/templates/layout.html
git commit -m "$(cat <<'EOF'
feat(web): load DM Sans for calm utility UI

Replace JetBrains Mono and warm theme-color with slate canvas.
EOF
)"
```

---

### Task 2: Light + dark CSS tokens and base styles

**Files:**
- Modify: `web/static/app.css` (top of file through ~line 120: `:root`, `body`, `.app-shell`, headings, `.btn`, inputs, focus)

- [ ] **Step 1: Replace `:root` with slate soft light tokens**

Use semantic names from the spec. Keep temporary aliases for `--cream` / `--cream2` / `--peach` / `--brown` / `--orange` / `--green` mapping to the closest calm equivalents **only if** Step 2 will immediately update call sites in the same PR — prefer deleting warm names and updating consumers in Tasks 3–5. Recommended final `:root`:

```css
:root {
  --ink: #0F172A;
  --slate: #334155;
  --mut: #64748B;
  --canvas: #F1F5F9;
  --paper: #F8FAFC;
  --surface: #FFFFFF;
  --line: #E2E8F0;
  --border: #CBD5E1;
  --cta: #0F172A;
  --cta-text: #FFFFFF;
  --focus-ring: rgba(15, 23, 42, 0.4);
  --danger-bg: #FEF2F2;
  --danger-text: #B91C1C;
  --danger-border: #FECACA;
  --info-bg: #F1F5F9;
  --info-text: #334155;
  --shadow: rgba(15, 23, 42, 0.1);
  --purple: #7C5CFF;
  --amber: #E8A02D;
  --radius: 12px;
  --viewfinder: #0B1220;
  --font: 'DM Sans', system-ui, -apple-system, 'Segoe UI', sans-serif;
  /* Temporary aliases removed after Task 3 remaps — do not leave --orange forever */
}
```

Also add semantic helpers used by templates (can live near `:root`):

```css
.amount { font-variant-numeric: tabular-nums; }
.section-label {
  font: 600 11px/1.2 var(--font);
  letter-spacing: .5px;
  text-transform: uppercase;
  color: var(--mut);
  margin: 0 0 10px;
}
.alert-error {
  background: var(--danger-bg);
  color: var(--danger-text);
  border: 1px solid var(--danger-border);
  padding: 12px 14px;
  border-radius: 12px;
  margin-bottom: 16px;
  font-size: 13px;
  font-weight: 600;
}
.alert-info {
  background: var(--info-bg);
  color: var(--info-text);
  border: 1px solid var(--line);
  padding: 12px 14px;
  border-radius: 12px;
  margin-bottom: 16px;
  font-size: 13px;
  font-weight: 600;
}
.brand-mark { font-size: 28px; font-weight: 700; letter-spacing: -.03em; }
.brand-dot { color: var(--mut); }
```

- [ ] **Step 2: Add dark overrides immediately after `:root`**

```css
@media (prefers-color-scheme: dark) {
  :root {
    --ink: #F1F5F9;
    --slate: #CBD5E1;
    --mut: #94A3B8;
    --canvas: #020617;
    --paper: #0B1220;
    --surface: #111827;
    --line: #1E293B;
    --border: #334155;
    --cta: #F8FAFC;
    --cta-text: #0F172A;
    --focus-ring: rgba(248, 250, 252, 0.5);
    --danger-bg: #450A0A;
    --danger-text: #FCA5A5;
    --danger-border: #7F1D1D;
    --info-bg: #1E293B;
    --info-text: #CBD5E1;
    --shadow: rgba(0, 0, 0, 0.45);
    --viewfinder: #020617;
  }
}
```

- [ ] **Step 3: Update base `body`, shell, focus, buttons, inputs**

Key mappings:

```css
body {
  margin: 0;
  font-family: var(--font);
  font-size: 15px;
  line-height: 1.45;
  color: var(--ink);
  background: var(--canvas);
  min-height: 100vh;
  min-height: 100dvh;
}

@media (min-width: 520px) {
  body { padding: 24px 16px; }
  .app-shell {
    max-width: 480px;
    min-height: calc(100dvh - 48px);
    border-radius: 28px;
    overflow: hidden;
    box-shadow: 0 24px 60px var(--shadow);
    background: var(--paper);
  }
}

h1, h2, h3, .display {
  font-family: var(--font);
  letter-spacing: -.02em;
  margin: 0;
  text-wrap: balance;
  font-weight: 650;
}

:focus-visible {
  outline: 3px solid var(--focus-ring);
  outline-offset: 2px;
  border-radius: 10px;
}

.btn {
  background: var(--cta);
  color: var(--cta-text);
  border: none;
  border-radius: var(--radius);
  padding: 16px;
  width: 100%;
  font: 600 15px var(--font);
  box-shadow: 0 8px 20px var(--shadow);
  /* keep cursor / tap / text-decoration rules */
}

.btn-outline {
  background: var(--surface);
  color: var(--ink);
  border: 1.5px solid var(--ink);
  box-shadow: none;
}

.btn-green {
  background: var(--cta);
  color: var(--cta-text);
  box-shadow: 0 8px 20px var(--shadow);
}

.btn-dark {
  background: var(--cta);
  color: var(--cta-text);
  box-shadow: 0 8px 20px var(--shadow);
}

input, textarea, select {
  width: 100%;
  background: var(--surface);
  border: 1.5px solid var(--border);
  border-radius: 12px;
  padding: 15px 16px;
  font: 500 15px var(--font);
  color: var(--ink);
  outline: none;
}
input:focus-visible, textarea:focus-visible {
  outline: none;
  border-color: var(--ink);
}

.card {
  background: var(--surface);
  border: 1px solid var(--line);
  border-radius: 14px;
  padding: 16px;
  box-shadow: 0 6px 18px var(--shadow);
}

.muted { color: var(--slate); }
.hint { color: var(--mut); }
.link { color: var(--ink); font-weight: 600; font-size: 14px; text-decoration: none; background: none; border: none; cursor: pointer; }
```

Remove every hardcoded `'JetBrains Mono'` `font:` shorthand in this header section; use `var(--font)` or `inherit`.

- [ ] **Step 4: Sanity-check CSS parses (no missing braces)**

Run: `python3 -c "import pathlib; t=pathlib.Path('web/static/app.css').read_text(); assert t.count('{')==t.count('}'), (t.count('{'), t.count('}'))"`
Expected: no assertion error

- [ ] **Step 5: Commit**

```bash
git add web/static/app.css
git commit -m "$(cat <<'EOF'
feat(web): add slate light/dark design tokens

Introduce DM Sans base styles and prefers-color-scheme overrides.
EOF
)"
```

---

### Task 3: Remap shared component rules in `app.css`

**Files:**
- Modify: `web/static/app.css` (remainder: back-chip, capture, share, scan, summary, items, badges, progress, laser, etc.)

- [ ] **Step 1: Global-replace role mappings inside CSS class rules**

Apply these substitutions systematically through the rest of the file (grep each, fix manually where opacity/rgba needs new ink values):

| Old | New |
|-----|-----|
| `var(--orange)` for CTA/focus/selected/links/scan | `var(--cta)` or `var(--ink)` as appropriate |
| `var(--green)` for paid/progress/success | `var(--slate)` text + `var(--line)` bg, or `var(--ink)` for fills |
| `var(--cream)` / `var(--cream2)` backgrounds | `var(--canvas)` |
| `var(--peach)` chips | `var(--line)` bg + `var(--slate)` text |
| `var(--brown)` | `var(--slate)` |
| `background: #fff` on sticky chrome / cards | `var(--surface)` |
| `background: #FDEAE3` | `var(--line)` |
| `#EAF6F1` green-tint chips | `var(--line)` |
| `rgba(242, 92, 59, …)` orange shadows | `var(--shadow)` or `rgba(15, 23, 42, .12)` |
| `rgba(24, 160, 122, …)` | same slate shadow |
| Any remaining `'JetBrains Mono'` in `font:` | `var(--font)` |

Concrete examples:

```css
.back-chip { background: var(--line); color: var(--ink); /* … */ }
.share-link-btn { color: var(--ink); }
.share-link-btn.is-copied { color: var(--slate); }
.qty-tag {
  width: 28px; height: 28px;
  background: var(--line); color: var(--slate);
  border-radius: 8px;
  display: flex; align-items: center; justify-content: center;
  font: 700 12px var(--font); flex: none;
}
.items { flex: 1; overflow: auto; padding: 14px 18px; background: var(--canvas); }
.item.selected {
  border: 2px solid var(--ink);
  padding: 12.5px 13.5px;
  box-shadow: 0 4px 12px var(--shadow);
}
.item.selected .check { background: var(--ink); border-color: var(--ink); }
.progress-fill { height: 100%; background: var(--ink); border-radius: 999px; }
.badge-paid { background: var(--line); color: var(--slate); }
.shared-tag { background: var(--line); color: var(--slate); font: 700 10px var(--font); /* … */ }
.shared-item.in {
  border: 2px solid var(--ink);
  padding: 12.5px 13.5px;
  box-shadow: 0 4px 12px var(--shadow);
}
.shared-pill { background: var(--cta); color: var(--cta-text); /* … */ font: 700 12px var(--font); }
.shared-pill.in {
  background: var(--line); color: var(--slate);
  border: 1.5px solid var(--border); padding: 6px 14px;
}
.corner { border: 3px solid var(--cta); }
.shutter { border: 4px solid var(--cta); }
.shutter-inner { background: var(--cta); }
.scan-step.done { color: var(--slate); }
.scan-step.active { color: var(--ink); }
/* laser line */
background: linear-gradient(90deg, transparent, var(--ink), transparent);
```

Also set sticky footers / white panels that used `#fff` to `var(--surface)`.

- [ ] **Step 2: Verify no warm token names remain in CSS**

Run: `rg -n "JetBrains|--orange|--green|--cream|--peach|--brown|#F25C3B|#18A07A|#FDEAE3|#F6F1EA|#EAF6F1|#FBEFE8" web/static/app.css`
Expected: no matches (avatar purple/amber vars may remain if unused; purple/amber in `:root` for avatars is OK until Task 4)

- [ ] **Step 3: Commit**

```bash
git add web/static/app.css
git commit -m "$(cat <<'EOF'
feat(web): remap components to slate monochrome tokens

Retire orange/green/cream usage from shared CSS rules.
EOF
)"
```

---

### Task 4: Avatar palette in `app.js`

**Files:**
- Modify: `web/static/app.js` (line 1 `avatarColors`)

- [ ] **Step 1: Replace warm avatar hues with slate-compatible set**

```js
const avatarColors = ['#0F172A', '#334155', '#475569', '#1E293B'];
```

(Dark mode: these stay dark-on-white letter chips — they remain readable on both themes because avatar text is white.)

- [ ] **Step 2: Commit**

```bash
git add web/static/app.js
git commit -m "$(cat <<'EOF'
feat(web): use slate avatar colors

Align participant chips with monochrome palette.
EOF
)"
```

---

### Task 5: Owner templates sweep

**Files:**
- Modify:
  - `web/templates/owner/signin.html`
  - `web/templates/owner/signup.html`
  - `web/templates/owner/capture.html`
  - `web/templates/owner/settings.html`
  - `web/templates/owner/setup.html`
  - `web/templates/owner/share.html`
  - `web/templates/owner/review.html`
  - `web/templates/owner/track.html`
  - `web/templates/owner/scanning.html`
  - `web/templates/partials/summary_strip.html`
  - `web/templates/partials/track_participants.html`

- [ ] **Step 1: Auth pages — brand, alerts, links**

`signin.html` / `signup.html` pattern:

```html
<h1 class="display brand-mark">Pisah<span class="brand-dot"> ·</span></h1>
{{if .Error}}
  <div role="alert" class="alert-error">{{.Error}}</div>
{{end}}
{{if .Info}}
  <div role="status" class="alert-info">{{.Info}}</div>
{{end}}
```

Footer links: `style="color:var(--ink);font-weight:600"` (not `--green`).

- [ ] **Step 2: Replace JetBrains section labels with `.section-label`**

Capture / settings / etc.:

```html
<p class="section-label" style="margin:16px 0 10px">Your name</p>
```

(or keep margin utilities inline). Remove all `font:…'JetBrains Mono'` inline styles.

- [ ] **Step 3: Remap accent inline styles**

Examples:

| Location | Change |
|----------|--------|
| settings Saved | `color:var(--slate)` |
| settings QR borders `#EFE6D8` | `var(--border)` |
| settings empty QR `var(--cream)` | `var(--canvas)` |
| settings checkbox accent | `accent-color:var(--ink)` |
| share success circle | `background:var(--line); color:var(--ink)` |
| share URL input bg | `var(--canvas)` |
| review page bg | `var(--canvas)` |
| review Total color | `var(--ink)` |
| track collected color | `var(--ink)` |
| track cream card | `var(--canvas)` |
| track “All friends paid” | `background:var(--line);color:var(--slate)` |
| scanning AI badge | ink/slate rgba instead of orange |
| scanning ✓ | `color:var(--slate)` |
| summary outstanding/collected | both `var(--ink)` (or outstanding `var(--ink)`, collected `var(--slate)`) |
| setup/settings QR `#EFE6D8` | `var(--border)` |

- [ ] **Step 4: Grep owner + partials for leftovers**

Run: `rg -n "JetBrains|--orange|--green|--cream|--peach|#FDEAE3|#EAF6F1|#EFE6D8|#F25C3B" web/templates/owner web/templates/partials`
Expected: no matches

- [ ] **Step 5: Commit**

```bash
git add web/templates/owner web/templates/partials
git commit -m "$(cat <<'EOF'
feat(web): restyle owner templates for slate theme

Drop mono/orange/green inline styles; use shared alert classes.
EOF
)"
```

---

### Task 6: Friend templates sweep

**Files:**
- Modify:
  - `web/templates/friend/landing.html`
  - `web/templates/friend/pick.html`
  - `web/templates/friend/share.html`
  - `web/templates/friend/pay.html`
  - `web/templates/friend/done.html`

- [ ] **Step 1: Landing**

```html
<header style="padding:48px 28px 24px;text-align:center;background:linear-gradient(180deg,var(--line),transparent)">
  <div aria-hidden="true" style="width:52px;height:52px;border-radius:50%;background:var(--cta);color:var(--cta-text);display:flex;align-items:center;justify-content:center;font:700 19px var(--font);margin:0 auto 14px">{{initial .Split.OwnerName}}</div>
  …
  <div style="font:700 16px var(--font)">{{.Split.Merchant}}</div>
```

Remove JetBrains from labels; use `var(--font)` or omit font (inherits).

- [ ] **Step 2: Pick / share / pay**

- Pick: replace JetBrains; `color:var(--orange)` “You” → `var(--ink)`; header `background:#fff` → `var(--surface)`.
- Share: cream panel → `var(--canvas)`; green “You owe” bar → `background:var(--cta);color:var(--cta-text)`.
- Pay: orange/peach pill → `color:var(--ink);background:var(--line)`; QR border `#EFE6D8` → `var(--border)`; cream empty state → `var(--canvas)`; orange dots → `var(--ink)`.

- [ ] **Step 3: Done**

```html
<div role="status" style="…;background:linear-gradient(180deg,var(--line),transparent)">
  <div aria-hidden="true" style="width:88px;height:88px;border-radius:50%;background:var(--cta);color:var(--cta-text);…;box-shadow:0 12px 30px var(--shadow)">
```

Checkmark borders can stay contrasting against CTA (use `var(--cta-text)` for the L-shaped check).

- [ ] **Step 4: Grep friend leftovers**

Run: `rg -n "JetBrains|--orange|--green|--cream|--peach|#EAF6F1|#EFE6D8|rgba\(24,160,122" web/templates/friend`
Expected: no matches

- [ ] **Step 5: Commit**

```bash
git add web/templates/friend
git commit -m "$(cat <<'EOF'
feat(web): restyle friend flow for slate theme

Align landing, pick, share, pay, and done with calm tokens.
EOF
)"
```

---

### Task 7: Final verification

**Files:** none (verification only)

- [ ] **Step 1: Repo-wide leftover scan**

Run:

```bash
rg -n "JetBrains|--orange|--green|--cream2?|--peach|--brown|#F25C3B|#18A07A|#FDEAE3|#F6F1EA|#F3ECE2|#FBEFE8|#EAF6F1|#EFE6D8" web/
```

Expected: no matches in `web/` (prototype HTML under `Receipt splitting with AI/` is out of scope and may still match if searched from repo root — scope to `web/`).

- [ ] **Step 2: Brace balance + tests**

```bash
python3 -c "import pathlib; t=pathlib.Path('web/static/app.css').read_text(); assert t.count('{')==t.count('}')"
make test
```

Expected: assertion OK; `make test` PASS

- [ ] **Step 3: Manual visual smoke (if server available)**

`make run`, then spot-check light + dark OS appearance on: `/` capture, review, share, `/r/<slug>` friend flow, track, settings, signin.

Checklist:
- [ ] DM Sans loaded (no mono UI)
- [ ] Primary buttons ink / inverted in dark
- [ ] Selected items / qty chips / paid badges slate, readable
- [ ] Error alerts rose (light) / deep red (dark)
- [ ] No cream/orange chrome

- [ ] **Step 4: Final commit only if Step 3 found fixes; otherwise done**

If smoke finds stray hex, fix and:

```bash
git add -u web/
git commit -m "$(cat <<'EOF'
fix(web): clean remaining warm accents after restyle
EOF
)"
```

---

## Spec coverage checklist

| Spec requirement | Task |
|------------------|------|
| DM Sans in layout | 1 |
| Light slate tokens | 2 |
| Dark `prefers-color-scheme` | 2 |
| Monochrome CTAs / chips / progress | 3 |
| Base type size / display weight | 2 |
| Component remaps in CSS | 3 |
| Avatar colors | 4 |
| Template hardcoded accents | 5–6 |
| Error/info alert classes | 2 + 5 |
| Retired orange/green/cream | 3, 5, 6, 7 |
| `make test` still passes | 7 |
| No theme toggle / no IA change | N/A (non-goal) |

## Plan self-review notes

- No TBD placeholders; CSS/token values and verification commands are explicit.
- TDD adapted to `rg` + `make test` because there is no CSS test harness.
- Transient aliases for `--orange` etc. are discouraged — Tasks 3–6 update call sites so old names can be deleted in Task 2–3.
