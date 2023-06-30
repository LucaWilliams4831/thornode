package thorchain

import (
	"errors"

	. "gopkg.in/check.v1"

	"gitlab.com/thorchain/thornode/x/thorchain/keeper/types"
	types2 "gitlab.com/thorchain/thornode/x/thorchain/types"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
)

type SlashingV92Suite struct{}

var _ = Suite(&SlashingV92Suite{})

func (s *SlashingV92Suite) SetUpSuite(_ *C) {
	SetupConfigForTest()
}

func (s *SlashingV92Suite) TestObservingSlashing(c *C) {
	var err error
	ctx, k := setupKeeperForTest(c)
	naActiveAfterTx := GetRandomValidatorNode(NodeActive)
	naActiveAfterTx.ActiveBlockHeight = 1030
	nas := NodeAccounts{
		GetRandomValidatorNode(NodeActive),
		GetRandomValidatorNode(NodeActive),
		GetRandomValidatorNode(NodeStandby),
		naActiveAfterTx,
	}
	for _, item := range nas {
		c.Assert(k.SetNodeAccount(ctx, item), IsNil)
	}
	height := int64(1024)
	txOut := NewTxOut(height)
	txHash := GetRandomTxHash()
	observedTx := GetRandomObservedTx()
	txVoter := NewObservedTxVoter(txHash, []ObservedTx{
		observedTx,
	})
	txVoter.FinalisedHeight = 1024
	txVoter.Add(observedTx, nas[0].NodeAddress)
	txVoter.Tx = txVoter.Txs[0]
	k.SetObservedTxInVoter(ctx, txVoter)

	txOut.TxArray = append(txOut.TxArray, TxOutItem{
		Chain:       common.BNBChain,
		InHash:      txHash,
		ToAddress:   GetRandomBNBAddress(),
		VaultPubKey: GetRandomPubKey(),
		Coin:        common.NewCoin(common.BNBAsset, cosmos.NewUint(1024)),
		Memo:        "whatever",
	})

	c.Assert(k.SetTxOut(ctx, txOut), IsNil)

	ctx = ctx.WithBlockHeight(height + 300)
	ver := GetCurrentVersion()
	constAccessor := constants.GetConstantValues(ver)

	slasher := newSlasherV92(k, NewDummyEventMgr())
	// should slash na2 only
	lackOfObservationPenalty := constAccessor.GetInt64Value(constants.LackOfObservationPenalty)
	err = slasher.LackObserving(ctx, constAccessor)
	c.Assert(err, IsNil)
	slashPoint, err := k.GetNodeAccountSlashPoints(ctx, nas[0].NodeAddress)
	c.Assert(err, IsNil)
	c.Assert(slashPoint, Equals, int64(0))

	slashPoint, err = k.GetNodeAccountSlashPoints(ctx, nas[1].NodeAddress)
	c.Assert(err, IsNil)
	c.Assert(slashPoint, Equals, lackOfObservationPenalty)

	// standby node should not be slashed
	slashPoint, err = k.GetNodeAccountSlashPoints(ctx, nas[2].NodeAddress)
	c.Assert(err, IsNil)
	c.Assert(slashPoint, Equals, int64(0))

	// if node is active after the tx get observed , it should not be slashed
	slashPoint, err = k.GetNodeAccountSlashPoints(ctx, nas[3].NodeAddress)
	c.Assert(err, IsNil)
	c.Assert(slashPoint, Equals, int64(0))

	ctx = ctx.WithBlockHeight(height + 301)
	err = slasher.LackObserving(ctx, constAccessor)

	c.Assert(err, IsNil)
	slashPoint, err = k.GetNodeAccountSlashPoints(ctx, nas[0].NodeAddress)
	c.Assert(err, IsNil)
	c.Assert(slashPoint, Equals, int64(0))

	slashPoint, err = k.GetNodeAccountSlashPoints(ctx, nas[1].NodeAddress)
	c.Assert(err, IsNil)
	c.Assert(slashPoint, Equals, lackOfObservationPenalty)
}

