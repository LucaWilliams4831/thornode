package types

import (
	. "gopkg.in/check.v1"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
)

type TypeObservedTxSuite struct{}

var _ = Suite(&TypeObservedTxSuite{})

func (s TypeObservedTxSuite) TestVoter(c *C) {
	txID := GetRandomTxHash()
	bnb := GetRandomBNBAddress()
	acc1 := GetRandomBech32Addr()
	acc2 := GetRandomBech32Addr()
	acc3 := GetRandomBech32Addr()
	acc4 := GetRandomBech32Addr()

	accConsPub1 := GetRandomBech32ConsensusPubKey()
	accConsPub2 := GetRandomBech32ConsensusPubKey()
	accConsPub3 := GetRandomBech32ConsensusPubKey()
	accConsPub4 := GetRandomBech32ConsensusPubKey()

	accPubKeySet1 := GetRandomPubKeySet()
	accPubKeySet2 := GetRandomPubKeySet()
	accPubKeySet3 := GetRandomPubKeySet()
	accPubKeySet4 := GetRandomPubKeySet()

	tx1 := GetRandomTx()
	tx1.Memo = "hello"
	tx2 := GetRandomTx()
	observePoolAddr := GetRandomPubKey()
	voter := NewObservedTxVoter(txID, nil)

	obTx1 := NewObservedTx(tx1, 0, observePoolAddr, 1)
	obTx2 := NewObservedTx(tx2, 0, observePoolAddr, 1)

	c.Check(len(obTx1.String()) > 0, Equals, true)

	voter.Add(obTx1, acc1)
	c.Assert(voter.Txs, HasLen, 1)

	voter.Add(obTx1, acc1) // check THORNode don't duplicate the same signer
	c.Assert(voter.Txs, HasLen, 1)
	c.Assert(voter.Txs[0].Signers, HasLen, 1)

	voter.Add(obTx1, acc2) // append a signature
	c.Assert(voter.Txs, HasLen, 1)
	c.Assert(voter.Txs[0].Signers, HasLen, 2)

	voter.Add(obTx2, acc1) // same validator seeing a different version of tx
	c.Assert(voter.Txs, HasLen, 2)
	c.Assert(voter.Txs[0].Signers, HasLen, 2)

	voter.Add(obTx2, acc3) // second version
	c.Assert(voter.Txs, HasLen, 2)
	c.Assert(voter.Txs[0].Signers, HasLen, 2)
	c.Assert(voter.Txs[1].Signers, HasLen, 2)

	obTx1Finalised := NewObservedTx(tx1, 1, observePoolAddr, 1)
	obTx2Finalised := NewObservedTx(tx2, 1, observePoolAddr, 1)

	voter.Add(obTx1Finalised, acc1)
	c.Assert(voter.Txs, HasLen, 3)

	voter.Add(obTx1Finalised, acc1) // check THORNode don't duplicate the same signer
	c.Assert(voter.Txs, HasLen, 3)
	c.Assert(voter.Txs[2].Signers, HasLen, 1)

	voter.Add(obTx1Finalised, acc2) // append a signature
	c.Assert(voter.Txs, HasLen, 3)
	c.Assert(voter.Txs[2].Signers, HasLen, 2)

	voter.Add(obTx2Finalised, acc1) // same validator seeing a different version of tx
	c.Assert(voter.Txs, HasLen, 4)
	c.Assert(voter.Txs[3].Signers, HasLen, 1)

	voter.Add(obTx2Finalised, acc3) // second version
	c.Assert(voter.Txs, HasLen, 4)
	c.Assert(voter.Txs[2].Signers, HasLen, 2)
	c.Assert(voter.Txs[3].Signers, HasLen, 2)

	trusts3 := NodeAccounts{
		NodeAccount{
			NodeAddress:         acc1,
			Status:              NodeStatus_Active,
			PubKeySet:           accPubKeySet1,
			ValidatorConsPubKey: accConsPub1,
		},
		NodeAccount{
			NodeAddress:         acc2,
			Status:              NodeStatus_Active,
			PubKeySet:           accPubKeySet2,
			ValidatorConsPubKey: accConsPub2,
		},
		NodeAccount{
			NodeAddress:         acc3,
			Status:              NodeStatus_Active,
			PubKeySet:           accPubKeySet3,
			ValidatorConsPubKey: accConsPub3,
		},
	}
	trusts4 := NodeAccounts{
		NodeAccount{
			NodeAddress:         acc1,
			Status:              NodeStatus_Active,
			PubKeySet:           accPubKeySet1,
			ValidatorConsPubKey: accConsPub1,
		},
		NodeAccount{
			NodeAddress:         acc2,
			Status:              NodeStatus_Active,
			PubKeySet:           accPubKeySet2,
			ValidatorConsPubKey: accConsPub2,
		},
		NodeAccount{
			NodeAddress:         acc3,
			Status:              NodeStatus_Active,
			PubKeySet:           accPubKeySet3,
			ValidatorConsPubKey: accConsPub3,
		},
		NodeAccount{
			NodeAddress:         acc4,
			Status:              NodeStatus_Active,
			PubKeySet:           accPubKeySet4,
			ValidatorConsPubKey: accConsPub4,
		},
	}

	tx := voter.GetTx(trusts3)
	c.Check(tx.Tx.Memo, Equals, "hello")
	txFinalised := voter.GetTx(trusts3)
	c.Check(txFinalised.Tx.Memo, Equals, "hello")
	voter.Tx = ObservedTx{} // reset the final observed tx
	tx = voter.GetTx(trusts4)
	c.Check(tx.IsEmpty(), Equals, true)
	c.Assert(voter.HasConsensus(trusts3), Equals, true)
	voter.Tx = ObservedTx{} // reset the final observed tx
	c.Check(voter.HasConsensus(trusts4), Equals, false)
	c.Check(voter.Key().Equals(txID), Equals, true)
	c.Check(voter.String() == txID.String(), Equals, true)

	voter.Tx = ObservedTx{}
	txFinalised = voter.GetTx(trusts4)
	c.Check(txFinalised.IsEmpty(), Equals, true)
	c.Check(voter.HasFinalised(trusts3), Equals, true)
	voter.Tx = ObservedTx{}
	c.Check(voter.HasFinalised(trusts4), Equals, false)
	c.Check(voter.Key().Equals(txID), Equals, true)
	c.Check(voter.String() == txID.String(), Equals, true)

	thorchainCoins := common.Coins{
		common.NewCoin(common.RuneAsset(), cosmos.NewUint(100)),
		common.NewCoin(common.BNBAsset, cosmos.NewUint(100)),
	}
	inputs := []struct {
		coins           common.Coins
		memo            string
		sender          common.Address
		observePoolAddr common.PubKey
		blockHeight     int64
	}{
		{
			coins:           nil,
			memo:            "test",
			sender:          bnb,
			observePoolAddr: observePoolAddr,
			blockHeight:     1024,
		},
		{
			coins:           common.Coins{},
			memo:            "test",
			sender:          bnb,
			observePoolAddr: observePoolAddr,
			blockHeight:     1024,
		},
		{
			coins:           thorchainCoins,
			memo:            "test",
			sender:          common.NoAddress,
			observePoolAddr: observePoolAddr,
			blockHeight:     1024,
		},
		{
			coins:           thorchainCoins,
			memo:            "test",
			sender:          bnb,
			observePoolAddr: common.EmptyPubKey,
			blockHeight:     1024,
		},
		{
			coins:           thorchainCoins,
			memo:            "test",
			sender:          bnb,
			observePoolAddr: observePoolAddr,
			blockHeight:     0,
		},
	}

	for _, item := range inputs {
		tx := common.Tx{
			ID:          GetRandomTxHash(),
			Chain:       common.BNBChain,
			FromAddress: item.sender,
			ToAddress:   GetRandomBNBAddress(),
			Coins:       item.coins,
			Gas:         BNBGasFeeSingleton,
			Memo:        item.memo,
		}
		txIn := NewObservedTx(tx, item.blockHeight, item.observePoolAddr, item.blockHeight)
		c.Assert(txIn.Valid(), NotNil)
	}
}

