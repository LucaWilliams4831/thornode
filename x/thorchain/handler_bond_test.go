package thorchain

import (
	"errors"
	"fmt"
	"strings"

	se "github.com/cosmos/cosmos-sdk/types/errors"
	. "gopkg.in/check.v1"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
	"gitlab.com/thorchain/thornode/x/thorchain/keeper"
)

type HandlerBondSuite struct{}

type TestBondKeeper struct {
	keeper.Keeper
	standbyNodeAccount  NodeAccount
	failGetNodeAccount  NodeAccount
	notEmptyNodeAccount NodeAccount
}

func (k *TestBondKeeper) GetNodeAccount(_ cosmos.Context, addr cosmos.AccAddress) (NodeAccount, error) {
	if k.standbyNodeAccount.NodeAddress.Equals(addr) {
		return k.standbyNodeAccount, nil
	}
	if k.failGetNodeAccount.NodeAddress.Equals(addr) {
		return NodeAccount{}, fmt.Errorf("you asked for this error")
	}
	if k.notEmptyNodeAccount.NodeAddress.Equals(addr) {
		return k.notEmptyNodeAccount, nil
	}
	return NodeAccount{}, nil
}

var _ = Suite(&HandlerBondSuite{})

func (HandlerBondSuite) TestBondHandler_ValidateActive(c *C) {
	ctx, k := setupKeeperForTest(c)

	activeNodeAccount := GetRandomValidatorNode(NodeActive)
	c.Assert(k.SetNodeAccount(ctx, activeNodeAccount), IsNil)

	vault := GetRandomVault()
	vault.Status = RetiringVault
	c.Assert(k.SetVault(ctx, vault), IsNil)

	handler := NewBondHandler(NewDummyMgrWithKeeper(k))

	txIn := common.NewTx(
		GetRandomTxHash(),
		GetRandomBNBAddress(),
		GetRandomBNBAddress(),
		common.Coins{
			common.NewCoin(common.RuneAsset(), cosmos.NewUint(10*common.One)),
		},
		BNBGasFeeSingleton,
		"bond",
	)
	msg := NewMsgBond(txIn, activeNodeAccount.NodeAddress, cosmos.NewUint(10*common.One), activeNodeAccount.BondAddress, nil, activeNodeAccount.NodeAddress, -1)

	// happy path
	c.Assert(handler.validate(ctx, *msg), IsNil)

	vault.Status = ActiveVault
	c.Assert(k.SetVault(ctx, vault), IsNil)

	// node should be able to bond even it is active
	c.Assert(handler.validate(ctx, *msg), IsNil)
}

func (HandlerBondSuite) TestBondHandler_Run(c *C) {
	ctx, k1 := setupKeeperForTest(c)

	standbyNodeAccount := GetRandomValidatorNode(NodeStandby)
	k := &TestBondKeeper{
		Keeper:              k1,
		standbyNodeAccount:  standbyNodeAccount,
		failGetNodeAccount:  GetRandomValidatorNode(NodeStandby),
		notEmptyNodeAccount: GetRandomValidatorNode(NodeStandby),
	}
	// happy path
	c.Assert(k1.SetNodeAccount(ctx, standbyNodeAccount), IsNil)
	handler := NewBondHandler(NewDummyMgrWithKeeper(k1))
	ver := GetCurrentVersion()
	constAccessor := constants.GetConstantValues(ver)
	minimumBondInRune := constAccessor.GetInt64Value(constants.MinimumBondInRune)
	txIn := common.NewTx(
		GetRandomTxHash(),
		GetRandomTHORAddress(),
		GetRandomTHORAddress(),
		common.Coins{
			common.NewCoin(common.RuneAsset(), cosmos.NewUint(uint64(minimumBondInRune+common.One))),
		},
		BNBGasFeeSingleton,
		"bond",
	)
	FundModule(c, ctx, k1, BondName, uint64(minimumBondInRune))
	msg := NewMsgBond(txIn, GetRandomValidatorNode(NodeStandby).NodeAddress, cosmos.NewUint(uint64(minimumBondInRune)+common.One), GetRandomTHORAddress(), nil, standbyNodeAccount.NodeAddress, -1)
	_, err := handler.Run(ctx, msg)
	c.Assert(err, IsNil)
	coin := common.NewCoin(common.RuneNative, cosmos.NewUint(common.One))
	nativeRuneCoin, err := coin.Native()
	c.Assert(err, IsNil)
	c.Assert(k1.HasCoins(ctx, msg.NodeAddress, cosmos.NewCoins(nativeRuneCoin)), Equals, true)
	na, err := k1.GetNodeAccount(ctx, msg.NodeAddress)
	c.Assert(err, IsNil)
	c.Assert(na.Status.String(), Equals, NodeWhiteListed.String())
	c.Assert(na.Bond.Equal(cosmos.NewUint(uint64(minimumBondInRune))), Equals, true)

	// simulate fail to get node account
	handler = NewBondHandler(NewDummyMgrWithKeeper(k))
	msg = NewMsgBond(txIn, k.failGetNodeAccount.NodeAddress, cosmos.NewUint(uint64(minimumBondInRune)), GetRandomTHORAddress(), nil, standbyNodeAccount.NodeAddress, -1)
	_, err = handler.Run(ctx, msg)
	c.Assert(errors.Is(err, errInternal), Equals, true)

	// When node account is standby , it is ok to bond
	msg = NewMsgBond(txIn, k.notEmptyNodeAccount.NodeAddress, cosmos.NewUint(uint64(minimumBondInRune)), common.Address(k.notEmptyNodeAccount.NodeAddress.String()), nil, standbyNodeAccount.NodeAddress, -1)
	_, err = handler.Run(ctx, msg)
	c.Assert(err, IsNil)
}

