
.PHONY: all
all:
	go build ./cmd/ecslog

.PHONY: run
run:
	go run ./cmd/ecslog

.PHONY: tidy
tidy:
	go mod tidy
