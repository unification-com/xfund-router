const { constants, expectRevert } = require("@openzeppelin/test-helpers")

const { expect } = require("chai")

const xFUNDTestnet = artifacts.require("xFUNDTestnet") // Loads a compiled contract
const Router = artifacts.require("Router") // Loads a compiled contract

contract("Router - deploy", (accounts) => {
  const [admin, eoa] = accounts
  const decimals = 9
  const initSupply = 1000 * 10 ** decimals

  before(async function () {
    // admin deploy Token contract
    this.xFUNDTestnetContract = await xFUNDTestnet.new("xFUND", "xFUND", initSupply, decimals, {
      from: admin,
    })
  })

  it("can deploy Router with Token - has correct token address", async function () {
    const RouterContract = await Router.new(this.xFUNDTestnetContract.address, { from: admin })
    expect(await RouterContract.getTokenAddress()).to.equal(this.xFUNDTestnetContract.address)
  })

  it("must deploy with token", async function () {
    await expectRevert(
      Router.new(constants.ZERO_ADDRESS, { from: admin }),
      "token cannot be zero address",
    )
  })

  it("token address must be a contract", async function () {
    await expectRevert(Router.new(eoa, { from: admin }), "token address must be a contract")
  })
})
