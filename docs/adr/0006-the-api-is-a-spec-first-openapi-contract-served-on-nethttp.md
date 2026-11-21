---
id: ADR-0006
title: "The API is a spec-first OpenAPI contract served on net/http"
status: Proposed
author: Donald Gifford
created: 2026-11-06
---

<!-- markdownlint-disable-file MD025 MD041 -->

# ADR-0006: The API is a spec-first OpenAPI contract served on net/http

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

spt's API is HTTP with JSON bodies, described first in an OpenAPI
document, and served by generated handlers on the standard library
`net/http.ServeMux` with plain `func(http.Handler) http.Handler`
middleware. Not gRPC, because the consumers are a browser, a CLI and
shell scripts. Not a router framework, because Go's `ServeMux` now routes
by method and path pattern and every middleware spt needs, including the
OpenTelemetry one, already speaks `http.Handler`. Not code-first, because
the UI's types must be generated from a contract that exists before the
handlers do.

<!--docz:summary:end-->

<!--docz:context:start-->
## Context

Three clients call the API: the operator UI through its BFF (ADR-0007),
the `spt` CLI, and whatever the operator writes in a shell. The surface is
small and stable: watches, listings and their decisions, alerts and the
actions on them, labels, catalog and proposals, runs, registry, decision
statistics and calibration, status. Nothing streams. The api role also
holds a Temporal client to start and signal workflows, and it must stay
free of workflow code (ADR-0002).

The forces:

1. **A contract that exists before the code.** The UI generates
   TypeScript types from the API description and fails CI when they
   drift. That only works if the description is a checked-in file, not an
   artifact the Go binary emits at runtime.
2. **Browser, CLI and curl.** Every client must be able to call the API
   with a bearer token and JSON over plain HTTP, through an Ingress, with
   no special client library.
3. **Observability as middleware.** Request spans and route-labelled
   metrics come from the OpenTelemetry `otelhttp` handler, which wraps an
   `http.Handler`. Any framework that owns the handler signature needs an
   adapter in the hot path.
4. **Authentication from the first endpoint.** The API accepts writes, so
   bearer-JWT verification and a group check are middleware from day one,
   ordered after tracing and logging and before parameter binding, so an
   anonymous request is refused before a malformed one is reported.
5. **The fleet's shape.** repo-guardian serves its API this way on the
   same cluster: handler, problem-document, cursor, authn, authz and
   middleware code with a production history.

<!--docz:context:end-->

<!--docz:decision:start-->
## Decision

- `api/openapi.yaml` is the contract, written before handlers and linted
  with vacuum. oapi-codegen generates a strict server, models and the
  embedded spec for `std-http-server`; `just generate-api` regenerates and
  `just lint-api` fails on drift.
- Routing is `net/http.ServeMux` with method-and-pattern routes. The
  middleware chain is, outermost first: `otelhttp`, request ID and access
  log, panic recovery, authentication, authorization; the matched
  `r.Pattern` labels the span and metrics.
- Errors are RFC 9457 problem documents. Lists page by keyset cursor.
  Authentication is bearer JWT against the OIDC issuer; authorization is
  one operator group claim.
- The CLI's client is generated from the same spec; the UI's types are
  generated from the same spec with a drift check.
- `/healthz`, `/readyz` and `/metrics` are on the metrics address, outside
  the API mux. The endpoint set is specified in DESIGN-0006.

<!--docz:decision:end-->

<!--docz:consequences:start-->
## Consequences

<!--docz:positive:start-->
### Positive

- One contract drives the server, the CLI client and the UI types, with
  drift checks on both sides.
- No framework in the request path; every middleware is the standard
  signature, so `otelhttp` and the auth middleware compose directly.
- The handler, auth, paging and problem-document code has a production
  reference on the same cluster.

<!--docz:positive:end-->

<!--docz:negative:start-->
### Negative

- Writing OpenAPI by hand is more ceremony per endpoint than annotating
  Go. Accepted for a surface this small.
- `oapi-codegen` and `vacuum` join the toolchain.
- `ServeMux` has no route groups or per-group middleware; the auth split
  between public and protected routes is two muxes, which is explicit but
  manual.

<!--docz:negative:end-->

<!--docz:neutral:start-->
### Neutral

- Authentication exists before there is a UI, because the CLI sends a
  bearer token too. A `--insecure-no-auth` flag exists for loopback
  development only.
