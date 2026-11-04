SHELL := /bin/bash
SVC ?=
PHASE ?=
.PHONY: help setup deps contracts up down logs ps seed smoke test typecheck package packs data-dictionary clean kyc-test kyc-record integration web web-e2e mobile-e2e

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

KYC_TEST_RUN = docker build -q -t udyam-kyc-test -f go/Dockerfile.kyctest go >/dev/null && 	MSYS_NO_PATHCONV=1 docker run --rm -v "$(CURDIR):/repo" -v udyam-gomod:/go/pkg/mod -w /repo/go -e KYC_OCR=tesseract

kyc-test: ## KYC 12-applicant table against real tesseract (Docker); plain `make test` replays recorded OCR
	$(KYC_TEST_RUN) udyam-kyc-test go test ./pkg/kyc -count=1 -run 'Specimen|Checks' -v

kyc-record: ## re-record tesseract output for the KYC replay tests (after changing image preprocessing)
	rm -f go/pkg/kyc/testdata/ocr/*.txt
	$(KYC_TEST_RUN) -e KYC_RECORD=1 udyam-kyc-test go test ./pkg/kyc -count=1 -run Specimen

integration: ## lead -> quote -> KYC -> activation on the live stack, with services and Kafka stopped mid-flow (needs make up seed)
	cd go && go test -tags integration ./integration -count=1 -v -timeout 15m

web: ## build and serve the web app on :3000 (production build)
	cd ts/apps/web && npx next build && npx next start -p 3000

web-e2e: ## demo path in a real browser (Edge/Chrome), phone + desktop; screenshots to docs/screens (needs make up seed, LLM key, make web)
	cd ts/apps/web && node scripts/e2e.mjs

mobile-e2e: ## the Expo app's journey on its web build at phone size; screenshots to docs/screens
	cd ts/apps/mobile && npx expo export --platform web --output-dir dist-web
	cd ts/apps/web && MOBILE_DIR=../mobile/dist-web node scripts/e2e-mobile.mjs

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
