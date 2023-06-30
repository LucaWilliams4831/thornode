package thorchain

import (
	"errors"
	"fmt"

	se "github.com/cosmos/cosmos-sdk/types/errors"
	abci "github.com/tendermint/tendermint/abci/types"
	. "gopkg.in/check.v1"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
	"gitlab.com/thorchain/thornode/x/thorchain/keeper"
)

type HandlerUnBondSuite struct{}

var errReturnYgg = errors.New("returnYgg")

type BlankValidatorManager struct {
	ValidatorDummyMgr
}

func (vm BlankValidatorManager) BeginBlock(_ cosmos.Context, _ Manager, _ []string) error {
	return nil
}

func (vm BlankValidatorManager) EndBlock(_ cosmos.Context, _ Manager) []abci.ValidatorUpdate {
	return nil
}

func (vm BlankValidatorManager) RequestYggReturn(_ cosmos.Context, _ NodeAccount, _ Manager) error {
	return errReturnYgg
}

func (vm BlankValidatorManager) processRagnarok(_ cosmos.Context, _ Manager) error {
	return nil
}

func (vm BlankValidatorManager) NodeAccountPreflightCheck(ctx cosmos.Context, na NodeAccount, constAccessor constants.ConstantValues) (NodeStatus, error) {
	return NodeActive, nil
}

type TestUnBondKeeper struct {
	keeper.KVStoreDummy
	activeNodeAccount   NodeAccount
	failGetNodeAccount  NodeAccount
	notEmptyNodeAccount NodeAccount
	jailNodeAccount     NodeAccount
	vault               Vault
}

func (k *TestUnBondKeeper) GetNodeAccount(_ cosmos.Context, addr cosmos.AccAddress) (NodeAccount, error) {
	if k.activeNodeAccount.NodeAddress.Equals(addr) {
		return k.activeNodeAccount, nil
	}
	if k.failGetNodeAccount.NodeAddress.Equals(addr) {
		return NodeAccount{}, fmt.Errorf("you asked for this error")
	}
	if k.notEmptyNodeAccount.NodeAddress.Equals(addr) {
		return k.notEmptyNodeAccount, nil
	}
	if k.jailNodeAccount.NodeAddress.Equals(addr) {
		return k.jailNodeAccount, nil
	}
	return NodeAccount{}, nil
}

func (k *TestUnBondKeeper) GetVault(ctx cosmos.Context, pk common.PubKey) (Vault, error) {
	if k.vault.PubKey.Equals(pk) {
		return k.vault, nil
	}
	return k.KVStoreDummy.GetVault(ctx, pk)
}

func (k *TestUnBondKeeper) VaultExists(ctx cosmos.Context, pkey common.PubKey) bool {
	return k.vault.PubKey.Equals(pkey)
}

func (k *TestUnBondKeeper) GetNodeAccountJail(ctx cosmos.Context, addr cosmos.AccAddress) (Jail, error) {
	if k.jailNodeAccount.NodeAddress.Equals(addr) {
		return Jail{
			NodeAddress:   addr,
			ReleaseHeight: ctx.BlockHeight() + 100,
			Reason:        "bad boy",
		}, nil
	}
	return Jail{}, nil
}

func (k *TestUnBondKeeper) GetBondProviders(_ cosmos.Context, acc cosmos.AccAddress) (BondProviders, error) {
	return NewBondProviders(acc), nil
}

func (k *TestUnBondKeeper) GetAsgardVaultsByStatus(_ cosmos.Context, status VaultStatus) (Vaults, error) {
	if status == k.vault.Status {
		return Vaults{k.vault}, nil
	}
	return nil, nil
}

func (k *TestUnBondKeeper) GetMostSecure(_ cosmos.Context, vaults Vaults, _ int64) Vault {
	if len(vaults) == 0 {
		return Vault{}
	}
	return vaults[0]
}

var _ = Suite(&HandlerUnBondSuite{})

