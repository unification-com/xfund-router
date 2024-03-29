const { constants, expectRevert } = require("@openzeppelin/test-helpers")

const { expect } = require("chai")

const xFUNDTestnet = artifacts.require("xFUNDTestnet") // Loads a compiled contract
const Router = artifacts.require("Router") // Loads a compiled contract
const MockConsumer = artifacts.require("MockConsumer") // Loads a compiled contract

contract("Consumer - deploy", (accounts) => {
  const [admin, dataConsumerOwner, eoa] = accounts
  const decimals = 9
  const initSupply = 1000 * 10 ** decimals

  before(async function () {
    // admin deploy Token contract
    this.xFUNDTestnetContract = await xFUNDTestnet.new("xFUND", "xFUND", initSupply, decimals, {
      from: admin,
    })

    // admin deploy Router contract
    this.RouterContract = await Router.new(this.xFUNDTestnetContract.address, { from: admin })
  })

  it("can deploy a Consumer contract with router address - has correct router address", async function () {
    const MockConsumerContract = await MockConsumer.new(this.RouterContract.address, this.xFUNDTestnetContract.address, {
        from: dataConsumerOwner,
      },
    )
    expect(await MockConsumerContract.getRouterAddress()).to.equal(this.RouterContract.address)
  })

  it("must deploy with router contract address", async function () {
    await expectRevert(
      MockConsumer.new(constants.ZERO_ADDRESS, this.xFUNDTestnetContract.address, { from: dataConsumerOwner }),
      "router cannot be the zero address",
    )
  })

  it("must deploy with xfund contract address", async function () {
    await expectRevert(
      MockConsumer.new(this.xFUNDTestnetContract.address, constants.ZERO_ADDRESS, { from: dataConsumerOwner }),
      "xfund cannot be the zero address",
    )
  })
})