func (HandlerBondSuite) TestBondHandlerFailValidation(c *C) {
	ctx, k := setupKeeperForTest(c)
	standbyNodeAccount := GetRandomValidatorNode(NodeStandby)
	c.Assert(k.SetNodeAccount(ctx, standbyNodeAccount), IsNil)
	handler := NewBondHandler(NewDummyMgrWithKeeper(k))
	ver := GetCurrentVersion()
	constAccessor := constants.GetConstantValues(ver)
	minimumBondInRune := constAccessor.GetInt64Value(constants.MinimumBondInRune)
	txIn := common.NewTx(
		GetRandomTxHash(),
		GetRandomTHORAddress(),
		GetRandomTHORAddress(),
		common.Coins{
			common.NewCoin(common.RuneAsset(), cosmos.NewUint(uint64(minimumBondInRune))),
		},
		BNBGasFeeSingleton,
		"apply",
	)
	txInNoTxID := txIn
	txInNoTxID.ID = ""
	testCases := []struct {
		name        string
		msg         *MsgBond
		expectedErr error
	}{
		{
			name:        "empty node address",
			msg:         NewMsgBond(txIn, cosmos.AccAddress{}, cosmos.NewUint(uint64(minimumBondInRune)), GetRandomTHORAddress(), nil, standbyNodeAccount.NodeAddress, -1),
			expectedErr: se.ErrInvalidAddress,
		},
		{
			name:        "zero bond",
			msg:         NewMsgBond(txIn, GetRandomValidatorNode(NodeStandby).NodeAddress, cosmos.ZeroUint(), GetRandomTHORAddress(), nil, standbyNodeAccount.NodeAddress, -1),
			expectedErr: se.ErrUnknownRequest,
		},
		{
			name:        "empty bond address",
			msg:         NewMsgBond(txIn, GetRandomValidatorNode(NodeStandby).NodeAddress, cosmos.NewUint(uint64(minimumBondInRune)), common.Address(""), nil, standbyNodeAccount.NodeAddress, -1),
			expectedErr: se.ErrInvalidAddress,
		},
		{
			name:        "empty request hash",
			msg:         NewMsgBond(txInNoTxID, GetRandomValidatorNode(NodeStandby).NodeAddress, cosmos.NewUint(uint64(minimumBondInRune)), GetRandomTHORAddress(), nil, standbyNodeAccount.NodeAddress, -1),
			expectedErr: se.ErrUnknownRequest,
		},
		{
			name:        "empty signer",
			msg:         NewMsgBond(txIn, GetRandomValidatorNode(NodeStandby).NodeAddress, cosmos.NewUint(uint64(minimumBondInRune)), GetRandomTHORAddress(), nil, cosmos.AccAddress{}, -1),
			expectedErr: se.ErrInvalidAddress,
		},
		{
			name:        "active node",
			msg:         NewMsgBond(txIn, GetRandomValidatorNode(NodeActive).NodeAddress, cosmos.NewUint(uint64(minimumBondInRune)), GetRandomBNBAddress(), nil, cosmos.AccAddress{}, -1),
			expectedErr: se.ErrInvalidAddress,
		},
	}
	for _, item := range testCases {
		c.Log(item.name)
		_, err := handler.Run(ctx, item.msg)
		c.Check(errors.Is(err, item.expectedErr), Equals, true, Commentf("name: %s, %s != %s", item.name, item.expectedErr, err))
	}
}

