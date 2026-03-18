GO ?= go
HUB_REPO ?=
IMAGE ?= ghcr.io/gitopshq-io/agent
VERSION ?= dev
OCI_CHART_REPO ?= oci://ghcr.io/gitopshq-io/charts

.PHONY: test run lint chart-template chart-package chart-push docker-build sync-proto

test:
	GOCACHE=$${GOCACHE:-/tmp/gitopshq-agent-gocache} GOPROXY=$${GOPROXY:-off} $(GO) test ./...

lint:
	buf lint

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
