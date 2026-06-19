#!/usr/bin/env bash
#
# Dev-env entrypoint: start a deterministic anvil chain, deploy the contracts, seed
# accounts and register the provider oracle, then keep the container alive on anvil.
#
set -e
cd /root/xfund-router

# Deterministic accounts (#3 is the provider) from the same mnemonic ganache-cli used with
# --deterministic, so account #3 stays the registered provider key. anvil is a modern London+
# EVM (EIP-1559 capable), replacing the pre-London ganache-cli. --chain-id 696969 matches the
# go-ooo dev config's network_id (used for EIP-155 signing); --block-time 5 keeps blocks
# ticking while idle, which go-ooo's confirmation logic relies on.
anvil --mnemonic "myth like bonus scare over problem client lizard pioneer submit female collect" \
  --accounts 20 --host 0.0.0.0 --port 8545 --chain-id 696969 --block-time 5 &
ANVIL_PID=$!

# Wait for the RPC (bash /dev/tcp - no netcat dependency).
echo "waiting for anvil..."
until (echo > /dev/tcp/127.0.0.1/8545) 2>/dev/null; do sleep 0.5; done

echo "deploying contracts..."
npx truffle deploy --network=develop

echo "seeding accounts + registering provider..."
npx truffle exec init-dev-env.js --network=develop
# init-dev-env.js prints "done" on success; the integration harness waits for it.

# Hand the container's lifetime to anvil.
wait "$ANVIL_PID"