func (HandlerBondSuite) TestBondProvider_Validate(c *C) {
	ctx, k := setupKeeperForTest(c)
	activeNodeAccount := GetRandomValidatorNode(NodeActive)
	c.Assert(k.SetNodeAccount(ctx, activeNodeAccount), IsNil)
	standbyNodeAccount := GetRandomValidatorNode(NodeStandby)
	c.Assert(k.SetNodeAccount(ctx, standbyNodeAccount), IsNil)
	handler := NewBondHandler(NewDummyMgrWithKeeper(k))
	txIn := GetRandomTx()
	amt := cosmos.NewUint(100 * common.One)
	txIn.Coins = common.NewCoins(common.NewCoin(common.RuneAsset(), amt))
	activeNA := activeNodeAccount.NodeAddress
	activeNAAddress := common.Address(activeNA.String())
	standbyNA := standbyNodeAccount.NodeAddress
	standbyNAAddress := common.Address(standbyNA.String())
	additionalBondAddress := GetRandomBech32Addr()

	errCheck := func(c *C, err error, str string) {
		c.Check(strings.Contains(err.Error(), str), Equals, true, Commentf("%s != %w", str, err))
	}

	// TEST VALIDATION //
	// happy path
	msg := NewMsgBond(txIn, standbyNA, amt, standbyNAAddress, additionalBondAddress, activeNA, -1)
	err := handler.validate(ctx, *msg)
	c.Assert(err, IsNil)

	// try to bond while node account is active should success
	msg = NewMsgBond(txIn, activeNA, amt, activeNAAddress, nil, activeNA, -1)
	err = handler.validate(ctx, *msg)
	c.Assert(err, IsNil)

	// try to bond with a bnb address
	msg = NewMsgBond(txIn, standbyNA, amt, GetRandomBNBAddress(), nil, activeNA, -1)
	err = handler.validate(ctx, *msg)
	errCheck(c, err, "bonding address is NOT a THORChain address")

	// try to bond with a valid additional bond provider
	bp := NewBondProviders(standbyNA)
	bp.Providers = []BondProvider{NewBondProvider(additionalBondAddress)}
	c.Assert(k.SetBondProviders(ctx, bp), IsNil)
	msg = NewMsgBond(txIn, standbyNA, amt, common.Address(additionalBondAddress.String()), nil, activeNA, -1)
	err = handler.validate(ctx, *msg)
	c.Assert(err, IsNil)

	// try to bond with an invalid additional bond provider
	msg = NewMsgBond(txIn, standbyNA, amt, GetRandomTHORAddress(), nil, activeNA, -1)
	err = handler.validate(ctx, *msg)
	errCheck(c, err, "bond address is not valid for node account")
}

