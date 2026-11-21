---
id: ADR-0004
title: "Canonical identities come from a catalog grown by proposal and approval"
status: Proposed
author: Donald Gifford
created: 2026-11-06
---

<!-- markdownlint-disable-file MD025 MD041 -->

# ADR-0004: Canonical identities come from a catalog grown by proposal and approval

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

The identity part of a product key (which server model, which CPU SKU,
which GPU) is an open-ended string that sellers write many ways. spt makes
it finite with data: a Postgres catalog of the things that exist and the
names they are called, trigram retrieval to a short candidate list, and a
typed decision over that list. The catalog grows by proposals the
escalation model writes and the operator approves. Not canonicalisation
rules in code, because every rule fixed one observed spelling and none was
data an operator could add to. Not an external product database, because
none covers used enterprise hardware and all would still need aliases.
Not automatic growth, because a wrong row silently merges two products'
baselines.

<!--docz:summary:end-->

<!--docz:context:start-->
## Context

Pricing needs a stable product key: every listing of the same thing must
land on the same key, or the baseline fragments and the comparison is
wrong. For memory and drives the key is typed attributes (generation,
capacity, speed) and ADR-0001 covers it. For servers, CPUs, GPUs,
workstations and desktops the key is a model name: "PowerEdge R740XD",
"R740xd", "Dell R740 XD 12-bay"; "Tesla P40", "P40 24GB Pascal";
"E5-2690v4", "E5-2690 v4". A decision model cannot answer an open
question, and a text model answers it differently from run to run.

The forces:

1. **Same thing, same key.** Across every spelling, abbreviation and
   vendor prefix a seller uses.
2. **The set is not closed.** New hardware appears and no seed is
   complete, so the system must be able to grow the set from what it
   sees.
3. **Growth must be reviewed.** A wrong row, or a duplicate of an
   existing one, merges or splits two products' price history silently.
   The cost of a wrong addition is higher than the cost of a delayed one.
4. **"Not any of these" is a real answer.** A listing for a model the
   system does not know must be detected as such, not forced onto the
   nearest key.
5. **Fits the decision contract.** Whatever narrows the open string must
   produce a list short enough for a `choice` question (at most 255
   options, in practice tens).
6. **No new service.** Retrieval has to work with Postgres as it is
   deployed.

<!--docz:context:end-->

<!--docz:decision:start-->
## Decision

- A `catalog` table holds one row per canonical thing: component type,
  canonical key, display name, attributes (vendor, family, model and
  per-type attributes) and aliases, with a `pg_trgm` index over display
  name and aliases.
- A string field the registry marks `resolve` is matched against item
  specifics first, then retrieved by trigram similarity to at most 50
  candidates (hard cap 255), then decided by a `choice` over the
  candidates with a companion yes/no question asking whether the listing
  is one of them. A confident match sets `catalog_ref`; a confident "no"
  proposes; anything in between escalates.
- The escalation model may propose a row with evidence. It never writes
  to `catalog`. Proposals are approved, with edits, or rejected by the
  operator through signals to `CatalogReviewWorkflow`.
- Product-key templates reference catalog attributes; canonical spelling
  is the row's job, not the listing's.
- The initial catalog is seeded as proposals from existing data, for one
  approval pass. The resolution algorithm and schema are in DESIGN-0005.

<!--docz:decision:end-->

<!--docz:consequences:start-->
## Consequences

<!--docz:positive:start-->
### Positive

- Canonicalisation code, denylists and overrides have no equivalent.
- The operator grows the catalog from the review page; the system
  proposes what it could not resolve, with evidence.
- Product keys are stable across spellings because the key is rendered
  from the row.
- "None of these" is an explicit, measured answer rather than a silently
  wrong key.

<!--docz:positive:end-->

<!--docz:negative:start-->
### Negative

- Seeding is real work and is the largest risk to the escalation-rate
  target. Mitigated by seeding from existing data as proposals and by
  measuring escalation rate from the first tick.
- Proposal review is a recurring operator task. Mitigated by the daily
  summary and the pending-proposals badge in the UI.
- A wrong approval merges two products until corrected; per-key sample
  counts and the calibration report make such merges visible.

<!--docz:negative:end-->

<!--docz:neutral:start-->
### Neutral

- Open-numeric attributes (TDP, cache, base clock) are catalog attributes
  derived from the resolved row, not questions asked of the model.
