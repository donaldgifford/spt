# spt Helm chart

The chart skeleton lands with [IMPL-0002 Phase 7 (dashgen)](../../docs/impl/0002-developer-tooling-port-and-rewrite-from-old-spt.md#phase-7-dashgen) so the dashgen tool has a `charts/spt/files/` tree to write dashboards + Prometheus rules into.

The full chart (Deployment, Service, Ingress, RBAC, ServiceMonitor) lands with the packaging IMPL. Today, `Chart.yaml` and `values.yaml` are the only first-class chart files — `files/dashboards/*.json` and `files/rules/*.yml` are operator-consumable artifacts the chart's templates will eventually wrap.

## Regenerate dashboards + rules

```bash
# Write the dashboards and rules into charts/spt/files/.
just dashboards-gen

# CI guard: assert files/ is up to date.
just validate-dashboards
```
