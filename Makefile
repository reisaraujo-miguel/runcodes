# Developer entry points for the RunCodes stack.
#
# The observability overlay is opt-in. It starts the existing application stack
# plus Prometheus, Loki, Tempo, Grafana, Alertmanager, an OpenTelemetry Collector
# and the infrastructure exporters. See docs-site/docs/observability/local.md.

SHELL := /bin/bash
.DEFAULT_GOAL := help

# Podman is the default Compose engine. Pass CONTAINER_ENGINE=docker or
# COMPOSE="docker compose" to use Docker Compose v2 with the observability
# targets. The helper scripts read CONTAINER_ENGINE, so keep it in sync with
# COMPOSE and export it.
CONTAINER_ENGINE ?= podman

ifeq ($(origin COMPOSE),undefined)
  COMPOSE := $(CONTAINER_ENGINE) compose
else
  CONTAINER_ENGINE := $(firstword $(COMPOSE))
endif

export CONTAINER_ENGINE
OBS_COMPOSE = $(COMPOSE) -f docker-compose.yml -f docker-compose.observability.yml -f docker-compose.observability.build.yml

.PHONY: help rootfs-smoke observability-init observability-up observability-down observability-status observability-logs observability-smoke-test observability-browser-smoke-test observability-submission-smoke-test observability-validate observability-destroy

help:
	@echo "RunCodes developer targets:"
	@echo "  make observability-up          start app + observability stack (builds images)"
	@echo "  make observability-down        stop and remove the stack containers (keeps volumes)"
	@echo "  make observability-status      show container status"
	@echo "  make observability-logs        follow all stack logs"
	@echo "  make observability-smoke-test  generate traffic and verify metrics, logs, traces, alerts"
	@echo "  make observability-browser-smoke-test     verify a real browser trace in Tempo"
	@echo "  make observability-submission-smoke-test  authenticated submission + per-case span (needs SMOKE_ADMIN_PASSWORD)"
	@echo "  make observability-validate    validate configs (compose, promtool, amtool, collector)"
	@echo "  make observability-destroy     DESTRUCTIVE: remove observability volumes (prompts; FORCE=1 skips)"
	@echo "  make rootfs-smoke              compile/run a C case under the read-only runner rootfs (Podman host)"
	@echo ""
	@echo "  Podman is the default Compose engine; add CONTAINER_ENGINE=docker for Docker Compose v2."

# Real-container check of the runner's read-only rootfs and capped tmpfs.
rootfs-smoke:
	./judge/test/rootfs-smoke.sh

# Generate the local Grafana admin password (gitignored).
observability-init:
	./scripts/observability/init-secrets.sh

observability-up: observability-init
	$(OBS_COMPOSE) up -d --build

observability-down:
	$(OBS_COMPOSE) down

observability-status:
	$(OBS_COMPOSE) ps

observability-logs:
	$(OBS_COMPOSE) logs -f --tail=100

observability-smoke-test: observability-init
	./scripts/observability/smoke-test.sh

observability-browser-smoke-test:
	./scripts/observability/browser-smoke-test.sh

observability-submission-smoke-test:
	./scripts/observability/submission-smoke-test.sh

observability-validate: observability-init
	./scripts/observability/validate.sh

# Removes the observability volumes only (NOT the application database/object
# store volumes). Prompts unless FORCE=1 is set.
observability-destroy:
	FORCE=$(FORCE) ./scripts/observability/destroy.sh
