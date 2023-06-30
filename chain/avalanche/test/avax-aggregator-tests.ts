import { ethers, getNamedAccounts, network, deployments } from "hardhat";
import { SignerWithAddress } from "@nomiclabs/hardhat-ethers/signers";
import { expect } from "chai";
import { Contract, Signer } from "ethers";
import { pangolinRouterAbi } from "./abis/pangolinRouterAbi";
import { USDCE_ADDRESS, USDCE_WHALE, WAVAX_ADDRESS } from "./constants";
import { AvaxAggregator, AvaxRouter } from "../typechain-types";
import ERC20 from "@openzeppelin/contracts/build/contracts/ERC20.json";

describe("AvaxAggregator", function () {
  let accounts: SignerWithAddress[];
  let avaxAggregator: AvaxAggregator;
  let avaxRouter: AvaxRouter;
  let usdceToken: Contract;
  let pangolin: any;
  const pangolinRouter = "0xE54Ca86531e17Ef3616d22Ca28b0D458b6C89106";

  beforeEach(async () => {
    const { admin, wallet1, wallet2 } = await getNamedAccounts();
    const usdceAmount = "150000000000"; // 6 dec

    accounts = await ethers.getSigners();
    await deployments.fixture();

    pangolin = new ethers.Contract(
      pangolinRouter,
      pangolinRouterAbi,
      accounts[0]
    );
    const avaxRouterDeployment = await ethers.getContractFactory("AvaxRouter");
    avaxRouter = await avaxRouterDeployment.deploy();

    const avaxAggregatorDeployment = await ethers.getContractFactory(
      "AvaxAggregator"
    );

    avaxAggregator = await avaxAggregatorDeployment.deploy(
      WAVAX_ADDRESS,
      pangolinRouter
    );
    usdceToken = new ethers.Contract(USDCE_ADDRESS, ERC20.abi, accounts[0]);

    await network.provider.request({
      method: "hardhat_impersonateAccount",
      params: [USDCE_WHALE],
    });

    const whaleSigner = await ethers.getSigner(USDCE_WHALE);
    const usdceWhale = usdceToken.connect(whaleSigner);
    await usdceWhale.transfer(admin, usdceAmount);
    await usdceWhale.transfer(wallet1, usdceAmount);
    await usdceWhale.transfer(wallet2, usdceAmount);
    let usdceBalance = await usdceWhale.balanceOf(wallet1);
    expect(usdceBalance).gt(0);
    usdceBalance = await usdceWhale.balanceOf(wallet2);
    expect(usdceBalance).gt(0);
  });

  describe("initialize", function () {
    it("Should init", async () => {
      expect(ethers.utils.isAddress(pangolin.address)).eq(true);
      expect(pangolin.address).to.not.eq(ethers.constants.AddressZero);
    });
  });

  describe("Swap In and Out", function () {
    it("Should swap AVAX for USDC.e in pangolin", async () => {
      const { wallet1 } = await getNamedAccounts();

      const amountOutMin = "1000";

      const wallet1Signer = accounts.find(
        (account) => account.address === wallet1
      );
      const pangolinWallet1 = pangolin.connect(wallet1Signer as Signer);
      const currentBlock = await ethers.provider.getBlockNumber();
      const currentTime = (await ethers.provider.getBlock(currentBlock))
        .timestamp;

      await pangolinWallet1.swapExactAVAXForTokens(
        amountOutMin,
        [WAVAX_ADDRESS, USDCE_ADDRESS],
        wallet1,
        currentTime + 1000000000,
        { value: ethers.utils.parseEther("0.1") }
      );

      const usdceContract = await ethers.getContractAt("IERC20", USDCE_ADDRESS);
      const balanceOfUsdce = await usdceContract.balanceOf(wallet1);
      // Doesn't matter what the result from pangolin is
      expect(balanceOfUsdce).gt(0);
    });
    it("Should Swap In Token for AVAX", async function () {
      const { wallet2, asgard1 } = await getNamedAccounts();

      const transferAmount = "10000000000";
      const initialAvaxBalance = "10000000000000000000000";
      expect(await ethers.provider.getBalance(asgard1)).to.equal(
        initialAvaxBalance
      );

      const wallet2Signer = accounts.find(
        (account) => account.address === wallet2
      );

      // approve usdce transfer
      const usdceTokenWallet2 = usdceToken.connect(wallet2Signer as Signer);
      const avaxAggregatorWallet2 = avaxAggregator.connect(
        wallet2Signer as Signer
      );
      await usdceTokenWallet2.approve(
        avaxAggregator.address,
        "10000000000000000000"
      );

      const deadline = ~~(Date.now() / 1000) + 100;

      const tx = await avaxAggregatorWallet2.swapIn(
        asgard1,
        avaxRouter.address,
        "SWAP:THOR.RUNE:tthor1uuds8pd92qnnq0udw0rpg0szpgcslc9p8lluej",
        usdceToken.address,
        transferAmount,
        0,
        deadline
      );
      tx.wait();

      expect(await usdceToken.balanceOf(wallet2)).to.equal("140000000000");
      expect(await ethers.provider.getBalance(wallet2)).lt(initialAvaxBalance);
      expect(await ethers.provider.getBalance(asgard1)).gt(initialAvaxBalance);
    });
    it("Should Swap In USDC.e for AVAX", async function () {
      const { wallet2, asgard1 } = await getNamedAccounts();
      const transferAmount = "10000000000";
      const initialAvaxBalance = "10000000000000000000000";
      expect(await ethers.provider.getBalance(asgard1)).to.equal(
        initialAvaxBalance
      );

      const wallet2Signer = accounts.find(
        (account) => account.address === wallet2
      );

      // approve usdce transfer
      const usdceTokenWallet2 = usdceToken.connect(wallet2Signer as Signer);
      const avaxAggregatorWallet2 = avaxAggregator.connect(
        wallet2Signer as Signer
      );
      await usdceTokenWallet2.approve(
        avaxAggregator.address,
        "10000000000000000000"
      );

      const deadline = ~~(Date.now() / 1000) + 100;

      await avaxAggregatorWallet2.swapIn(
        asgard1,
        avaxRouter.address,
        "SWAP:BTC.BTC:bc1Address:",
        usdceToken.address,
        transferAmount,
        0,
        deadline
      );

      expect(await usdceToken.balanceOf(wallet2)).to.equal("140000000000");
      expect(await ethers.provider.getBalance(wallet2)).eq(
        "9999944116300000000000"
      );
      expect(await ethers.provider.getBalance(asgard1)).eq(
        "10585860241166246434944"
      );
    });

    it("Should Swap Out using Aggregator", async function () {
      const { wallet2, asgard1 } = await getNamedAccounts();
      expect(await usdceToken.balanceOf(wallet2)).to.equal("150000000000");
      expect(await ethers.provider.getBalance(wallet2)).eq(
        "10000000000000000000000"
      );

      const wallet2Signer = accounts.find(
        (account) => account.address === wallet2
      );
      const asgard1Signer = accounts.find(
        (account) => account.address === asgard1
      );

      // approve usdce transfer
      const usdceTokenWallet2 = usdceToken.connect(wallet2Signer as Signer);
      const avaxRouterAsgard1 = avaxRouter.connect(asgard1Signer as Signer);

      await usdceTokenWallet2.approve(
        avaxAggregator.address,
        "10000000000000000000"
      );

      // Send 10 token to agg, which sends it to Sushi for 1 WETH,
      // Then unwraps to 1 ETH, then sends 1 ETH to Asgard vault
      await avaxRouterAsgard1.transferOutAndCall(
        avaxAggregator.address,
        usdceToken.address,
        wallet2,
        "0",
        "OUT:HASH",
        { value: ethers.utils.parseEther("1") }
      );

      expect(await ethers.provider.getBalance(asgard1)).eq(
        "9998968562775000000000"
      );
      expect(await usdceToken.balanceOf(wallet2)).to.equal("150016858977");
    });

    it("Should Fail Swap Out using Aggregator", async function () {
      const { wallet2, asgard1 } = await getNamedAccounts();
      expect(await usdceToken.balanceOf(wallet2)).to.equal("150000000000");
      expect(await ethers.provider.getBalance(wallet2)).eq(
        "10000000000000000000000"
      );

      const wallet2Signer = accounts.find(
        (account) => account.address === wallet2
      );
      const asgard1Signer = accounts.find(
        (account) => account.address === asgard1
      );

      // approve usdce transfer
      const usdceTokenWallet2 = usdceToken.connect(wallet2Signer as Signer);
      const avaxRouterAsgard1 = avaxRouter.connect(asgard1Signer as Signer);

      await usdceTokenWallet2.approve(
        avaxAggregator.address,
        "10000000000000000000"
      );

      // Send 10 token to agg, which sends it to Sushi for 1 WETH,
      // Then unwraps to 1 ETH, then sends 1 ETH to Asgard vault
      await avaxRouterAsgard1.transferOutAndCall(
        avaxAggregator.address,
        usdceToken.address,
        wallet2,
        "99999999999999999999999999999999999",
        "OUT:HASH",
        { value: ethers.utils.parseEther("1") }
      );

      expect(await ethers.provider.getBalance(asgard1)).eq(
        "9998982338400000000000"
      );
      expect(await ethers.provider.getBalance(wallet2)).eq(
        "10000989446600000000000"
      );
      expect(await usdceToken.balanceOf(wallet2)).to.equal("150000000000");
    });

    it("Should Fail Swap Out with AVAX using Aggregator", async function () {
      const { wallet2, asgard1 } = await getNamedAccounts();
      expect(await usdceToken.balanceOf(wallet2)).to.equal("150000000000");
      expect(await ethers.provider.getBalance(wallet2)).eq(
        "10000000000000000000000"
      );
      expect(await ethers.provider.getBalance(ethers.constants.AddressZero)).eq(
        "7836438818935998343"
      );

      const wallet2Signer = accounts.find(
        (account) => account.address === wallet2
      );
      const asgard1Signer = accounts.find(
        (account) => account.address === asgard1
      );

      // approve usdce transfer
      const usdceTokenWallet2 = usdceToken.connect(wallet2Signer as Signer);
      const avaxRouterAsgard1 = avaxRouter.connect(asgard1Signer as Signer);

      await usdceTokenWallet2.approve(
        avaxAggregator.address,
        "10000000000000000000"
      );

      // Send 10 token to agg, which sends it to Sushi for 1 WETH,
      // Then unwraps to 1 ETH, then sends 1 ETH to Asgard vault
      await avaxRouterAsgard1.transferOutAndCall(
        avaxAggregator.address,
        ethers.constants.AddressZero,
        wallet2,
        "99999999999999999999999999999999999",
        "OUT:HASH",
        { value: ethers.utils.parseEther("1") }
      );

      expect(await ethers.provider.getBalance(asgard1)).eq(
        "9998984118600000000000"
      );
      expect(await ethers.provider.getBalance(ethers.constants.AddressZero)).eq(
        "7836438818935998343"
      );
      expect(await usdceToken.balanceOf(wallet2)).to.equal("150000000000");
    });
  });
});