func (HandlerUnBondSuite) TestUnBondHandler_Run(c *C) {
	ctx, k1 := setupKeeperForTest(c)
	// happy path
	activeNodeAccount := GetRandomValidatorNode(NodeActive)
	standbyNodeAccount := GetRandomValidatorNode(NodeStandby)
	c.Assert(k1.SetNodeAccount(ctx, activeNodeAccount), IsNil)
	c.Assert(k1.SetNodeAccount(ctx, standbyNodeAccount), IsNil)
	vault := NewVault(12, ActiveVault, YggdrasilVault, standbyNodeAccount.PubKeySet.Secp256k1, nil, []ChainContract{})
	c.Assert(k1.SetVault(ctx, vault), IsNil)
	vault = NewVault(12, ActiveVault, AsgardVault, GetRandomPubKey(), nil, []ChainContract{})
	vault.Coins = common.Coins{
		common.NewCoin(common.RuneAsset(), cosmos.NewUint(10000*common.One)),
	}
	c.Assert(k1.SetVault(ctx, vault), IsNil)

	handler := NewUnBondHandler(NewDummyMgrWithKeeper(k1))
	txIn := common.NewTx(
		GetRandomTxHash(),
		standbyNodeAccount.BondAddress,
		GetRandomBNBAddress(),
		common.Coins{
			common.NewCoin(common.RuneAsset(), cosmos.NewUint(uint64(1))),
		},
		BNBGasFeeSingleton,
		"unbond me please",
	)
	msg := NewMsgUnBond(txIn, standbyNodeAccount.NodeAddress, cosmos.NewUint(uint64(5*common.One)), standbyNodeAccount.BondAddress, nil, activeNodeAccount.NodeAddress)
	_, err := handler.Run(ctx, msg)
	c.Assert(err, IsNil)
	na, err := k1.GetNodeAccount(ctx, standbyNodeAccount.NodeAddress)
	c.Assert(err, IsNil)
	c.Assert(na.Bond.Equal(cosmos.NewUint(95*common.One+1)), Equals, true, Commentf("%d", na.Bond.Uint64()))

	// test unbonding for 1 rune, should fail, and NOT increase bond with inbound rune
	mgrBad := NewDummyMgr()
	mgrBad.txOutStore = NewTxStoreFailDummy()
	handler.mgr = mgrBad
	msg = NewMsgUnBond(txIn, standbyNodeAccount.NodeAddress, cosmos.NewUint(uint64(1*common.One)), standbyNodeAccount.BondAddress, nil, activeNodeAccount.NodeAddress)
	_, err = handler.Run(ctx, msg)
	c.Assert(err, NotNil)
	na, err = k1.GetNodeAccount(ctx, standbyNodeAccount.NodeAddress)
	c.Assert(err, IsNil)
	c.Check(na.Bond.Equal(cosmos.NewUint(95*common.One+1)), Equals, true, Commentf("%d", na.Bond.Uint64()))
	handler.mgr = NewDummyMgr()

	k := &TestUnBondKeeper{
		activeNodeAccount:   activeNodeAccount,
		failGetNodeAccount:  GetRandomValidatorNode(NodeActive),
		notEmptyNodeAccount: standbyNodeAccount,
		jailNodeAccount:     GetRandomValidatorNode(NodeStandby),
	}
	mgr := NewDummyMgrWithKeeper(k)
	mgr.validatorMgr = BlankValidatorManager{}
	handler = NewUnBondHandler(mgr)

	// simulate fail to get node account
	msg = NewMsgUnBond(txIn, k.failGetNodeAccount.NodeAddress, cosmos.NewUint(uint64(1)), GetRandomBNBAddress(), nil, activeNodeAccount.NodeAddress)
	_, err = handler.Run(ctx, msg)
	c.Assert(errors.Is(err, errInternal), Equals, true)

	// simulate vault with funds
	k.vault = Vault{
		Type: YggdrasilVault,
		Coins: common.Coins{
			common.NewCoin(common.BNBAsset, cosmos.NewUint(uint64(1))),
		},
		PubKey: standbyNodeAccount.PubKeySet.Secp256k1,
	}
	msg = NewMsgUnBond(txIn, standbyNodeAccount.NodeAddress, cosmos.NewUint(uint64(1)), common.Address(standbyNodeAccount.NodeAddress.String()), nil, standbyNodeAccount.NodeAddress)
	_, err = handler.Run(ctx, msg)
	c.Assert(errors.Is(err, errReturnYgg), Equals, true)

	// simulate fail to get vault
	k.vault = GetRandomVault()
	msg = NewMsgUnBond(txIn, activeNodeAccount.NodeAddress, cosmos.NewUint(uint64(1)), GetRandomBNBAddress(), nil, activeNodeAccount.NodeAddress)
	result, err := handler.Run(ctx, msg)
	c.Assert(err, NotNil)
	c.Assert(result, IsNil)

	// simulate vault is not yggdrasil

	k.vault = Vault{
		Type:   AsgardVault,
		PubKey: standbyNodeAccount.PubKeySet.Secp256k1,
	}

	msg = NewMsgUnBond(txIn, standbyNodeAccount.NodeAddress, cosmos.NewUint(uint64(1)), GetRandomBNBAddress(), nil, standbyNodeAccount.NodeAddress)
	result, err = handler.Run(ctx, msg)
	c.Assert(err, NotNil)
	c.Assert(result, IsNil)

	// simulate jail nodeAccount can't unbound
	msg = NewMsgUnBond(txIn, k.jailNodeAccount.NodeAddress, cosmos.NewUint(uint64(1)), GetRandomBNBAddress(), nil, k.jailNodeAccount.NodeAddress)
	result, err = handler.Run(ctx, msg)
	c.Assert(err, NotNil)
	c.Assert(result, IsNil)

	// invalid message should cause error
	result, err = handler.Run(ctx, NewMsgMimir("whatever", 1, GetRandomBech32Addr()))
	c.Assert(err, NotNil)
	c.Assert(result, IsNil)
}

