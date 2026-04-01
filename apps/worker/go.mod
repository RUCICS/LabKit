module labkit.local/apps/worker

go 1.26

toolchain go1.26.1

require (
	github.com/google/uuid v1.6.0
	github.com/jackc/pgx/v5 v5.7.5
	labkit.local/packages/go/db v0.0.0
	labkit.local/packages/go/evaluator v0.0.0
	labkit.local/packages/go/jobs v0.0.0
	labkit.local/packages/go/labkit v0.0.0
	labkit.local/packages/go/manifest v0.0.0
)

require (
	github.com/BurntSushi/toml v1.5.0 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	github.com/lib/pq v1.12.1 // indirect
	github.com/xi2/xz v0.0.0-20171230120015-48954b6210f8 // indirect
	golang.org/x/crypto v0.37.0 // indirect
	golang.org/x/sync v0.13.0 // indirect
	golang.org/x/text v0.24.0 // indirect
)

replace labkit.local/packages/go/db => ../../packages/go/db

replace labkit.local/packages/go/evaluator => ../../packages/go/evaluator

replace labkit.local/packages/go/jobs => ../../packages/go/jobs

replace labkit.local/packages/go/labkit => ../../packages/go/labkit

replace labkit.local/packages/go/manifest => ../../packages/go/manifest
