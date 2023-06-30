package thorchain

import (
	"crypto/sha256"
	"fmt"

	. "gopkg.in/check.v1"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
)

type StoreManagerTestSuite struct{}

var _ = Suite(&StoreManagerTestSuite{})

func (s *StoreManagerTestSuite) TestRemoveTransactions(c *C) {
	ctx, mgr := setupManagerForTest(c)
	storeMgr := newStoreMgr(mgr)
	vault := NewVault(1024, ActiveVault, AsgardVault, GetRandomPubKey(), []string{
		common.ETHChain.String(),
	}, nil)

	c.Assert(storeMgr.mgr.Keeper().SaveNetworkFee(ctx, common.ETHChain, NetworkFee{
		Chain: common.ETHChain, TransactionSize: 80000, TransactionFeeRate: 30,
	}), IsNil)

	inTxID, err := common.NewTxID("BC68035CE2C8A2C549604FF7DB59E07931F39040758B138190338FA697338DB3")
	c.Assert(err, IsNil)
	tx := common.NewTx(inTxID,
		"0x3a196410a0f5facd08fd7880a4b8551cd085c031",
		"0xf56cBa49337A624E94042e325Ad6Bc864436E370",
		common.NewCoins(common.NewCoin(common.ETHAsset, cosmos.NewUint(200*common.One))),
		common.Gas{
			common.NewCoin(common.RuneNative, cosmos.NewUint(2000000)),
		}, "SWAP:ETH.AAVE-0x7Fc66500c84A76Ad7e9c93437bFc5Ac33E2DDaE9")
	observedTx := NewObservedTx(tx, 1281323, vault.PubKey, 1281323)
	voter := NewObservedTxVoter(inTxID, []ObservedTx{
		observedTx,
	})
	aaveAsset, _ := common.NewAsset("ETH.AAVE-0x7Fc66500c84A76Ad7e9c93437bFc5Ac33E2DDaE9")
	voter.Actions = []TxOutItem{
		{
			Chain:       common.ETHChain,
			ToAddress:   "0x3a196410a0f5facd08fd7880a4b8551cd085c031",
			VaultPubKey: vault.PubKey,
			Coin:        common.NewCoin(aaveAsset, cosmos.NewUint(1422136902)),
			Memo:        "OUT:BC68035CE2C8A2C549604FF7DB59E07931F39040758B138190338FA697338DB3",
			InHash:      "BC68035CE2C8A2C549604FF7DB59E07931F39040758B138190338FA697338DB3",
		},
		{
			Chain:       common.ETHChain,
			ToAddress:   "0x3a196410a0f5facd08fd7880a4b8551cd085c031",
			VaultPubKey: vault.PubKey,
			Coin:        common.NewCoin(aaveAsset, cosmos.NewUint(1330195098)),
			Memo:        "OUT:BC68035CE2C8A2C549604FF7DB59E07931F39040758B138190338FA697338DB3",
			InHash:      "BC68035CE2C8A2C549604FF7DB59E07931F39040758B138190338FA697338DB3",
		},
	}
	voter.Tx = voter.Txs[0]
	storeMgr.mgr.Keeper().SetObservedTxInVoter(ctx, voter)
	hegicAsset, _ := common.NewAsset("ETH.HEGIC-0x584bC13c7D411c00c01A62e8019472dE68768430")
	inTxID1, err := common.NewTxID("5DA19C1C5C2F6BBDF9D4FB0E6FF16A0DF6D6D7FE1F8E95CA755E5B3C6AADA458")
	c.Assert(err, IsNil)
	tx1 := common.NewTx(inTxID1,
		"0x3a196410a0f5facd08fd7880a4b8551cd085c031",
		"0xf56cBa49337A624E94042e325Ad6Bc864436E370",
		common.NewCoins(common.NewCoin(common.ETHAsset, cosmos.NewUint(200*common.One))),
		common.Gas{
			common.NewCoin(common.RuneNative, cosmos.NewUint(2000000)),
		}, "SWAP:ETH.HEGIC-0x584bC13c7D411c00c01A62e8019472dE68768430")
	observedTx1 := NewObservedTx(tx1, 1281323, vault.PubKey, 1281323)
	voter1 := NewObservedTxVoter(inTxID1, []ObservedTx{
		observedTx1,
	})
	voter1.Actions = []TxOutItem{
		{
			Chain:       common.ETHChain,
			ToAddress:   "0x3a196410a0f5facd08fd7880a4b8551cd085c031",
			VaultPubKey: vault.PubKey,
			Coin:        common.NewCoin(hegicAsset, cosmos.NewUint(3083783295390)),
			Memo:        "OUT:5DA19C1C5C2F6BBDF9D4FB0E6FF16A0DF6D6D7FE1F8E95CA755E5B3C6AADA458",
			InHash:      "5DA19C1C5C2F6BBDF9D4FB0E6FF16A0DF6D6D7FE1F8E95CA755E5B3C6AADA458",
		},
		{
			Chain:       common.ETHChain,
			ToAddress:   "0x3a196410a0f5facd08fd7880a4b8551cd085c031",
			VaultPubKey: vault.PubKey,
			Coin:        common.NewCoin(hegicAsset, cosmos.NewUint(2481151780248)),
			Memo:        "OUT:5DA19C1C5C2F6BBDF9D4FB0E6FF16A0DF6D6D7FE1F8E95CA755E5B3C6AADA458",
			InHash:      "5DA19C1C5C2F6BBDF9D4FB0E6FF16A0DF6D6D7FE1F8E95CA755E5B3C6AADA458",
		},
	}
	voter1.Tx = voter.Txs[0]
	storeMgr.mgr.Keeper().SetObservedTxInVoter(ctx, voter1)

	inTxID2, err := common.NewTxID("D58D1EF6D6E49EB99D0524128C16115893396FD05877EF4856FCE474B5BA09A7")
	c.Assert(err, IsNil)
	tx2 := common.NewTx(inTxID2,
		"0x3a196410a0f5facd08fd7880a4b8551cd085c031",
		"0xf56cBa49337A624E94042e325Ad6Bc864436E370",
		common.NewCoins(common.NewCoin(common.ETHAsset, cosmos.NewUint(150005145000))),
		common.Gas{
			common.NewCoin(common.RuneNative, cosmos.NewUint(2000000)),
		}, "SWAP:ETH.ETH")
	observedTx2 := NewObservedTx(tx2, 1281323, vault.PubKey, 1281323)
	voter2 := NewObservedTxVoter(inTxID2, []ObservedTx{
		observedTx2,
	})
	voter2.Actions = []TxOutItem{
		{
			Chain:       common.ETHChain,
			ToAddress:   "0x3a196410a0f5facd08fd7880a4b8551cd085c031",
			VaultPubKey: vault.PubKey,
			Coin:        common.NewCoin(hegicAsset, cosmos.NewUint(150003465000)),
			Memo:        "REFUND:D58D1EF6D6E49EB99D0524128C16115893396FD05877EF4856FCE474B5BA09A7",
			InHash:      "D58D1EF6D6E49EB99D0524128C16115893396FD05877EF4856FCE474B5BA09A7",
		},
	}
	voter2.Tx = voter.Txs[0]
	storeMgr.mgr.Keeper().SetObservedTxInVoter(ctx, voter2)

	allTxIDs := []common.TxID{
		inTxID, inTxID1, inTxID2,
	}
	removeTransactions(ctx, mgr, inTxID.String(), inTxID1.String(), inTxID2.String())
	for _, txID := range allTxIDs {
		voterAfter, err := storeMgr.mgr.Keeper().GetObservedTxInVoter(ctx, txID)
		c.Assert(err, IsNil)
		txAfter := voterAfter.GetTx(NodeAccounts{})
		c.Assert(txAfter.IsDone(len(voterAfter.Actions)), Equals, true)
	}
}

