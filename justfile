# Startup ADK web srever on localhost
web:
	go run main.go web api webui

# Score the configured lake spots and record the run to the ledger
check *ARGS:
	go run ./cmd/surfcheck {{ARGS}}

# Score without writing anything, showing the reasoning behind each verdict
dry:
	go run ./cmd/surfcheck -dry-run -v

test:
	go test ./...

lint:
	gofmt -l . && go vet ./...
