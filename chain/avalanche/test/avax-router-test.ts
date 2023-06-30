import { deployments, ethers, getNamedAccounts, network } from "hardhat";
import { SignerWithAddress } from "@nomiclabs/hardhat-ethers/signers";
import { expect } from "chai";
import { BigNumber, Contract, Signer } from "ethers";
import { USDCE_ADDRESS, USDCE_WHALE } from "./constants";
import ERC20 from "@openzeppelin/contracts/build/contracts/ERC20.json";
import { AvaxRouter } from "../typechain-types";
import { Receipt } from "hardhat-deploy/dist/types";

describe("AvaxRouter", function () {
  let accounts: SignerWithAddress[];
  let avaxRouter: AvaxRouter;
  let avaxRouter2: AvaxRouter;
  let usdceToken: Contract;
  const AVAX = ethers.constants.AddressZero;

  beforeEach(async () => {
    const { wallet1, wallet2 } = await getNamedAccounts();

    accounts = await ethers.getSigners();
    await deployments.fixture();
    const avaxRouterDeployment = await ethers.getContractFactory("AvaxRouter");
    avaxRouter = await avaxRouterDeployment.deploy();
    avaxRouter2 = await avaxRouterDeployment.deploy();

    usdceToken = new ethers.Contract(USDCE_ADDRESS, ERC20.abi, accounts[0]);

    // Transfer UCDCE to wallet1
    const usdceAmount = "150000000000"; // 6 dec

    await network.provider.request({
      method: "hardhat_impersonateAccount",
      params: [USDCE_WHALE],
    });

    const whaleSigner = await ethers.getSigner(USDCE_WHALE);
    const usdceWhale = usdceToken.connect(whaleSigner);
    await usdceWhale.transfer(wallet1, usdceAmount);
    await usdceWhale.transfer(wallet2, usdceAmount);

    let usdceBalance = await usdceWhale.balanceOf(wallet1);
    expect(usdceBalance).gt(0);
    usdceBalance = await usdceWhale.balanceOf(wallet2);
    expect(usdceBalance).gt(0);
  });

  describe("initialize", function () {
    it("Should init", async () => {
      expect(ethers.utils.isAddress(avaxRouter.address)).eq(true);
      expect(avaxRouter.address).to.not.eq(ethers.constants.AddressZero);
    });
  });

  describe("User Deposit Assets", function () {
    it("Should Deposit AVAX To Asgard", async function () {
      const { asgard1 } = await getNamedAccounts();
      const amount = ethers.utils.parseEther("1000");

      const startBal = BigNumber.from(
        await ethers.provider.getBalance(asgard1)
      );
      const tx = await avaxRouter.deposit(
        asgard1,
        AVAX,
        amount,
        "SWAP:THOR.RUNE",
        { value: amount }
      );
      const receipt = await tx.wait();

      expect(receipt?.events?.[0].event).to.equal("Deposit");
      expect(tx.value).to.equal(amount);
      expect(receipt?.events?.[0]?.args?.asset).to.equal(AVAX);
      expect(receipt?.events?.[0]?.args?.memo).to.equal("SWAP:THOR.RUNE");

      const endBal = BigNumber.from(await ethers.provider.getBalance(asgard1));
      const changeBal = BigNumber.from(endBal).sub(startBal);
      expect(changeBal).to.equal(amount);
    });
    it("Should revert expired Deposit AVAX To Asgard1", async function () {
      const { asgard1 } = await getNamedAccounts();
      const amount = ethers.utils.parseEther("1000");

      await expect(
        avaxRouter.depositWithExpiry(
          asgard1,
          AVAX,
          amount,
          "SWAP:THOR.RUNE:tthor1uuds8pd92qnnq0udw0rpg0szpgcslc9p8lluej",
          BigNumber.from(0),
          { value: amount }
        )
      ).to.be.revertedWith("THORChain_Router: expired");
    });
    it("Should Deposit Token to Asgard1", async function () {
      const { wallet1, asgard1 } = await getNamedAccounts();
      const amount = "500000000";

      const wallet1Signer = accounts.find(
        (account) => account.address === wallet1
      );
      const avaxRouterWallet1 = avaxRouter.connect(wallet1Signer as Signer);

      // approve usdce transfer
      const usdceTokenWallet1 = usdceToken.connect(wallet1Signer as Signer);
      await usdceTokenWallet1.approve(avaxRouterWallet1.address, amount);

      const tx = await avaxRouterWallet1.deposit(
        asgard1,
        usdceToken.address,
        amount,
        "SWAP:THOR.RUNE"
      );
      const receipt: Receipt = await tx.wait();

      const event = receipt?.events?.find((event: any) => event.logIndex === 2);
      expect(event.event).to.equal("Deposit");
      expect(event.args?.asset.toLowerCase()).to.equal(USDCE_ADDRESS);
      expect(event.args?.to).to.equal(asgard1);
      expect(event.args?.memo).to.equal("SWAP:THOR.RUNE");
      expect(event.args?.amount).to.equal(amount);

      expect(await usdceToken.balanceOf(avaxRouter.address)).to.equal(amount);
      expect(
        await avaxRouterWallet1.vaultAllowance(asgard1, usdceToken.address)
      ).to.equal(amount);
    });
    it("Should revert Deposit Token to Asgard1", async function () {
      const { asgard1 } = await getNamedAccounts();
      const amount = "500000000";

      await expect(
        avaxRouter.depositWithExpiry(
          asgard1,
          usdceToken.address,
          amount,
          "SWAP:THOR.RUNE:tthor1uuds8pd92qnnq0udw0rpg0szpgcslc9p8lluej",
          BigNumber.from(0)
        )
      ).to.be.revertedWith("THORChain_Router: expired");
    });
    it("Should revert when AVAX sent during ARC20 Deposit", async function () {
      const { asgard1 } = await getNamedAccounts();
      const amount = ethers.utils.parseEther("1000");

      await expect(
        avaxRouter.deposit(
          asgard1,
          usdceToken.address,
          amount,
          "SWAP:THOR.RUNE",
          { value: amount }
        )
      ).to.be.revertedWith("unexpected avax");
    });
  });
  describe("Fund Yggdrasil, Yggdrasil Transfer Out", function () {
    it("Should fund yggdrasil with AVAX", async function () {
      const { asgard1, yggdrasil } = await getNamedAccounts();
      const amount400 = ethers.utils.parseEther("400");
      const amount300 = ethers.utils.parseEther("300");

      const asgard1Signer = accounts.find(
        (account) => account.address === asgard1
      );
      const avaxRouterAsgard1 = avaxRouter.connect(asgard1Signer as Signer);

      const startBal = BigNumber.from(
        await ethers.provider.getBalance(yggdrasil)
      );
      const tx = await avaxRouterAsgard1.transferOut(
        yggdrasil,
        AVAX,
        amount300,
        "ygg+:123",
        { value: amount400 }
      );
      const receipt: Receipt = await tx.wait();

      expect(receipt.events?.[0].event).to.equal("TransferOut");
      expect(receipt.events?.[0].args?.asset).to.equal(AVAX);
      expect(receipt.events?.[0].args?.vault).to.equal(asgard1);
      expect(receipt.events?.[0].args?.amount).to.equal(amount400);
      expect(receipt.events?.[0].args?.memo).to.equal("ygg+:123");

      const endBal = BigNumber.from(
        await ethers.provider.getBalance(yggdrasil)
      );
      const changeBal = endBal.sub(startBal).toString();
      expect(changeBal).to.equal(amount400);
    });

    it("Should fund yggdrasil with tokens", async function () {
      const { wallet1, asgard1, yggdrasil } = await getNamedAccounts();
      const amount = "10000000000";

      // give asgard1 usdce
      const wallet1Signer = accounts.find(
        (account) => account.address === wallet1
      );
      const asgard1Signer = accounts.find(
        (account) => account.address === asgard1
      );

      const usdceWallet1 = usdceToken.connect(wallet1Signer as Signer);
      await usdceWallet1.approve(avaxRouter.address, amount);

      const avaxRouterWallet1 = avaxRouter.connect(wallet1Signer as Signer);

      let tx = await avaxRouterWallet1.deposit(
        asgard1,
        usdceToken.address,
        amount,
        "SWAP:THOR.RUNE"
      );

      const avaxRouterAsgard1 = avaxRouter.connect(asgard1Signer as Signer);

      // approve usdce transfer
      const usdceTokenAsgard1 = usdceToken.connect(asgard1Signer as Signer);
      await usdceTokenAsgard1.approve(avaxRouter.address, amount);

      tx = await avaxRouterAsgard1.transferAllowance(
        avaxRouter.address,
        yggdrasil,
        usdceToken.address,
        amount,
        "yggdrasil+:1234"
      );
      const receipt: Receipt = await tx.wait();
      expect(receipt.events?.[0]?.event).to.equal("TransferAllowance");
      expect(receipt.events?.[0]?.args?.newVault).to.equal(yggdrasil);
      expect(receipt.events?.[0]?.args?.amount).to.equal(amount);

      expect(await usdceToken.balanceOf(avaxRouter.address)).to.equal(amount);
      expect(
        await avaxRouter.vaultAllowance(yggdrasil, usdceToken.address)
      ).to.equal(amount);
      expect(
        await avaxRouter.vaultAllowance(asgard1, usdceToken.address)
      ).to.equal("0");
    });

    it("Should transfer AVAX to Wallet2", async function () {
      const { wallet2, yggdrasil } = await getNamedAccounts();

      const amount = ethers.utils.parseEther("10");

      const yggdrasilSigner = accounts.find(
        (account) => account.address === yggdrasil
      );
      const avaxRouterYggdrasil = avaxRouter.connect(yggdrasilSigner as Signer);

      const startBal = BigNumber.from(
        await ethers.provider.getBalance(wallet2)
      );
      const tx = await avaxRouterYggdrasil.transferOut(
        wallet2,
        AVAX,
        amount,
        "OUT:",
        { value: amount }
      );
      const receipt: Receipt = await tx.wait();

      expect(receipt.events?.[0]?.event).to.equal("TransferOut");
      expect(receipt.events?.[0]?.args?.to).to.equal(wallet2);
      expect(receipt.events?.[0]?.args?.asset).to.equal(AVAX);
      expect(receipt.events?.[0]?.args?.memo).to.equal("OUT:");
      expect(receipt.events?.[0]?.args?.amount).to.equal(amount);

      const endBal = BigNumber.from(await ethers.provider.getBalance(wallet2));
      const changeBal = endBal.sub(startBal);
      expect(changeBal).to.equal(amount);
    });

    it("Should take AVAX amount from the amount in transaction, instead of the amount parameter", async function () {
      const { wallet2, yggdrasil } = await getNamedAccounts();

      const amount20 = ethers.utils.parseEther("20");
      const amount10 = ethers.utils.parseEther("10");

      const yggdrasilSigner = accounts.find(
        (account) => account.address === yggdrasil
      );
      const avaxRouterYggdrasil = avaxRouter.connect(yggdrasilSigner as Signer);

      const startBal = BigNumber.from(
        await ethers.provider.getBalance(wallet2)
      );
      const tx = await avaxRouterYggdrasil.transferOut(
        wallet2,
        AVAX,
        amount20,
        "OUT:",
        { value: amount10 }
      );
      const receipt: Receipt = await tx.wait();
      expect(receipt.events?.[0]?.event).to.equal("TransferOut");
      expect(receipt.events?.[0]?.args?.to).to.equal(wallet2);
      expect(receipt.events?.[0]?.args?.asset).to.equal(AVAX);
      expect(receipt.events?.[0]?.args?.memo).to.equal("OUT:");
      expect(receipt.events?.[0]?.args?.amount).to.equal(amount10);

      const endBal = BigNumber.from(await ethers.provider.getBalance(wallet2));
      const changeBal = endBal.sub(startBal);
      expect(changeBal).to.equal(amount10);
    });

    it("Should transfer tokens to Wallet2", async function () {
      const { wallet2, yggdrasil, asgard1 } = await getNamedAccounts();
      const initialAmount = BigNumber.from("5000000000");
      const amount = initialAmount.div(2);

      const yggdrasilSigner = accounts.find(
        (account) => account.address === yggdrasil
      );
      const avaxRouterYggdrasilSigner = avaxRouter.connect(
        yggdrasilSigner as Signer
      );

      const wallet2Signer = accounts.find(
        (account) => account.address === wallet2
      );

      const asgard1Signer = accounts.find(
        (account) => account.address === asgard1
      );
      const avaxRouterAsgard1Signer = avaxRouter.connect(
        asgard1Signer as Signer
      );

      const usdceWallet2 = usdceToken.connect(wallet2Signer as Signer);
      await usdceWallet2.approve(avaxRouter.address, initialAmount);

      const avaxRouterWallet2 = avaxRouter.connect(wallet2Signer as Signer);

      await avaxRouterWallet2.deposit(
        asgard1,
        usdceToken.address,
        initialAmount,
        "SWAP:THOR.RUNE"
      );

      await avaxRouterAsgard1Signer.transferAllowance(
        avaxRouter.address,
        yggdrasil,
        usdceToken.address,
        initialAmount,
        "yggdrasil+:1234"
      );
      expect(
        await avaxRouter.vaultAllowance(yggdrasil, usdceToken.address)
      ).to.equal(initialAmount);

      const usdceWallet1 = usdceToken.connect(wallet2Signer as Signer);
      await usdceWallet1.approve(avaxRouter.address, initialAmount);

      const tx = await avaxRouterYggdrasilSigner.transferOut(
        wallet2,
        usdceToken.address,
        amount,
        "OUT:"
      );
      const receipt: Receipt = await tx.wait();

      const event = receipt?.events?.find((event: any) => event.logIndex === 1);

      expect(event.event).to.equal("TransferOut");
      expect(event.args?.to).to.equal(wallet2);
      expect(event.args?.asset.toLowerCase()).to.equal(usdceToken.address);
      expect(event.args?.memo).to.equal("OUT:");
      expect(event.args?.amount).to.equal(amount);

      expect(
        await avaxRouter.vaultAllowance(yggdrasil, usdceToken.address)
      ).to.equal(amount);
      expect(await usdceToken.balanceOf(avaxRouter.address)).to.equal(amount);
    });
  });

  describe("Yggdrasil Returns Funds, Asgard Churns, Old Vaults can't spend", function () {
    it("Ygg returns", async function () {
      const { wallet1, asgard1, yggdrasil } = await getNamedAccounts();

      const avaxBal = ethers.utils.parseEther("20");
      const tokenAmount = "200000000";

      const coins = {
        asset: usdceToken.address,
        amount: "200000000",
      };

      const yggdrasilSigner = accounts.find(
        (account) => account.address === yggdrasil
      );
      const avaxRouterYggdrasil = avaxRouter.connect(yggdrasilSigner as Signer);

      const wallet1Signer = accounts.find(
        (account) => account.address === wallet1
      );
      const avaxRouterWallet1 = avaxRouter.connect(wallet1Signer as Signer);

      const asgard1Signer = accounts.find(
        (account) => account.address === asgard1
      );
      const avaxRouterAsgard1 = avaxRouter.connect(asgard1Signer as Signer);

      const usdceWallet1 = usdceToken.connect(wallet1Signer as Signer);
      await usdceWallet1.approve(avaxRouter.address, tokenAmount);

      let tx = await avaxRouterWallet1.deposit(
        asgard1,
        usdceToken.address,
        tokenAmount,
        "SWAP:THOR.RUNE"
      );

      // approve usdce transfer
      const usdceTokenAsgard1 = usdceToken.connect(asgard1Signer as Signer);
      await usdceTokenAsgard1.approve(avaxRouter.address, tokenAmount);

      expect(await usdceToken.balanceOf(avaxRouter.address)).to.equal(
        tokenAmount
      );
      expect(
        await avaxRouterWallet1.vaultAllowance(asgard1, usdceToken.address)
      ).to.equal(tokenAmount);

      tx = await avaxRouterAsgard1.transferAllowance(
        avaxRouter.address,
        yggdrasil,
        usdceToken.address,
        tokenAmount,
        "yggdrasil+:1234"
      );

      tx = await avaxRouterYggdrasil.returnVaultAssets(
        avaxRouter.address,
        asgard1,
        [coins],
        "yggdrasil-:1234",
        { from: yggdrasil, value: avaxBal }
      );
      const receipt = await tx.wait();
      expect(receipt.events?.[0]?.event).to.equal("VaultTransfer");
      expect(receipt.events?.[0]?.args?.coins[0].asset.toLowerCase()).to.equal(
        usdceToken.address
      );
      expect(receipt.events?.[0]?.args?.coins[0].amount).to.equal(tokenAmount);
      expect(receipt.events?.[0]?.args?.memo).to.equal("yggdrasil-:1234");

      expect(await usdceToken.balanceOf(avaxRouter.address)).to.equal(
        tokenAmount
      );
      expect(
        await avaxRouter.vaultAllowance(yggdrasil, usdceToken.address)
      ).to.equal("0");
      expect(
        await avaxRouter.vaultAllowance(asgard1, usdceToken.address)
      ).to.equal(tokenAmount);
    });
    it("Asgard Churns", async function () {
      const { wallet1, asgard1, asgard2 } = await getNamedAccounts();
      const amount = "10000000000";

      const wallet1Signer = accounts.find(
        (account) => account.address === wallet1
      );
      const avaxRouterWallet1 = avaxRouter.connect(wallet1Signer as Signer);

      const asgard1Signer = accounts.find(
        (account) => account.address === asgard1
      );
      const avaxRouterAsgard1 = avaxRouter.connect(asgard1Signer as Signer);

      const usdceWallet1 = usdceToken.connect(wallet1Signer as Signer);
      await usdceWallet1.approve(avaxRouter.address, amount);

      let tx = await avaxRouterWallet1.deposit(
        asgard1,
        usdceToken.address,
        amount,
        "SWAP:THOR.RUNE"
      );

      // approve usdce transfer
      const usdceTokenAsgard1 = usdceToken.connect(asgard1Signer as Signer);
      await usdceTokenAsgard1.approve(avaxRouter.address, amount);

      tx = await avaxRouterAsgard1.transferAllowance(
        avaxRouter.address,
        asgard2,
        usdceToken.address,
        amount,
        "migrate:1234"
      );
      const receipt = await tx.wait();

      expect(receipt.events?.[0]?.event).to.equal("TransferAllowance");
      expect(receipt.events?.[0]?.args?.asset.toLowerCase()).to.equal(
        usdceToken.address
      );
      expect(receipt.events?.[0]?.args?.amount).to.equal(amount);

      expect(await usdceToken.balanceOf(avaxRouter.address)).to.equal(amount);
      expect(
        await avaxRouter.vaultAllowance(asgard1, usdceToken.address)
      ).to.equal("0");
      expect(
        await avaxRouter.vaultAllowance(asgard2, usdceToken.address)
      ).to.equal(amount);
    });
    it("Should fail to when old Asgard interacts", async function () {
      const { asgard1, asgard2, wallet2 } = await getNamedAccounts();
      const amount5k = ethers.utils.parseEther("5000");
      const asgard1Signer = accounts.find(
        (account) => account.address === asgard1
      );
      const avaxRouterAsgard1 = avaxRouter.connect(asgard1Signer as Signer);

      await expect(
        avaxRouterAsgard1.transferAllowance(
          avaxRouter.address,
          asgard2,
          usdceToken.address,
          amount5k,
          "migrate:1234"
        )
      ).to.be.reverted;
      await expect(
        avaxRouterAsgard1.transferOut(
          wallet2,
          usdceToken.address,
          amount5k,
          "OUT:"
        )
      ).to.be.reverted;
    });
    it("Should fail to when old Yggdrasil interacts", async function () {
      const { yggdrasil, asgard2, wallet2 } = await getNamedAccounts();

      const yggdrasilSigner = accounts.find(
        (account) => account.address === yggdrasil
      );
      const avaxRouterYggdrasil = avaxRouter.connect(yggdrasilSigner as Signer);

      const amount5k = ethers.utils.parseEther("5000");

      await expect(
        avaxRouterYggdrasil.transferAllowance(
          avaxRouter.address,
          asgard2,
          usdceToken.address,
          amount5k,
          "migrate:1234"
        )
      ).to.be.reverted;
      await expect(
        avaxRouterYggdrasil.transferOut(
          wallet2,
          usdceToken.address,
          amount5k,
          "OUT:"
        )
      ).to.be.reverted;
    });
  });

  describe("Upgrade contract", function () {
    it("should return vault assets to new router", async function () {
      const { yggdrasil, asgard1, asgard3, wallet1 } = await getNamedAccounts();
      const amount5kUsdce = "5000000000";

      const wallet1Signer = accounts.find(
        (account) => account.address === wallet1
      );
      const yggdrasilSigner = accounts.find(
        (account) => account.address === yggdrasil
      );
      const asgard1Signer = accounts.find(
        (account) => account.address === asgard1
      );
      const avaxRouterYggdrasil = avaxRouter.connect(yggdrasilSigner as Signer);
      const avaxRouterAsgard1 = avaxRouter.connect(asgard1Signer as Signer);
      const avaxRouterWallet1 = avaxRouter.connect(wallet1Signer as Signer);

      // approve usdce transfer
      const usdceTokenWallet1 = usdceToken.connect(wallet1Signer as Signer);
      await usdceTokenWallet1.approve(avaxRouter.address, amount5kUsdce);

      await avaxRouterWallet1.deposit(
        asgard1,
        usdceToken.address,
        amount5kUsdce,
        "SEED"
      );
      // await ROUTER1.deposit(yggdrasil, usdt.address, _50k, 'SEED', { from: USER1 });
      // await avaxRouterWallet2.deposit(yggdrasil, AVAX, '0', 'SEED AVAX', { value: amount1 });
      const avaxBal = ethers.utils.parseEther("20");

      // migrate _50k from asgard1 to asgard3 , to new avaxRouter2 contract
      const coin1 = {
        asset: usdceToken.address,
        amount: amount5kUsdce,
      };
      // let coin2 = {
      //     asset: usdt.address,
      //     amount: amount1
      // }

      const usdceTokenAsgard1 = usdceToken.connect(asgard1Signer as Signer);
      await usdceTokenAsgard1.approve(avaxRouter.address, amount5kUsdce);
      let tx = await avaxRouterAsgard1.transferAllowance(
        avaxRouter.address,
        yggdrasil,
        usdceToken.address,
        amount5kUsdce,
        "yggdrasil+:1234"
      );

      tx = await avaxRouterYggdrasil.returnVaultAssets(
        avaxRouter2.address,
        asgard3,
        [coin1],
        "yggdrasil-:1234",
        { value: avaxBal }
      );
      const receipt: Receipt = await tx.wait();

      const event = receipt?.events?.find((event) => event.logIndex === 3);
      expect(event?.event).to.equal("Deposit");
      expect(event?.args?.to).to.equal(asgard3);
      expect(event?.args?.asset.toLowerCase()).to.equal(usdceToken.address);
      expect(event?.args?.memo).to.equal("yggdrasil-:1234");
      expect(event?.args?.amount).to.equal(amount5kUsdce);

      // make sure the token had been transfer to asgardex3 and avaxRouter2
      expect(await usdceToken.balanceOf(avaxRouter2.address)).to.equal(
        amount5kUsdce
      );
      expect(
        await avaxRouter2.vaultAllowance(asgard3, usdceToken.address)
      ).to.equal(amount5kUsdce);
      expect(
        await avaxRouter.vaultAllowance(asgard1, usdceToken.address)
      ).to.equal("0");
    });

    it("should transfer all token and allowance to new contract", async function () {
      const { asgard1, asgard3, wallet1, wallet2 } = await getNamedAccounts();
      const amount5kUsdce = "5000000000";
      const amount2k = ethers.utils.parseEther("2000");
      const amount1 = ethers.utils.parseEther("1");

      const wallet1Signer = accounts.find(
        (account) => account.address === wallet1
      );
      const wallet2Signer = accounts.find(
        (account) => account.address === wallet2
      );
      const asgard1Signer = accounts.find(
        (account) => account.address === asgard1
      );
      const avaxRouterAsgard1 = avaxRouter.connect(asgard1Signer as Signer);
      const avaxRouterWallet1 = avaxRouter.connect(wallet1Signer as Signer);
      const avaxRouterWallet2 = avaxRouter.connect(wallet2Signer as Signer);

      const asgard1StartBalance = BigNumber.from(
        await ethers.provider.getBalance(asgard1)
      );

      const usdceTokenWallet1 = usdceToken.connect(wallet1Signer as Signer);
      await usdceTokenWallet1.approve(avaxRouter.address, amount5kUsdce);

      await avaxRouterWallet1.deposit(
        asgard1,
        usdceToken.address,
        amount5kUsdce,
        "SEED"
      );
      // await avaxRouterWallet1.deposit(asgard1, usdt.address, _50k, 'SEED');
      await avaxRouterWallet2.deposit(asgard1, AVAX, "0", "SEED AVAX", {
        value: amount1,
      });

      const asgard1EndBalance = BigNumber.from(
        await ethers.provider.getBalance(asgard1)
      );
      expect(asgard1EndBalance.sub(asgard1StartBalance)).to.equal(amount1);

      // migrate _50k from asgard1 to asgard3 , to new Router3 contract
      const tx = await avaxRouterAsgard1.transferAllowance(
        avaxRouter2.address,
        asgard3,
        usdceToken.address,
        amount5kUsdce,
        "MIGRATE:1"
      );
      const receipt: Receipt = await tx.wait();

      const event = receipt?.events?.find((event) => event.logIndex === 3);
      expect(event?.event).to.equal("Deposit");
      expect(event?.args?.to).to.equal(asgard3);
      expect(event?.args?.asset.toLowerCase()).to.equal(usdceToken.address);
      expect(event?.args?.memo).to.equal("MIGRATE:1");
      expect(event?.args?.amount).to.equal(amount5kUsdce);

      // make sure the token had been transfer to ASGARD3 and Router3
      expect(await usdceToken.balanceOf(avaxRouter2.address)).to.equal(
        amount5kUsdce
      );
      expect(
        await avaxRouter2.vaultAllowance(asgard3, usdceToken.address)
      ).to.equal(amount5kUsdce);
      expect(
        await avaxRouter.vaultAllowance(asgard1, usdceToken.address)
      ).to.equal("0");

      //   let tx2 = await avaxRouterAsgard1.transferAllowance(ROUTER3.address, ASGARD3, usdt.address, _50k, 'MIGRATE:1', { from: asgard1 });
      //   const receipt2 = await tx2.wait()

      //   expect(receipt2.events?.[0]?.event).to.equal('Deposit');
      //   expect(receipt2.events?.[0]?.args?.to).to.equal(asgard3);
      //   expect(receipt2.events?.[0]?.args?.asset).to.equal(usdt.address);
      //   expect(receipt2.events?.[0]?.args?.memo).to.equal('MIGRATE:1');
      //   expect(receipt2.events?.[0]?.args?.amount).to.equal(_50k);

      // make sure the token had been transfer to ASGARD3 and Router3
      //   expect((await usdt.balanceOf(ROUTER3.address))).to.equal(_100k);
      //   expect((await avaxRouter2.vaultAllowance(asgard3, usdt.address))).to.equal(_100k); // router3
      //   expect((await avaxRouter.vaultAllowance(asgard1, usdt.address))).to.equal('0');

      const asgard3StartBalance = BigNumber.from(
        await ethers.provider.getBalance(asgard3)
      );
      // this ignore the gas cost on ASGARD1
      // transfer out AVAX.AVAX
      const tx3 = await avaxRouterAsgard1.transferOut(
        asgard3,
        AVAX,
        "0",
        "MIGRATE:1",
        { value: amount2k }
      );
      const receipt3 = await tx3.wait();

      expect(receipt3.events?.[0]?.event).to.equal("TransferOut");
      expect(receipt3.events?.[0]?.args?.vault).to.equal(asgard1);
      expect(receipt3.events?.[0]?.args?.to).to.equal(asgard3);
      expect(receipt3.events?.[0]?.args?.asset).to.equal(AVAX);
      expect(receipt3.events?.[0]?.args?.memo).to.equal("MIGRATE:1");

      const asgard3EndBalance = BigNumber.from(
        await ethers.provider.getBalance(asgard3)
      );
      expect(asgard3EndBalance.sub(asgard3StartBalance)).to.equal(amount2k);
    });
  });
});