func (HandlerUnBondSuite) TestUnBondHandlerFailValidation(c *C) {
	ctx, k := setupKeeperForTest(c)
	activeNodeAccount := GetRandomValidatorNode(NodeActive)
	c.Assert(k.SetNodeAccount(ctx, activeNodeAccount), IsNil)
	handler := NewUnBondHandler(NewDummyMgrWithKeeper(k))
	txIn := common.NewTx(
		GetRandomTxHash(),
		activeNodeAccount.BondAddress,
		GetRandomBNBAddress(),
		common.Coins{
			common.NewCoin(common.RuneAsset(), cosmos.NewUint(uint64(1))),
		},
		BNBGasFeeSingleton,
		"unbond it",
	)
	txInNoTxID := txIn
	txInNoTxID.ID = ""
	testCases := []struct {
		name        string
		msg         *MsgUnBond
		expectedErr error
	}{
		{
			name:        "empty node address",
			msg:         NewMsgUnBond(txIn, cosmos.AccAddress{}, cosmos.NewUint(uint64(1)), activeNodeAccount.BondAddress, nil, activeNodeAccount.NodeAddress),
			expectedErr: se.ErrInvalidAddress,
		},
		{
			name:        "zero bond",
			msg:         NewMsgUnBond(txIn, GetRandomValidatorNode(NodeStandby).NodeAddress, cosmos.ZeroUint(), activeNodeAccount.BondAddress, nil, activeNodeAccount.NodeAddress),
			expectedErr: se.ErrUnknownRequest,
		},
		{
			name:        "empty bond address",
			msg:         NewMsgUnBond(txIn, GetRandomValidatorNode(NodeStandby).NodeAddress, cosmos.NewUint(uint64(1)), common.Address(""), nil, activeNodeAccount.NodeAddress),
			expectedErr: se.ErrInvalidAddress,
		},
		{
			name:        "empty request hash",
			msg:         NewMsgUnBond(txInNoTxID, GetRandomValidatorNode(NodeStandby).NodeAddress, cosmos.NewUint(uint64(1)), activeNodeAccount.BondAddress, nil, activeNodeAccount.NodeAddress),
			expectedErr: se.ErrUnknownRequest,
		},
		{
			name:        "empty signer",
			msg:         NewMsgUnBond(txIn, GetRandomValidatorNode(NodeStandby).NodeAddress, cosmos.NewUint(uint64(1)), activeNodeAccount.BondAddress, nil, cosmos.AccAddress{}),
			expectedErr: se.ErrInvalidAddress,
		},
		{
			name:        "account shouldn't be active",
			msg:         NewMsgUnBond(txIn, activeNodeAccount.NodeAddress, cosmos.NewUint(uint64(1)), activeNodeAccount.BondAddress, nil, activeNodeAccount.NodeAddress),
			expectedErr: se.ErrUnknownRequest,
		},
		{
			name:        "request not from original bond address should not be accepted",
			msg:         NewMsgUnBond(GetRandomTx(), GetRandomBech32Addr(), cosmos.NewUint(uint64(1)), activeNodeAccount.BondAddress, nil, activeNodeAccount.NodeAddress),
			expectedErr: se.ErrUnauthorized,
		},
	}
	for _, item := range testCases {
		c.Log(item.name)
		_, err := handler.Run(ctx, item.msg)

		c.Check(errors.Is(err, item.expectedErr), Equals, true, Commentf("name: %s, %s", item.name, err))
	}
}

