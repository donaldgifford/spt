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

<!-- markdownlint-capture -->
<!-- markdownlint-disable MD013 -->
| ID | Title | Status | Date | Author | Link |
| --- | --- | --- | --- | --- | --- |
| ADR-0001 | Every finite question is a typed decision with a calibrated probability | Proposed | 2026-11-06 | Donald Gifford | [0001-every-finite-question-is-a-typed-decision-with-a-calibrated.md](0001-every-finite-question-is-a-typed-decision-with-a-calibrated.md) |
| ADR-0002 | Temporal is the execution plane; Postgres is the system of record | Proposed | 2026-11-06 | Donald Gifford | [0002-temporal-is-the-execution-plane-postgres-is-the-system-of-record.md](0002-temporal-is-the-execution-plane-postgres-is-the-system-of-record.md) |
| ADR-0003 | The domain schema is data: an HCL registry embedded in the binary | Proposed | 2026-11-06 | Donald Gifford | [0003-the-domain-schema-is-data-an-hcl-registry-embedded-in-the-binary.md](0003-the-domain-schema-is-data-an-hcl-registry-embedded-in-the-binary.md) |
| ADR-0004 | Canonical identities come from a catalog grown by proposal and approval | Proposed | 2026-11-06 | Donald Gifford | [0004-canonical-identities-come-from-a-catalog-grown-by-proposal-and.md](0004-canonical-identities-come-from-a-catalog-grown-by-proposal-and.md) |
| ADR-0005 | Configuration is HCL decoded with hclkit | Proposed | 2026-11-06 | Donald Gifford | [0005-configuration-is-hcl-decoded-with-hclkit.md](0005-configuration-is-hcl-decoded-with-hclkit.md) |
| ADR-0006 | The API is a spec-first OpenAPI contract served on net/http | Proposed | 2026-11-06 | Donald Gifford | [0006-the-api-is-a-spec-first-openapi-contract-served-on-nethttp.md](0006-the-api-is-a-spec-first-openapi-contract-served-on-nethttp.md) |
| ADR-0007 | The operator UI is a React SPA behind a Bun BFF | Proposed | 2026-11-06 | Donald Gifford | [0007-the-operator-ui-is-a-react-spa-behind-a-bun-bff.md](0007-the-operator-ui-is-a-react-spa-behind-a-bun-bff.md) |
<!-- markdownlint-restore -->
<!-- END DOCZ AUTO-GENERATED -->
