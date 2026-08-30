# Startup ADK web srever on localhost
web:
	go run main.go web api webui

# Score the configured lake spots and record the run to the ledger
check *ARGS:
	go run ./cmd/surfcheck {{ARGS}}

# Score without writing anything, showing the reasoning behind each verdict
dry:
	go run ./cmd/surfcheck -dry-run -v

# Preview the alerts a run would send, without sending or recording anything
preview MIN="Good":
	go run ./cmd/surfcheck -dry-run -notify -min-rating {{MIN}}

# Score, record, and alert through ntfy. Needs NTFY_TOPIC set.
alert MIN="Good":
	go run ./cmd/surfcheck -notify -min-rating {{MIN}}

test:
	go test ./...

lint:
	gofmt -l . && go vet ./...
