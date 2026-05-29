## spt migrate

Run SQL migrations against the canonical Postgres store

### Options

```
  -h, --help                    help for migrate
      --migrations-dir string   Override the embedded migration set with files from this directory.
```

### Options inherited from parent commands

```
      --admin-addr string     Address for the admin server (/healthz, /readyz, /metrics). (default ":9090")
      --config strings        Path to an HCL config file (repeatable; later files override earlier).
      --config-dir string     Directory of HCL config files loaded in lexical order (before --config files).
      --ebay-app-id string    eBay App ID (overrides ebay.app_id from config / EBAY_APP_ID).
      --ebay-cert-id string   eBay Cert ID (overrides ebay.cert_id from config / EBAY_CERT_ID).
      --log-format string     Log output format ("text", "json", or "auto" — TTY-detected on stderr). (default "auto")
      --log-level string      Log level ("debug", "info", "warn", "error"). (default "info")
      --meili-url string      Meilisearch URL (overrides meilisearch.url from config / MEILI_URL).
      --postgres-dsn string   Postgres DSN (overrides postgres.dsn from config / DATABASE_URL).
      --valkey-addr string    Valkey address (overrides valkey.addr from config / VALKEY_ADDR).
```

### SEE ALSO

* [spt](spt.md)	 - Server Price Tracker
* [spt migrate down](spt_migrate_down.md)	 - Roll back the most recently applied migration
* [spt migrate status](spt_migrate_status.md)	 - Print applied/pending migration state
* [spt migrate up](spt_migrate_up.md)	 - Apply all pending migrations

