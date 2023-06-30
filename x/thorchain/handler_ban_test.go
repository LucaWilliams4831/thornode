package thorchain

import (
	"errors"

	"github.com/blang/semver"
	se "github.com/cosmos/cosmos-sdk/types/errors"
	. "gopkg.in/check.v1"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
	"gitlab.com/thorchain/thornode/x/thorchain/keeper"
)

var _ = Suite(&HandlerBanSuite{})

type HandlerBanSuite struct{}

type TestBanKeeper struct {
	keeper.KVStoreDummy
	ban     BanVoter
	toBan   NodeAccount
	banner1 NodeAccount
	banner2 NodeAccount
	network Network
	err     error
	modules map[string]int64
}

func (k *TestBanKeeper) SendFromModuleToModule(_ cosmos.Context, from, to string, coins common.Coins) error {
	k.modules[from] -= int64(coins[0].Amount.Uint64())
	k.modules[to] += int64(coins[0].Amount.Uint64())
	return nil
}

func (k *TestBanKeeper) ListActiveValidators(_ cosmos.Context) (NodeAccounts, error) {
	return NodeAccounts{k.toBan, k.banner1, k.banner2}, k.err
}

func (k *TestBanKeeper) GetNodeAccount(_ cosmos.Context, addr cosmos.AccAddress) (NodeAccount, error) {
	if addr.Equals(k.toBan.NodeAddress) {
		return k.toBan, k.err
	}
	if addr.Equals(k.banner1.NodeAddress) {
		return k.banner1, k.err
	}
	if addr.Equals(k.banner2.NodeAddress) {
		return k.banner2, k.err
	}
	return NodeAccount{}, errors.New("could not find node account, oops")
}

func (k *TestBanKeeper) SetNodeAccount(_ cosmos.Context, na NodeAccount) error {
	if na.NodeAddress.Equals(k.toBan.NodeAddress) {
		k.toBan = na
		return k.err
	}
	if na.NodeAddress.Equals(k.banner1.NodeAddress) {
		k.banner1 = na
		return k.err
	}
	if na.NodeAddress.Equals(k.banner2.NodeAddress) {
		k.banner2 = na
		return k.err
	}
	return k.err
}

func (k *TestBanKeeper) GetNetwork(ctx cosmos.Context) (Network, error) {
	return k.network, nil
}

func (k *TestBanKeeper) SetNetwork(ctx cosmos.Context, data Network) error {
	k.network = data
	return nil
}

func (k *TestBanKeeper) GetBanVoter(_ cosmos.Context, addr cosmos.AccAddress) (BanVoter, error) {
	return k.ban, k.err
}

func (k *TestBanKeeper) SetBanVoter(_ cosmos.Context, ban BanVoter) {
	k.ban = ban
}

func (s *HandlerBanSuite) TestValidate(c *C) {
	ctx, _ := setupKeeperForTest(c)

	toBan := GetRandomValidatorNode(NodeActive)
	banner1 := GetRandomValidatorNode(NodeActive)
	banner2 := GetRandomValidatorNode(NodeActive)

	keeper := &TestBanKeeper{
		toBan:   toBan,
		banner1: banner1,
		banner2: banner2,
	}

	handler := NewBanHandler(NewDummyMgrWithKeeper(keeper))
	// happy path
	msg := NewMsgBan(toBan.NodeAddress, banner1.NodeAddress)
	err := handler.validate(ctx, *msg)
	c.Assert(err, IsNil)

	// invalid msg
	msg = &MsgBan{}
	err = handler.validate(ctx, *msg)
	c.Assert(err, NotNil)
}

