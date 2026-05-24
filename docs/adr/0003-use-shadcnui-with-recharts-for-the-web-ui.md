---
id: ADR-0003
title: "Use shadcn/ui with Recharts for the web UI"
status: Proposed
author: Donald Gifford
created: 2026-05-23
---
<!-- markdownlint-disable-file MD025 MD041 -->

# 0003. Use shadcn/ui with Recharts for the web UI

<!--toc:start-->
- [Status](#status)
- [Context](#context)
- [Decision](#decision)
- [Consequences](#consequences)
  - [Positive](#positive)
  - [Negative](#negative)
  - [Neutral](#neutral)
- [Alternatives Considered](#alternatives-considered)
- [References](#references)
<!--toc:end-->

## Status

Proposed

## Context

The frontend ([RFC-0001](../rfc/0001-server-price-tracker-platform.md)) is a thin wrapper around the backend API: watch-list management, per-watch configuration, listing search, and market-data display. The market-data surface is chart-heavy — moving averages, percentile bands, time-to-sell distributions — so the UI stack needs:

- A component library that gives us decent defaults without locking us into a rigid design system.
- A charting library that handles time-series and distribution charts cleanly and composes with the component library's styling.
- Source-level access to components so we can adjust them as the product evolves.

## Decision

We will use **shadcn/ui** for the base component library and **Recharts** for charting. shadcn components are copied into the repo (not imported from a package), styled with Tailwind, and built on Radix primitives. Recharts is the default charting layer shadcn's own chart components wrap.

## Consequences

### Positive

- shadcn components live in our source tree — easy to read, easy to modify, no library upgrades breaking our UI.
- Tailwind + Radix is well-trodden; accessibility primitives are solid out of the box.
- Recharts is React-native, composable, and integrates with shadcn's chart wrappers.
- Component additions are `bunx shadcn add <name>` and a code review — no design-system gatekeeping.

### Negative

- Copy-in pattern means we own the components; if upstream improves them, we have to manually adopt changes.
- Tailwind class strings can get long; needs a class-merge utility and lint discipline.
- Recharts is fine for our chart types but isn't D3-grade for unusual visualizations.

### Neutral

- Tailwind is now a hard dependency of the frontend.

## Alternatives Considered

- **MUI / Mantine / Chakra** — packaged component libraries. Faster to start, but harder to modify and more visually opinionated.
- **Headless UI + custom components** — maximum control but reinvents what shadcn already gives us.
- **ECharts / Victory / D3 directly** — more powerful charting, but heavier API surface and weaker shadcn integration. Recharts is the right starting point; we can swap a specific chart to a heavier library if a visualization genuinely needs it.

## References

- [RFC-0001 — Server Price Tracker Platform](../rfc/0001-server-price-tracker-platform.md)
- shadcn/ui: <https://ui.shadcn.com>
- Recharts: <https://recharts.org>
