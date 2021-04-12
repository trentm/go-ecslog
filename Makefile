
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
	golint -set_exit_status ./cmd/... ./internal/...
	@echo "# check fmt with goimports..."
	@changes=$$(find . -name "*.go" | xargs goimports -d) && \
			if [[ -n "$$changes" ]]; then \
					echo "$$changes"; \
					exit 1; \
			fi

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
	rm -rf dist

# Ensure the top CHANGELOG.md h2 section and ecslog.Version are the same.
.PHONY: check-version
check-version:
	@ver="$(shell go run ./cmd/ecslog --version | head -1 | cut -d' ' -f2)" && \
		changelogVer="$(shell egrep '^## ' CHANGELOG.md | head -1 | cut -d' ' -f2)" && \
		[[ -n "$$ver" && "$$ver" == "$$changelogVer" ]] || \
		(echo "check-version: error: ecslog.Version ($$ver) != CHANGELOG.md version ($$changelogVer)" && exit 1)

# Tag and release a new release based on the current ecslog.Version.
# This long bit of Makefile does the following:
# - ensure the repo isn't dirty (changed files)
# - warn if we have a tag for this release already
# - interactively confirm
# - git tag
# - generate release notes (from CHANGELOG.md mostly)
# - call goreleaser to release
.PHONY: cutarelease
cutarelease: tidy check check-version
	[[ -z `git status --short` ]]  # If this fails, the working dir is dirty.
	@which goreleaser >/dev/null || (echo "cutarelease: error: missing 'goreleaser'" && exit 1)
	@ver=$$(go run ./cmd/ecslog --version | head -1 | cut -d' ' -f2) && \
		name=$$(grep ^module go.mod | cut -d' ' -f2) && \
		haveTag=$$(git tag -l "$$ver") && \
		if [[ -n "$$haveTag" ]]; then \
			echo ""; \
			echo "** Warning: $$ver tag already exists! Continue anyway?"; \
			echo "** Enter to continue, Ctrl+C to abort."; \
			read; \
		fi && \
		echo "" && \
		echo "** Confirm you want to tag and release $$name@$$ver" && \
		echo "** Enter to continue, Ctrl+C to abort." && \
		read && \
		if [[ -z "$$haveTag" ]]; then \
			date=$(shell date -u "+%Y-%m-%d"); \
			echo "Creating tag $$ver"; \
			git tag -a "$$ver" -m "$$ver ($$date)"; \
		fi && \
		git push origin "$$ver" && \
		mkdir -p ./tmp && ./scripts/gen-release-notes "$$ver" > ./tmp/release-notes.md && \
		GORELEASER_PREVIOUS_TAG="$$ver" goreleaser release --rm-dist --release-header=./tmp/release-notes.md