func (TypeObservedTxSuite) TestSetTxToComplete(c *C) {
	activeNodes := NodeAccounts{
		GetRandomValidatorNode(NodeStatus_Active),
		GetRandomValidatorNode(NodeStatus_Active),
		GetRandomValidatorNode(NodeStatus_Active),
		GetRandomValidatorNode(NodeStatus_Active),
	}
	tx1 := GetRandomTx()
	tx1.Memo = "whatever"
	voter := NewObservedTxVoter(tx1.ID, nil)
	observePoolAddr := GetRandomPubKey()
	observedTx := NewObservedTx(tx1, 1024, observePoolAddr, 1028)
	voter.Add(observedTx, activeNodes[0].NodeAddress)
	voter.Add(observedTx, activeNodes[1].NodeAddress)
	voter.Add(observedTx, activeNodes[2].NodeAddress)
	c.Assert(voter.HasConsensus(activeNodes), Equals, true)
	consensusTx := voter.GetTx(activeNodes)
	c.Assert(consensusTx.IsEmpty(), Equals, false)
	c.Assert(voter.Tx.IsEmpty(), Equals, false)

	observedTx = NewObservedTx(tx1, 1024, observePoolAddr, 1024)
	voter.Add(observedTx, activeNodes[0].NodeAddress)
	voter.Add(observedTx, activeNodes[1].NodeAddress)
	voter.Add(observedTx, activeNodes[2].NodeAddress)
	c.Assert(voter.HasFinalised(activeNodes), Equals, true)
	finalisedTx := voter.GetTx(activeNodes)
	c.Assert(finalisedTx.IsEmpty(), Equals, false)
	c.Assert(voter.Tx.IsEmpty(), Equals, false)

	tx := GetRandomTx()
	addr, err := observePoolAddr.GetAddress(common.BNBChain)
	c.Assert(err, IsNil)
	tx.FromAddress = addr
	toi := TxOutItem{
		Chain:       tx.Chain,
		ToAddress:   tx.ToAddress,
		VaultPubKey: observePoolAddr,
		Coin:        tx.Coins[0],
		Memo:        "",
		GasRate:     1,
	}
	voter.Actions = append(voter.Actions, toi)
	version := GetCurrentVersion()
	c.Assert(voter.AddOutTx(version, tx), Equals, true)
	// add it again should return true, but without any real action
	c.Assert(voter.AddOutTx(version, tx), Equals, true)
	c.Assert(voter.AddOutTx(version, GetRandomTx()), Equals, false)
	c.Assert(voter.Tx.Status, Equals, Status_done)
	c.Assert(voter.Tx.OutHashes[0], Equals, tx.ID.String())

	c.Assert(voter.IsDone(), Equals, true)
	voter.Tx = voter.GetTx(activeNodes)
	c.Assert(voter.GetTx(activeNodes).Equals(voter.Tx), Equals, true)
}

