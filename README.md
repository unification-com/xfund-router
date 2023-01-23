# xFUND Router & Data Consumer Solidity Smart Contracts

[![npm version](http://img.shields.io/npm/v/@unification-com/xfund-router.svg?style=flat)](https://npmjs.org/package/@unification-com/xfund-router "View this project on npm")
![sc unit tests](https://github.com/unification-com/xfund-router/actions/workflows/test-contracts.yml/badge.svg)
[![Latest go-ooo Release](https://img.shields.io/github/v/release/unification-com/xfund-router?display_name=tag)](https://github.com/unification-com/xfund-router/releases/latest)

A suite of smart contracts to enable price data from external sources (such as Finchains.io, or supported DEXs)
to be included in your smart contracts. The suite comprises of:

1) A deployed Router smart contract. This facilitates receiving and forwarding data requests,
   between Consumers and Providers, in addition to processing xFUND payments for data provision.
2) A ConsumerBase smart contract, which is integrated into your own smart contract in 
   order for data requests to be initialised (via the Router), and data to be received (from
   a designated Provider)
3) The Provider Oracle software, run by data providers

For an end-user integration guide, and how to use the suite in your own smart contracts, please
see the [OoO Documentation](https://docs.unification.io/ooo)

## Repo Overview

This repo consists of a few different packages and applications:

| Directory         | Description                                          |
|-------------------|------------------------------------------------------|
| `docker`          | Docker files for running a development environment   |
| `go-ooo`          | Go implementation of the OoO Provider Oracle         |
| `smart-contracts` | OoO Router smart contracts and end-user contract SDK |

See each directory's respective `README` for more info aimed at developers.
