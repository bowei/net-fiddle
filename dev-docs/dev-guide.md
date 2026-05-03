# Developer Guide

## Prerequisites

Node.js (any recent LTS) and npm. Run `npm install` once to install dependencies; the Makefile does this automatically as a prerequisite.

Go 1.22 or later is required for the scanner (`scanner/`). No separate install step needed — `go build` fetches nothing; the module has no external dependencies.

---

## Common commands

All commands are available via `make` or directly via `npm run`.

| Task | Make | npm |
|---|---|---|
| Start dev server | `make dev` | `npm run dev` |
| Production build | `make build` | `npm run build` |
| Run tests (once) | `make test` | `npm run test` |
| Watch tests | — | `npm run test:watch` |
| Type-check only | `make typecheck` | `npm run typecheck` |
| Format source | `make format` | `npm run format` |
| Check formatting | `make format-check` | `npm run format:check` |
| Delete dist + node_modules | `make clean` | — |

The dev server runs at **http://localhost:5173** with hot-module replacement.

### Running a single test file

```bash
npx vitest run src/test/registry.test.ts
```

### Running tests matching a name pattern

```bash
npx vitest run --reporter=verbose -t "REGISTRY"
```

---

## Build pipeline

`make build` runs two steps in sequence:

1. **`tsc`** — full type-check with `tsconfig.json` (strict mode, `noUnusedLocals`, `noUnusedParameters`). Fails the build on any type error.
2. **`vite build`** — bundles the app into `dist/`.

Type-check and bundle are separate so CI can fail fast on type errors before invoking Vite.

---

## Type checking

TypeScript is configured in strict mode. Key settings that catch common mistakes:

- `strict: true` — enables all strict checks (nullability, implicit any, etc.)
- `noUnusedLocals` / `noUnusedParameters` — unused variables are errors, not warnings
- `noFallthroughCasesInSwitch` — switch fall-through is an error

Run `make typecheck` (or `npx tsc --noEmit`) at any point without triggering a full bundle. This is faster than `make build` when you only want to verify types.

---

## Scanner tests

The scanner has integration tests in `scanner/scan_test.go` that build real network topologies and assert on the collected structs. They require **root** and the **test daemon** to be running.

### Starting the test daemon

```bash
cd scanner
sudo go run ./cmd/testdaemon
```

The daemon listens on `http://localhost:7777` by default. Override with `NETFIDDLE_DAEMON_ADDR`.

### Running the scanner tests

In a second terminal (also as root, from `scanner/`):

```bash
sudo go test -v -count=1 .
```

Run a single test by name:

```bash
sudo go test -v -count=1 -run Test_VETHCROSS01 .
```

Run a section by prefix:

```bash
sudo go test -v -count=1 -run Test_NFT .
```

Tests that require clang (BPF stubs) skip automatically if clang is not installed. Tests that require netkit skip if the running kernel does not support it.

### Test structure

Each test calls `requireDaemon(t)`, which skips if the daemon is unreachable and registers `client.Reset()` as cleanup. Tests then call `doScan(t)` which runs `EnumerateNamespaces → CollectAll → linker.Link → builder.Build` and returns a `scanResult` with all three outputs.

Helper shorthand mirrors the spec in `doc/scanner-test-cases.md`:

| Helper | Returns |
|---|---|
| `snap(t, sr, name)` | `*NsSnapshot` for the named namespace |
| `iface(t, snap, name)` | `*InterfaceInfo` with `IfName == name` |
| `nodeOf(t, sr, id)` | `topology.Node` with `ID == id` |
| `edgeOf(t, sr, src, tgt)` | `topology.Edge` with matching source/target |
| `pairOf(t, sr, ns, iface)` | `linker.PairInfo` for `(ns.Inode, iface.IfIndex)` |

---

## Frontend tests

Tests live in `src/test/` and use **Vitest** with a `jsdom` environment.

Currently two frontend test files:

- `src/test/anchorUtils.test.ts` — unit tests for `anchorUtils.ts` (handle ID generation, flow colors, style calculations, `resolveHandles`)
- `src/test/registry.test.ts` — integration tests for the component registry: field validation, anchor structure, `REGISTRY` lookup, `SIDEBAR_ITEMS` consistency, connector validation

Tests run in `jsdom` (configured in `vite.config.ts`) so React component code can be imported without a browser. No snapshot tests; all assertions are explicit `expect()` calls.

When adding a new component type, update `registry.test.ts`:

1. Import the new singleton at the top
2. Add it to `ALL_DEFS`
3. If it should appear in the sidebar, ensure it is NOT in `REGISTRY_ONLY`; if it is canvas-only (auto-generated, like veth-end), add it to `REGISTRY_ONLY`

---

## Formatting and style

**Prettier** is configured (`.prettierrc`). Settings: single quotes, semicolons, 2-space indent, 100-char print width, trailing commas in ES5 positions.

Run `make format` to auto-format all source files in `src/`. Run `make format-check` (or in CI: `npm run format:check`) to verify without writing.

Style conventions enforced by TypeScript (not Prettier):

- `as const` on literal tuple anchors to preserve narrowed types
- Component singletons exported as `const name = new NameClass()` at the bottom of each file

When TypeScript emits `noUnusedLocals` or `noUnusedParameters` errors, fix the code rather than suppressing with `// @ts-ignore` or `_` prefixes.

---

## Adding a new component type

1. Create `src/components/myType.ts` — extend `ComponentDef` (or `ContainerComponentDef`), export a singleton `export const myType = new MyTypeComponent()`
2. Import the singleton in `src/components/registry.ts` and add it to the appropriate `SIDEBAR_GROUPS` entry
3. If the component is canvas-only (never appears in the sidebar directly), also add it to the `REGISTRY` map manually alongside `vethEnd`, `netkitPrimary`, and `netkitPeer`
4. Add any Linux ordering position to `src/rules/linuxOrder.ts`
5. Add any placement constraints as a new rule file in `src/rules/` and register it in `src/rules/index.ts`
6. Update `src/test/registry.test.ts` (see Tests section above)

---

## Adding a new validation rule

1. Create `src/rules/myRule.ts` implementing the `GraphRule` interface from `./types`
2. Export the rule constant and add it to `ALL_RULES` in `src/rules/index.ts`

Rules receive the full `nodes` and `edges` arrays and return `GraphViolation[]`. Violations with an `edgeId` cause that edge to turn red and show a hover tooltip. The rule runs on every graph change via `useMemo` in `App.tsx`.