func (TypeObservedTxSuite) TestAddOutTx(c *C) {
	activeNodes := NodeAccounts{
		GetRandomValidatorNode(NodeStatus_Active),
		GetRandomValidatorNode(NodeStatus_Active),
		GetRandomValidatorNode(NodeStatus_Active),
		GetRandomValidatorNode(NodeStatus_Active),
	}
	tx1 := GetRandomTx()
	tx1.Memo = "whatever"
	voter := NewObservedTxVoter(tx1.ID, nil)
	observePoolAddr := GetRandomPubKey()

	observedTx := NewObservedTx(tx1, 1024, observePoolAddr, 1024)
	voter.Add(observedTx, activeNodes[0].NodeAddress)
	voter.Add(observedTx, activeNodes[1].NodeAddress)
	voter.Add(observedTx, activeNodes[2].NodeAddress)
	c.Assert(voter.HasFinalised(activeNodes), Equals, true)
	finalisedTx := voter.GetTx(activeNodes)
	c.Assert(finalisedTx.IsEmpty(), Equals, false)
	c.Assert(voter.Tx.IsEmpty(), Equals, false)

	tx := common.NewTx(
		GetRandomTxHash(),
		GetRandomBNBAddress(),
		GetRandomBNBAddress(),
		common.Coins{
			common.NewCoin(common.BTCAsset, cosmos.NewUint(100010000)),
		},
		common.Gas{
			{Asset: common.BTCAsset, Amount: cosmos.NewUint(27500)},
		},
		"",
	)
	addr, err := observePoolAddr.GetAddress(common.BNBChain)
	c.Assert(err, IsNil)
	tx.FromAddress = addr

	toi := TxOutItem{
		Chain:       tx.Chain,
		ToAddress:   tx.ToAddress,
		VaultPubKey: observePoolAddr,
		Coin:        common.NewCoin(common.BTCAsset, cosmos.NewUint(common.One)),
		Memo:        "",
		GasRate:     1,
		MaxGas: common.Gas{
			{Asset: common.BTCAsset, Amount: cosmos.NewUint(37500)},
		},
	}
	voter.Actions = append(voter.Actions, toi)
	version := GetCurrentVersion()
	c.Assert(voter.AddOutTx(version, tx), Equals, true)
	// add it again should return true, but without any real action
	c.Assert(voter.AddOutTx(version, tx), Equals, true)
	c.Assert(voter.AddOutTx(version, GetRandomTx()), Equals, false)
	if !voter.Tx.IsEmpty() {
		c.Assert(voter.Tx.Status, Equals, Status_done)
		c.Assert(voter.Tx.OutHashes[0], Equals, tx.ID.String())
	}

	c.Assert(voter.IsDone(), Equals, true)
	voter.Tx = voter.GetTx(activeNodes)
	c.Assert(voter.GetTx(activeNodes).Equals(voter.Tx), Equals, true)
}

