// Package cli builds the cobra command tree for the spt binary. The cobra
// layer is intentionally thin: it parses flags, constructs config, and
// dispatches to the role packages under internal/app.
package cli
