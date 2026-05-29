# test/config/example.hcl
#
# Annotated reference config covering every documented block. Used by
# internal/config tests and as the canonical example for operators.
# Documented in internal/config/README.md.

log {
  format = "json"
  level  = "info"
}

admin {
  addr = ":9090"
}

ebay {
  # The env() function reads from the process environment. Empty values
  # are returned as the empty string; callers fall back to file-config
  # literals or rely on the validator to flag missing required fields.
  app_id      = env("EBAY_APP_ID")
  cert_id     = env("EBAY_CERT_ID")
  marketplace = "EBAY_US"
  rate_limit  = 5
}

postgres {
  dsn            = env("DATABASE_URL")
  max_open_conns = 25
  max_idle_conns = 5
}

valkey {
  addr     = env("VALKEY_ADDR")
  db       = 0
  password = env("VALKEY_PASSWORD")
}

meilisearch {
  url     = env("MEILI_URL")
  api_key = env("MEILI_API_KEY")
}

obs {
  otlp_endpoint        = env("OTEL_EXPORTER_OTLP_ENDPOINT")
  langfuse_host        = "https://cloud.langfuse.com"
  langfuse_public_key  = env("LANGFUSE_PUBLIC_KEY")
  langfuse_secret_key  = env("LANGFUSE_SECRET_KEY")
  span_sampling        = 1.0
}

api {
  addr          = ":8080"
  read_timeout  = "15s"
  write_timeout = "15s"
}

scheduler {
  tick_interval           = "5s"
  bulk_reconcile_interval = "12h"
  sync_interval           = "5m"
}

worker {
  # One pools "<stage>" { ... } block per stage. See DESIGN-0005 §
  # "Worker pool model" for stage-cost rationale.
  pools "poll"             { concurrency = 4  }
  pools "extract"          { concurrency = 8  }
  pools "score"            { concurrency = 16 }
  pools "judge"            { concurrency = 4  }
  pools "index"            { concurrency = 8  }
  pools "notify"           { concurrency = 4  }
  pools "reconcile_alerts" { concurrency = 4  }
  pools "reconcile_bulk"   { concurrency = 2  }
  pools "eval_alerts"      { concurrency = 8  }
}

# Bootstrap Watch declarations. Seed-from-HCL is implemented in a later
# IMPL; Phase 3 only parses + validates these blocks. Runtime CRUD goes
# through the API (per IMPL-0001 Resolved Decision #4).
watch "dell_r730xd" {
  query             = "Dell PowerEdge R730xd"
  cadence           = "15m"
  judge_sample_rate = 0.1

  notify {
    channel = "webhook"
    threshold {
      max_percentile = 25.0
    }
  }
}