func (TypeObservedTxSuite) TestObservedTxEquals(c *C) {
	coins1 := common.Coins{
		common.NewCoin(common.BNBAsset, cosmos.NewUint(100*common.One)),
		common.NewCoin(common.RuneAsset(), cosmos.NewUint(100*common.One)),
	}
	coins2 := common.Coins{
		common.NewCoin(common.BNBAsset, cosmos.NewUint(100*common.One)),
	}
	coins3 := common.Coins{
		common.NewCoin(common.BNBAsset, cosmos.NewUint(200*common.One)),
		common.NewCoin(common.RuneAsset(), cosmos.NewUint(100*common.One)),
	}
	coins4 := common.Coins{
		common.NewCoin(common.RuneAsset(), cosmos.NewUint(100*common.One)),
		common.NewCoin(common.RuneAsset(), cosmos.NewUint(100*common.One)),
	}
	bnb, err := common.NewAddress("bnb1xlvns0n2mxh77mzaspn2hgav4rr4m8eerfju38")
	c.Assert(err, IsNil)
	bnb1, err := common.NewAddress("bnb1yk882gllgv3rt2rqrsudf6kn2agr94etnxu9a7")
	c.Assert(err, IsNil)
	observePoolAddr := GetRandomPubKey()
	observePoolAddr1 := GetRandomPubKey()
	inputs := []struct {
		tx    ObservedTx
		tx1   ObservedTx
		equal bool
	}{
		{
			tx:    NewObservedTx(common.Tx{FromAddress: bnb, ToAddress: GetRandomBNBAddress(), Coins: coins1, Memo: "memo", Gas: BNBGasFeeSingleton}, 0, observePoolAddr, 0),
			tx1:   NewObservedTx(common.Tx{FromAddress: bnb, ToAddress: GetRandomBNBAddress(), Coins: coins1, Memo: "memo1", Gas: BNBGasFeeSingleton}, 0, observePoolAddr, 0),
			equal: false,
		},
		{
			tx:    NewObservedTx(common.Tx{FromAddress: bnb, ToAddress: GetRandomBNBAddress(), Coins: coins1, Memo: "memo", Gas: BNBGasFeeSingleton}, 0, observePoolAddr, 0),
			tx1:   NewObservedTx(common.Tx{FromAddress: bnb1, ToAddress: GetRandomBNBAddress(), Coins: coins1, Memo: "memo", Gas: BNBGasFeeSingleton}, 0, observePoolAddr, 0),
			equal: false,
		},
		{
			tx:    NewObservedTx(common.Tx{Coins: coins2, Memo: "memo", FromAddress: bnb, ToAddress: GetRandomBNBAddress(), Gas: BNBGasFeeSingleton}, 0, observePoolAddr, 0),
			tx1:   NewObservedTx(common.Tx{Coins: coins1, Memo: "memo", FromAddress: bnb, ToAddress: GetRandomBNBAddress(), Gas: BNBGasFeeSingleton}, 0, observePoolAddr, 0),
			equal: false,
		},
		{
			tx:    NewObservedTx(common.Tx{Coins: coins3, Memo: "memo", FromAddress: bnb, ToAddress: GetRandomBNBAddress(), Gas: BNBGasFeeSingleton}, 0, observePoolAddr, 0),
			tx1:   NewObservedTx(common.Tx{Coins: coins1, Memo: "memo", FromAddress: bnb, ToAddress: GetRandomBNBAddress(), Gas: BNBGasFeeSingleton}, 0, observePoolAddr, 0),
			equal: false,
		},
		{
			tx:    NewObservedTx(common.Tx{Coins: coins4, Memo: "memo", FromAddress: bnb, ToAddress: GetRandomBNBAddress(), Gas: BNBGasFeeSingleton}, 0, observePoolAddr, 0),
			tx1:   NewObservedTx(common.Tx{Coins: coins1, Memo: "memo", FromAddress: bnb, ToAddress: GetRandomBNBAddress(), Gas: BNBGasFeeSingleton}, 0, observePoolAddr, 0),
			equal: false,
		},
		{
			tx:    NewObservedTx(common.Tx{Coins: coins1, Memo: "memo", FromAddress: bnb, ToAddress: GetRandomBNBAddress(), Gas: BNBGasFeeSingleton}, 0, observePoolAddr, 0),
			tx1:   NewObservedTx(common.Tx{Coins: coins1, Memo: "memo", FromAddress: bnb, ToAddress: GetRandomBNBAddress(), Gas: BNBGasFeeSingleton}, 0, observePoolAddr1, 0),
			equal: false,
		},
		{
			tx:    NewObservedTx(common.Tx{Coins: coins1, Memo: "memo", FromAddress: bnb, ToAddress: GetRandomBNBAddress(), Gas: BNBGasFeeSingleton}, 0, observePoolAddr, 0),
			tx1:   NewObservedTx(common.Tx{Coins: coins1, Memo: "memo", FromAddress: bnb, ToAddress: GetRandomBNBAddress(), Gas: BNBGasFeeSingleton}, 0, observePoolAddr, 0),
			equal: false,
		},
	}
	for _, item := range inputs {
		c.Assert(item.tx.Equals(item.tx1), Equals, item.equal)
	}

	// test aggregator scenarios
	addr := GetRandomBNBAddress()
	tx1 := NewObservedTx(common.Tx{FromAddress: bnb, ToAddress: addr, Coins: coins1, Memo: "memo", Gas: BNBGasFeeSingleton}, 0, observePoolAddr, 0)
	tx2 := NewObservedTx(common.Tx{FromAddress: bnb, ToAddress: addr, Coins: coins1, Memo: "memo", Gas: BNBGasFeeSingleton}, 0, observePoolAddr, 0)
	c.Assert(tx1.Equals(tx2), Equals, true)

	tx1.Aggregator = GetRandomBNBAddress().String()
	c.Assert(tx1.Equals(tx2), Equals, false)
	tx2.Aggregator = GetRandomBNBAddress().String()
	c.Assert(tx1.Equals(tx2), Equals, false)
	tx2.Aggregator = tx1.Aggregator
	c.Assert(tx1.Equals(tx2), Equals, true)

	tx1.AggregatorTarget = GetRandomETHAddress().String()
	c.Assert(tx1.Equals(tx2), Equals, false)
	tx2.AggregatorTarget = GetRandomETHAddress().String()
	c.Assert(tx1.Equals(tx2), Equals, false)
	tx2.AggregatorTarget = tx1.AggregatorTarget
	c.Assert(tx1.Equals(tx2), Equals, true)

	targetLimit := cosmos.NewUint(100)
	tx1.AggregatorTargetLimit = &targetLimit
	c.Assert(tx1.Equals(tx2), Equals, false)
	targetLimit = cosmos.NewUint(200)
	tx1.AggregatorTargetLimit = &targetLimit
	c.Assert(tx1.Equals(tx2), Equals, false)

	targetLimit = cosmos.NewUint(100)
	tx2.AggregatorTargetLimit = &targetLimit
	c.Assert(tx1.Equals(tx2), Equals, true)
}