- Drive and RAM part numbers stay open-ended; their keys use typed
  attributes and treat the part number as an optional resolved field.

<!--docz:neutral:end-->
<!--docz:consequences:end-->

<!--docz:alternatives:start-->
## Alternatives Considered

- **Canonicalisation rules in code.** The first implementation's path:
  vendor and line canonicalisers, a line denylist added after 31
  Precision T3620 listings landed under `workstation:dell:poweredge:t3620`,
  a GPU model and family canonicaliser with a family override added when
  the model alternated between "Tesla" and "Ampere". Each rule fixed one
  spelling after it was observed in production (force 1 by patching),
  none of them could grow without a release (force 2), and a listing the
  rules did not cover was forced onto whatever the model wrote (force 4).
- **Trust the text model's canonical name.** Ask the escalation model for
  the canonical model and use its answer as the key. The T3620 case is
  the evidence: the same listing resolves differently across runs, and
  the baseline fragments across the variants. There is also nothing to
  measure "none of these" against.
- **No catalog: keys from typed attributes only.** Works for memory and
  drives, where the identity is the attributes. Fails for servers, CPUs
  and GPUs, whose identity is the model name and whose attributes (cores,
  TDP, VRAM) follow from it rather than define it.
- **An external product database.** eBay's product catalog (ePIDs),
  Intel ARK, the vendor spec sites, Wikidata. None covers used enterprise
  gear well, each keys products differently, and every one of them would
  still need the alias table because sellers do not write catalog names.
  They are seed sources for proposals, not the catalog.
- **The catalog inside the registry file.** One versioned source for all
  domain data. The cadences differ: the registry changes with a release
  every few weeks; the catalog gains rows daily as the escalator proposes
  them. Putting SKUs in the registry would make every new model a
  release, which is force 2 failing on purpose.
- **Automatic approval when the escalator is confident and the vendor
  exists.** Faster growth with less operator work. It fails force 3: a
  confident wrong row merges two products silently, and there is no label
  data yet to say how often the escalator is confidently wrong. Revisit
  when proposal precision has been measured.
- **Embeddings (`pgvector`) instead of trigram retrieval.** Better recall
  on paraphrases and typos. It needs an embedding model in the loop and
  an extension the database does not have today (force 6). SKU-like
  strings are short and share long substrings with their aliases, which
  is where trigram similarity is at its best. Reserved for the case where
  trigram recall is shown insufficient on the golden dataset.

What tipped it: a table with aliases satisfies forces 1, 2 and 4 with
data instead of code, trigram retrieval plus a `choice` satisfies 5 and 6
with nothing new to deploy, and the proposal workflow is the only shape
that gives force 3 a human gate without stalling the pipeline.

<!--docz:alternatives:end-->

<!--docz:references:start-->
## References

- [RFC-0001 spt: durable execution of typed decisions over watched eBay listings](../rfc/0001-spt-durable-execution-of-typed-decisions-over-watched-ebay.md)
- [ADR-0001 Every finite question is a typed decision with a calibrated probability](../adr/0001-every-finite-question-is-a-typed-decision-with-a-calibrated.md)
- [ADR-0002 Temporal is the execution plane; Postgres is the system of record](../adr/0002-temporal-is-the-execution-plane-postgres-is-the-system-of-record.md) (`CatalogReviewWorkflow` and signals)
- [DESIGN-0005 Catalog: canonical identities, retrieval and proposals](../design/0005-catalog-canonical-identities-retrieval-and-proposals.md)
- [server-price-tracker `pkg/extract/system_normalize.go`](https://github.com/donaldgifford/server-price-tracker/blob/main/pkg/extract/system_normalize.go) and [`gpu_normalize.go`](https://github.com/donaldgifford/server-price-tracker/blob/main/pkg/extract/gpu_normalize.go) (the canonicalisers, denylist and family override); [DESIGN-0012 Add GPU as a new component type](https://github.com/donaldgifford/server-price-tracker/blob/main/docs/design/0012-add-gpu-as-a-new-component-type.md); [DESIGN-0015 Add workstation and desktop as component types](https://github.com/donaldgifford/server-price-tracker/blob/main/docs/design/0015-add-workstation-and-desktop-as-component-types.md)
- [TypeSafe API reference](https://docs.typesafe.ai/api) (255-option limit; two-stage score-then-choose for larger sets)
- [PostgreSQL `pg_trgm`](https://www.postgresql.org/docs/current/pgtrgm.html)

<!--docz:references:end-->
