# syntax=docker/dockerfile:1
#
# Local dev-env: a deterministic ganache chain with the Router + demo consumer deployed
# and the provider oracle (account #3) registered. Used by the smart-contract tests and
# the go-ooo integration harness (go-ooo/scripts/integration).
#
# The smart-contract toolchain (truffle 5 / ganache-cli 6 / solc 0.8.3) is pinned to
# Node 12 - bumping Node needs a migration off truffle, out of scope here.
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

# --- build: compile the contracts (downloads the pinned solc once) ----------
FROM deps AS build
COPY smart-contracts/contracts ./contracts/
COPY smart-contracts/migrations ./migrations/
RUN npx truffle compile

# --- runtime: the dev chain + deploy/seed scripts ---------------------------
FROM build AS runtime
COPY docker/assets/init-dev-env.js docker/assets/request-data.js \
     docker/assets/request.sh docker/assets/entrypoint.sh ./
RUN chmod +x request.sh entrypoint.sh
EXPOSE 8545
ENTRYPOINT ["./entrypoint.sh"]
