---
id: ADR-0001
title: "Every finite question is a typed decision with a calibrated probability"
status: Proposed
author: Donald Gifford
created: 2026-11-06
---

<!-- markdownlint-disable-file MD025 MD041 -->

# ADR-0001: Every finite question is a typed decision with a calibrated probability

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

Every question spt asks about a listing that has a finite answer set
(which component type, which DDR generation, is it ECC, how many in the
lot, is this a deal) goes through one contract: the listing's state and a
declared question set in, one declared option per question with a
probability out. Control flow branches on probability thresholds, never on
returned text. Not free-text generation with parsing, because that is the
shape that produced nine thousand lines of repair code. Not constrained
decoding alone, because it fixes the format and not the inconsistency or
the missing probability. Jev, a decision model, is the primary
implementation; text models (Anthropic, OpenAI and the OpenCode gateway
in front of both, Ollama locally) answer the same question sets through
one compiled schema as fallbacks and cross-checks; a cascade composes
them per question.

<!--docz:summary:end-->

<!--docz:context:start-->
## Context

Understanding a listing means answering a few dozen questions. Most have
a finite answer set: a component type from a known list, an enum (`ddr4`,
`sff`, `sata`), a boolean (`ecc`, `rails_included`), a bounded numeric
from a small real-world set (16, 32, 64 GB), a quantity, a verdict. The
open-ended ones (which exact model, which part number) can be made finite
by looking them up first (ADR-0004).

The forces:

1. **The answer must be in the set.** Downstream code keys baselines,
   renders product keys and branches on these values. A value outside the
   declared set must be impossible, not repaired.
2. **The same listing must get the same answer.** Baselines fragment and
   alerts flap when extraction is nondeterministic across runs.
3. **Confidence must mean something.** The pipeline has to decide when to
   accept, when to ask a second model and when to ask a human. That needs
   a per-question probability that tracks accuracy and can be validated
   against labels later, so thresholds are tuned rather than guessed.
4. **Cheap and fast at volume.** Tens of questions per listing, on the
   order of ten thousand listings a day. Seconds and cents per listing is
   the budget's ceiling, not its target.
5. **Vendor independence, and models checking each other.** The primary
   model is a hosted service. The contract must let any other model
   answer the same question set with no per-model change to the
   questions, so implementations can be compared on one dataset, swapped
   by configuration, and run against each other on live traffic to
   measure agreement.
6. **Injection surface.** Listings are seller-written text. The less a
   model can emit, the less an injected instruction can do.

<!--docz:context:end-->

<!--docz:decision:start-->
## Decision

- A `Decider` contract in `pkg/decide`: input is the listing's state
  (title, item specifics, condition, price, quantity) and a question set
  of `choice`, `bool` and `score` questions with criteria; output is, per
  question, one declared option, a probability distribution, a confidence
  and which implementation answered.
- Guarantees: the value is always a declared option, and an
  implementation that cannot guarantee this at the wire validates and
  returns an error that the pipeline treats as a failure to escalate, not
  a value to repair. `Decide` is pure, so it is safe as a retried
  activity.
- Control flow branches on per-question thresholds declared in the
  registry (ADR-0003) and never inspects the text of a value.
- Implementations by kind: `jev` (TypeSafe directly or through the
  OpenCode gateway; primary), `anthropic` and `openai` (structured
  outputs through one schema compiled from the question set; the OpenCode
  gateway serves both formats), `ollama` (the same schema locally),
  `mock`. One question set, one compiled schema, no per-model branches:
  a model that cannot pass the contract suite is not a decider. A
  cascade re-asks each sub-threshold question of the next implementation;
  a cross-check samples listings to a second decider and records
  disagreement.
- Free-text work (bundle decomposition, catalog proposals) is a separate
  `Escalator` contract whose output is validated against the registry
  exactly like a decision.
- Every call is a Langfuse generation under the listing's session so
  calibration is measured from the same records the regression runner
  reads. Wire details, the schema compiler and thresholds are in
  DESIGN-0003.

<!--docz:decision:end-->

<!--docz:consequences:start-->
## Consequences

<!--docz:positive:start-->
### Positive

- The repair layer does not exist as a category. A malformed value is a
  failing test, not a normaliser arm.
- Every decision carries a probability that can be validated against
  labels, so "required field missing" becomes "probability below
  threshold: escalate" and thresholds are tuned from measured calibration.
- Prompt-injection surface shrinks to the escalation path: a decider can
  only return a declared option.
- Jev, Claude and Ollama are configuration; the regression runner
  compares them side by side on the golden dataset.

<!--docz:positive:end-->

<!--docz:negative:start-->
### Negative

- The primary implementation depends on a hosted service with
  self-reported calibration. Mitigated by the cascade, the cross-check
  and by measuring calibration before trusting thresholds.