func (s *SlashingV92Suite) TestLackObservingErrors(c *C) {
	ctx, _ := setupKeeperForTest(c)

	nas := NodeAccounts{
		GetRandomValidatorNode(NodeActive),
		GetRandomValidatorNode(NodeActive),
	}
	keeper := &TestSlashObservingKeeper{
		nas:      nas,
		addrs:    []cosmos.AccAddress{nas[0].NodeAddress},
		slashPts: make(map[string]int64),
	}
	ver := GetCurrentVersion()
	constAccessor := constants.GetConstantValues(ver)
	slasher := newSlasherV92(keeper, NewDummyEventMgr())
	err := slasher.LackObserving(ctx, constAccessor)
	c.Assert(err, IsNil)
}

func (s *SlashingV92Suite) TestNodeSignSlashErrors(c *C) {
	testCases := []struct {
		name        string
		condition   func(keeper *TestSlashingLackKeeper)
		shouldError bool
	}{
		{
			name: "fail to get tx out should return an error",
			condition: func(keeper *TestSlashingLackKeeper) {
				keeper.failGetTxOut = true
			},
			shouldError: true,
		},
		{
			name: "fail to get vault should return an error",
			condition: func(keeper *TestSlashingLackKeeper) {
				keeper.failGetVault = true
			},
			shouldError: false,
		},
		{
			name: "fail to get node account by pub key should return an error",
			condition: func(keeper *TestSlashingLackKeeper) {
				keeper.failGetNodeAccountByPubKey = true
			},
			shouldError: false,
		},
		{
			name: "fail to get asgard vault by status should return an error",
			condition: func(keeper *TestSlashingLackKeeper) {
				keeper.failGetAsgardByStatus = true
			},
			shouldError: true,
		},
		{
			name: "fail to get observed tx voter should return an error",
			condition: func(keeper *TestSlashingLackKeeper) {
				keeper.failGetObservedTxVoter = true
			},
			shouldError: true,
		},
		{
			name: "fail to set tx out should return an error",
			condition: func(keeper *TestSlashingLackKeeper) {
				keeper.failSetTxOut = true
			},
			shouldError: true,
		},
	}
	for _, item := range testCases {
		c.Logf("name:%s", item.name)
		ctx, _ := setupKeeperForTest(c)
		ctx = ctx.WithBlockHeight(201) // set blockheight
		ver := GetCurrentVersion()
		constAccessor := constants.GetConstantValues(ver)
		na := GetRandomValidatorNode(NodeActive)
		inTx := common.NewTx(
			GetRandomTxHash(),
			GetRandomBNBAddress(),
			GetRandomBNBAddress(),
			common.Coins{
				common.NewCoin(common.BNBAsset, cosmos.NewUint(320000000)),
				common.NewCoin(common.RuneAsset(), cosmos.NewUint(420000000)),
			},
			nil,
			"SWAP:BNB.BNB",
		)

		txOutItem := TxOutItem{
			Chain:       common.BNBChain,
			InHash:      inTx.ID,
			VaultPubKey: na.PubKeySet.Secp256k1,
			ToAddress:   GetRandomBNBAddress(),
			Coin: common.NewCoin(
				common.BNBAsset, cosmos.NewUint(3980500*common.One),
			),
		}
		txOut := NewTxOut(3)
		txOut.TxArray = append(txOut.TxArray, txOutItem)

		ygg := GetRandomVault()
		ygg.Type = YggdrasilVault
		keeper := &TestSlashingLackKeeper{
			txOut:  txOut,
			na:     na,
			vaults: Vaults{ygg},
			voter: ObservedTxVoter{
				Actions: []TxOutItem{txOutItem},
			},
			slashPts: make(map[string]int64),
		}
		signingTransactionPeriod := constAccessor.GetInt64Value(constants.SigningTransactionPeriod)
		ctx = ctx.WithBlockHeight(3 + signingTransactionPeriod)
		slasher := newSlasherV92(keeper, NewDummyEventMgr())
		item.condition(keeper)
		if item.shouldError {
			c.Assert(slasher.LackSigning(ctx, NewDummyMgr()), NotNil)
		} else {
			c.Assert(slasher.LackSigning(ctx, NewDummyMgr()), IsNil)
		}
	}
}

