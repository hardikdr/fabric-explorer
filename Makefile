.PHONY: build test run web-dev e2e
build:
	cd web && npm ci && npm run build
	go build -o bin/fabric-explorer ./cmd/fabric-explorer
test:
	go test -race ./...
	cd web && npm ci && npm test
run:
	./bin/fabric-explorer --mode fixture --listen 127.0.0.1:8099
web-dev:
	cd web && npm run dev

e2e: build
	cd web && npx playwright install chromium && npm run test:e2e