func (s *HandlerBanSuite) TestHandle(c *C) {
	ctx, _ := setupKeeperForTest(c)
	constAccessor := constants.GetConstantValues(GetCurrentVersion())
	minBond := constAccessor.GetInt64Value(constants.MinimumBondInRune)

	toBan := GetRandomValidatorNode(NodeActive)
	toBan.Bond = cosmos.NewUint(uint64(minBond))
	banner1 := GetRandomValidatorNode(NodeActive)
	banner1.Bond = cosmos.NewUint(uint64(minBond))
	banner2 := GetRandomValidatorNode(NodeActive)
	banner2.Bond = cosmos.NewUint(uint64(minBond))

	keeper := &TestBanKeeper{
		ban:     NewBanVoter(toBan.NodeAddress),
		toBan:   toBan,
		banner1: banner1,
		banner2: banner2,
		network: NewNetwork(),
		modules: make(map[string]int64),
	}

	handler := NewBanHandler(NewDummyMgrWithKeeper(keeper))

	// ban with banner 1
	msg := NewMsgBan(toBan.NodeAddress, banner1.NodeAddress)
	_, err := handler.handle(ctx, *msg)
	c.Assert(err, IsNil)
	c.Check(int64(keeper.banner1.Bond.Uint64()), Equals, int64(99900000))
	c.Check(keeper.modules[ReserveName], Equals, int64(100000))
	c.Check(keeper.toBan.ForcedToLeave, Equals, false)
	c.Check(keeper.ban.Signers, HasLen, 1)

	// ensure banner 1 can't ban twice
	_, err = handler.handle(ctx, *msg)
	c.Assert(err, IsNil)
	c.Check(int64(keeper.banner1.Bond.Uint64()), Equals, int64(99900000))
	c.Check(keeper.modules[ReserveName], Equals, int64(100000))
	c.Check(keeper.toBan.ForcedToLeave, Equals, false)
	c.Check(keeper.ban.Signers, HasLen, 1)

	// ban with banner 2, which should actually ban the node account
	msg = NewMsgBan(toBan.NodeAddress, banner2.NodeAddress)
	_, err = handler.handle(ctx, *msg)
	c.Assert(err, IsNil)
	c.Check(int64(keeper.banner2.Bond.Uint64()), Equals, int64(99900000))
	c.Check(keeper.modules[ReserveName], Equals, int64(200000))
	c.Check(keeper.toBan.ForcedToLeave, Equals, true)
	c.Check(keeper.toBan.LeaveScore, Equals, uint64(1))
	c.Check(keeper.ban.Signers, HasLen, 2)
	c.Check(keeper.ban.BlockHeight, Equals, int64(18))
}

type TestBanKeeperHelper struct {
	keeper.Keeper
	toBanNodeAddr              cosmos.AccAddress
	bannerNodeAddr             cosmos.AccAddress
	failToGetToBanAddr         bool
	failToGetBannerNodeAccount bool
	failToListActiveValidators bool
	failToGetBanVoter          bool
	failToGetNetwork           bool
	failToSetNetwork           bool
	failToSaveBanner           bool
	failToSaveToBan            bool
}

func NewTestBanKeeperHelper(k keeper.Keeper) *TestBanKeeperHelper {
	return &TestBanKeeperHelper{
		Keeper: k,
	}
}

func (k *TestBanKeeperHelper) GetNodeAccount(ctx cosmos.Context, addr cosmos.AccAddress) (NodeAccount, error) {
	if addr.Equals(k.toBanNodeAddr) && k.failToGetToBanAddr {
		return NodeAccount{}, errKaboom
	}
	if addr.Equals(k.bannerNodeAddr) && k.failToGetBannerNodeAccount {
		return NodeAccount{}, errKaboom
	}
	return k.Keeper.GetNodeAccount(ctx, addr)
}

func (k *TestBanKeeperHelper) ListActiveValidators(ctx cosmos.Context) (NodeAccounts, error) {
	if k.failToListActiveValidators {
		return NodeAccounts{}, errKaboom
	}
	return k.Keeper.ListActiveValidators(ctx)
}

func (k *TestBanKeeperHelper) GetBanVoter(ctx cosmos.Context, addr cosmos.AccAddress) (BanVoter, error) {
	if k.failToGetBanVoter {
		return BanVoter{}, errKaboom
	}
	return k.Keeper.GetBanVoter(ctx, addr)
}

func (k *TestBanKeeperHelper) GetNetwork(ctx cosmos.Context) (Network, error) {
	if k.failToGetNetwork {
		return Network{}, errKaboom
	}
	return k.Keeper.GetNetwork(ctx)
}

func (k *TestBanKeeperHelper) SetNetwork(ctx cosmos.Context, network Network) error {
	if k.failToSetNetwork {
		return errKaboom
	}
	return k.Keeper.SetNetwork(ctx, network)
}