func (HandlerBondSuite) TestBondProvider_OperatorFee(c *C) {
	ctx, k := setupKeeperForTest(c)
	handler := NewBondHandler(NewDummyMgrWithKeeper(k))

	standbyNodeAccount := GetRandomValidatorNode(NodeStandby)
	operatorBondAddress := GetRandomRUNEAddress()
	operatorAccAddress, _ := operatorBondAddress.AccAddress()
	providerBondAddress := GetRandomRUNEAddress()
	providerAccAddr, _ := providerBondAddress.AccAddress()
	standbyNodeAccount.BondAddress = operatorBondAddress

	c.Assert(k.SetNodeAccount(ctx, standbyNodeAccount), IsNil)

	standbyNodeAddr := standbyNodeAccount.NodeAddress
	amt := cosmos.NewUint(100 * common.One)
	txIn := GetRandomTx()
	txIn.Coins = common.NewCoins(common.NewCoin(common.RuneAsset(), amt))

	/* Test Validation and Handling */

	// happy path should be able to set node operator fee
	msg := NewMsgBond(txIn, standbyNodeAddr, amt, operatorBondAddress, providerAccAddr, operatorAccAddress, 5000)
	err := handler.validate(ctx, *msg)
	c.Assert(err, IsNil)

	err = handler.handle(ctx, *msg)
	c.Assert(err, IsNil)
	bp, _ := k.GetBondProviders(ctx, standbyNodeAccount.NodeAddress)
	c.Assert(bp.NodeOperatorFee.Uint64(), Equals, uint64(5000))

	// Check that a bond provider for the operator + new provider was added
	c.Assert(len(bp.Providers), Equals, 2)

	// try to increase operator fee after provider has bonded, should success , because bond providers should trust each other
	bp.Providers[1].Bond = cosmos.NewUint(100)
	bp.NodeOperatorFee = cosmos.NewUint(5000)
	c.Assert(k.SetBondProviders(ctx, bp), IsNil)
	msg = NewMsgBond(txIn, standbyNodeAddr, amt, operatorBondAddress, providerAccAddr, operatorAccAddress, 6000)
	err = handler.validate(ctx, *msg)
	c.Assert(err, IsNil)

	// Should be able to decrease operator fee after provider has bonded
	msg = NewMsgBond(txIn, standbyNodeAddr, amt, operatorBondAddress, providerAccAddr, operatorAccAddress, 4000)
	err = handler.validate(ctx, *msg)
	c.Assert(err, IsNil)

	err = handler.handle(ctx, *msg)
	c.Assert(err, IsNil)
	bp, _ = k.GetBondProviders(ctx, standbyNodeAccount.NodeAddress)
	c.Assert(bp.NodeOperatorFee.Uint64(), Equals, uint64(4000))

	// Only operator can set operator fee
	msg = NewMsgBond(txIn, standbyNodeAddr, amt, providerBondAddress, providerAccAddr, providerAccAddr, 0)
	err = handler.validate(ctx, *msg)
	c.Assert(err.Error(), Equals, "only node operator can set fee: unknown request")

	msg = NewMsgBond(txIn, standbyNodeAddr, amt, providerBondAddress, providerAccAddr, providerAccAddr, 4000)
	err = handler.validate(ctx, *msg)
	c.Assert(err.Error(), Equals, "only node operator can set fee: unknown request")

	// If nodeAcc.BondAddress is empty, any address should be able to set operator fee (and become bonder address)
	standbyNodeAccount.BondAddress = common.NoAddress
	c.Assert(k.SetNodeAccount(ctx, standbyNodeAccount), IsNil)
	msg = NewMsgBond(txIn, standbyNodeAddr, amt, providerBondAddress, providerAccAddr, providerAccAddr, 4000)
	err = handler.validate(ctx, *msg)
	c.Assert(err, IsNil)
}

