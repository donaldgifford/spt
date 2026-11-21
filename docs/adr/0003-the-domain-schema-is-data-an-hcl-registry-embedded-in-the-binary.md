---
id: ADR-0003
title: "The domain schema is data: an HCL registry embedded in the binary"
status: Proposed
author: Donald Gifford
created: 2026-11-06
---

<!-- markdownlint-disable-file MD025 MD041 -->

# ADR-0003: The domain schema is data: an HCL registry embedded in the binary

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

The component types, their questions, options, criteria, thresholds,
product-key templates and resolve rules live in one HCL file, embedded in
the binary and hashed to a `schema_version` that is stamped on every
decision. Not Go code, because the schema is the thing that changes most
and every change was a multi-file code change plus a migration. Not a
database table or a runtime-mounted file, because the schema is what
calibration is measured against and must be reviewable, versioned and
identical across roles. HCL because the registry is a configuration-shaped
document and ADR-0005 already chose the language and decoder.

<!--docz:summary:end-->

<!--docz:context:start-->
## Context

A decision model (ADR-0001) needs its questions and options declared
somewhere. That declaration is the domain: which component types exist,
what is asked about each, what the allowed answers are and how each is
described, which fields are required, what probability is good enough,
how a product key is rendered, which string fields resolve against the
catalog. It is the part of the product that changes most. Adding GPUs,
then workstations and desktops, was the work of the first implementation's
last three releases.

The forces:

1. **Change without code.** Adding a type, an option or a criterion
   should not touch Go, SQL or prompts. The first implementation needed
   eight touchpoints per type plus a `CHECK`-constraint migration, and a
   missed one failed at `watches create` with a constraint error.
2. **Versioned, reproducibly.** Thresholds are tuned from measured
   calibration against a specific question set. Every decision must
   record which version of the declarations produced it, and that version
   must be derivable from source, so a calibration number can never be
   compared across different question sets by accident.
3. **Reviewed before it takes effect.** A changed criterion can shift
   accuracy on every listing. The change must be a diff a reviewer sees,
   with the regression runner's accuracy and calibration tables attached,
   before it runs.
4. **Identical across roles.** The api role renders the label form and
   validates labels; the worker asks the questions. They must load the
   same declarations.
5. **Human-readable with criteria text.** Each option carries a plain
   sentence of criteria the model reads. The format needs a natural home
   for that text, nested under the option, under the question, under the
   component.
6. **Validated at load.** Template placeholders must reference declared
   questions; option counts must respect the decider's limit; thresholds
   must be in range. A bad registry must fail before a tick runs.

<!--docz:context:end-->

<!--docz:decision:start-->
## Decision

- The registry is `pkg/registry/registry.hcl`, embedded with `go:embed`
  and decoded with hclkit at process start by both roles. Failure to load
  or validate is fatal.
- `schema_version` is the first 12 hex characters of SHA-256 over the
  file. It is stamped on every listing and decision row, carried in every
  `ExtractWorkflow` ID and search attribute, and exposed by
  `spt registry version` and `GET /api/v1/registry`.
- A registry change is a pull request. `spt registry validate` and
  `spt registry diff <ref>` run in CI; the PR template requires the
  regression tables when the registry changes. The release that ships it
  bumps `schema_version` by construction; re-extraction is
  `spt ingest --full --schema-version <new>`.
- Database columns for component type and attributes carry no `CHECK`
  constraints; the registry validates.
- The block grammar, validation rules and an example registry are in
  DESIGN-0004.

<!--docz:decision:end-->

<!--docz:consequences:start-->
## Consequences

<!--docz:positive:start-->
### Positive

- Adding a component type is one file change plus catalog rows and
  labels; no Go, no migration.
- `schema_version` is reproducible from source and visible on every
  decision, workflow and API response.
- One decoder and validation style shared with configuration.
- No version skew between api and worker, because they release from one
  tag.
- The file is the schema documentation.

<!--docz:positive:end-->

<!--docz:negative:start-->
### Negative

- Iterating on a criterion requires a release, not a live edit. Accepted:
  thresholds and criteria are tuned from the calibration report, which is
  release-paced, and `spt decide --registry <file>` tries an edited
  registry locally before the PR.
- hclkit becomes a dependency of an exported `pkg/` package. Nothing
  external imports it.

