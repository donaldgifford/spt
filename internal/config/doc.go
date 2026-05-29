// Package config loads spt's typed configuration from HCL2 files with
// env-var and CLI-flag overrides (file → env → flag precedence). The
// Config struct is the single shape every role's Run receives. See
// DESIGN-0001 for the layering rules and HCL block schema.
package config
