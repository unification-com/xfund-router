#!/usr/bin/env bash
#
# End-to-end integration harness: run go-ooo against the local dev-env (anvil +
# Router + a registered provider) and assert it fulfils a request on-chain. This
# exercises the real keystore signing, nonce management, job state machine and
# graceful shutdown paths.
#
# Prereq: the dev-env image must be built once -> `make dev-env` (or
# `docker build -t ooo_dev_env -f docker/dev.Dockerfile .` from the repo root).
# The harness starts/stops a dev-env container itself if the chain isn't already up.
#
# Usage:   go-ooo/scripts/integration/run.sh
#          KEEP=1 go-ooo/scripts/integration/run.sh        # leave env up for debugging
#          REQUEST="BONE WETH AD" go-ooo/scripts/integration/run.sh
#
set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
GO_OOO_DIR="$(cd "$SCRIPT_DIR/../.." && pwd)"

# deterministic account #3 (from the shared dev mnemonic) = the provider registered by init-dev-env.js
DEV_KEY="${DEV_KEY:-0x646f1ce2fdad0e6deeeb5c7e8e5543bdde65e86029e2fd9fc169899c440a7913}"
PASSPHRASE="${PASSPHRASE:-integration-test-pass}"
RPC_HOST="${RPC_HOST:-127.0.0.1}"
RPC_PORT="${RPC_PORT:-8545}"
CONTAINER="${CONTAINER:-ooo_dev_env}"
REQUEST="${REQUEST:-BTC GBP PR AVC 1H}"
FULFIL_TIMEOUT="${FULFIL_TIMEOUT:-180}"
KEEP="${KEEP:-0}"

HOME_DIR="$(mktemp -d)"
GO_OOO_BIN="$GO_OOO_DIR/build/go-ooo"
GO_OOO_LOG="$HOME_DIR/go-ooo.log"
PASS_FILE="$HOME_DIR/pass.txt"
GO_OOO_PID=""
STARTED_DEVENV=0

log()  { echo "[integration] $*"; }
fail() { echo "[integration] FAIL: $*" >&2; exit 1; }

cleanup() {
  if [ -n "$GO_OOO_PID" ] && kill -0 "$GO_OOO_PID" 2>/dev/null; then
    kill -INT "$GO_OOO_PID" 2>/dev/null
    for _ in $(seq 1 20); do kill -0 "$GO_OOO_PID" 2>/dev/null || break; sleep 0.5; done
    kill -9 "$GO_OOO_PID" 2>/dev/null || true
  fi
  if [ "$STARTED_DEVENV" = "1" ]; then
    log "removing dev-env container"
    docker rm -f "$CONTAINER" >/dev/null 2>&1 || true
  fi
  rm -rf "$HOME_DIR"
}
[ "$KEEP" = "1" ] || trap cleanup EXIT

chain_up() { (exec 3<>"/dev/tcp/$RPC_HOST/$RPC_PORT") 2>/dev/null; }

# 1. dev-env -----------------------------------------------------------------
if chain_up; then
  log "dev chain already up on $RPC_HOST:$RPC_PORT - using it"
else
  command -v docker >/dev/null || fail "docker not found and the dev chain is not running"
  docker image inspect ooo_dev_env >/dev/null 2>&1 || fail "image 'ooo_dev_env' not built - run 'make dev-env' image build first"
  log "starting dev-env detached..."
  docker rm -f "$CONTAINER" >/dev/null 2>&1 || true
  docker run -d --name "$CONTAINER" -p "$RPC_PORT:8545" ooo_dev_env >/dev/null || fail "could not start dev-env"
  STARTED_DEVENV=1
  log "waiting for contracts to deploy + provider to register (up to ~4 min)..."
  for _ in $(seq 1 120); do
    docker logs "$CONTAINER" 2>&1 | grep -q "^done$" && break
    docker inspect -f '{{.State.Running}}' "$CONTAINER" 2>/dev/null | grep -q true || fail "dev-env container exited; see: docker logs $CONTAINER"
    sleep 2
  done
  docker logs "$CONTAINER" 2>&1 | grep -q "^done$" || fail "dev-env did not finish init in time"
  log "dev-env ready"
fi