- Text-model deciders have no real distribution; their confidence is
  self-reported in the schema, so cascade decisions on it are coarser.
  Accepted for fallbacks and cross-checks.
- A Jev HTTP client is new code, though small.

<!--docz:negative:end-->

<!--docz:neutral:start-->
### Neutral

- Open-string fields are out of scope for a decider by definition;
  ADR-0004 makes them finite through the catalog.
- Local-first remains an option, not the default.

<!--docz:neutral:end-->
<!--docz:consequences:end-->

<!--docz:alternatives:start-->
## Alternatives Considered

- **Free-text generation with a prompt and a parser.** The first
  implementation: a text model returns JSON, the code parses it and
  normalises every field. The evidence is the code that accumulated
  around it: about 9,400 lines of normalisers, denylists, overrides and
  repair arms, each added after an observed failure, plus one
  self-reported confidence per listing that did not track accuracy. It
  fails force 1 by construction (the output is text), force 2 (the same
  listing extracted differently across runs) and force 3 (no usable
  probability). It is cheap only because its failures are deferred to the
  operator.
- **A text model with constrained decoding as the only implementation.**
  Claude structured outputs enforce types, enums and required fields at
  the wire, which removes malformed JSON and most of force 1. It does not
  remove run-to-run inconsistency (force 2), it yields one invented
  confidence rather than a distribution (force 3), enum matching is not
  case-guaranteed and ranges are not enforced so client validation
  remains, and it costs seconds and cents per listing (force 4). It is
  the right fallback and cross-check, and the same compiled schema works
  for every provider that supports strict structured output.
- **A decision model (Jev).** Takes state and a set of typed questions,
  returns one declared option per question with a probability
  distribution, in one parallel pass, in well under a second, with input
  billed at a few cents per million tokens and no output cost. It is the
  only shape that satisfies forces 1 through 4 directly. Its costs are
  real: hosted, no Go SDK, calibration self-reported. Those are what
  force 5, the cascade and the cross-check are for.
- **Trained classifiers on the golden dataset.** Fully local, cheap, good
  for the type question. Attributes need a model per question family and
  more labels than exist; the catalog chooser's option set changes with
  every approved row, which a trained classifier cannot follow without
  retraining. The contract leaves room for one as an implementation for
  the questions where labels are plentiful.
- **Rules and regular expressions over the title.** Deterministic and
  free. The first implementation's accessory pre-filter and model
  canonicalisers were this, and they are the denylist story: every rule
  fixed one spelling and none generalised. Right for a handful of
  pre-filters, wrong as the extraction method.
- **Decisions for classification only; free text for attributes.** Halves
  the work. It leaves the normalisers in place for exactly the fields
  where they were largest (capacity, speed, form factor), which are also
  the bounded-numeric and enum questions a decision model answers best.

What tipped it: force 1 and force 3 together. Only a contract whose
output is a declared option with a probability makes both the repair
layer and guessed thresholds structurally unnecessary, and the first
implementation's history shows what each of those costs when it exists.

<!--docz:alternatives:end-->

<!--docz:references:start-->
## References

- [RFC-0001 spt: durable execution of typed decisions over watched eBay listings](../rfc/0001-spt-durable-execution-of-typed-decisions-over-watched-ebay.md)
- [ADR-0003 The domain schema is data: an HCL registry embedded in the binary](../adr/0003-the-domain-schema-is-data-an-hcl-registry-embedded-in-the-binary.md)
- [ADR-0004 Canonical identities come from a catalog grown by proposal and approval](../adr/0004-canonical-identities-come-from-a-catalog-grown-by-proposal-and.md)
- [DESIGN-0003 Decisions: the Decider contract, cascade, escalation and calibration](../design/0003-decisions-the-decider-contract-cascade-escalation-and.md)
- [TypeSafe AI, Introducing System One Models and Jev](https://typesafe.ai/blog/introducing-system-one-models-and-jev); [API reference](https://docs.typesafe.ai/api)
- [Claude API, Structured outputs](https://platform.claude.com/docs/en/build-with-claude/structured-outputs)
- [server-price-tracker `pkg/extract`](https://github.com/donaldgifford/server-price-tracker/tree/main/pkg/extract) (the repair layer); [DESIGN-0002 LLM Extraction Pipeline](https://github.com/donaldgifford/server-price-tracker/blob/main/docs/design/0002-llm-extraction-pipeline.md); [DESIGN-0011 Reduce alert noise via scoring recalibration and accessory pre-filter](https://github.com/donaldgifford/server-price-tracker/blob/main/docs/design/0011-reduce-alert-noise-via-scoring-recalibration-and-accessory-pre.md)

<!--docz:references:end-->
