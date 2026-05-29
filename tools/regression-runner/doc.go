// Package main hosts the spt-regression-runner tool.
//
// **DO NOT WIRE THIS TOOL INTO CI.** Anti-CI rationale preserved from
// prior art (donaldgifford/server-price-tracker/tools/regression-runner):
// regression-runner needs ANTHROPIC_API_KEY / OPENAI_API_KEY to invoke
// the production model backends. Wiring this into PR CI exposes those
// keys to fork-PR contributors via workflow logs / `env:` echoing,
// since `pull_request_target` workflows or even careless `pull_request`
// workflows can leak secrets. Release-gating against accuracy
// regression happens via the maintainer's local invocation, not PR CI.
//
// See DESIGN-0006 "regression-runner" and IMPL-0002 Phase 6.
package main
