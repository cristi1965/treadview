WAILS ?= $(shell command -v wails 2>/dev/null || printf "%s/go/bin/wails" "$$HOME")

.PHONY: test verify build-backend build-frontend build-desktop-assets build-desktop dev-desktop build check ci-smoke

test:
	cd app/backend && go test ./...

verify:
	./scripts/verify-local.sh

build-backend:
	cd app/backend && go build -o /tmp/tradingagents-backend .

build-frontend:
	cd app/frontend && npm run build

build-desktop-assets:
	cd app/frontend && ./scripts/build-wails.sh

build-desktop: build-desktop-assets
	test -x "$(WAILS)" || (echo "Wails CLI missing. Install: go install github.com/wailsapp/wails/v2/cmd/wails@latest" && exit 1)
	cd app/backend/cmd/desktop && "$(WAILS)" build

dev-desktop:
	test -x "$(WAILS)" || (echo "Wails CLI missing. Install: go install github.com/wailsapp/wails/v2/cmd/wails@latest" && exit 1)
	cd app/backend/cmd/desktop && "$(WAILS)" dev

build: build-frontend build-backend

check: test build-frontend
	@echo "Static checks OK. Start backend then run: make verify"

ci-smoke: test build
	@echo "Backend binary: /tmp/tradingagents-backend"
