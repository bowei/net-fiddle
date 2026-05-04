.PHONY: build test typecheck dev format format-check clean publish \
        scanner-build scanner-test testdaemon-build

build: node_modules
	npm run build

test: node_modules
	npm run test

typecheck: node_modules
	npm run typecheck

dev: node_modules
	npm run dev

node_modules: package.json
	npm install
	@touch node_modules

format: node_modules
	npm run format

format-check: node_modules
	npm run format:check

scanner-build:
	cd scanner && CGO_ENABLED=0 go build -o net-fiddle-scan ./cmd/net-fiddle-scan
	cd scanner && CGO_ENABLED=0 go test -c .

scanner-test:
	cd scanner && go test ./...

testdaemon-build:
	cd scanner && CGO_ENABLED=0 go build -o testdaemon ./cmd/testdaemon

publish: build
	cp -r dist/. docs/

clean:
	rm -rf dist node_modules
	rm -f scanner/net-fiddle-scan scanner/testdaemon