func (TypeObservedTxSuite) TestObservedTxVote(c *C) {
	tx := GetRandomTx()
	voter := NewObservedTxVoter("", []ObservedTx{NewObservedTx(tx, 1, GetRandomPubKey(), 1)})
	c.Check(voter.Valid(), NotNil)

	voter1 := NewObservedTxVoter(GetRandomTxHash(), []ObservedTx{NewObservedTx(tx, 0, "", 0)})
	c.Check(voter1.Valid(), NotNil)

	voter2 := NewObservedTxVoter(GetRandomTxHash(), []ObservedTx{NewObservedTx(tx, 1024, GetRandomPubKey(), 1024)})
	c.Check(voter2.Valid(), IsNil)

	observedTx := NewObservedTx(GetRandomTx(), 1024, GetRandomPubKey(), 1024)
	addr := GetRandomBech32Addr()
	c.Check(observedTx.Sign(addr), Equals, true)
	c.Check(observedTx.Sign(addr), Equals, false)

	observedTx1 := NewObservedTx(observedTx.Tx, 1024, GetRandomPubKey(), 1024)
	c.Assert(observedTx.Equals(observedTx1), Equals, false)
	txID := GetRandomTxHash()
	version := GetCurrentVersion()
	observedTx1.SetDone(version, txID, 2)
	observedTx1.SetDone(version, txID, 2)
	c.Check(observedTx1.IsDone(2), Equals, false)
}

