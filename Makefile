
.PHONY: all
all:
	go build cmd/ecslog/main.go -o ecslog

.PHONY: run
run:
	go run cmd/ecslog/main.go