func (s *SlashingV92Suite) TestNotSigningSlash(c *C) {
	ctx, _ := setupKeeperForTest(c)
	ctx = ctx.WithBlockHeight(201) // set blockheight
	txOutStore := NewTxStoreDummy()
	ver := GetCurrentVersion()
	constAccessor := constants.GetConstantValues(ver)
	na := GetRandomValidatorNode(NodeActive)
	inTx := common.NewTx(
		GetRandomTxHash(),
		GetRandomBNBAddress(),
		GetRandomBNBAddress(),
		common.Coins{
			common.NewCoin(common.BNBAsset, cosmos.NewUint(320000000)),
			common.NewCoin(common.RuneAsset(), cosmos.NewUint(420000000)),
		},
		nil,
		"SWAP:BNB.BNB",
	)

	txOutItem := TxOutItem{
		Chain:       common.BNBChain,
		InHash:      inTx.ID,
		VaultPubKey: na.PubKeySet.Secp256k1,
		ToAddress:   GetRandomBNBAddress(),
		Coin: common.NewCoin(
			common.BNBAsset, cosmos.NewUint(3980500*common.One),
		),
	}
	txOut := NewTxOut(3)
	txOut.TxArray = append(txOut.TxArray, txOutItem)

	ygg := GetRandomVault()
	ygg.Type = YggdrasilVault
	ygg.Coins = common.Coins{
		common.NewCoin(common.BNBAsset, cosmos.NewUint(5000000*common.One)),
	}
	keeper := &TestSlashingLackKeeper{
		txOut:  txOut,
		na:     na,
		vaults: Vaults{ygg},
		voter: ObservedTxVoter{
			Actions: []TxOutItem{txOutItem},
		},
		slashPts: make(map[string]int64),
	}
	signingTransactionPeriod := constAccessor.GetInt64Value(constants.SigningTransactionPeriod)
	ctx = ctx.WithBlockHeight(3 + signingTransactionPeriod)
	mgr := NewDummyMgr()
	mgr.txOutStore = txOutStore
	slasher := newSlasherV92(keeper, NewDummyEventMgr())
	c.Assert(slasher.LackSigning(ctx, mgr), IsNil)

	c.Check(keeper.slashPts[na.NodeAddress.String()], Equals, int64(600), Commentf("%+v\n", na))

	outItems, err := txOutStore.GetOutboundItems(ctx)
	c.Assert(err, IsNil)
	c.Assert(outItems, HasLen, 1)
	c.Assert(outItems[0].VaultPubKey.Equals(keeper.vaults[0].PubKey), Equals, true)
	c.Assert(outItems[0].Memo, Equals, "")
	c.Assert(keeper.voter.Actions, HasLen, 1)
	// ensure we've updated our action item
	c.Assert(keeper.voter.Actions[0].VaultPubKey.Equals(outItems[0].VaultPubKey), Equals, true)
	c.Assert(keeper.txOut.TxArray[0].OutHash.IsEmpty(), Equals, false)
}

func (s *SlashingV92Suite) TestNewSlasher(c *C) {
	nas := NodeAccounts{
		GetRandomValidatorNode(NodeActive),
		GetRandomValidatorNode(NodeActive),
	}
	keeper := &TestSlashObservingKeeper{
		nas:      nas,
		addrs:    []cosmos.AccAddress{nas[0].NodeAddress},
		slashPts: make(map[string]int64),
	}
	slasher := newSlasherV92(keeper, NewDummyEventMgr())
	c.Assert(slasher, NotNil)
}

func (s *SlashingV92Suite) TestDoubleSign(c *C) {
	ctx, _ := setupKeeperForTest(c)
	constAccessor := constants.GetConstantValues(GetCurrentVersion())

	na := GetRandomValidatorNode(NodeActive)
	na.Bond = cosmos.NewUint(100 * common.One)

	keeper := &TestDoubleSlashKeeper{
		na:      na,
		network: NewNetwork(),
		modules: make(map[string]int64),
	}
	slasher := newSlasherV92(keeper, NewDummyEventMgr())

	pk, err := cosmos.GetPubKeyFromBech32(cosmos.Bech32PubKeyTypeConsPub, na.ValidatorConsPubKey)
	c.Assert(err, IsNil)
	err = slasher.HandleDoubleSign(ctx, pk.Address(), 0, constAccessor)
	c.Assert(err, IsNil)

	c.Check(keeper.na.Bond.Equal(cosmos.NewUint(9995000000)), Equals, true, Commentf("%d", keeper.na.Bond.Uint64()))
	c.Check(keeper.modules[ReserveName], Equals, int64(5000000))
}