func (s *StoreManagerTestSuite) TestMigrateStoreV92(c *C) {
	ctx, mgr := setupManagerForTest(c)
	storeMgr := newStoreMgr(mgr)
	ustAsset, err := common.NewAsset("TERRA.UST")
	c.Assert(err, IsNil)
	vault := NewVault(1024, ActiveVault, AsgardVault, GetRandomPubKey(), []string{
		common.ETHChain.String(),
		common.TERRAChain.String(),
	}, nil)

	vault.Coins = common.NewCoins(common.NewCoin(common.LUNAAsset, cosmos.NewUint(100*common.One)),
		common.NewCoin(ustAsset, cosmos.NewUint(1*common.One)),
		common.NewCoin(common.BTCAsset, cosmos.NewUint(10*common.One)))
	c.Assert(mgr.Keeper().SetVault(ctx, vault), IsNil)

	vault1 := NewVault(1024, RetiringVault, AsgardVault, GetRandomPubKey(), []string{
		common.ETHChain.String(),
		common.TERRAChain.String(),
	}, nil)

	vault1.Coins = common.NewCoins(common.NewCoin(common.LUNAAsset, cosmos.NewUint(100*common.One)),
		common.NewCoin(ustAsset, cosmos.NewUint(1*common.One)),
		common.NewCoin(common.BTCAsset, cosmos.NewUint(10*common.One)))
	c.Assert(mgr.Keeper().SetVault(ctx, vault1), IsNil)

	c.Assert(storeMgr.migrate(ctx, 92), IsNil)
	vaultAfter, err := mgr.Keeper().GetVault(ctx, vault.PubKey)
	c.Assert(err, IsNil)
	c.Assert(vaultAfter.HasAsset(common.LUNAAsset), Equals, false)
	c.Assert(vaultAfter.HasAsset(ustAsset), Equals, false)
	c.Assert(vaultAfter.HasAsset(common.BTCAsset), Equals, true)

	vaultAfter1, err := mgr.Keeper().GetVault(ctx, vault1.PubKey)
	c.Assert(err, IsNil)
	c.Assert(vaultAfter1.HasAsset(common.LUNAAsset), Equals, false)
	c.Assert(vaultAfter1.HasAsset(ustAsset), Equals, false)
	c.Assert(vaultAfter1.HasAsset(common.BTCAsset), Equals, true)
}

