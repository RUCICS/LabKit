module labkit.local/packages/go/jobs

go 1.26

toolchain go1.26.1

require (
	github.com/fergusstrange/embedded-postgres v1.32.0
	github.com/google/uuid v1.6.0
	github.com/jackc/pgx/v5 v5.7.5
	labkit.local/packages/go/db v0.0.0
)

replace labkit.local/packages/go/db => ../db