func (s *SlashingV92Suite) TestIncreaseDecreaseSlashPoints(c *C) {
	ctx, _ := setupKeeperForTest(c)

	na := GetRandomValidatorNode(NodeActive)
	na.Bond = cosmos.NewUint(100 * common.One)

	keeper := &TestDoubleSlashKeeper{
		na:          na,
		network:     NewNetwork(),
		slashPoints: make(map[string]int64),
	}
	slasher := newSlasherV92(keeper, NewDummyEventMgr())
	addr := GetRandomBech32Addr()
	slasher.IncSlashPoints(ctx, 1, addr)
	slasher.DecSlashPoints(ctx, 1, addr)
	c.Assert(keeper.slashPoints[addr.String()], Equals, int64(0))
}

func (s *SlashingV92Suite) TestSlashVault(c *C) {
	ctx, mgr := setupManagerForTest(c)
	slasher := newSlasherV92(mgr.Keeper(), mgr.EventMgr())
	// when coins are empty , it should return nil
	c.Assert(slasher.SlashVault(ctx, GetRandomPubKey(), common.NewCoins(), mgr), IsNil)

	// when vault is not available , it should return an error
	err := slasher.SlashVault(ctx, GetRandomPubKey(), common.NewCoins(common.NewCoin(common.BTCAsset, cosmos.NewUint(common.One))), mgr)
	c.Assert(err, NotNil)
	c.Assert(errors.Is(err, types.ErrVaultNotFound), Equals, true)

	// create a node
	node := GetRandomValidatorNode(NodeActive)
	c.Assert(mgr.Keeper().SetNodeAccount(ctx, node), IsNil)
	FundModule(c, ctx, mgr.Keeper(), BondName, node.Bond.Uint64())
	vault := GetRandomVault()
	vault.Type = YggdrasilVault
	vault.Status = types2.VaultStatus_ActiveVault
	vault.PubKey = node.PubKeySet.Secp256k1
	vault.Membership = []string{
		node.PubKeySet.Secp256k1.String(),
	}
	vault.Coins = common.NewCoins(
		common.NewCoin(common.BTCAsset, cosmos.NewUint(2*common.One)),
	)
	c.Assert(mgr.Keeper().SetVault(ctx, vault), IsNil)

	pool := NewPool()
	pool.Asset = common.BTCAsset
	pool.BalanceRune = cosmos.NewUint(100 * common.One)
	pool.BalanceAsset = cosmos.NewUint(100 * common.One)
	pool.Status = PoolAvailable

	c.Assert(mgr.Keeper().SetPool(ctx, pool), IsNil)

	asgardBeforeSlash := mgr.Keeper().GetRuneBalanceOfModule(ctx, AsgardName)
	bondBeforeSlash := mgr.Keeper().GetRuneBalanceOfModule(ctx, BondName)
	reserveBeforeSlash := mgr.Keeper().GetRuneBalanceOfModule(ctx, ReserveName)
	poolBeforeSlash := pool.BalanceRune

	err = slasher.SlashVault(ctx, vault.PubKey, common.NewCoins(common.NewCoin(common.BTCAsset, cosmos.NewUint(common.One))), mgr)
	c.Assert(err, IsNil)
	nodeTemp, err := mgr.Keeper().GetNodeAccountByPubKey(ctx, vault.PubKey)
	c.Assert(err, IsNil)
	expectedBond := cosmos.NewUint(99850000000)
	c.Assert(nodeTemp.Bond.Equal(expectedBond), Equals, true, Commentf("%d", nodeTemp.Bond.Uint64()))

	asgardAfterSlash := mgr.Keeper().GetRuneBalanceOfModule(ctx, AsgardName)
	bondAfterSlash := mgr.Keeper().GetRuneBalanceOfModule(ctx, BondName)
	reserveAfterSlash := mgr.Keeper().GetRuneBalanceOfModule(ctx, ReserveName)

	pool, err = mgr.Keeper().GetPool(ctx, pool.Asset)
	c.Assert(err, IsNil)
	poolAfterSlash := pool.BalanceRune

	// ensure pool's change is in sync with asgard's change
	c.Assert(asgardAfterSlash.Sub(asgardBeforeSlash).Uint64(), Equals, poolAfterSlash.Sub(poolBeforeSlash).Uint64(), Commentf("%d", "pool/asgard rune mismatch"))

	c.Check(asgardAfterSlash.Sub(asgardBeforeSlash).Uint64(), Equals, uint64(100000000), Commentf("%d", asgardAfterSlash.Sub(asgardBeforeSlash).Uint64()))
	c.Check(asgardAfterSlash.Sub(asgardBeforeSlash).Uint64(), Equals, uint64(100000000), Commentf("%d", asgardAfterSlash.Sub(asgardBeforeSlash).Uint64()))
	c.Check(bondBeforeSlash.Sub(bondAfterSlash).Uint64(), Equals, uint64(150000000), Commentf("%d", bondBeforeSlash.Sub(bondAfterSlash).Uint64()))
	c.Check(reserveAfterSlash.Sub(reserveBeforeSlash).Uint64(), Equals, uint64(50000000), Commentf("%d", reserveAfterSlash.Sub(reserveBeforeSlash).Uint64()))

	// add one more node , slash asgard
	node1 := GetRandomValidatorNode(NodeActive)
	c.Assert(mgr.Keeper().SetNodeAccount(ctx, node1), IsNil)
	FundModule(c, ctx, mgr.Keeper(), BondName, node1.Bond.Uint64())

	vault1 := GetRandomVault()
	vault1.Type = AsgardVault
	vault1.Status = types2.VaultStatus_ActiveVault
	vault1.PubKey = GetRandomPubKey()
	vault1.Membership = []string{
		node.PubKeySet.Secp256k1.String(),
		node1.PubKeySet.Secp256k1.String(),
	}
	vault1.Coins = common.NewCoins(
		common.NewCoin(common.BTCAsset, cosmos.NewUint(2*common.One)),
	)
	c.Assert(mgr.Keeper().SetVault(ctx, vault1), IsNil)
	nodeBeforeSlash, err := mgr.Keeper().GetNodeAccount(ctx, node.NodeAddress)
	c.Assert(err, IsNil)
	nodeBondBeforeSlash := nodeBeforeSlash.Bond
	node1BondBeforeSlash := node1.Bond
	mgr.Keeper().SetMimir(ctx, "PauseOnSlashThreshold", 1)
	err = slasher.SlashVault(ctx, vault1.PubKey, common.NewCoins(common.NewCoin(common.BTCAsset, cosmos.NewUint(common.One))), mgr)
	c.Assert(err, IsNil)

	nodeAfterSlash, err := mgr.Keeper().GetNodeAccount(ctx, node.NodeAddress)
	c.Assert(err, IsNil)
	node1AfterSlash, err := mgr.Keeper().GetNodeAccount(ctx, node1.NodeAddress)
	c.Assert(err, IsNil)
	nodeBondAfterSlash := nodeAfterSlash.Bond
	node1BondAfterSlash := node1AfterSlash.Bond

	c.Check(nodeBondBeforeSlash.Sub(nodeBondAfterSlash).Uint64(), Equals, uint64(76457722), Commentf("%d", nodeBondBeforeSlash.Sub(nodeBondAfterSlash).Uint64()))
	c.Check(node1BondBeforeSlash.Sub(node1BondAfterSlash).Uint64(), Equals, uint64(76572581), Commentf("%d", node1BondBeforeSlash.Sub(node1BondAfterSlash).Uint64()))

	val, err := mgr.Keeper().GetMimir(ctx, mimirStopFundYggdrasil)
	c.Assert(err, IsNil)
	c.Assert(val, Equals, int64(18), Commentf("%d", val))

	val, err = mgr.Keeper().GetMimir(ctx, "HaltBTCChain")
	c.Assert(err, IsNil)
	c.Assert(val, Equals, int64(18), Commentf("%d", val))
}

