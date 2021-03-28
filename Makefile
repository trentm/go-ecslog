
.PHONY: all
all: ecslog

# Limitation: this doesn't catch all deps
ecslog: cmd/ecslog/*.go internal/*/*.go
	go build ./cmd/ecslog

.PHONY: test
test:
	go test ./cmd/... ./internal/...

.PHONY: check
check:
	go vet ./cmd/... ./internal/...
	golint ./cmd/... ./internal/...
	find . -name "*.go" | xargs goimports -l

.PHONY: fmt
fmt:
	find . -name "*.go" | xargs goimports -l -w

.PHONY: rundemo
rundemo:
	go run ./cmd/ecslog demo.log

.PHONY: tidy
tidy:
	go mod tidy

.PHONY: clean
clean:
	rm -f ecslog
	rm -f cmd/ecslog/ecslog-for-test
