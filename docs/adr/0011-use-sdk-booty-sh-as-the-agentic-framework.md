---
id: ADR-0011
title: "Use sdk-booty-sh as the agentic framework"
status: Proposed
author: Donald Gifford
created: 2026-05-23
---
<!-- markdownlint-disable-file MD025 MD041 -->

# 0011. Use sdk-booty-sh as the agentic framework

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

spt's agentic layer ([RFC-0001](../rfc/0001-server-price-tracker-platform.md)) needs a framework that gives us:

- A clean abstraction over LLM providers (we don't want to hard-code OpenAI/Anthropic API shapes throughout the codebase).
- Tool-calling primitives (the agent needs to call internal functions: query Postgres, fetch a listing, request a score).
- Native OTel instrumentation hooks so spans flow into ClickHouse + Langfuse ([ADR-0008](0008-use-otel-clickhouse-langfuse-for-agent-observability-and-evals.md)) without bespoke glue.
- A Go-native API (we're not introducing Python into the backend).

The Go agentic-framework space is younger than Python's, and most options are thin wrappers around provider SDKs. We want enough abstraction to swap providers and to instrument cleanly, without a framework so opinionated that it dictates application structure.

## Decision

We will use **sdk-booty-sh** as the agentic framework. Agentic workflows in spt are expressed using its tool-calling and run primitives; OTel instrumentation is wired through its hooks.

> **Note:** The framework name as captured here (`sdk-booty-sh`) should be confirmed before this ADR is accepted. If this is a placeholder or typo for a different framework, this ADR should be updated or superseded.

## Consequences

### Positive

- Provider abstraction lets us swap or A/B LLM providers without touching agent logic.
- Built-in OTel instrumentation aligns directly with ADR-0008's observability stack.
- Go-native, no Python sidecar.

### Negative

- The Go agentic ecosystem is less mature than Python's; framework churn and breaking changes are likely.
- Coupling to one framework's abstractions makes a future swap moderately expensive — we should keep the framework usage isolated behind a thin internal interface.

### Neutral

- We commit to keeping framework usage behind an internal package boundary so a future swap is mechanical, not architectural.

## Alternatives Considered

- **Direct provider SDKs (Anthropic Go SDK, OpenAI Go SDK)** — simplest, but locks us to one provider and we end up reinventing tool-calling and tracing abstractions.
- **LangChain Go / similar ports** — generally feel like rough ports of Python libraries; not idiomatic Go.
- **No framework, just `net/http` against provider APIs** — purest std-lib approach but pushes too much boilerplate into application code (retries, tool-call parsing, structured outputs, tracing) and dilutes the value of the agentic layer.
- **Python sidecar with a richer framework (e.g., DSPy, LlamaIndex)** — best library ecosystem, but adds a second runtime and breaks the single-binary deployment story.

## References

- [RFC-0001 — Server Price Tracker Platform](../rfc/0001-server-price-tracker-platform.md)
- [ADR-0008 — Use OTel + ClickHouse + Langfuse for agent observability and evals](0008-use-otel-clickhouse-langfuse-for-agent-observability-and-evals.md)
