package thorchain

import (
	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
	. "gopkg.in/check.v1"
)

type HandlerLoanRepaymentSuite struct{}

var _ = Suite(&HandlerLoanRepaymentSuite{})

func (s *HandlerLoanRepaymentSuite) TestLoanValidate(c *C) {
	ctx, mgr := setupManagerForTest(c)

	pool := NewPool()
	pool.Asset = common.BNBAsset
	c.Assert(mgr.Keeper().SetPool(ctx, pool), IsNil)

	pool = NewPool()
	pool.Asset = common.BTCAsset
	c.Assert(mgr.Keeper().SetPool(ctx, pool), IsNil)

	owner := GetRandomBNBAddress()
	signer := GetRandomBech32Addr()

	loan := NewLoan(owner, common.BNBAsset, 0)
	loan.DebtUp = cosmos.NewUint(10 * common.One)
	loan.CollateralUp = cosmos.NewUint(10 * common.One)
	mgr.Keeper().SetLoan(ctx, loan)

	handler := NewLoanRepaymentHandler(mgr)

	// happy path
	msg := NewMsgLoanRepayment(owner, common.BNBAsset, cosmos.OneUint(), owner, common.NewCoin(common.TOR, cosmos.NewUint(10*common.One)), signer)
	c.Check(handler.validate(ctx.WithBlockHeight(14400000), *msg), IsNil)

	// unhappy path: loan hasn't matured
	loan.LastOpenHeight = 10000
	mgr.Keeper().SetLoan(ctx, loan)
	c.Check(handler.validate(ctx, *msg), NotNil)

	// unhappy path: no debt/collateral
	loan.CollateralUp = cosmos.ZeroUint()
	loan.LastOpenHeight = 0
	mgr.Keeper().SetLoan(ctx, loan)
	c.Check(handler.validate(ctx, *msg), NotNil)

	// unhappy path: loan paused
	loan.DebtUp = cosmos.NewUint(10 * common.One)
	mgr.Keeper().SetLoan(ctx, loan)
	mgr.Keeper().SetMimir(ctx, "PauseLoans", 1)
	c.Check(handler.validate(ctx.WithBlockHeight(14400000), *msg), NotNil)
}

func (s *HandlerLoanRepaymentSuite) TestLoanRepaymentHandleWithTOR(c *C) {
	ctx, mgr := setupManagerForTest(c)
	mgr.Keeper().SetMimir(ctx, "DerivedDepthBasisPts", 10_000)

	pool := NewPool()
	pool.Asset = common.BTCAsset
	pool.LPUnits = cosmos.NewUint(346168413758888)
	pool.BalanceAsset = cosmos.NewUint(64394417894)
	pool.BalanceRune = cosmos.NewUint(523467094850166)
	c.Assert(mgr.Keeper().SetPool(ctx, pool), IsNil)

	signer := GetRandomBech32Addr()
	owner, _ := common.NewAddress("bcrt1q8ln0p2d4mwng7x20nl7hku25d282sjgf2v74nt")

	vault := GetRandomVault()
	vault.AddFunds(common.NewCoins(common.NewCoin(common.BTCAsset, cosmos.NewUint(10*common.One))))
	c.Assert(mgr.Keeper().SetVault(ctx, vault), IsNil)
	// set max gas
	c.Assert(mgr.Keeper().SaveNetworkFee(ctx, common.BTCChain, NewNetworkFee(common.BTCChain, 10, 10)), IsNil)

	// mint TOR to burn later
	coin := common.NewCoin(common.TOR, cosmos.NewUint(1000*common.One))
	err := mgr.Keeper().MintToModule(ctx, ModuleName, coin)
	c.Assert(err, IsNil)
	err = mgr.Keeper().SendFromModuleToModule(ctx, ModuleName, LendingName, common.NewCoins(coin))
	c.Assert(err, IsNil)

	// mint derived btc to transfer later
	dCoin := common.NewCoin(common.BTCAsset.GetDerivedAsset(), cosmos.NewUint(1e8))
	err = mgr.Keeper().MintToModule(ctx, ModuleName, dCoin)
	c.Assert(err, IsNil)
	err = mgr.Keeper().SendFromModuleToModule(ctx, ModuleName, LendingName, common.NewCoins(dCoin))
	c.Assert(err, IsNil)

	loan := NewLoan(owner, common.BTCAsset, 0)
	loan.CollateralUp = cosmos.NewUint(1e8)
	loan.DebtUp = cosmos.NewUint(10 * common.One)
	mgr.Keeper().SetLoan(ctx, loan)
	mgr.Keeper().SetTotalCollateral(ctx, common.BTCAsset, loan.CollateralUp)

	handler := NewLoanRepaymentHandler(mgr)

	// happy path
	txid, _ := common.NewTxID("29FC8D032CF17380AA1DC86F85A479CA9433E85887A9317C5D70D87EF56EAFAA")
	msg := NewMsgLoanRepayment(owner, common.BTCAsset, cosmos.OneUint(), owner, common.NewCoin(common.TOR, cosmos.NewUint(10*common.One)), signer)
	c.Check(handler.handle(ctx.WithValue(constants.CtxLoanTxID, txid), *msg), IsNil)

	loan, err = mgr.Keeper().GetLoan(ctx, common.BTCAsset, owner)
	c.Assert(err, IsNil)
	c.Check(loan.DebtDown.Uint64(), Equals, uint64(10*common.One), Commentf("%d", loan.DebtDown.Uint64()))
	c.Check(loan.CollateralDown.Uint64(), Equals, uint64(1e8), Commentf("%d", loan.CollateralDown.Uint64()))

	totalCollateral, err := mgr.Keeper().GetTotalCollateral(ctx, common.BTCAsset)
	c.Assert(err, IsNil)
	c.Check(totalCollateral.Uint64(), Equals, common.SafeSub(loan.CollateralUp, loan.CollateralDown).Uint64())
}