func (k *TestBanKeeperHelper) SetNodeAccount(ctx cosmos.Context, na NodeAccount) error {
	if k.failToSaveBanner && na.NodeAddress.Equals(k.bannerNodeAddr) {
		return errKaboom
	}
	if k.failToSaveToBan && na.NodeAddress.Equals(k.toBanNodeAddr) {
		return errKaboom
	}
	return k.Keeper.SetNodeAccount(ctx, na)
}

func (s *HandlerBanSuite) TestBanHandlerValidation(c *C) {
	toBanAddr := GetRandomBech32Addr()
	banner := GetRandomValidatorNode(NodeActive)
	bannerNodeAddr := banner.NodeAddress
	testCases := []struct {
		name              string
		messageProvider   func(ctx cosmos.Context, helper *TestBanKeeperHelper) cosmos.Msg
		validator         func(c *C, result *cosmos.Result, err error, helper *TestBanKeeperHelper, name string)
		skipForNativeRUNE bool
	}{
		{
			name: "invalid msg should return an error",
			messageProvider: func(ctx cosmos.Context, helper *TestBanKeeperHelper) cosmos.Msg {
				return NewMsgNetworkFee(1024, common.BNBChain, 1, bnbSingleTxFee.Uint64(), GetRandomBech32Addr())
			},
			validator: func(c *C, result *cosmos.Result, err error, helper *TestBanKeeperHelper, name string) {
				c.Check(err, NotNil, Commentf(name))
				c.Check(result, IsNil, Commentf(name))
				c.Check(errors.Is(err, errInvalidMessage), Equals, true, Commentf(name))
			},
		},
		{
			name: "MsgBan failed validation should return error",
			messageProvider: func(ctx cosmos.Context, helper *TestBanKeeperHelper) cosmos.Msg {
				return NewMsgBan(cosmos.AccAddress{}, GetRandomBech32Addr())
			},
			validator: func(c *C, result *cosmos.Result, err error, helper *TestBanKeeperHelper, name string) {
				c.Check(err, NotNil, Commentf(name))
				c.Check(result, IsNil, Commentf(name))
				c.Check(errors.Is(err, se.ErrInvalidAddress), Equals, true, Commentf(name))
			},
		},
		{
			name: "MsgBan not signed by an active account should return error",
			messageProvider: func(ctx cosmos.Context, helper *TestBanKeeperHelper) cosmos.Msg {
				return NewMsgBan(GetRandomBech32Addr(), GetRandomBech32Addr())
			},
			validator: func(c *C, result *cosmos.Result, err error, helper *TestBanKeeperHelper, name string) {
				c.Check(err, NotNil, Commentf(name))
				c.Check(result, IsNil, Commentf(name))
				c.Check(errors.Is(err, se.ErrUnauthorized), Equals, true, Commentf(name))
			},
		},
		{
			name: "fail to get to ban node account should return an error",
			messageProvider: func(ctx cosmos.Context, helper *TestBanKeeperHelper) cosmos.Msg {
				helper.failToGetToBanAddr = true
				return NewMsgBan(toBanAddr, bannerNodeAddr)
			},
			validator: func(c *C, result *cosmos.Result, err error, helper *TestBanKeeperHelper, name string) {
				c.Check(err, NotNil, Commentf(name))
				c.Check(result, IsNil, Commentf(name))
				c.Check(errors.Is(err, errInternal), Equals, true, Commentf(name))
			},
		},
		{
			name: "to ban node account is not valid should return an error",
			messageProvider: func(ctx cosmos.Context, helper *TestBanKeeperHelper) cosmos.Msg {
				return NewMsgBan(toBanAddr, bannerNodeAddr)
			},
			validator: func(c *C, result *cosmos.Result, err error, helper *TestBanKeeperHelper, name string) {
				c.Check(err, NotNil, Commentf(name))
				c.Check(result, IsNil, Commentf(name))
			},
		},
		{
			name: "to ban node account has been banned already should not do any thing",
			messageProvider: func(ctx cosmos.Context, helper *TestBanKeeperHelper) cosmos.Msg {
				na := GetRandomValidatorNode(NodeActive)
				na.ForcedToLeave = true
				c.Assert(helper.SetNodeAccount(ctx, na), IsNil)
				return NewMsgBan(na.NodeAddress, bannerNodeAddr)
			},
			validator: func(c *C, result *cosmos.Result, err error, helper *TestBanKeeperHelper, name string) {
				c.Check(err, IsNil, Commentf(name))
				c.Check(result, NotNil, Commentf(name))
			},
		},
		{
			name: "ban an not active account should return an error",
			messageProvider: func(ctx cosmos.Context, helper *TestBanKeeperHelper) cosmos.Msg {
				na := GetRandomValidatorNode(NodeStandby)
				c.Assert(helper.SetNodeAccount(ctx, na), IsNil)
				return NewMsgBan(na.NodeAddress, bannerNodeAddr)
			},
			validator: func(c *C, result *cosmos.Result, err error, helper *TestBanKeeperHelper, name string) {
				c.Check(err, NotNil, Commentf(name))
				c.Check(result, IsNil, Commentf(name))
			},
		},
		{
			name: "banner is invalid return an error",
			messageProvider: func(ctx cosmos.Context, helper *TestBanKeeperHelper) cosmos.Msg {
				toBanAcct := GetRandomValidatorNode(NodeActive)
				c.Assert(helper.SetNodeAccount(ctx, toBanAcct), IsNil)
				newBanner := banner
				newBanner.BondAddress = common.NoAddress
				c.Assert(helper.SetNodeAccount(ctx, newBanner), IsNil)
				return NewMsgBan(toBanAcct.NodeAddress, bannerNodeAddr)
			},
			validator: func(c *C, result *cosmos.Result, err error, helper *TestBanKeeperHelper, name string) {
				c.Check(err, NotNil, Commentf(name))
				c.Check(result, IsNil, Commentf(name))
			},
		},
		{
			name: "fail to list active node account should return an error",
			messageProvider: func(ctx cosmos.Context, helper *TestBanKeeperHelper) cosmos.Msg {
				toBanAcct := GetRandomValidatorNode(NodeActive)
				c.Assert(helper.SetNodeAccount(ctx, toBanAcct), IsNil)
				helper.failToListActiveValidators = true
				return NewMsgBan(toBanAcct.NodeAddress, bannerNodeAddr)
			},
			validator: func(c *C, result *cosmos.Result, err error, helper *TestBanKeeperHelper, name string) {
				c.Check(err, NotNil, Commentf(name))
				c.Check(result, IsNil, Commentf(name))
			},
		},
		{
			name: "fail to get ban voter should return an error",
			messageProvider: func(ctx cosmos.Context, helper *TestBanKeeperHelper) cosmos.Msg {
				toBanAcct := GetRandomValidatorNode(NodeActive)
				c.Assert(helper.SetNodeAccount(ctx, toBanAcct), IsNil)
				helper.failToGetBanVoter = true
				return NewMsgBan(toBanAcct.NodeAddress, bannerNodeAddr)
			},
			validator: func(c *C, result *cosmos.Result, err error, helper *TestBanKeeperHelper, name string) {
				c.Check(err, NotNil, Commentf(name))
				c.Check(result, IsNil, Commentf(name))
			},
		},
		{
			name: "fail to get network data should return an error",
			messageProvider: func(ctx cosmos.Context, helper *TestBanKeeperHelper) cosmos.Msg {
				toBanAcct := GetRandomValidatorNode(NodeActive)
				c.Assert(helper.SetNodeAccount(ctx, toBanAcct), IsNil)
				helper.failToGetNetwork = true
				return NewMsgBan(toBanAcct.NodeAddress, bannerNodeAddr)
			},
			validator: func(c *C, result *cosmos.Result, err error, helper *TestBanKeeperHelper, name string) {
				c.Check(err, NotNil, Commentf(name))
				c.Check(result, IsNil, Commentf(name))
			},
			skipForNativeRUNE: true,
		},
		{
			name: "fail to set network data should return an error",
			messageProvider: func(ctx cosmos.Context, helper *TestBanKeeperHelper) cosmos.Msg {
				toBanAcct := GetRandomValidatorNode(NodeActive)
				c.Assert(helper.SetNodeAccount(ctx, toBanAcct), IsNil)
				helper.failToSetNetwork = true
				return NewMsgBan(toBanAcct.NodeAddress, bannerNodeAddr)
			},
			validator: func(c *C, result *cosmos.Result, err error, helper *TestBanKeeperHelper, name string) {
				c.Check(err, NotNil, Commentf(name))
				c.Check(result, IsNil, Commentf(name))
			},
			skipForNativeRUNE: true,
		},
		{
			name: "fail to save banner should return an error",
			messageProvider: func(ctx cosmos.Context, helper *TestBanKeeperHelper) cosmos.Msg {
				toBanAcct := GetRandomValidatorNode(NodeActive)
				c.Assert(helper.SetNodeAccount(ctx, toBanAcct), IsNil)
				helper.failToSaveBanner = true
				return NewMsgBan(toBanAcct.NodeAddress, bannerNodeAddr)
			},
			validator: func(c *C, result *cosmos.Result, err error, helper *TestBanKeeperHelper, name string) {
				c.Check(err, NotNil, Commentf(name))
				c.Check(result, IsNil, Commentf(name))
			},
			skipForNativeRUNE: true,
		},
		{
			name: "when voter had been processed , it should not error",
			messageProvider: func(ctx cosmos.Context, helper *TestBanKeeperHelper) cosmos.Msg {
				toBanAcct := GetRandomValidatorNode(NodeActive)
				c.Assert(helper.SetNodeAccount(ctx, toBanAcct), IsNil)
				voter, _ := helper.GetBanVoter(ctx, toBanAcct.NodeAddress)
				activeNode := GetRandomValidatorNode(NodeActive)
				c.Assert(helper.SetNodeAccount(ctx, activeNode), IsNil)
				voter.Sign(activeNode.NodeAddress)
				voter.BlockHeight = ctx.BlockHeight()
				helper.SetBanVoter(ctx, voter)
				return NewMsgBan(toBanAcct.NodeAddress, bannerNodeAddr)
			},
			validator: func(c *C, result *cosmos.Result, err error, helper *TestBanKeeperHelper, name string) {
				c.Check(err, IsNil, Commentf(name))
				c.Check(result, NotNil, Commentf(name))
			},
		},
		{
			name: "fail to save to ban account, it should return an error",
			messageProvider: func(ctx cosmos.Context, helper *TestBanKeeperHelper) cosmos.Msg {
				toBanAcct := GetRandomValidatorNode(NodeActive)
				c.Assert(helper.SetNodeAccount(ctx, toBanAcct), IsNil)
				voter, _ := helper.GetBanVoter(ctx, toBanAcct.NodeAddress)
				activeNode := GetRandomValidatorNode(NodeActive)
				c.Assert(helper.SetNodeAccount(ctx, activeNode), IsNil)
				voter.Sign(activeNode.NodeAddress)
				helper.SetBanVoter(ctx, voter)
				helper.failToSaveToBan = true
				helper.toBanNodeAddr = toBanAcct.NodeAddress
				return NewMsgBan(toBanAcct.NodeAddress, bannerNodeAddr)
			},
			validator: func(c *C, result *cosmos.Result, err error, helper *TestBanKeeperHelper, name string) {
				c.Check(err, NotNil, Commentf(name))
				c.Check(result, IsNil, Commentf(name))
			},
		},
	}
	versions := []semver.Version{
		GetCurrentVersion(),
	}
	for _, tc := range testCases {
		if tc.skipForNativeRUNE {
			continue
		}
		for _, ver := range versions {
			ctx, mgr := setupManagerForTest(c)
			c.Assert(mgr.Keeper().SetNodeAccount(ctx, banner), IsNil)
			helper := NewTestBanKeeperHelper(mgr.Keeper())
			helper.toBanNodeAddr = toBanAddr
			helper.bannerNodeAddr = bannerNodeAddr
			mgr.K = helper
			mgr.currentVersion = ver
			mgr.constAccessor = constants.GetConstantValues(ver)
			handler := NewBanHandler(mgr)
			result, err := handler.Run(ctx, tc.messageProvider(ctx, helper))
			tc.validator(c, result, err, helper, tc.name)
		}
	}
}
