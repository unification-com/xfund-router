# Smart Contracts

Smart contracts required to run the OoO network, including the Router, demo consumer and end-user SDK contracts

## Development and Testing

### Prerequisites

#### NodeJS

The `openzeppelin/test-environment` packages and dependencies require
NodeJS <= `v12.18.3` in order to correctly install.
We recommend using [nvm](https://github.com/nvm-sh/nvm) to manage NodeJS
installations.

#### Yarn

[Yarn](https://classic.yarnpkg.com/en/docs/install) is recommended for package management.

### Compile Contracts

All smart contract code is in the `smart-contracts` directory.

```bash
cd smart-contracts
```

Run:

```bash
yarn install
```

to install the Node packages and dependencies

Run:
```bash 
npx truffle compile
```

to compile smart contracts

### Unit Tests

To run all tests:

```bash
yarn test
```

Test Coverage

```bash
yarn run coverage
```

Running unit test coverage will take a long time. Results are saved to
`./converage.json` and `./coverage`

## Docker Developer Environment

If the [Developer Environment](../docker/README.md) is running, these will have been deployed automatically, along with
a number of funded test accounts for both an OoO Provider and end-users

#### Dev Notes

Verify contracts on Etherscan after deployment - from the `smart-contracts` dir set `ETHERSCAN_API` in `.env`, then run:

```bash 
npx truffle run verify [ContractName] --network=[network]
```