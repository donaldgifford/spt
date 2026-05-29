## spt completion bash

Generate the autocompletion script for bash

### Synopsis

Generate the autocompletion script for the bash shell.

This script depends on the 'bash-completion' package.
If it is not installed already, you can install it via your OS's package manager.

To load completions in your current shell session:

	source <(spt completion bash)

To load completions for every new session, execute once:

#### Linux:

	spt completion bash > /etc/bash_completion.d/spt

#### macOS:

	spt completion bash > $(brew --prefix)/etc/bash_completion.d/spt

You will need to start a new shell for this setup to take effect.


```
spt completion bash
```

### Options

```
  -h, --help              help for bash
      --no-descriptions   disable completion descriptions
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

* [spt completion](spt_completion.md)	 - Generate the autocompletion script for the specified shell

