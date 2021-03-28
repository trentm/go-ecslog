
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

.PHONY: fmt
fmt:
	go fmt ./cmd/... ./internal/...

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
