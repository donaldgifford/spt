# Architecture Decision Records (ADRs)

This directory contains Architecture Decision Records documenting significant
technical decisions.

## What are ADRs?

ADRs document **technical implementation decisions** for specific architectural
components. Each ADR focuses on a single decision and includes:

- **Context**: The problem or constraint that led to this decision
- **Decision**: What was chosen and why
- **Consequences**: Trade-offs, pros, and cons
- **Alternatives**: Other options that were considered

## Creating a New ADR

```bash
docz create adr "Your ADR Title"
```

## ADR Status

- **Proposed**: Under discussion, not yet approved
- **Accepted**: Approved and being implemented or already implemented
- **Deprecated**: No longer relevant or superseded
- **Superseded by ADR-XXXX**: Replaced by another ADR

<!-- BEGIN DOCZ AUTO-GENERATED -->
## All ADRs

| ID | Title | Status | Date | Author | Link |
|----|-------|--------|------|--------|------|
| ADR-0001 | Use Go for the backend | Proposed | 2026-05-23 | Donald Gifford | [0001-use-go-for-the-backend.md](0001-use-go-for-the-backend.md) |
| ADR-0002 | Use Bun as the frontend runtime and bundler | Proposed | 2026-05-23 | Donald Gifford | [0002-use-bun-as-the-frontend-runtime-and-bundler.md](0002-use-bun-as-the-frontend-runtime-and-bundler.md) |
| ADR-0003 | Use shadcn/ui with Recharts for the web UI | Proposed | 2026-05-23 | Donald Gifford | [0003-use-shadcnui-with-recharts-for-the-web-ui.md](0003-use-shadcnui-with-recharts-for-the-web-ui.md) |
| ADR-0004 | Use PostgreSQL as the canonical relational store | Proposed | 2026-05-23 | Donald Gifford | [0004-use-postgresql-as-the-canonical-relational-store.md](0004-use-postgresql-as-the-canonical-relational-store.md) |
| ADR-0005 | Use Valkey for queues and caching | Proposed | 2026-05-23 | Donald Gifford | [0005-use-valkey-for-queues-and-caching.md](0005-use-valkey-for-queues-and-caching.md) |
| ADR-0006 | Use Meilisearch for listing search | Proposed | 2026-05-23 | Donald Gifford | [0006-use-meilisearch-for-listing-search.md](0006-use-meilisearch-for-listing-search.md) |
| ADR-0007 | Package and deploy via Docker and a Helm chart | Proposed | 2026-05-23 | Donald Gifford | [0007-package-and-deploy-via-docker-and-a-helm-chart.md](0007-package-and-deploy-via-docker-and-a-helm-chart.md) |
| ADR-0008 | Use OTel + ClickHouse + Langfuse for agent observability and evals | Proposed | 2026-05-23 | Donald Gifford | [0008-use-otel-clickhouse-langfuse-for-agent-observability-and-evals.md](0008-use-otel-clickhouse-langfuse-for-agent-observability-and-evals.md) |
| ADR-0009 | Use Prometheus for system metrics | Proposed | 2026-05-23 | Donald Gifford | [0009-use-prometheus-for-system-metrics.md](0009-use-prometheus-for-system-metrics.md) |
| ADR-0010 | Generate the frontend API client from OpenAPI | Proposed | 2026-05-23 | Donald Gifford | [0010-generate-the-frontend-api-client-from-openapi.md](0010-generate-the-frontend-api-client-from-openapi.md) |
| ADR-0011 | Use sdk-booty-sh as the agentic framework | Proposed | 2026-05-23 | Donald Gifford | [0011-use-sdk-booty-sh-as-the-agentic-framework.md](0011-use-sdk-booty-sh-as-the-agentic-framework.md) |
| ADR-0012 | Build a custom scheduler and pipeline orchestrator | Accepted | 2026-05-23 | Donald Gifford | [0012-build-a-custom-scheduler-and-pipeline-orchestrator.md](0012-build-a-custom-scheduler-and-pipeline-orchestrator.md) |
<!-- END DOCZ AUTO-GENERATED -->
