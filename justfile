export GOOSE_DRIVER := "postgres"
export GOOSE_DBSTRING := "postgres://postgres:password@localhost:5432/observations"
export GOOSE_MIGRATION_DIR := "./scripts/migs"

alias build := up
up *ARGS:
    docker compose up --wait -d {{ ARGS }}
    goose up

conn:
    psql $GOOSE_DBSTRING

test: lint
	go test ./...

lint:
	gofmt -l . && go vet ./...

alias clean := down
down *ARGS:
    docker compose down --remove-orphans {{ ARGS }}

alias dc := deepclean
deepclean:
    just down -v
    goose down