func (s *SlashingV92Suite) TestSlashAndUpdateNodeAccount(c *C) {
	ctx, mgr := setupManagerForTest(c)
	slasher := newSlasherV92(mgr.Keeper(), mgr.EventMgr())

	// create a node + ygg
	node := GetRandomValidatorNode(NodeActive)
	c.Assert(mgr.Keeper().SetNodeAccount(ctx, node), IsNil)
	FundModule(c, ctx, mgr.Keeper(), BondName, node.Bond.Uint64())
	ygg := GetRandomYggVault()

	totalBond := node.Bond
	runeAmtToSlash := node.Bond.Mul(cosmos.NewUint(2))
	// only used to emit telemetry metrics
	bnbCoin := common.NewCoin(common.BNBAsset, cosmos.ZeroUint())

	// If amount to slash is greater than bond slash bond to zero and ban node
	slashedVal := slasher.slashAndUpdateNodeAccount(ctx, node, bnbCoin, ygg, totalBond, runeAmtToSlash)
	c.Assert(slashedVal.Equal(node.Bond), Equals, true)

	updatedNode, err := mgr.Keeper().GetNodeAccount(ctx, node.NodeAddress)
	c.Assert(err, IsNil)
	c.Assert(updatedNode.Bond.IsZero(), Equals, true)
	c.Assert(updatedNode.ForcedToLeave, Equals, true)
	c.Assert(updatedNode.LeaveScore, Equals, uint64(1))

	// If amount to slash is less than bond, just subtract and don't ban node
	node2 := GetRandomValidatorNode(NodeActive)
	c.Assert(mgr.Keeper().SetNodeAccount(ctx, node), IsNil)
	FundModule(c, ctx, mgr.Keeper(), BondName, node2.Bond.Uint64())

	totalBond = node2.Bond
	runeAmtToSlash = node2.Bond.Quo(cosmos.NewUint(2))
	slashedVal = slasher.slashAndUpdateNodeAccount(ctx, node2, bnbCoin, ygg, totalBond, runeAmtToSlash)
	c.Assert(slashedVal.Equal(runeAmtToSlash), Equals, true)

	updatedNode, err = mgr.Keeper().GetNodeAccount(ctx, node2.NodeAddress)
	c.Assert(err, IsNil)
	c.Assert(updatedNode.Bond.Uint64(), Equals, totalBond.Sub(runeAmtToSlash).Uint64())
	c.Assert(updatedNode.ForcedToLeave, Equals, false)
}

