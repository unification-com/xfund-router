# Docker Dev Environment

A full Docker environment for testing the OoO smart contracts and `go-ooo`.

This is a complete, self-contained `ganache-cli` development environment, which can be used
to test contracts, developing and testing Consumer contracts and testing the `go-ooo` Provider Oracle app.

## Running

To run the development environment, from the **repo root directory**, run

```bash
make dev-env
```

This will run a local `ganache-cli` chain, deploy the Router and demo consumer contract, and also initialise
the accounts with some Dev xFUND. The chain's RPC endpoint will be exposed on `http://127.0.0.1:8545`, and can be
accessed via the `truffle-config`'s `develop` network.

The image (`docker/dev.Dockerfile`) is a multi-stage build on the official `node:12.18.3` image
(no apt step, so it doesn't depend on an EOL distro's package mirrors), with the deps and the
contract compile in cached layers and the chain/deploy/seed logic in
[`assets/entrypoint.sh`](assets/entrypoint.sh). Node 12 + truffle 5 + solc 0.8.3 are pinned by the
smart-contract toolchain. The first build downloads the solc compiler (needs network).

> For an automated end-to-end run of `go-ooo` against this dev-env (build → init → fulfil → graceful
> shutdown), see the integration harness in [`go-ooo/scripts/integration`](../go-ooo/scripts/integration).

## Interaction

**Note**: Both this Docker environment and [`go-ooo`](../go-ooo/README.md) must be running (on this `dev` network) in 
order to test data requests.

#### Requesting Data

The development environment is deployed with a pre-configured and deployed demo Consumer contract, along with a script
for requesting and waiting for data. This script can be called using `docker exec`:

```bash
docker exec -it ooo_dev_env /root/xfund-router/request.sh <BASE> <TARGET> <TYPE> [SUBTYPE] [SUPP1] [SUPP2]
```

For example querying Finchains tracked data:

```bash
docker exec -it ooo_dev_env /root/xfund-router/request.sh BTC GBP PR AVC 1H
```

This will request data using the OoO endpoint `BTC.GBP.PR.AVC.1H`, which is the mean GBP price of Bitcoin for the
past hour, using the Chauvenet Criterion to remove statistical outliers.

Or and Ad-Hoc request:

```bash
docker exec -it ooo_dev_env /root/xfund-router/request.sh BONE WETH AD
```

See the [OoO API Guide](https://docs.unification.io/ooo/guide/ooo_api.html) for more information on endpoint 
construction.
