# Changelog

All notable changes to this project are documented here. The format is
based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and
this project adheres to [Semantic Versioning](https://semver.org/).
## [unreleased]

### Features

- *(scaffold)* IMPL-0001 Phase 1 — repo skeleton and module hygiene
- *(cli)* IMPL-0001 Phase 2 — cobra root and role scaffolding
- *(config)* IMPL-0001 Phase 3 — HCL2 config loader
- *(obs)* IMPL-0001 Phase 4 — observability core
- *(health)* IMPL-0001 Phase 5 — admin endpoints
- *(interfaces)* IMPL-0001 Phase 6 — service interface skeletons
- *(testing)* IMPL-0001 Phase 7 — testing infrastructure
- *(migrations)* IMPL-0001 Phase 8 — SQL migration scaffolding
- *(tools)* IMPL-0002 Phase 1 — mock-server
- *(cli)* IMPL-0002 Phase 2 — docgen inline as `spt gen-docs`
- *(tools)* IMPL-0002 Phase 3 — dataset-bootstrap
- *(tools)* IMPL-0002 Phase 4 — dataset-upload
- *(tools)* IMPL-0002 Phase 5 — judge-bootstrap
- *(tools)* IMPL-0002 Phase 6 — regression-runner
- *(tools)* IMPL-0002 Phase 7 — dashgen + Helm chart skeleton

### Bug Fixes

- *(cli)* Disable cobra/doc auto-gen footer; refresh CLI docs + changelog

### Documentation

- Updated and add design/impl docs
- *(impl-0002)* Check off Testing Plan items + record IMPL closure

### Miscellaneous Tasks

- *(labels)* Mode
- Trigger workflows on PR #7
- Nudge after Actions outage
- *(ci)* Add LICENSE, allow SQL migrations in Docker, refresh changelog
- Nudge after second Actions outage

### Testing

- *(obs)* Cover NewTracerProvider + Setup; finalize IMPL-0001

