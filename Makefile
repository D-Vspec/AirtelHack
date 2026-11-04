SHELL := /bin/bash
SVC ?=
PHASE ?=
.PHONY: help setup deps contracts up down logs ps seed smoke test typecheck package packs data-dictionary clean

help: ## list targets
	@grep -E '^[a-z-]+:.*##' Makefile | sed 's/:.*##/\t/' | column -t -s $$'\t'

setup: ## one-time: create .env and install Go + TS dependencies
	test -f .env || cp .env.example .env
	$(MAKE) deps

deps: ## go mod tidy (creates go.sum) and pnpm install
	cd go && go mod tidy
	cd ts && pnpm install

contracts: ## regenerate Go/TS constants and Kafka topic list from /contracts
	node scripts/gen-contracts.mjs
	-gofmt -w go/pkg/events

up: ## build and start the whole stack
	test -f .env || cp .env.example .env
	docker compose up -d --build
	@echo "gateway http://localhost:8080  kafka-ui http://localhost:8089  advisor http://localhost:8090"
	@echo "web: make -C . web   (or: cd ts && pnpm dev:web)"

down: ## stop the stack (add -v via 'docker compose down -v' to wipe data)
	docker compose down

logs: ## follow logs; SVC=notify to filter
	docker compose logs -f --tail=100 $(SVC)

ps: ## container status
	docker compose ps

seed: ## load data/*.csv into Postgres (idempotent), then write docs/analytics.md
	cd go && go run ./cmd/seed -data ../data && go run ./cmd/analytics -out ../docs/analytics.md

smoke: ## end-to-end Phase 0 check (health + Kafka round trip)
	./scripts/smoke.sh

test: ## Go and TS tests
	cd go && go test ./...
	cd ts && pnpm test

typecheck: ## TypeScript typecheck for every package
	cd ts && pnpm typecheck

package: ## zip the current repo state as this phase's output: make package PHASE=2
	@test -n "$(PHASE)" || (echo "usage: make package PHASE=<n>"; exit 1)
	./scripts/package-phase.sh $(PHASE)

packs: ## rebuild the prompt-pack zips for phases 1-8 into dist/
	python3 scripts/build_phase_packs.py

data-dictionary: ## regenerate docs/data-dictionary.md
	python3 scripts/gen_data_dictionary.py

clean: ## remove build output
	rm -rf dist ts/apps/web/.next