func (HandlerUnBondSuite) TestUnBondHanlder_retiringvault(c *C) {
	ctx, k1 := setupKeeperForTest(c)
	// happy path
	activeNodeAccount := GetRandomValidatorNode(NodeActive)
	standbyNodeAccount := GetRandomValidatorNode(NodeStandby)
	c.Assert(k1.SetNodeAccount(ctx, activeNodeAccount), IsNil)
	c.Assert(k1.SetNodeAccount(ctx, standbyNodeAccount), IsNil)
	vault := NewVault(12, ActiveVault, YggdrasilVault, standbyNodeAccount.PubKeySet.Secp256k1, []string{
		common.BNBChain.String(), common.BTCChain.String(), common.ETHChain.String(), common.LTCChain.String(), common.BCHChain.String(),
	}, []ChainContract{})
	c.Assert(k1.SetVault(ctx, vault), IsNil)
	vault = NewVault(12, ActiveVault, AsgardVault, GetRandomPubKey(), nil, []ChainContract{})
	vault.Coins = common.Coins{
		common.NewCoin(common.RuneAsset(), cosmos.NewUint(10000*common.One)),
	}
	c.Assert(k1.SetVault(ctx, vault), IsNil)
	retiringVault := NewVault(12, RetiringVault, AsgardVault, GetRandomPubKey(), []string{
		common.BNBChain.String(), common.BTCChain.String(), common.ETHChain.String(), common.LTCChain.String(), common.BCHChain.String(),
	}, []ChainContract{})
	retiringVault.Membership = []string{
		activeNodeAccount.PubKeySet.Secp256k1.String(),
		standbyNodeAccount.PubKeySet.Secp256k1.String(),
	}
	retiringVault.Coins = common.Coins{
		common.NewCoin(common.RuneAsset(), cosmos.NewUint(10000*common.One)),
	}
	c.Assert(k1.SetVault(ctx, retiringVault), IsNil)
	handler := NewUnBondHandler(NewDummyMgrWithKeeper(k1))
	txIn := common.NewTx(
		GetRandomTxHash(),
		standbyNodeAccount.BondAddress,
		GetRandomBNBAddress(),
		common.Coins{
			common.NewCoin(common.RuneAsset(), cosmos.NewUint(uint64(1))),
		},
		BNBGasFeeSingleton,
		"unbond me please",
	)
	msg := NewMsgUnBond(txIn, standbyNodeAccount.NodeAddress, cosmos.NewUint(uint64(5*common.One)), standbyNodeAccount.BondAddress, nil, activeNodeAccount.NodeAddress)
	_, err := handler.Run(ctx, msg)
	c.Assert(err, NotNil)
}

func (HandlerUnBondSuite) TestBondProviders_Validate(c *C) {
	ctx, k := setupKeeperForTest(c)
	activeNodeAccount := GetRandomValidatorNode(NodeActive)
	standbyNodeAccount := GetRandomValidatorNode(NodeStandby)
	c.Assert(k.SetNodeAccount(ctx, activeNodeAccount), IsNil)
	c.Assert(k.SetNodeAccount(ctx, standbyNodeAccount), IsNil)
	txIn := GetRandomTx()
	txIn.Coins = common.NewCoins(common.NewCoin(common.RuneAsset(), cosmos.NewUint(100*common.One)))
	handler := NewUnBondHandler(NewDummyMgrWithKeeper(k))

	// happy path
	msg := NewMsgUnBond(txIn, standbyNodeAccount.NodeAddress, cosmos.NewUint(5*common.One), standbyNodeAccount.BondAddress, nil, activeNodeAccount.NodeAddress)
	err := handler.validate(ctx, *msg)
	c.Assert(err, IsNil)

	// cannot unbond an active node
	msg = NewMsgUnBond(txIn, activeNodeAccount.NodeAddress, cosmos.NewUint(5*common.One), activeNodeAccount.BondAddress, nil, activeNodeAccount.NodeAddress)
	err = handler.validate(ctx, *msg)
	c.Assert(err, NotNil)

	// test unbonding a bond provider
	bp := NewBondProviders(standbyNodeAccount.NodeAddress)
	p := NewBondProvider(GetRandomBech32Addr())
	bp.Providers = []BondProvider{p}
	c.Assert(k.SetBondProviders(ctx, bp), IsNil)

	msg = NewMsgUnBond(txIn, standbyNodeAccount.NodeAddress, cosmos.NewUint(5*common.One), common.Address(p.BondAddress.String()), nil, activeNodeAccount.NodeAddress)
	err = handler.validate(ctx, *msg)
	c.Assert(err, IsNil)
}