func (s *SlashingV92Suite) TestUpdatePoolFromSlash(c *C) {
	ctx, mgr := setupManagerForTest(c)
	slasher := newSlasherV92(mgr.Keeper(), mgr.EventMgr())

	pool := NewPool()
	pool.Asset = common.BTCAsset
	pool.BalanceRune = cosmos.NewUint(1000 * common.One)
	pool.BalanceAsset = cosmos.NewUint(1000 * common.One)
	pool.Status = PoolAvailable
	c.Assert(mgr.Keeper().SetPool(ctx, pool), IsNil)

	deductAsset := cosmos.NewUint(250 * common.One)
	creditRune := cosmos.NewUint(500 * common.One)
	stolenAsset := common.NewCoin(common.BTCAsset, deductAsset)
	slasher.updatePoolFromSlash(ctx, pool, stolenAsset, creditRune, mgr)

	pool, err := mgr.Keeper().GetPool(ctx, common.BTCAsset)
	c.Assert(err, IsNil)
	c.Assert(pool.BalanceAsset.Uint64(), Equals, uint64(750*common.One))
	c.Assert(pool.BalanceRune.Uint64(), Equals, uint64(1500*common.One))
}

func (s *SlashingV92Suite) TestNetworkShouldNotSlashMorethanVaultAmount(c *C) {
	ctx, mgr := setupManagerForTest(c)
	slasher := newSlasherV92(mgr.Keeper(), mgr.EventMgr())

	// create a node
	node := GetRandomValidatorNode(NodeActive)
	c.Assert(mgr.Keeper().SetNodeAccount(ctx, node), IsNil)
	FundModule(c, ctx, mgr.Keeper(), BondName, node.Bond.Uint64())
	vault := GetRandomVault()
	vault.Type = YggdrasilVault
	vault.Status = types2.VaultStatus_ActiveVault
	vault.PubKey = node.PubKeySet.Secp256k1
	vault.Membership = []string{
		node.PubKeySet.Secp256k1.String(),
	}
	vault.Coins = common.NewCoins(
		common.NewCoin(common.BTCAsset, cosmos.NewUint(common.One/2)),
	)
	c.Assert(mgr.Keeper().SetVault(ctx, vault), IsNil)

	pool := NewPool()
	pool.Asset = common.BTCAsset
	pool.BalanceRune = cosmos.NewUint(100 * common.One)
	pool.BalanceAsset = cosmos.NewUint(100 * common.One)
	pool.Status = PoolAvailable

	c.Assert(mgr.Keeper().SetPool(ctx, pool), IsNil)

	asgardBeforeSlash := mgr.Keeper().GetRuneBalanceOfModule(ctx, AsgardName)
	bondBeforeSlash := mgr.Keeper().GetRuneBalanceOfModule(ctx, BondName)
	reserveBeforeSlash := mgr.Keeper().GetRuneBalanceOfModule(ctx, ReserveName)
	poolBeforeSlash := pool.BalanceRune

	// vault only has 0.5 BTC , however the outbound is 1 BTC , make sure we don't over slash the vault
	err := slasher.SlashVault(ctx, vault.PubKey, common.NewCoins(common.NewCoin(common.BTCAsset, cosmos.NewUint(common.One))), mgr)
	c.Assert(err, IsNil)
	nodeTemp, err := mgr.Keeper().GetNodeAccountByPubKey(ctx, vault.PubKey)
	c.Assert(err, IsNil)
	expectedBond := cosmos.NewUint(99925000000)
	c.Assert(nodeTemp.Bond.Equal(expectedBond), Equals, true, Commentf("%d", nodeTemp.Bond.Uint64()))

	asgardAfterSlash := mgr.Keeper().GetRuneBalanceOfModule(ctx, AsgardName)
	bondAfterSlash := mgr.Keeper().GetRuneBalanceOfModule(ctx, BondName)
	reserveAfterSlash := mgr.Keeper().GetRuneBalanceOfModule(ctx, ReserveName)

	pool, err = mgr.Keeper().GetPool(ctx, pool.Asset)
	c.Assert(err, IsNil)
	poolAfterSlash := pool.BalanceRune

	// ensure pool's change is in sync with asgard's change
	c.Assert(asgardAfterSlash.Sub(asgardBeforeSlash).Uint64(), Equals, poolAfterSlash.Sub(poolBeforeSlash).Uint64(), Commentf("%d", "pool/asgard rune mismatch"))

	c.Check(asgardAfterSlash.Sub(asgardBeforeSlash).Uint64(), Equals, uint64(50000000), Commentf("%d", asgardAfterSlash.Sub(asgardBeforeSlash).Uint64()))
	c.Check(bondBeforeSlash.Sub(bondAfterSlash).Uint64(), Equals, uint64(75000000), Commentf("%d", bondBeforeSlash.Sub(bondAfterSlash).Uint64()))
	c.Check(reserveAfterSlash.Sub(reserveBeforeSlash).Uint64(), Equals, uint64(25000000), Commentf("%d", reserveAfterSlash.Sub(reserveBeforeSlash).Uint64()))

	// add one more node , slash asgard
	node1 := GetRandomValidatorNode(NodeActive)
	c.Assert(mgr.Keeper().SetNodeAccount(ctx, node1), IsNil)
	FundModule(c, ctx, mgr.Keeper(), BondName, node1.Bond.Uint64())

	vault1 := GetRandomVault()
	vault1.Type = AsgardVault
	vault1.Status = types2.VaultStatus_ActiveVault
	vault1.PubKey = GetRandomPubKey()
	vault1.Membership = []string{
		node.PubKeySet.Secp256k1.String(),
		node1.PubKeySet.Secp256k1.String(),
	}
	vault1.Coins = common.NewCoins(
		common.NewCoin(common.BTCAsset, cosmos.NewUint(common.One/2)),
	)
	c.Assert(mgr.Keeper().SetVault(ctx, vault1), IsNil)
	nodeBeforeSlash, err := mgr.Keeper().GetNodeAccount(ctx, node.NodeAddress)
	c.Assert(err, IsNil)
	nodeBondBeforeSlash := nodeBeforeSlash.Bond
	node1BondBeforeSlash := node1.Bond
	mgr.Keeper().SetMimir(ctx, "PauseOnSlashThreshold", 1)
	err = slasher.SlashVault(ctx, vault1.PubKey, common.NewCoins(common.NewCoin(common.BTCAsset, cosmos.NewUint(common.One))), mgr)
	c.Assert(err, IsNil)

	nodeAfterSlash, err := mgr.Keeper().GetNodeAccount(ctx, node.NodeAddress)
	c.Assert(err, IsNil)
	node1AfterSlash, err := mgr.Keeper().GetNodeAccount(ctx, node1.NodeAddress)
	c.Assert(err, IsNil)
	nodeBondAfterSlash := nodeAfterSlash.Bond
	node1BondAfterSlash := node1AfterSlash.Bond

	c.Check(nodeBondBeforeSlash.Sub(nodeBondAfterSlash).Uint64(), Equals, uint64(37862675), Commentf("%d", nodeBondBeforeSlash.Sub(nodeBondAfterSlash).Uint64()))
	c.Check(node1BondBeforeSlash.Sub(node1BondAfterSlash).Uint64(), Equals, uint64(37891094), Commentf("%d", node1BondBeforeSlash.Sub(node1BondAfterSlash).Uint64()))

	val, err := mgr.Keeper().GetMimir(ctx, mimirStopFundYggdrasil)
	c.Assert(err, IsNil)
	c.Assert(val, Equals, int64(18), Commentf("%d", val))

	val, err = mgr.Keeper().GetMimir(ctx, "HaltBTCChain")
	c.Assert(err, IsNil)
	c.Assert(val, Equals, int64(18), Commentf("%d", val))

	// Attempt to slash more than node has, pool should only be deducted what was successfully slashed
	pool.BalanceRune = cosmos.NewUint(4000 * common.One)
	pool.BalanceAsset = cosmos.NewUint(4000 * common.One)
	c.Assert(mgr.Keeper().SetPool(ctx, pool), IsNil)

	node2 := GetRandomValidatorNode(NodeActive)
	c.Assert(mgr.Keeper().SetNodeAccount(ctx, node2), IsNil)
	FundModule(c, ctx, mgr.Keeper(), BondName, node2.Bond.Uint64())

	vault = GetRandomYggVault()
	vault.Status = types2.VaultStatus_ActiveVault
	vault.PubKey = node.PubKeySet.Secp256k1
	vault.Membership = []string{
		node2.PubKeySet.Secp256k1.String(),
	}
	vault.Coins = common.NewCoins(
		common.NewCoin(common.BTCAsset, cosmos.NewUint(4000*common.One)),
	)
	c.Assert(mgr.Keeper().SetVault(ctx, vault), IsNil)

	err = slasher.SlashVault(ctx, vault.PubKey, common.NewCoins(common.NewCoin(common.BTCAsset, cosmos.NewUint(2000*common.One))), mgr)
	c.Assert(err, IsNil)
	updatedPool, err := mgr.Keeper().GetPool(ctx, common.BTCAsset)
	c.Assert(err, IsNil)

	// Even though the total rune value to slash is 3000, the node only has 1000 RUNE bonded, so only slash and credit that much to the pool's rune side
	c.Assert(updatedPool.BalanceRune.Uint64(), Equals, cosmos.NewUint(5000*common.One).Uint64())
	// But deduct full stolen amount from asset side
	c.Assert(updatedPool.BalanceAsset.Uint64(), Equals, cosmos.NewUint(2000*common.One).Uint64())
}