func (s *StoreManagerTestSuite) TestMigrateStoreV92_UpdateUSDCBalance(c *C) {
	ctx, mgr := setupManagerForTest(c)
	config := cosmos.GetConfig()
	config.SetBech32PrefixForAccount("thor", "thorpub")
	storeMgr := newStoreMgr(mgr)
	usdcAsset, err := common.NewAsset("ETH.USDC-0XA0B86991C6218B36C1D19D4A2E9EB0CE3606EB48")
	c.Assert(err, IsNil)
	firstVault := GetRandomVault()
	firstVault.Type = AsgardVault
	firstVault.Status = ActiveVault
	firstVault.Coins = common.NewCoins(
		common.NewCoin(usdcAsset, cosmos.NewUint(778569000000)))
	firstVault.PubKey = common.PubKey("thorpub1addwnpepqvxy3grgfsm6e9r7zdcfm5tuvmuefk6vcaen8x7mep4ywpa9jqeu6puyxnw")
	c.Assert(mgr.Keeper().SetVault(ctx, firstVault), IsNil)
	secondVault := GetRandomVault()
	secondVault.Type = AsgardVault
	secondVault.Status = ActiveVault
	secondVault.Coins = common.NewCoins(
		common.NewCoin(usdcAsset, cosmos.NewUint(557933000000)))
	secondVault.PubKey = common.PubKey("thorpub1addwnpepqd6lgh7qjsfrkfxud68k7kc43f945x9s7wt2tzsttfde783u59mlyaql6cy")
	c.Assert(mgr.Keeper().SetVault(ctx, secondVault), IsNil)
	thirdVault := GetRandomVault()
	thirdVault.Type = AsgardVault
	thirdVault.Status = ActiveVault
	thirdVault.Coins = common.NewCoins(
		common.NewCoin(usdcAsset, cosmos.NewUint(1337164000000)))
	thirdVault.PubKey = common.PubKey("thorpub1addwnpepq25tc6wckjrpnx2rc7l0lzz4vjsa8cseq8p39jyhm7jr9sk3hqjns80xmmm")
	c.Assert(mgr.Keeper().SetVault(ctx, thirdVault), IsNil)
	p := NewPool()
	p.Asset = usdcAsset
	p.Status = PoolAvailable
	p.BalanceAsset = cosmos.NewUint(2673666000000)
	c.Assert(mgr.Keeper().SetPool(ctx, p), IsNil)

	c.Assert(storeMgr.migrate(ctx, 92), IsNil)
	firstVaultAfter, err := mgr.Keeper().GetVault(ctx, firstVault.PubKey)
	c.Assert(err, IsNil)
	c.Assert(firstVaultAfter.Coins.IsEmpty(), Equals, true)
	secondVaultAfter, err := mgr.Keeper().GetVault(ctx, secondVault.PubKey)
	c.Assert(err, IsNil)
	c.Assert(secondVaultAfter.Coins.IsEmpty(), Equals, true)
	thirdVaultAfter, err := mgr.Keeper().GetVault(ctx, thirdVault.PubKey)
	c.Assert(err, IsNil)
	c.Assert(thirdVaultAfter.Coins.IsEmpty(), Equals, true)
	pAfter, err := mgr.Keeper().GetPool(ctx, usdcAsset)
	c.Assert(err, IsNil)
	c.Assert(pAfter.BalanceAsset.IsZero(), Equals, true)
}

