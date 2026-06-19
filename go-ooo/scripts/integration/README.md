# go-ooo integration harness

End-to-end validation of `go-ooo` against the local dev-env (anvil + Router +
a registered provider). It exercises the real paths the unit tests can't:

- **keystore signing** — a v3-keystore-signed fulfilment the Router accepts;
- **nonce management** — chain-anchored per-send nonce;
- **job state machine** — `INITIALISED → FETCHING_DATA → … → SUCCESS`;
- **graceful shutdown** — `SIGINT` → clean exit.

## Prerequisites

- Docker, Go, and a one-time dev-env image build (from the repo root):

      docker build -t ooo_dev_env -f docker/dev.Dockerfile .

- Network access to the data source the request needs (the Finchains API for
  `*.PR.*` endpoints, DEX subgraphs for `*.AD`) — fulfilment fetches real data.

## Run

From `go-ooo/`:

    make integration

or directly:

    scripts/integration/run.sh

The harness starts a dev-env container if the chain isn't already on `:8545`, builds
go-ooo, inits a throwaway home with the dev provider key (non-interactively), starts
go-ooo, issues a request, waits for the on-chain fulfilment, then `SIGINT`s go-ooo and
checks it stopped cleanly. It tears everything down on exit and exits non-zero if the
request was not fulfilled.

## Options (env vars)

| Var | Default | Purpose |
|-----|---------|---------|
| `REQUEST` | `BTC GBP PR AVC 1H` | the request to issue (`request.sh` args) |
| `FULFIL_TIMEOUT` | `180` | seconds to wait for fulfilment |
| `KEEP` | `0` | `1` = leave the dev-env + temp home up for debugging |
| `PASSPHRASE` | `integration-test-pass` | keystore passphrase |

## Troubleshooting

- **go-ooo never logs `DataRequested`:** the dev Router address in go-ooo's config
  must match the deployed address — check `InitForDevNet` in `config/config.go` against
  the truffle deploy.
- **fulfilment times out but the event was seen:** the external data source was
  unreachable (look for an `run api query` error in the go-ooo log) — not a go-ooo bug.
- Inspect with `KEEP=1`, then read the printed `go-ooo.log` / `request.log` paths.
