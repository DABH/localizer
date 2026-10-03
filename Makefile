GO ?= go

.PHONY: all test race vet fmt fmt-check fuzz bench python site site-dev

all: fmt-check vet test python

test:
	$(GO) test ./...

race:
	$(GO) test -race ./...

vet:
	$(GO) vet ./...
	GOOS=windows $(GO) vet ./internal/locale/
	GOOS=linux $(GO) vet ./internal/locale/

fmt:
	gofmt -w $$(git ls-files '*.go')

fmt-check:
	@test -z "$$(gofmt -l $$(git ls-files '*.go'))" || (echo "gofmt needed:"; gofmt -l $$(git ls-files '*.go'); exit 1)

fuzz:
	$(GO) test ./msgfmt/ -run '^$$' -fuzz FuzzReverseMatch -fuzztime 30s
	$(GO) test ./internal/locale/ -run '^$$' -fuzz FuzzAppleLanguages -fuzztime 30s
	$(GO) test ./catalog/ -run '^$$' -fuzz FuzzParseFast -fuzztime 30s

bench:
	$(GO) test -run '^$$' -bench . -benchmem . ./engine/

# The Python runtime (python/, published on PyPI as "localizer"). Needs uv.
python:
	cd python && uv run --with-editable . --with pytest --with typer --with click pytest -q

# The documentation site (site/, Astro Starlight; published at https://locale.dev). `make site-dev` serves it at
# http://localhost:4321/.
site:
	cd site && npm ci && npm run build

site-dev:
	cd site && npm ci && npm run dev
