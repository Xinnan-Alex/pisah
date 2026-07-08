# Calm utility visual redesign

**Date:** 2026-07-08  
**Status:** Approved for planning  
**Scope:** Typography and color system for the server-rendered web UI (`web/`)

## Problem

The live UI uses **JetBrains Mono** for all text on a **warm cream + terracotta** palette (`--cream` / `--orange` / peach accents). For a receipt-splitting and payment tool, that reads too large, playful, and “poster-like.” The prototype also used Bricolage Grotesque + Figtree; shipping code drifted to full mono.

## Goals

- Feel like a **calm utility**: quiet, clear, trustworthy — not a brand splash.
- Reduce optical size / playfulness of type while keeping mobile readability.
- Stay monochrome for actions and status (no teal/orange brand accent).
- Support **light and dark** via OS preference.
- Visual-only change: no flows, copy, layout IA, or backend work.

## Non-goals

- Manual theme toggle
- Marketing / landing redesign
- New illustration or empty-state art
- Changing the “Pisah” name or product behavior
- Refactoring template structure beyond swapping colors/fonts/classes needed for tokens

## Direction (decisions)

| Choice | Decision |
|--------|----------|
| Personality | Calm utility |
| Accent | Almost monochrome (ink CTAs, slate status chips) |
| Typeface | DM Sans (body + brand) |
| Surface direction | Slate soft (cool grays, not warm stone) |
| Dark mode | Yes — `prefers-color-scheme: dark`, no in-app toggle |

## Architecture

Keep the existing CSS-variable approach in `web/static/app.css`. Replace the token set; load DM Sans from Google Fonts in `web/templates/layout.html`. Override the same variables under a dark media query so components that already use `var(--*)` pick up both themes. Move hardcoded warm/orange/green hex in templates onto variables (or semantic classes) so dark mode inherits.

```
layout.html  →  font link + theme-color
app.css      →  :root tokens, dark overrides, component remaps
templates    →  remove inline peach/orange/cream/green where they bypass vars
```

## Light tokens

| Token / role | Value | Use |
|--------------|-------|-----|
| `--ink` | `#0F172A` | Primary text, primary buttons, selected border, progress fill |
| `--slate` (replaces `--brown`) | `#334155` | Secondary text / chip text |
| `--mut` | `#64748B` | Hints, labels |
| `--canvas` (replaces cream page) | `#F1F5F9` | Page / items background |
| `--paper` / shell | `#F8FAFC` | App shell, headers |
| Card surface | `#FFFFFF` | Cards, inputs |
| `--line` | `#E2E8F0` | Dividers, chip backgrounds |
| `--border` | `#CBD5E1` | Input borders |
| Body background | Flat `--canvas` (no warm radial gradient) | |
| Shadow | Slate-tinted, lower warmth | e.g. `rgba(15, 23, 42, .08–.14)` |

**Retired from chrome:** `--orange` (`#F25C3B`), peach (`#FDEAE3`), warm cream gradient, green CTAs (`#18A07A`) for primary actions/progress/paid chips.

**Variable strategy:** Prefer renaming/repurposing tokens to match roles (`--canvas`, `--slate`) and deleting unused warm accents. Do not leave `--orange` aliased to ink long-term — update call sites so “orange” is not a semantic lie. Transient aliases during the PR are fine if they shrink diff noise.

**Viewfinder** may remain near-black (`#1C1610` or slate-near-black) on both themes.

## Dark tokens

Applied under `@media (prefers-color-scheme: dark)` by reassigning the same variable names where practical:

| Role | Value |
|------|-------|
| Page canvas | `#020617` |
| Paper / shell | `#0B1220` |
| Card surface | `#111827` |
| Primary text | `#F1F5F9` |
| CTA fill (inverted ink) | `#F8FAFC` with text `#0F172A` |
| Muted | `#94A3B8` |
| Line / chip bg | `#1E293B` |
| Border | `#334155` |
| Selected | Pale border (`#F8FAFC`) on dark card |

CTA invert keeps the monochrome rule: dark theme uses a light button on a dark canvas.

## Typography

- **Family:** `DM Sans` via Google Fonts (`opsz` + weights 400–700), with system-ui fallback.
- **Remove:** JetBrains Mono from `layout.html` and hardcoded mono stacks in CSS (buttons, inputs, labels).
- **Defaults:** Body ~15px / ~1.45 line-height; hero/display ~22–28px on mobile, weight 650–700 (not 800 mono).
- **Amounts:** `font-variant-numeric: tabular-nums` on money figures.
- **Cleanup:** Replace one-off inline hero hacks (e.g. signup `font-size:40px;font-weight:800`) with shared heading styles.

## Component mapping

| Today | After |
|-------|--------|
| Orange primary `.btn` | Ink primary (light) / inverted light button (dark) |
| `.btn-green` | Ink primary or outline; no green fill |
| Focus outline orange | Ink-tinted focus ring (~40% light / ~50% dark) |
| Selected item orange border/glow | Ink / pale border; soft slate shadow |
| Qty / peach tags | Slate chip (`--line` bg, `--slate` text) |
| Paid / success green chips | Slate chips (copy conveys status) |
| Progress green fill | Ink fill |
| Orange links / share “copied” green | Ink links; copied state can stay ink + label change |
| Scan laser / AI badge orange | Slate/ink highlight — no orange glow |
| Cream card nests | White (light) / `#111827` (dark) on canvas |

Radii may tighten slightly (~10–12px on controls vs 16–18) for a more utility feel; no layout restructure.

## Errors and accessibility

- **Errors (light):** Soft rose surface (`#FEF2F2`), text `#B91C1C`, border `#FECACA` — replace peach+orange alert pattern.
- **Errors (dark):** Deep red surface (`#450A0A`), text `#FCA5A5`, border `#7F1D1D`.
- **Success:** Prefer slate chip + clear copy; do not reintroduce green as brand accent.
- **Contrast:** Body text vs canvas meets WCAG AA; muted only for secondary hints.
- **`theme-color`:** Prefer `#F1F5F9` (light). Dark address-bar color is optional; OS dark already drives CSS.

## Implementation touchpoints

1. `web/templates/layout.html` — font link, `theme-color`
2. `web/static/app.css` — `:root`, dark media query, component rules that hardcode JetBrains / orange / green / cream
3. Templates with hardcoded warm/accent hex (owner + friend + partials) — point at variables or neutral classes
4. Asset cache: bump relies on existing `?v={{.Version}}` if versioning already changes on deploy

## Testing / acceptance

- Visual smoke in **light and dark OS**: capture, scanning, review, share, friend landing/pick/pay/done, track, settings, sign-in/sign-up.
- No peach / terracotta / warm-cream chrome left in UI (viewfinder exception OK).
- DM Sans loads; JetBrains Mono gone from layout.
- Selected/paid/qty chips and primary buttons readable in both themes (no “white text on white card” from inherited theme).
- `make test` still passes (CSS/template-only change).

## Open decisions resolved during brainstorm

- Paper quiet vs slate soft → **slate soft**
- Monochrome vs teal accent → **monochrome**
- Dark mode → **in scope**, preference-based only
