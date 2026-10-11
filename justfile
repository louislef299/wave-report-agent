export GOOSE_DRIVER := "postgres"
export GOOSE_DBSTRING := "postgres://postgres:password@localhost:5432/postgres"
export GOOSE_MIGRATION_DIR := "./scripts/migs"

alias build := up
up *ARGS:
    docker compose up -d {{ ARGS }}
    goose up

test: lint
	go test ./...

lint:
	gofmt -l . && go vet ./...

alias clean := down
down *ARGS:
    docker compose down --remove-orphans {{ ARGS }}
