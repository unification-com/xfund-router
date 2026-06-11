# syntax=docker/dockerfile:1
#
# Local dev-env: a deterministic anvil chain with the Router + demo consumer deployed
# and the provider oracle (account #3) registered. Used by the smart-contract tests and
# the go-ooo integration harness (go-ooo/scripts/integration).
#
# The chain is Foundry's anvil - a modern London+ EVM (EIP-1559 capable), replacing the
# pre-London ganache-cli. The smart-contract TOOLCHAIN (truffle 5 / solc 0.8.3) stays pinned
# to Node 12 - it only compiles + deploys, and anvil is an independent static binary, so the
# two never conflict. (ganache-cli remains a devDependency for the truffle unit tests.)
#
# Build (from the repo root):  docker build -t ooo_dev_env -f docker/dev.Dockerfile .

# --- deps: install node deps + compile native modules -----------------------
# The full node:12.18.3 image bundles the build toolchain (gcc/g++/make/python/git)
# the native node modules need, so there is no apt step and no reliance on an EOL
# distro's package mirrors (the reason the old ubuntu:bionic image is fragile).
FROM node:12.18.3 AS deps
WORKDIR /root/xfund-router
# Only the files needed to resolve deps, so this layer caches across source changes.
COPY smart-contracts/package.json smart-contracts/yarn.lock smart-contracts/truffle-config.js ./
RUN yarn install --frozen-lockfile

# --- build: compile the contracts -------------------------------------------
FROM deps AS build
# truffle 5.3 resolves solc by fetching from the now-defunct solc-bin.ethereum.org host, which
# breaks a clean build. Pre-seed the pinned compiler into truffle's cache from the live
# binaries.soliditylang.org, so `truffle compile` finds it locally and needs no fetch.
RUN mkdir -p /root/.config/truffle/compilers/node_modules \
 && curl -fsSL https://binaries.soliditylang.org/bin/soljson-v0.8.3+commit.8d00100c.js \
      -o /root/.config/truffle/compilers/node_modules/soljson-v0.8.3+commit.8d00100c.js
COPY smart-contracts/contracts ./contracts/
COPY smart-contracts/migrations ./migrations/
RUN npx truffle compile

# --- runtime: the dev chain + deploy/seed scripts ---------------------------
FROM build AS runtime
# anvil is the local dev chain. The static (musl) build runs on this Debian base regardless of
# its glibc; the `anvil --version` check fails the build loudly if the binary can't run. Tracks
# Foundry's vetted `stable` channel (matching the existing devDependency caret-range pinning).
RUN curl -fsSL https://github.com/foundry-rs/foundry/releases/download/stable/foundry_stable_alpine_amd64.tar.gz \
      | tar -xz -C /usr/local/bin anvil \
 && anvil --version
COPY docker/assets/init-dev-env.js docker/assets/request-data.js \
     docker/assets/request.sh docker/assets/entrypoint.sh ./
RUN chmod +x request.sh entrypoint.sh
EXPOSE 8545
ENTRYPOINT ["./entrypoint.sh"]