- Single operator today, so authorization is one group claim; per-watch
  scopes fit the same middleware later.

<!--docz:neutral:end-->
<!--docz:consequences:end-->

<!--docz:alternatives:start-->
## Alternatives Considered

- **gRPC, or Connect-RPC.** A typed contract and generated clients in Go
  and TypeScript, which is most of force 1. It fails force 2: the browser
  needs grpc-web or Connect's HTTP mapping, shell scripts need a
  dedicated client, and the Ingress and the BFF proxy need special
  handling for the content types. Connect's JSON mapping narrows the gap
  but leaves no OpenAPI document for anything outside the generated
  clients. Nothing in the surface streams, which is the case where gRPC
  would earn its cost.
- **Code-first with Huma on Echo.** The first implementation's choice.
  Good Go ergonomics and a generated document, but the document is
  produced by the running binary, so the UI's drift check runs backwards
  (force 1), and Echo owns the handler signature, so `otelhttp` and any
  standard middleware go through adapters (force 3). The first
  implementation's UI routes ended up outside Huma for that reason, and
  its CLI client was hand-written against the emitted document.
- **chi.** A `net/http`-compatible router with groups, per-group
  middleware and URL params. Everything it adds over `ServeMux` since Go
  1.22 is convenience: method routing and `{param}` patterns are now in
  the standard library, and `r.Pattern` gives the matched route for
  metrics. chi would be a dependency for route groups spt expresses as
  two muxes. If the surface grows to need groups, chi is a drop-in
  because the handler signature is the same.
- **Echo or Gin without Huma.** Fast routers with their own context and
  handler types. Force 3 again: the OpenTelemetry handler, the generated
  strict server and the auth middleware would all need the framework's
  adapters, and the generated code would target the framework instead of
  the standard library.
- **ogen.** Spec-first like oapi-codegen, with stricter validation and a
  generated client, but it generates its own router and middleware
  interfaces rather than plain `http.Handler`, which puts it on the wrong
  side of force 3 and away from the fleet's shape.
- **Code-first with the spec exported at build time.** Keeps Go
  ergonomics and produces a file. The file is still derived from code, so
  API changes are reviewed as Go diffs rather than spec diffs, and the
  export step has to run before the UI build.

What tipped it: `ServeMux` plus `otelhttp` plus oapi-codegen's
`std-http-server` means zero adapters anywhere in the request path, and
the spec-first flow is the one the UI's drift check needs.

<!--docz:alternatives:end-->

<!--docz:references:start-->
## References

- [RFC-0001 spt: durable execution of typed decisions over watched eBay listings](../rfc/0001-spt-durable-execution-of-typed-decisions-over-watched-ebay.md)
- [ADR-0002 Temporal is the execution plane; Postgres is the system of record](../adr/0002-temporal-is-the-execution-plane-postgres-is-the-system-of-record.md) (api role holds no workflow code)
- [ADR-0007 The operator UI is a React SPA behind a Bun BFF](../adr/0007-the-operator-ui-is-a-react-spa-behind-a-bun-bff.md)
- [DESIGN-0006 API: the OpenAPI contract, authentication and clients](../design/0006-api-the-openapi-contract-authentication-and-clients.md)
- [repo-guardian `internal/api/server.go`](https://github.com/donaldgifford/repo-guardian/blob/main/internal/api/server.go) (middleware order, two-mux public/protected split, `r.Pattern` labelling) and [`api/oapi-codegen.yaml`](https://github.com/donaldgifford/repo-guardian/blob/main/api/oapi-codegen.yaml)
- [repo-guardian DESIGN-0027 v2 read-only API, business UI and status page](https://github.com/donaldgifford/repo-guardian/blob/main/docs/design/0027-v2-read-only-api-business-ui-and-status-page.md)
- [Go 1.22 `net/http` routing enhancements](https://go.dev/blog/routing-enhancements); [`otelhttp`](https://pkg.go.dev/go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp)
- [oapi-codegen](https://github.com/oapi-codegen/oapi-codegen); [ogen](https://github.com/ogen-go/ogen); [vacuum](https://github.com/daveshanley/vacuum); [RFC 9457 Problem Details](https://www.rfc-editor.org/rfc/rfc9457)
- [server-price-tracker `go.mod`](https://github.com/donaldgifford/server-price-tracker/blob/main/go.mod) (Huma v2 on Echo v4)

<!--docz:references:end-->