<!--docz:negative:end-->

<!--docz:neutral:start-->
### Neutral

- The JSON schema the Claude decider compiles is a pure function of the
  registry, so it is stable per `schema_version` and the API's grammar
  cache hits.
- The chart README generator can render the registry as a reference
  table.

<!--docz:neutral:end-->
<!--docz:consequences:end-->

<!--docz:alternatives:start-->
## Alternatives Considered

- **Go code: typed structs, constants and switch arms.** Full compile-time
  checking and no parser. It is the first implementation's shape, and the
  evidence is its history: GPU, workstation and desktop each shipped as a
  DESIGN, an IMPL, Go changes in eight places and a migration. The version
  of the schema is "the binary", which satisfies force 2 only if every
  decision records the build; it cannot be diffed or validated by
  anything but a Go build; and criteria text ends up in string constants
  or prompt files with no structure (force 5).
- **Prompt files as the schema.** The questions written into prompts and
  parsed back out of answers. Unversioned except by file hash, with no
  structure a validator can check, and no way to compile one source into
  both a Jev question set and a Claude JSON schema.
- **A database table edited through the UI or API.** Live edits with no
  release. It fails force 3 (no diff to review, no regression gate before
  it takes effect) and force 2 (the version is a row id or timestamp, not
  derivable from source). It also puts the schema and the data it
  validates in the same store, so a restore or a migration can change
  which question set a decision belongs to.
- **A runtime-mounted file (ConfigMap) with hot reload.** Faster
  iteration than a release. The version is no longer tied to a build, the
  api and worker can load different files between rollouts (force 4), and
  a bad edit takes the worker down with no rollback other than another
  edit. The release train already makes "file change equals release"
  cheap, so the iteration speed is bought at the price of forces 2, 3, 4
  and 6.
- **JSON Schema per component, compiled to questions.** Closest to the
  Claude wire format. It is the worst of the formats for force 5: no
  comments, criteria text has to be smuggled into `description` fields,
  and labelled nesting (component, question, option) becomes nested
  objects with `properties` keys. The registry compiles to JSON Schema
  anyway; authoring in it gains nothing.
- **YAML.** Readable and nested, and what the first implementation used
  for configuration. No labelled blocks (a list of maps with a `name`
  key), no functions, validation is hand-written, and it is a second
  language next to ADR-0005's configuration with a second decoder.
- **CUE.** Real constraints in the language, stronger than hclkit's
  validators. A new language and toolchain for the operator and the fleet
  to carry for one file whose validation rules fit in a few functions.

What tipped it: the registry is a configuration-shaped document with
labelled nested blocks and short validation rules, which is exactly what
HCL and hclkit already do for ADR-0005, and embedding it makes the version
a property of the release rather than of a deployment.

<!--docz:alternatives:end-->

<!--docz:references:start-->
## References

- [RFC-0001 spt: durable execution of typed decisions over watched eBay listings](../rfc/0001-spt-durable-execution-of-typed-decisions-over-watched-ebay.md)
- [ADR-0001 Every finite question is a typed decision with a calibrated probability](../adr/0001-every-finite-question-is-a-typed-decision-with-a-calibrated.md)
- [ADR-0005 Configuration is HCL decoded with hclkit](../adr/0005-configuration-is-hcl-decoded-with-hclkit.md)
- [DESIGN-0004 Registry: the domain schema as an embedded HCL file](../design/0004-registry-the-domain-schema-as-an-embedded-hcl-file.md)
- [server-price-tracker `CLAUDE.md`](https://github.com/donaldgifford/server-price-tracker/blob/main/CLAUDE.md) (the eight touchpoints per component type); [DESIGN-0012 Add GPU as a new component type](https://github.com/donaldgifford/server-price-tracker/blob/main/docs/design/0012-add-gpu-as-a-new-component-type.md); [DESIGN-0015 Add workstation and desktop as component types](https://github.com/donaldgifford/server-price-tracker/blob/main/docs/design/0015-add-workstation-and-desktop-as-component-types.md)
- [hclkit](https://github.com/donaldgifford/hclkit); [boop ADR-0004 No CRDs: configuration from a chart-shipped file](https://github.com/donaldgifford/boop/blob/main/docs/adr/0004-no-crds-configuration-from-a-chart-shipped-file.md)

<!--docz:references:end-->
