package main

// RuleGroupSpec mirrors Prometheus's rule_files layout for a single
// group with recording + alerting rules.
type RuleGroupSpec struct {
	Name  string `yaml:"name"`
	Rules []Rule `yaml:"rules"`
}

// Rule is a recording or alerting rule. Set Alert or Record (not
// both); the YAML marshaler emits only the populated field.
type Rule struct {
	Alert       string            `yaml:"alert,omitempty"`
	Record      string            `yaml:"record,omitempty"`
	Expr        string            `yaml:"expr"`
	For         string            `yaml:"for,omitempty"`
	Labels      map[string]string `yaml:"labels,omitempty"`
	Annotations map[string]string `yaml:"annotations,omitempty"`
}

// RuleFileSpec ties a rule group to its on-disk filename.
type RuleFileSpec struct {
	File   string
	Groups []RuleGroupSpec
}

// RuleFiles returns the per-file rule groups dashgen writes.
func RuleFiles() []RuleFileSpec {
	return []RuleFileSpec{
		{
			File: "rules/spt-alerts.yml",
			Groups: []RuleGroupSpec{
				{
					Name: "spt.alerts",
					Rules: []Rule{
						{
							Alert:  "SptEbayQuotaExhausted",
							Expr:   `spt_ebay_quota_exhausted == 1`,
							For:    "30m",
							Labels: map[string]string{"severity": "warning"},
							Annotations: map[string]string{
								"summary":     "eBay API quota exhausted",
								"description": "spt has been at the eBay daily quota for 30m. Pollers should be deferring.",
							},
						},
						{
							Alert:  "SptStaleAlertsAccumulating",
							Expr:   `spt_alerts_stale_total > 0`,
							For:    "30m",
							Labels: map[string]string{"severity": "warning"},
							Annotations: map[string]string{
								"summary":     "Stale alerts are accumulating",
								"description": "Reconciliation has marked alerts stale for 30m. Check upstream eBay availability.",
							},
						},
						{
							Alert:  "SptSchedulerLeaderMissing",
							Expr:   `sum(spt_scheduler_role{role="leader"}) != 1`,
							For:    "60s",
							Labels: map[string]string{"severity": "critical"},
							Annotations: map[string]string{
								"summary":     "Exactly-one scheduler leader invariant violated",
								"description": "Either zero or multiple scheduler pods have the leader role. DAG runs may stall or double-fire.",
							},
						},
						{
							Alert:  "SptSchedulerSweepRecovering",
							Expr:   `rate(spt_scheduler_sweep_recovered_total[5m]) > 0`,
							For:    "15m",
							Labels: map[string]string{"severity": "warning"},
							Annotations: map[string]string{
								"summary":     "Scheduler sweep recovering work",
								"description": "Tasks have been timing out at a sustained rate for 15m. Check worker pool health.",
							},
						},
					},
				},
			},
		},
	}
}