func (HandlerUnBondSuite) TestBondProviders_Handler(c *C) {
	ctx, k := setupKeeperForTest(c)
	activeNodeAccount := GetRandomValidatorNode(NodeActive)
	standbyNodeAccount := GetRandomValidatorNode(NodeStandby)
	c.Assert(k.SetNodeAccount(ctx, activeNodeAccount), IsNil)
	c.Assert(k.SetNodeAccount(ctx, standbyNodeAccount), IsNil)
	vaultStandby := GetRandomVault()
	vaultStandby.Type = YggdrasilVault
	vaultStandby.PubKey = standbyNodeAccount.PubKeySet.Secp256k1
	c.Assert(k.SetVault(ctx, vaultStandby), IsNil)
	vaultActive := GetRandomVault()
	vaultActive.Type = YggdrasilVault
	vaultActive.PubKey = activeNodeAccount.PubKeySet.Secp256k1
	c.Assert(k.SetVault(ctx, vaultActive), IsNil)
	txIn := GetRandomTx()
	txIn.Coins = common.NewCoins(common.NewCoin(common.RuneAsset(), cosmos.NewUint(0)))
	handler := NewUnBondHandler(NewDummyMgrWithKeeper(k))

	// happy path
	msg := NewMsgUnBond(txIn, standbyNodeAccount.NodeAddress, cosmos.NewUint(5*common.One), standbyNodeAccount.BondAddress, nil, activeNodeAccount.NodeAddress)
	err := handler.handle(ctx, *msg)
	c.Assert(err, IsNil)
	na, _ := handler.mgr.Keeper().GetNodeAccount(ctx, standbyNodeAccount.NodeAddress)
	c.Check(na.Bond.Uint64(), Equals, uint64(95*common.One), Commentf("%d", na.Bond.Uint64()))
	bp, _ := handler.mgr.Keeper().GetBondProviders(ctx, standbyNodeAccount.NodeAddress)
	c.Check(bp.Get(standbyNodeAccount.NodeAddress).Bond.Uint64(), Equals, na.Bond.Uint64())

	// node operator unbonds/removes bond provider
	p := NewBondProvider(GetRandomBech32Addr())
	p.Bond = cosmos.NewUint(50 * common.One)
	bp.Providers = append(bp.Providers, p)
	na.Bond = na.Bond.Add(p.Bond)
	c.Assert(k.SetBondProviders(ctx, bp), IsNil)
	c.Assert(k.SetNodeAccount(ctx, na), IsNil)

	c.Assert(k.SetVault(ctx, vaultStandby), IsNil)
	msg = NewMsgUnBond(txIn, standbyNodeAccount.NodeAddress, cosmos.ZeroUint(), standbyNodeAccount.BondAddress, p.BondAddress, activeNodeAccount.NodeAddress)
	err = handler.handle(ctx, *msg)
	c.Assert(err, IsNil)
	na, _ = handler.mgr.Keeper().GetNodeAccount(ctx, standbyNodeAccount.NodeAddress)
	c.Check(na.Bond.Uint64(), Equals, uint64(95*common.One), Commentf("%d", na.Bond.Uint64()))
	bp, _ = handler.mgr.Keeper().GetBondProviders(ctx, standbyNodeAccount.NodeAddress)
	c.Check(bp.Has(p.BondAddress), Equals, false)

	// bond provider unbond themselves
	p = NewBondProvider(GetRandomBech32Addr())
	p.Bond = cosmos.NewUint(50 * common.One)
	bp.Providers = append(bp.Providers, p)
	na.Bond = na.Bond.Add(p.Bond)
	c.Assert(k.SetBondProviders(ctx, bp), IsNil)
	c.Assert(k.SetNodeAccount(ctx, na), IsNil)

	c.Assert(k.SetVault(ctx, vaultStandby), IsNil)
	msg = NewMsgUnBond(txIn, standbyNodeAccount.NodeAddress, cosmos.NewUint(10*common.One), common.Address(p.BondAddress.String()), nil, activeNodeAccount.NodeAddress)
	err = handler.handle(ctx, *msg)
	c.Assert(err, IsNil)
	na, _ = handler.mgr.Keeper().GetNodeAccount(ctx, standbyNodeAccount.NodeAddress)
	c.Check(na.Bond.Uint64(), Equals, uint64(135*common.One), Commentf("%d", na.Bond.Uint64()))
	bp, _ = handler.mgr.Keeper().GetBondProviders(ctx, standbyNodeAccount.NodeAddress)
	c.Check(bp.Has(p.BondAddress), Equals, true)
	c.Check(bp.Get(p.BondAddress).Bond.Uint64(), Equals, uint64(40*common.One))
}
