
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
	rm -rf release-bits

# Ensure CHANGELOG.md (top ver header) and ecslog.Version are the same.
.PHONY: check-version
check-version:
	@ver=$(shell go run ./cmd/ecslog --version | head -1 | cut -d' ' -f2) && \
		changelogVer=$(shell egrep '^## v\d+\.\d+\.\d+' CHANGELOG.md | head -1 | cut -dv -f2) && \
		[[ -n "$$ver" && "$$ver" == "$$changelogVer" ]] || \
		(echo "check-version: error: ecslog.Version ($$ver) != CHANGELOG.md version ($$changelogVer)" && exit 1)

# Build all the bits for a release, including extracting the top segment of the
# CHANGELOG.md to a file. (Limitation: that latter does not support there only
# being a single section in the file.)
.PHONY: release-bits
release-bits: ecslog
	rm -rf release-bits
	mkdir release-bits
	ver=$$(go run ./cmd/ecslog --version | head -1 | cut -d' ' -f2) && \
		GOOS=darwin  GOARCH=amd64 go build -o release-bits/ecslog-v$$ver-macos-amd64 ./cmd/ecslog && \
		GOOS=darwin  GOARCH=arm64 go build -o release-bits/ecslog-v$$ver-macos-arm64 ./cmd/ecslog && \
		GOOS=linux   GOARCH=amd64 go build -o release-bits/ecslog-v$$ver-linux-amd64 ./cmd/ecslog && \
		GOOS=windows GOARCH=amd64 go build -o release-bits/ecslog-v$$ver-windows-amd64.exe ./cmd/ecslog
	@echo "# Extract section from CHANGELOG.md to release-bits/changelog.md" && \
		start=$$(grep -n '^## v' CHANGELOG.md | head -1 | cut -d: -f1) && \
		end=$$(grep -n '^## v' CHANGELOG.md | head -2 | tail -1 | cut -d: -f1) && \
		sed -n "$$(( start + 1 )),$$(( end - 1 ))p" CHANGELOG.md > release-bits/changelog.md

# Tag, build and release (to GitHub releases) a new release based on the
# current ecslog.Version.
.PHONY: cutarelease
cutarelease: check-version release-bits
	[[ -z `git status --short` ]]  # If this fails, the working dir is dirty.
	@which gh >/dev/null || (echo "cutarelease: error: missing 'gh'" && exit 1)
	@ver=$$(go run ./cmd/ecslog --version | head -1 | cut -d' ' -f2) && \
		name=$$(grep ^module go.mod | cut -d' ' -f2) && \
		releasedVerInfo=$$(gh release view v$$ver >/dev/null 2>/dev/null || true) && \
		if [[ -n "$$releasedVerInfo" ]]; then \
			echo "cutarelease: error: v$$ver is already released to GitHub"; \
			exit 1; \
		fi && \
		echo "** Sure you want to tag and release $$name@$$ver to GitHub?" && \
		echo "** Enter to continue, Ctrl+C to abort." && \
		read
	@ver=$$(go run ./cmd/ecslog --version | head -1 | cut -d' ' -f2) && \
		date=$(shell date -u "+%Y-%m-%d") && \
		git tag -a "v$$ver" -m "version $$ver ($$date)" && \
		git push origin "v$$ver" && \
		gh release create "v$$ver" \
			$(shell ls release-bits/ecslog-*) \
			-t "v$$ver" -F release-bits/changelog.md
