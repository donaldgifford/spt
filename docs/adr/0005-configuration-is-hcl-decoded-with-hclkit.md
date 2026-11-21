---
id: ADR-0005
title: "Configuration is HCL decoded with hclkit"
status: Proposed
author: Donald Gifford
created: 2026-11-06
---

<!-- markdownlint-disable-file MD025 MD041 -->

# ADR-0005: Configuration is HCL decoded with hclkit

<!--toc:start-->
<!-- markdownlint-capture -->
<!-- markdownlint-disable MD013 -->
- [Summary](#summary)
- [Context](#context)
- [Decision](#decision)
- [Consequences](#consequences)
  - [Positive](#positive)
  - [Negative](#negative)
  - [Neutral](#neutral)
- [Alternatives Considered](#alternatives-considered)
- [References](#references)
<!-- markdownlint-restore -->
<!--toc:end-->

<!--docz:summary:start-->
## Summary

spt's application configuration is one HCL file decoded with hclkit, not
YAML, TOML, JSON or environment variables. HCL was chosen because the
configuration is nested and repeated (a cascade of deciders, a set of task
queues), because secrets must be referenced rather than written, and
because the decoder, validator, formatter and editor tooling already exist
and are shared with the other services on the cluster and with spt's own
registry. Temporal connection details stay in the environment because they
belong to the deployment.

<!--docz:summary:end-->

<!--docz:context:start-->
## Context

The configuration has a shape that a flat format handles badly. There is
a cascade of deciders, each an instance of a kind with its own endpoint,
model, limits and budget, in an order that matters. There is one block per
task queue with concurrency and rate settings. There are scoring weights,
workflow timings, eBay and Discord settings, and observability endpoints.
Roughly a dozen blocks, several of them repeated with labels.

Five forces decide the format:

1. **Repeated, labelled blocks.** "A decider named `jev` of kind Jev" and
   "a queue named `decide`" are the natural units. The format should
   express a named instance of a kind directly.
2. **Secrets must not be literals.** API keys, the database DSN and the
   webhook URL arrive from Kubernetes Secrets. The file must be able to
   say "this value comes from the environment" in a way the loader can
   enforce, so a literal secret is a validation error rather than a
   review catch.
3. **Validation with positions.** A wrong block, a missing required field
   or an out-of-range number should be reported together, with file and
   line, before the process starts.
4. **One language for everything an operator edits.** The registry
   (ADR-0003) is also a configuration-shaped document. Two languages means
   two decoders, two validators and two sets of editor tooling.
5. **Shared with the fleet.** boop and repo-guardian run on the same
   cluster and are maintained by the same operator. A format and loader
   they already use is a format with a known failure surface.

A separate force applies to the Temporal connection. Address, namespace,
TLS material and the OIDC client are per environment, mounted by the
chart, and read by a package shared with the other services through
`TEMPORAL_*` environment variables. Putting them in the file would mean
either templating secrets into a ConfigMap or forking the shared package.

<!--docz:context:end-->

<!--docz:decision:start-->
## Decision

- Application configuration is a single HCL file, `spt.hcl`, decoded by
  `internal/config` using hclkit with its standard function set and an
  `env()` function. Defaults, then the file, then CLI flags. All
  diagnostics are reported together; any error is fatal.
- Fields marked secret accept only an `env()` reference. A literal is a
  validation error.
- Temporal connection and auth are environment variables with the shared
  names (`TEMPORAL_ADDRESS`, `TEMPORAL_NAMESPACE`,
  `TEMPORAL_TASK_QUEUE_PREFIX`, `TEMPORAL_TLS_*`, `TEMPORAL_OIDC_*`,
  `TEMPORAL_BUILD_ID`), read by `internal/temporal`. The scaffold's
  chart-managed variables (`LISTEN_ADDR`, `METRICS_ADDR`, `LOG_LEVEL`,
  `POD_NAME`) override the matching fields.
- `spt config validate <file>` runs the same loader as the service, in CI
  against the chart's rendered ConfigMap.
- The block set and its fields are specified in DESIGN-0001.

<!--docz:decision:end-->

<!--docz:consequences:start-->
## Consequences

<!--docz:positive:start-->
### Positive

- One language and one decoder across spt's config, spt's registry, boop
  and repo-guardian; `hclkit fmt` and `hclkit validate` work on all of
  them.
- Secrets are structurally outside the file.
- Errors arrive together with source positions.
- The Temporal chart templates from the other services are reusable
  without edits.

<!--docz:positive:end-->

<!--docz:negative:start-->
### Negative

- Two sources: the file and the `TEMPORAL_*` environment. Accepted
  because the split follows ownership (application versus deployment).
- HCL is less familiar to anyone outside the Terraform world than YAML.
  Accepted: the operator is the author.
- An operator coming from the first implementation's YAML learns a new
  layout.

<!--docz:negative:end-->

<!--docz:neutral:start-->
### Neutral

- The chart's `configMap.data` and `secrets.stringData` keep working for
  ad hoc variables; the HCL file is an additional ConfigMap mounted as a
  file.
- hclkit is a first-party library; a bug in it is fixable in the fleet,
  not upstream.

<!--docz:neutral:end-->
<!--docz:consequences:end-->

<!--docz:alternatives:start-->
## Alternatives Considered

- **YAML with environment substitution.** The first implementation's
  format: a `yaml.v3` struct decode after expanding `${VAR}` over the
  whole text. It fails the forces in order. Repeated instances become
  lists of maps with a `name` key the decoder has to validate by hand.
  Secrets cannot be guarded: expansion runs over every string, so a
  literal key and a reference look identical to the loader. Errors are
  reported one at a time with no position beyond the decoder's line
  number. YAML's own hazards (indentation as structure, implicit typing
  of `no` and `1e3`, anchors) have no upside for a dozen blocks. And it
  is a second language next to the registry.
- **TOML.** Comments, explicit types and a stable spec. Repeated named
  blocks are `[[decider]]` arrays of tables, which read worse than
  labelled blocks and still need a hand-validated `name` field. No
  expressions or functions, so `env()` and the secret guard have to be
  layered on outside the format. Nothing else on the cluster uses it.
- **JSON.** No comments, no functions, and verbose for humans. It is the
  wire format the deciders use, not a format an operator edits.
- **Environment variables only.** Twelve-factor and chart-native, and
  right for flat services. A cascade of deciders with per-instance
  settings flattens into forty-plus variables with naming conventions
  standing in for structure. The chart's `envFrom` contract stays for the
  handful of values that are genuinely per deployment.
- **viper or koanf over several sources.** Buys merging across files,
  environment and flags, at the cost of a second configuration ecosystem,
  case-insensitive key handling, and no validation with positions. The
  layering spt needs (defaults, file, flags) is three lines with hclkit.
- **CUE.** Stronger constraints than hclkit's validators and a real type
  system. A new language and toolchain for the operator and the fleet,
  for one file whose validation rules are short.

What tipped it: hclkit gives labelled blocks, `env()` as a function the
loader can check, aggregated diagnostics with positions, a formatter and
validator CLI, typed decoding through cty, and a loader shape that boop
already runs in production and that the first v2 attempt of this product
already wrote and tested for this exact configuration.

<!--docz:alternatives:end-->

<!--docz:references:start-->
## References

- [RFC-0001 spt: durable execution of typed decisions over watched eBay listings](../rfc/0001-spt-durable-execution-of-typed-decisions-over-watched-ebay.md)
- [ADR-0003 The domain schema is data: an HCL registry embedded in the binary](../adr/0003-the-domain-schema-is-data-an-hcl-registry-embedded-in-the-binary.md)
- [DESIGN-0001 System architecture: roles, configuration, deployment and data model](../design/0001-system-architecture-roles-configuration-deployment-and-data.md) (configuration)
- [hclkit](https://github.com/donaldgifford/hclkit): `pkg/hclkit`, `pkg/hclkit/funcs` (`Std`, `Env`), `pkg/hclkit/ctytypes`, and the `fmt`, `validate` and `lint` subcommands
- [boop `internal/config`](https://github.com/donaldgifford/boop/tree/main/internal/config) (hclkit loader with validators and `env()`); [boop ADR-0004 No CRDs: configuration from a chart-shipped file](https://github.com/donaldgifford/boop/blob/main/docs/adr/0004-no-crds-configuration-from-a-chart-shipped-file.md)
- [repo-guardian `internal/temporal`](https://github.com/donaldgifford/repo-guardian/tree/main/internal/temporal) (the `TEMPORAL_*` contract)
- [server-price-tracker `internal/config/config.go`](https://github.com/donaldgifford/server-price-tracker/blob/main/internal/config/config.go) (the YAML loader being replaced)
- Reference loader: the first v2 attempt's `internal/config` (pre-scaffold history of this repository; no public URL)

<!--docz:references:end-->