func (s *SlashingV92Suite) TestNeedsNewVault(c *C) {
	ctx, mgr := setupManagerForTest(c)

	inhash := GetRandomTxHash()
	outhash := GetRandomTxHash()
	sig1 := GetRandomBech32Addr()
	sig2 := GetRandomBech32Addr()
	sig3 := GetRandomBech32Addr()
	pk := GetRandomPubKey()
	tx := GetRandomTx()
	tx.ID = outhash
	obs := NewObservedTx(tx, 0, pk, 0)
	obs.ObservedPubKey = pk
	obs.Signers = []string{sig1.String(), sig2.String(), sig3.String()}

	voter := NewObservedTxVoter(outhash, []ObservedTx{obs})
	mgr.Keeper().SetObservedTxOutVoter(ctx, voter)

	mgr.Keeper().SetObservedLink(ctx, inhash, outhash)
	slasher := newSlasherV92(mgr.Keeper(), mgr.EventMgr())

	c.Check(slasher.needsNewVault(ctx, mgr, 10, 300, 1, inhash, pk), Equals, false)
	ctx = ctx.WithBlockHeight(600)
	c.Check(slasher.needsNewVault(ctx, mgr, 10, 300, 1, inhash, pk), Equals, false)
	ctx = ctx.WithBlockHeight(900)
	c.Check(slasher.needsNewVault(ctx, mgr, 10, 300, 1, inhash, pk), Equals, false)
	ctx = ctx.WithBlockHeight(1600)
	c.Check(slasher.needsNewVault(ctx, mgr, 10, 300, 1, inhash, pk), Equals, true)

	// test that more than 1/3rd will always return false
	ctx = ctx.WithBlockHeight(999999999)
	c.Check(slasher.needsNewVault(ctx, mgr, 9, 300, 1, inhash, pk), Equals, false)
}