func (s *HandlerLoanRepaymentSuite) TestLoanRepaymentHandleWithSwap(c *C) {
	ctx, mgr := setupManagerForTest(c)
	mgr.Keeper().SetMimir(ctx, "DerivedDepthBasisPts", 10_000)

	pool := NewPool()
	pool.Asset = common.BTCAsset
	pool.BalanceAsset = cosmos.NewUint(83830778633)
	pool.BalanceRune = cosmos.NewUint(1022440798362209)
	pool.Decimals = 8
	c.Assert(mgr.Keeper().SetPool(ctx, pool), IsNil)

	mgr.Keeper().SetMimir(ctx, "TorAnchor-BNB-BUSD-BD1", 1) // enable BUSD pool as a TOR anchor
	busd := NewPool()
	busd.Asset, _ = common.NewAsset("BNB.BUSD-BD1")
	busd.Status = PoolAvailable
	busd.BalanceRune = cosmos.NewUint(433267688964312)
	busd.BalanceAsset = cosmos.NewUint(314031308608965)
	busd.Decimals = 8
	c.Assert(mgr.Keeper().SetPool(ctx, busd), IsNil)

	vault := GetRandomVault()
	vault.AddFunds(common.NewCoins(common.NewCoin(common.BTCAsset, cosmos.NewUint(10*common.One))))
	c.Assert(mgr.Keeper().SetVault(ctx, vault), IsNil)
	// set max gas
	c.Assert(mgr.Keeper().SaveNetworkFee(ctx, common.BTCChain, NewNetworkFee(common.BTCChain, 10, 10)), IsNil)

	// mint thor.btc collaterl
	coin := common.NewCoin(common.BTCAsset.GetDerivedAsset(), cosmos.NewUint(common.One))
	err := mgr.Keeper().MintToModule(ctx, ModuleName, coin)
	c.Assert(err, IsNil)
	err = mgr.Keeper().SendFromModuleToModule(ctx, ModuleName, LendingName, common.NewCoins(coin))
	c.Assert(err, IsNil)

	owner, _ := common.NewAddress("bcrt1q8ln0p2d4mwng7x20nl7hku25d282sjgf2v74nt")
	signer := GetRandomBech32Addr()

	loan := NewLoan(owner, common.BTCAsset, ctx.BlockHeight())
	loan.CollateralUp = cosmos.NewUint(1e8)
	loan.DebtUp = cosmos.NewUint(10_000 * common.One)
	mgr.Keeper().SetLoan(ctx, loan)

	handler := NewLoanRepaymentHandler(mgr)

	// happy path
	// overpay the loan to include swap fees
	txid, _ := common.NewTxID("29FC8D032CF17380AA1DC86F85A479CA9433E85887A9317C5D70D87EF56EAFAA")
	msg := NewMsgLoanRepayment(owner, common.BTCAsset, cosmos.OneUint(), owner, common.NewCoin(common.BTCAsset, cosmos.NewUint(1e8+15000000)), signer)
	ctx = ctx.WithBlockHeight(2 * 1440000)
	c.Check(handler.handle(ctx.WithValue(constants.CtxLoanTxID, txid), *msg), IsNil)
	c.Assert(mgr.SwapQ().EndBlock(ctx, mgr), IsNil) // swap into TOR
	c.Assert(mgr.SwapQ().EndBlock(ctx, mgr), IsNil) // swap out into collateral

	loan, err = mgr.Keeper().GetLoan(ctx, common.BTCAsset, owner)
	c.Assert(err, IsNil)
	// debt down is a little high due to overpaying the loan
	c.Check(loan.DebtDown.Uint64(), Equals, uint64(1007299944507), Commentf("%d", loan.DebtDown.Uint64()))
	c.Check(loan.CollateralDown.Uint64(), Equals, uint64(100000000), Commentf("%d", loan.CollateralDown.Uint64()))

	items, err := mgr.TxOutStore().GetOutboundItems(ctx)
	c.Assert(err, IsNil)
	c.Assert(items, HasLen, 1)
	item := items[0]
	c.Check(item.Coin.Asset.Equals(common.BTCAsset), Equals, true)
	c.Check(item.Coin.Amount.Uint64(), Equals, uint64(99525480), Commentf("%d", item.Coin.Amount.Uint64()))
	c.Check(item.ToAddress.String(), Equals, "bcrt1q8ln0p2d4mwng7x20nl7hku25d282sjgf2v74nt")
}
