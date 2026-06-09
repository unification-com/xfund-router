#!/usr/bin/env bash
#
# Dev-env entrypoint: start a deterministic ganache chain, deploy the contracts, seed
# accounts and register the provider oracle, then keep the container alive on ganache.
#
set -e
cd /root/xfund-router

# Deterministic accounts (#3 is the provider), dev network id, 5s blocks.
npx ganache-cli --deterministic --networkId 696969 --accounts 20 -h 0.0.0.0 --blockTime 5 &
GANACHE_PID=$!

# Wait for the RPC (bash /dev/tcp - no netcat dependency).
echo "waiting for ganache..."
until (echo > /dev/tcp/127.0.0.1/8545) 2>/dev/null; do sleep 0.5; done

echo "deploying contracts..."
npx truffle deploy --network=develop

echo "seeding accounts + registering provider..."
npx truffle exec init-dev-env.js --network=develop
# init-dev-env.js prints "done" on success; the integration harness waits for it.

# Hand the container's lifetime to ganache.
wait "$GANACHE_PID"
