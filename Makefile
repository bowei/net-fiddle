.PHONY: build test typecheck dev clean

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

clean:
	rm -rf dist node_modules