func (HandlerBondSuite) TestBondProvider_Handler(c *C) {
	ctx, k := setupKeeperForTest(c)
	activeNodeAccount := GetRandomValidatorNode(NodeActive)
	c.Assert(k.SetNodeAccount(ctx, activeNodeAccount), IsNil)
	standbyNodeAccount := GetRandomValidatorNode(NodeStandby)
	c.Assert(k.SetNodeAccount(ctx, standbyNodeAccount), IsNil)
	handler := NewBondHandler(NewDummyMgrWithKeeper(k))
	txIn := GetRandomTx()
	amt := cosmos.NewUint(100 * common.One)
	txIn.Coins = common.NewCoins(common.NewCoin(common.RuneAsset(), amt))
	activeNA := activeNodeAccount.NodeAddress
	standbyNA := standbyNodeAccount.NodeAddress
	standbyNAAddress := common.Address(standbyNA.String())
	additionalBondAddress := GetRandomBech32Addr()
	FundAccount(c, ctx, k, standbyNA, amt.Uint64())
	FundAccount(c, ctx, k, activeNA, amt.Uint64())

	// TEST HANDLER //
	// happy path, and add a whitelisted address
	msg := NewMsgBond(txIn, standbyNA, amt, standbyNAAddress, additionalBondAddress, activeNA, -1)
	err := handler.handle(ctx, *msg)
	c.Assert(err, IsNil)
	na, _ := handler.mgr.Keeper().GetNodeAccount(ctx, standbyNA)
	c.Check(na.Bond.Uint64(), Equals, amt.Uint64()+(100*common.One), Commentf("%d", na.Bond.Uint64()))
	bp, _ := k.GetBondProviders(ctx, standbyNA)
	c.Assert(bp.Providers, HasLen, 2)
	c.Assert(bp.Has(additionalBondAddress), Equals, true)
	// New BP should have no bond
	c.Assert(bp.Get(additionalBondAddress).Bond.Uint64(), Equals, uint64(0), Commentf("%d", bp.Get(additionalBondAddress).Bond.Uint64()))
	// First BP should have its added bond, and it should have the orignal 100 bond that the node was created with - bond is re-distributed
	// to current BPs before new bond is added
	c.Assert(bp.Get(standbyNA).Bond.Uint64(), Equals, cosmos.NewUint(200*common.One).Uint64(), Commentf("%d", bp.Get(standbyNA).Bond.Uint64()))

	// bond with additional bonder
	msg = NewMsgBond(txIn, standbyNA, amt, common.Address(additionalBondAddress.String()), nil, standbyNA, -1)
	err = handler.handle(ctx, *msg)
	c.Assert(err, IsNil)
	bp, _ = k.GetBondProviders(ctx, standbyNA)
	c.Assert(bp.Providers, HasLen, 2)
	c.Assert(bp.Has(additionalBondAddress), Equals, true)
	c.Assert(bp.Get(additionalBondAddress).Bond.Uint64(), Equals, amt.Uint64(), Commentf("%d", bp.Get(additionalBondAddress).Bond.Uint64()))

	// bond with random bonder (doesnt' add new provider, still 2) - effectively adding to rewards, since this random address is not a BP
	msg = NewMsgBond(txIn, standbyNA, amt, GetRandomTHORAddress(), nil, activeNA, -1)
	err = handler.handle(ctx, *msg)
	c.Assert(err, IsNil)
	bp, _ = k.GetBondProviders(ctx, standbyNA)
	c.Assert(bp.Providers, HasLen, 2)

	// Set the node operator fee to 5%
	bp, _ = k.GetBondProviders(ctx, standbyNA)
	bp.NodeOperatorFee = cosmos.NewUint(500)
	c.Assert(k.SetBondProviders(ctx, bp), IsNil)

	// Simulate Network Rewards by adding to the bond
	na, _ = handler.mgr.Keeper().GetNodeAccount(ctx, standbyNA)
	na.Bond = na.Bond.Add(cosmos.NewUint(200 * common.One))
	c.Assert(k.SetNodeAccount(ctx, na), IsNil)

	// Check total bond - 600 RUNE
	na, _ = handler.mgr.Keeper().GetNodeAccount(ctx, standbyNA)
	c.Check(na.Bond.Uint64(), Equals, cosmos.NewUint(600*common.One).Uint64(), Commentf("%d", na.Bond.Uint64()))

	// Check BP bond - Node Operator should have 200 (2/3rd share), BP #2 should have 100 (1/3rd share)
	// This means there are 300 RUNE in rewards to distribute (NodeAccount.Bond - sum(BondProviders bond))
	bp, _ = k.GetBondProviders(ctx, standbyNA)
	c.Assert(bp.Get(additionalBondAddress).Bond.Uint64(), Equals, cosmos.NewUint(100*common.One).Uint64(), Commentf("%d", bp.Get(additionalBondAddress).Bond.Uint64()))
	c.Assert(bp.Get(standbyNA).Bond.Uint64(), Equals, cosmos.NewUint(200*common.One).Uint64(), Commentf("%d", bp.Get(standbyNA).Bond.Uint64()))

	// BP #2 Adds more bond after rewards were earned - rewards should be distributed first
	msg = NewMsgBond(txIn, standbyNA, amt, common.Address(additionalBondAddress.String()), nil, additionalBondAddress, -1)
	err = handler.handle(ctx, *msg)
	c.Assert(err, IsNil)

	// Check BP Bond - Node Operator Should have 2/3rd rewards + 5% operator fee of BP#2 rewards (405)
	// BP#2 should have 1/3rd rewards - 5% operator fee + the 100 RUNE new bond (295)
	bp, _ = k.GetBondProviders(ctx, standbyNA)
	c.Assert(bp.Providers, HasLen, 2)
	c.Assert(bp.Get(additionalBondAddress).Bond.Uint64(), Equals, cosmos.NewUint(295*common.One).Uint64(), Commentf("%d", bp.Get(additionalBondAddress).Bond.Uint64()))
	c.Assert(bp.Get(standbyNA).Bond.Uint64(), Equals, cosmos.NewUint(405*common.One).Uint64(), Commentf("%d", bp.Get(standbyNA).Bond.Uint64()))
}
