# go-ooo

The Go implementation of the OoO Provider application, required to be run by providers to serve data

## Query endpoint format

Data requests use a dot-separated endpoint string. The canonical, going-forward form is:

```
base.target[.minutes]
```

- `base` / `target` - the pair symbols. These are **case-sensitive** (`xFUND` is not the same
  as `XFUND`).
- `minutes` - an optional lookback window: an integer clamped to `0`-`60`. `0` (the
  default) fetches the latest prices; a non-zero value averages over the past `nn` minutes.

Every request is a DEX price query: the provider looks the pair up across the supported DEX
subgraphs and returns a robust, liquidity-weighted mean (per-pool outliers removed with a
median + MAD filter), scaled to `price * 10^18`.

```
WETH.USDC          latest mean WETH/USDC price
WETH.USDC.30       mean WETH/USDC price over the last 30 minutes
```

### Legacy forms (deprecated)

The explicit-qualifier forms `base.target.AD[.minutes]` (AdHoc) and
`base.target.PR.subtype[...]` (Finchains) are still accepted but **deprecated** - new consumers
should use the suffix-less form above. Any trailing fields an AdHoc query cannot honour (such
as leftover Finchains parameters) are ignored and the price is still served from the DEX mean.
Legacy-form usage is recorded in the structured logs and the `ooo_legacy_endpoint_total`
Prometheus counter; the legacy parsing will be removed once that counter stays at zero (queries
are one-shot, so removal cannot break an in-flight request).

See the published [OoO Data API guide](https://docs.unification.io/ooo/guide/ooo_api.html) for
the full specification.

## Development and Testing

### Prerequisites

#### Go

Go v1.18+ is required to compile the `go-ooo` application.

## testapp

`testapp` can be used to test DEX price queries. It is useful for quickly testing price data retrieval without needing
to deploy and run a full development network stack (Docker `devnet`, `go-ooo` application, running on-chain queries etc.)

1. Build the `testapp`

```bash
make build-testapp
```

2. Populate the temp database

```bash
./build/testapp api update-pairs --graphnetapi [GRAPHNET_API_KEY]
```

3. Run price queries (the canonical suffix-less form; the legacy `WETH.USDC.AD` is also accepted)

```bash
./build/testapp api price WETH.USDC --graphnetapi [GRAPHNET_API_KEY]
```

By default the `testapp` uses a throwaway sqlite file (`/tmp/go-ooo_testapp.sqlite`). It can
instead run against a Postgres database — for example a restored production dump, which gives a
realistic curated-pair set to query and exercises the schema-migration path on real data:

```bash
./build/testapp api price WETH.USDC \
  --db-dialect postgres --db-host /path/to/socket-or-host --db-port 5432 \
  --db-user [USER] --db-name [DB] --db-pass [PASS] \
  --graphnetapi [GRAPHNET_API_KEY]
```

## go-ooo

`go-ooo`, the core OoO service application receives and processes data requests. The application requires on-chain 
communication, and a registered key. With the Docker `devnet`, this is already configured and ready to test.

First, build the Go application:

```bash
make build
```

`go-ooo` will need initialising before it can run:

```bash
./build/go-ooo init <network>
```

Where `<network>` is one of `dev`, `sepolia`, `mainnet` or `polygon`. Using `dev` will configure `go-ooo` for the Docker
[development environment](../docker/README.md).

This will save the default configuration to `$HOME/.go-ooo`, with the initial values for the `dev` network.
This config location can be changed using the `--home` flag to specify a custom location, e.g.

```bash
./build/go-ooo init dev --home $HOME/.go-ooo_dev
```

This initialisation script will ask whether you want to import an exisitng private key, or generate a new one.
You can enter anything for the account name. For the purposes of quick testing, the Docker development environment
initialises by pre-registering account #3 on the `ganache-cli` chain as a Provider Oracle. The private key to import is:

`0x646f1ce2fdad0e6deeeb5c7e8e5543bdde65e86029e2fd9fc169899c440a7913`

Make a note of the generated decryption password - this will be required to run the application (to decrypt the keystore)
and to execute any admin commands. For the sake of simplicity, save it to `$HOME/.go-ooo/pass.txt`

The application should now have the default configuration saved to `$HOME/.go-ooo/config.toml`. It will use `sqlite` as
the default database, but can easily be configured for PostgreSQL.

#### Upgrading & config migration

When you upgrade `go-ooo`, the config schema may have changed - new sections, or renamed keys. Bring an existing
`config.toml` up to the current schema **without** re-running `init` (which refuses to overwrite an existing config):

```bash
./build/go-ooo config migrate --home /path/to/.go-ooo
```

This preserves your customised values, carries any renamed settings to their new homes (for example the
`[adhoc_quality]` section was renamed to `[price_quality]`), adds any new sections with their defaults, drops settings
that no longer exist, and backs up the previous file to `config.toml.bak`. Pass `--dry-run` to preview the changes
first. It is a no-op on an already-current file.

> Note: the file is regenerated from the current template, so hand-added comments are not preserved - your **values**
> are. Your previous file is always kept at `config.toml.bak`.

#### Registering a new Oracle Provider

If `go-ooo` has been initialised for a network other than `dev`, and using a key other than the pre-defined test key,
then registration as an Oracle Provider is required. First, ensure the wallet being used has funds on the target
chain, then run the registration admin command:

```bash
./build/go-ooo admin register [FEE] --home /path/to/.go-ooo --pass /path/to/pass.txt
```

Where `[FEE]` is your fee, for example `1000000` for 0.001 xFUND.

#### Start the Oracle

Now, you can start the Provider Oracle:

```bash
./build/go-ooo start
```

This will prompt you for the decryption password, and start the application. If you saved the password, you can pass the
path to the file using the `--pass` flag, e.g.

```bash
./build/go-ooo start --home $HOME/.go-ooo_dev --pass $HOME/.go-ooo_dev/pass.txt
```

## Docker Developer Environment

If the [Developer Environment](../docker/README.md) is running, these will have been deployed automatically, along with
a number of funded test accounts for both an OoO Provider and end-users. The `dev` network provider above should be 
pre-registered in the Router smart contract.