# 2. build + non-interactive init -------------------------------------------
log "building go-ooo..."
( cd "$GO_OOO_DIR" && go build -o "$GO_OOO_BIN" . ) || fail "go-ooo build failed"

printf '%s' "$PASSPHRASE" > "$PASS_FILE"
log "init go-ooo (dev) in $HOME_DIR"
"$GO_OOO_BIN" init dev --home "$HOME_DIR" --account oracle \
  --import-key "$DEV_KEY" --pass "$PASS_FILE" < /dev/null > "$HOME_DIR/init.log" 2>&1 \
  || { cat "$HOME_DIR/init.log"; fail "go-ooo init failed"; }
log "wallet: $(grep -o '0x[0-9a-fA-F]\{40\}' "$HOME_DIR/init.log" | head -1)"

# 3. start go-ooo ------------------------------------------------------------
log "starting go-ooo..."
"$GO_OOO_BIN" start --home "$HOME_DIR" --pass "$PASS_FILE" < /dev/null > "$GO_OOO_LOG" 2>&1 &
GO_OOO_PID=$!
for _ in $(seq 1 60); do
  grep -qE "initialise event subscriptions|set initial query from block" "$GO_OOO_LOG" 2>/dev/null && break
  kill -0 "$GO_OOO_PID" 2>/dev/null || { cat "$GO_OOO_LOG"; fail "go-ooo exited during startup"; }
  sleep 1
done
grep -q "keystore unlocked" "$GO_OOO_LOG" && log "PASS: keystore unlocked (v3 signing path live)"
log "go-ooo running (pid $GO_OOO_PID); log: $GO_OOO_LOG"

# 4. issue a request ---------------------------------------------------------
log "issuing request: $REQUEST"
# shellcheck disable=SC2086
docker exec "$CONTAINER" /root/xfund-router/request.sh $REQUEST > "$HOME_DIR/request.log" 2>&1 &
REQ_PID=$!

# 5. wait for on-chain fulfilment -------------------------------------------
log "waiting up to ${FULFIL_TIMEOUT}s for on-chain fulfilment..."
fulfilled=0
deadline=$(( $(date +%s) + FULFIL_TIMEOUT ))
while [ "$(date +%s)" -lt "$deadline" ]; do
  if grep -qiE "status: fulfilled|price after" "$HOME_DIR/request.log" 2>/dev/null; then fulfilled=1; break; fi
  sleep 2
done
kill "$REQ_PID" 2>/dev/null || true

# 6. report ------------------------------------------------------------------
echo ""
log "=== go-ooo log (key lines) ==="
grep -aE "keystore unlocked|DataRequested|begin fetching data|run api query|price fetched|fulfill tx sent|give up|context cancelled" "$GO_OOO_LOG" | tail -25 || true
echo ""

if [ "$fulfilled" = "1" ]; then
  log "PASS: request fulfilled on-chain (keystore signing + nonce + state machine validated)"
else
  log "request NOT confirmed fulfilled within ${FULFIL_TIMEOUT}s"
  log "  - inspect $GO_OOO_LOG and $HOME_DIR/request.log"
  log "  - fulfilment needs the external data source reachable (Finchains API for *.PR.*, subgraphs for *.AD)"
  log "  - if go-ooo never logged 'DataRequested', check the dev Router address in config matches the deploy"
fi

# 7. graceful shutdown (#5) --------------------------------------------------
log "sending SIGINT to go-ooo (graceful-shutdown check)..."
kill -INT "$GO_OOO_PID" 2>/dev/null
graceful=0
for _ in $(seq 1 20); do kill -0 "$GO_OOO_PID" 2>/dev/null || { graceful=1; break; }; sleep 0.5; done
GO_OOO_PID=""
if [ "$graceful" = "1" ] && grep -q "oracle daemon stopped" "$GO_OOO_LOG"; then
  log "PASS: graceful shutdown (clean exit + 'oracle daemon stopped')"
else
  log "WARN: go-ooo did not shut down cleanly within 10s"
fi

[ "$KEEP" = "1" ] && log "KEEP=1: dev-env + temp home ($HOME_DIR) left in place"
[ "$fulfilled" = "1" ] || exit 1
log "integration run OK"