func (s *StoreManagerTestSuite) TestMigrateStoreV94(c *C) {
	ctx, mgr := setupManagerForTest(c)
	storeMgr := newStoreMgr(mgr)
	poolLuna := NewPool()
	poolLuna.Asset = common.LUNAAsset
	poolLuna.BalanceRune = cosmos.NewUint(77243905859)
	poolLuna.BalanceAsset = cosmos.ZeroUint()
	poolLuna.LPUnits = cosmos.NewUint(670538702)
	poolLuna.Status = PoolSuspended
	poolLuna.PendingInboundAsset = cosmos.NewUint(557871500)
	c.Assert(mgr.Keeper().SetPool(ctx, poolLuna), IsNil)
	ustAsset, err := common.NewAsset("TERRA.UST")
	c.Assert(err, IsNil)
	vault := NewVault(1024, ActiveVault, AsgardVault, GetRandomPubKey(), []string{
		common.ETHChain.String(),
		common.TERRAChain.String(),
	}, nil)

	vault.Coins = common.NewCoins(common.NewCoin(common.LUNAAsset, cosmos.NewUint(100*common.One)),
		common.NewCoin(ustAsset, cosmos.NewUint(1*common.One)),
		common.NewCoin(common.BTCAsset, cosmos.NewUint(10*common.One)))
	c.Assert(mgr.Keeper().SetVault(ctx, vault), IsNil)

	vault1 := NewVault(1024, RetiringVault, AsgardVault, GetRandomPubKey(), []string{
		common.ETHChain.String(),
		common.TERRAChain.String(),
	}, nil)

	vault1.Coins = common.NewCoins(common.NewCoin(common.LUNAAsset, cosmos.NewUint(100*common.One)),
		common.NewCoin(ustAsset, cosmos.NewUint(1*common.One)),
		common.NewCoin(common.BTCAsset, cosmos.NewUint(10*common.One)))
	c.Assert(mgr.Keeper().SetVault(ctx, vault1), IsNil)

	vaultYgg := NewVault(1024, ActiveVault, YggdrasilVault, GetRandomPubKey(), []string{
		common.ETHChain.String(),
		common.TERRAChain.String(),
	}, nil)

	vaultYgg.Coins = common.NewCoins(common.NewCoin(common.LUNAAsset, cosmos.NewUint(100*common.One)),
		common.NewCoin(ustAsset, cosmos.NewUint(1*common.One)),
		common.NewCoin(common.BTCAsset, cosmos.NewUint(10*common.One)))
	c.Assert(mgr.Keeper().SetVault(ctx, vaultYgg), IsNil)

	c.Assert(storeMgr.migrate(ctx, 94), IsNil)
	vaultAfter, err := mgr.Keeper().GetVault(ctx, vault.PubKey)
	c.Assert(err, IsNil)
	c.Assert(vaultAfter.HasAsset(common.LUNAAsset), Equals, false)
	c.Assert(vaultAfter.HasAsset(ustAsset), Equals, false)
	c.Assert(vaultAfter.HasAsset(common.BTCAsset), Equals, true)

	vaultAfter1, err := mgr.Keeper().GetVault(ctx, vault1.PubKey)
	c.Assert(err, IsNil)
	c.Assert(vaultAfter1.HasAsset(common.LUNAAsset), Equals, false)
	c.Assert(vaultAfter1.HasAsset(ustAsset), Equals, false)
	c.Assert(vaultAfter1.HasAsset(common.BTCAsset), Equals, true)

	vaultYggAfter, err := mgr.Keeper().GetVault(ctx, vaultYgg.PubKey)
	c.Assert(err, IsNil)
	c.Assert(vaultYggAfter.HasAsset(common.LUNAAsset), Equals, false)
	c.Assert(vaultYggAfter.HasAsset(ustAsset), Equals, false)
	c.Assert(vaultYggAfter.HasAsset(common.BTCAsset), Equals, true)
	poolLunaAfter, err := mgr.Keeper().GetPool(ctx, common.LUNAAsset)
	c.Assert(err, IsNil)
	c.Assert(poolLunaAfter.IsEmpty(), Equals, true)
}

// Check that the hashing behaves as expeected.
func (s *StoreManagerTestSuite) TestMemoHash(c *C) {
	inboundTxID := "B07A6B1B40ADBA2E404D9BCE1BEF6EDE6F70AD135E83806E4F4B6863CF637D0B"
	memo := fmt.Sprintf("REFUND:%s", inboundTxID)

	// This is the hash produced if using sha256 instead of Keccak-256
	// (which gave EE31ACC02D631DC3220990A1DD2E9030F4CFC227A61E975B5DEF1037106D1CCD)
	hash := fmt.Sprintf("%X", sha256.Sum256([]byte(memo)))
	fakeTxID, err := common.NewTxID(hash)
	c.Assert(err, IsNil)
	c.Assert(fakeTxID.String(), Equals, "AC0605F714563B3D5A34C64CCB6D90C1EA4EF13E1BA5E8638FE1FC796547332F")
}
