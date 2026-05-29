package domain

// Stage is the canonical name of one pipeline step. Stage values are
// the strings that show up in Postgres rows, Valkey queue keys
// (`spt:queue:stage:<stage>`), and metrics labels — they are part of
// the wire surface, not an internal detail.
//
// Per DESIGN-0002 the core DAG is:
//
//	poll ─▶ extract ─┬─▶ score ─┬─▶ judge   (sampled)
//	                 │          │
//	                 └─▶ index  └─▶ notify  (threshold)
//
// DESIGN-0004 adds the reconciliation/eval stages for the alert flow.
type Stage string

// Stage values are the strings persisted to Postgres and used as
// Valkey queue keys; they are part of the wire surface.
const (
	StagePoll            Stage = "poll"
	StageExtract         Stage = "extract"
	StageScore           Stage = "score"
	StageJudge           Stage = "judge"
	StageIndex           Stage = "index"
	StageNotify          Stage = "notify"
	StageReconcileAlerts Stage = "reconcile_alerts" // added by DESIGN-0004
	StageReconcileBulk   Stage = "reconcile_bulk"   // added by DESIGN-0004
	StageEvalAlerts      Stage = "eval_alerts"      // added by DESIGN-0004
)

// AllStages is the enumerated stage list used by config validation
// and the worker --pools default. New stages must be appended here.
var AllStages = []Stage{
	StagePoll,
	StageExtract,
	StageScore,
	StageJudge,
	StageIndex,
	StageNotify,
	StageReconcileAlerts,
	StageReconcileBulk,
	StageEvalAlerts,
}