func (TypeObservedTxSuite) TestObservedTxGetConsensus(c *C) {
	txID := GetRandomTxHash()
	acc1 := GetRandomBech32Addr()
	acc2 := GetRandomBech32Addr()
	acc3 := GetRandomBech32Addr()
	acc4 := GetRandomBech32Addr()

	accConsPub1 := GetRandomBech32ConsensusPubKey()
	accConsPub2 := GetRandomBech32ConsensusPubKey()
	accConsPub3 := GetRandomBech32ConsensusPubKey()
	accConsPub4 := GetRandomBech32ConsensusPubKey()

	accPubKeySet1 := GetRandomPubKeySet()
	accPubKeySet2 := GetRandomPubKeySet()
	accPubKeySet3 := GetRandomPubKeySet()
	accPubKeySet4 := GetRandomPubKeySet()

	tx1 := GetRandomTx()
	tx1.Memo = "hello"
	observePoolAddr := GetRandomPubKey()

	voter := NewObservedTxVoter(txID, nil)
	obTx1 := NewObservedTx(tx1, 1, observePoolAddr, 1)
	obTx2 := NewObservedTx(tx1, 1, observePoolAddr, 2)

	c.Check(len(obTx1.String()) > 0, Equals, true)

	voter.Add(obTx1, acc1)
	c.Assert(voter.Txs, HasLen, 1)

	voter.Add(obTx1, acc1) // check THORNode don't duplicate the same signer
	c.Assert(voter.Txs, HasLen, 1)
	c.Assert(voter.Txs[0].Signers, HasLen, 1)

	voter.Add(obTx1, acc2) // append a signature
	c.Assert(voter.Txs, HasLen, 1)
	c.Assert(voter.Txs[0].Signers, HasLen, 2)

	voter.Add(obTx2, acc1) // same validator seeing a different version of tx
	c.Assert(voter.Txs, HasLen, 2)
	c.Assert(voter.Txs[0].Signers, HasLen, 2)

	voter.Add(obTx2, acc3) // second version
	c.Assert(voter.Txs, HasLen, 2)
	c.Assert(voter.Txs[0].Signers, HasLen, 2)
	c.Assert(voter.Txs[1].Signers, HasLen, 2)

	obTx1Finalised := NewObservedTx(tx1, 2, observePoolAddr, 2)

	voter.Add(obTx1Finalised, acc2)
	c.Assert(voter.Txs, HasLen, 3)

	voter.Add(obTx1Finalised, acc1) // check THORNode don't duplicate the same signer
	c.Assert(voter.Txs, HasLen, 3)
	c.Assert(voter.Txs[2].Signers, HasLen, 2)

	voter.Add(obTx1Finalised, acc2) // append a signature
	c.Assert(voter.Txs, HasLen, 3)
	c.Assert(voter.Txs[2].Signers, HasLen, 2)

	trusts4 := NodeAccounts{
		NodeAccount{
			NodeAddress:         acc1,
			Status:              NodeStatus_Active,
			PubKeySet:           accPubKeySet1,
			ValidatorConsPubKey: accConsPub1,
		},
		NodeAccount{
			NodeAddress:         acc2,
			Status:              NodeStatus_Active,
			PubKeySet:           accPubKeySet2,
			ValidatorConsPubKey: accConsPub2,
		},
		NodeAccount{
			NodeAddress:         acc3,
			Status:              NodeStatus_Active,
			PubKeySet:           accPubKeySet3,
			ValidatorConsPubKey: accConsPub3,
		},
		NodeAccount{
			NodeAddress:         acc4,
			Status:              NodeStatus_Active,
			PubKeySet:           accPubKeySet4,
			ValidatorConsPubKey: accConsPub4,
		},
	}

	tx := voter.GetTx(trusts4)
	c.Assert(tx.IsEmpty(), Equals, true)

	voter.Add(obTx1Finalised, acc4) // append a signature
	c.Assert(voter.Txs, HasLen, 3)
	c.Assert(voter.Txs[2].Signers, HasLen, 3)

	tx = voter.GetTx(trusts4)
	c.Assert(tx.IsEmpty(), Equals, false)
	c.Assert(tx.Equals(obTx1), Equals, true)
}

