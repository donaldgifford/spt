// Package main hosts the spt-judge-bootstrap tool.
//
// Two-mode CLI: `list` surfaces candidate few-shot examples for
// operator review; `apply` writes the accepted ones to
// internal/agent/judge/examples.json. See DESIGN-0006
// "judge-bootstrap" and IMPL-0002 Phase 5.
package main
