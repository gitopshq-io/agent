GO ?= go
GOVULNCHECK_VERSION ?= v1.1.4
HUB_REPO ?=
IMAGE ?= ghcr.io/gitopshq-io/agent
VERSION ?= dev
OCI_CHART_REPO ?= oci://ghcr.io/gitopshq-io/charts

.PHONY: test run lint fmt-check vet proto-lint vulncheck chart-template chart-package chart-push docker-build sync-proto

test:
	GOCACHE=$${GOCACHE:-/tmp/gitopshq-agent-gocache} GOPROXY=$${GOPROXY:-off} $(GO) test ./...

lint: fmt-check vet proto-lint vulncheck

fmt-check:
	@files="$$(gofmt -l $$(git ls-files '*.go'))"; \
	if [ -n "$$files" ]; then \
		echo "$$files"; \
		exit 1; \
	fi

vet:
	$(GO) vet ./...

proto-lint:
	buf lint

vulncheck:
	$(GO) run golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION) ./...

chart-template:
	helm template gitopshq-agent ./charts/gitopshq-agent >/tmp/gitopshq-agent-chart.yaml

chart-package:
	mkdir -p dist
	helm package ./charts/gitopshq-agent --destination dist

chart-push: chart-package
	helm push $$(ls dist/gitopshq-agent-*.tgz | tail -n 1) $(OCI_CHART_REPO)

docker-build:
	docker build -t $(IMAGE):$(VERSION) .

sync-proto:
	./scripts/sync-proto.sh $(HUB_REPO)

run:
	$(GO) run ./cmd/gitopshq-agent