func (TypeObservedTxSuite) TestNewGetConsensusTx(c *C) {
	txID := GetRandomTxHash()
	acc1 := GetRandomBech32Addr()
	acc2 := GetRandomBech32Addr()
	acc3 := GetRandomBech32Addr()
	acc4 := GetRandomBech32Addr()

	accConsPub1 := GetRandomBech32ConsensusPubKey()
	accConsPub2 := GetRandomBech32ConsensusPubKey()
	accConsPub3 := GetRandomBech32ConsensusPubKey()
	accConsPub4 := GetRandomBech32ConsensusPubKey()

	accPubKeySet1 := GetRandomPubKeySet()
	accPubKeySet2 := GetRandomPubKeySet()
	accPubKeySet3 := GetRandomPubKeySet()
	accPubKeySet4 := GetRandomPubKeySet()

	trusts4 := NodeAccounts{
		NodeAccount{
			NodeAddress:         acc1,
			Status:              NodeStatus_Active,
			PubKeySet:           accPubKeySet1,
			ValidatorConsPubKey: accConsPub1,
		},
		NodeAccount{
			NodeAddress:         acc2,
			Status:              NodeStatus_Active,
			PubKeySet:           accPubKeySet2,
			ValidatorConsPubKey: accConsPub2,
		},
		NodeAccount{
			NodeAddress:         acc3,
			Status:              NodeStatus_Active,
			PubKeySet:           accPubKeySet3,
			ValidatorConsPubKey: accConsPub3,
		},
		NodeAccount{
			NodeAddress:         acc4,
			Status:              NodeStatus_Active,
			PubKeySet:           accPubKeySet4,
			ValidatorConsPubKey: accConsPub4,
		},
	}
	tx1 := GetRandomTx()
	tx1.Memo = "hello"
	tx1.ID = txID
	observePoolAddr := GetRandomPubKey()
	voter := NewObservedTxVoter(txID, nil)

	txForged := GetRandomTx()
	txForged.ID = txID
	obTx1 := NewObservedTx(txForged, 1, observePoolAddr, 1)
	obTx2 := NewObservedTx(tx1, 1, observePoolAddr, 2)

	obTx3 := NewObservedTx(tx1, 2, observePoolAddr, 2)

	c.Assert(voter.Add(obTx1, acc1), Equals, true)
	c.Assert(voter.Add(obTx2, acc2), Equals, true)
	c.Assert(voter.Add(obTx2, acc3), Equals, true)
	c.Assert(voter.Add(obTx2, acc4), Equals, true)
	c.Assert(voter.HasFinalised(trusts4), Equals, false)
	c.Assert(voter.HasConsensus(trusts4), Equals, true)
	tx := voter.GetTx(trusts4)

	c.Assert(tx.Tx.Equals(tx1), Equals, true)
	c.Assert(voter.Add(obTx3, acc2), Equals, true)
	c.Assert(voter.Add(obTx3, acc3), Equals, true)
	c.Assert(voter.Add(obTx3, acc4), Equals, true)

	c.Assert(voter.HasFinalised(trusts4), Equals, true)
	txGood := voter.GetTx(trusts4)
	c.Assert(txGood.IsEmpty(), Equals, false)
	c.Assert(txGood.Equals(obTx3), Equals, true)
}
