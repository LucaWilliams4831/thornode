package thorchain

import (
	. "gopkg.in/check.v1"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
	"gitlab.com/thorchain/thornode/x/thorchain/keeper"
)

type HandlerManageTHORNameSuite struct{}

var _ = Suite(&HandlerManageTHORNameSuite{})

type KeeperManageTHORNameTest struct {
	keeper.Keeper
}

func NewKeeperManageTHORNameTest(k keeper.Keeper) KeeperManageTHORNameTest {
	return KeeperManageTHORNameTest{Keeper: k}
}

func (s *HandlerManageTHORNameSuite) TestValidator(c *C) {
	ctx, mgr := setupManagerForTest(c)

	handler := NewManageTHORNameHandler(mgr)
	coin := common.NewCoin(common.RuneAsset(), cosmos.NewUint(100*common.One))
	addr := GetRandomTHORAddress()
	acc, _ := addr.AccAddress()
	name := NewTHORName("hello", 50, []THORNameAlias{{Chain: common.THORChain, Address: addr}})
	mgr.Keeper().SetTHORName(ctx, name)

	// happy path
	msg := NewMsgManageTHORName("I-am_the_99th_walrus+", common.THORChain, addr, coin, 0, common.BNBAsset, acc, acc)
	c.Assert(handler.validate(ctx, *msg), IsNil)

	// fail: address is wrong chain
	msg.Chain = common.BNBChain
	c.Assert(handler.validate(ctx, *msg), NotNil)

	// fail: address is wrong network
	mainnetBNBAddr, err := common.NewAddress("bnb1j08ys4ct2hzzc2hcz6h2hgrvlmsjynawtf2n0y")
	c.Assert(err, IsNil)
	msg.Address = mainnetBNBAddr
	c.Assert(handler.validate(ctx, *msg), NotNil)

	// restore to happy path
	msg.Chain = common.THORChain
	msg.Address = addr

	// fail: name is too long
	msg.Name = "this_name_is_way_too_long_to_be_a_valid_name"
	c.Assert(handler.validate(ctx, *msg), NotNil)

	// fail: bad characters
	msg.Name = "i am the walrus"
	c.Assert(handler.validate(ctx, *msg), NotNil)

	// fail: bad attempt to inflate expire block height
	msg.Name = "hello"
	msg.ExpireBlockHeight = 100
	c.Assert(handler.validate(ctx, *msg), NotNil)

	// fail: bad auth
	msg.ExpireBlockHeight = 0
	msg.Signer = GetRandomBech32Addr()
	c.Assert(handler.validate(ctx, *msg), NotNil)

	// fail: not enough funds for new THORName
	msg.Name = "bang"
	msg.Coin.Amount = cosmos.ZeroUint()
	c.Assert(handler.validate(ctx, *msg), NotNil)
}

func (s *HandlerManageTHORNameSuite) TestHandler(c *C) {
	ver := GetCurrentVersion()
	constAccessor := constants.GetConstantValues(ver)
	feePerBlock := constAccessor.GetInt64Value(constants.TNSFeePerBlock)
	registrationFee := constAccessor.GetInt64Value(constants.TNSRegisterFee)
	ctx, mgr := setupManagerForTest(c)

	blocksPerYear := mgr.GetConstants().GetInt64Value(constants.BlocksPerYear)
	handler := NewManageTHORNameHandler(mgr)
	coin := common.NewCoin(common.RuneAsset(), cosmos.NewUint(100*common.One))
	addr := GetRandomTHORAddress()
	acc, _ := addr.AccAddress()
	tnName := "hello"

	// add rune to addr for gas
	FundAccount(c, ctx, mgr.Keeper(), acc, 10*common.One)

	// happy path, register new name
	msg := NewMsgManageTHORName(tnName, common.THORChain, addr, coin, 0, common.RuneAsset(), acc, acc)
	_, err := handler.handle(ctx, *msg)
	c.Assert(err, IsNil)
	name, err := mgr.Keeper().GetTHORName(ctx, tnName)
	c.Assert(err, IsNil)
	c.Check(name.Owner.Empty(), Equals, false)
	c.Check(name.ExpireBlockHeight, Equals, ctx.BlockHeight()+blocksPerYear+(int64(coin.Amount.Uint64())-registrationFee)/feePerBlock)

	// happy path, set alt chain address
	bnbAddr := GetRandomBNBAddress()
	msg = NewMsgManageTHORName(tnName, common.BNBChain, bnbAddr, coin, 0, common.RuneAsset(), acc, acc)
	_, err = handler.handle(ctx, *msg)
	c.Assert(err, IsNil)
	name, err = mgr.Keeper().GetTHORName(ctx, tnName)
	c.Assert(err, IsNil)
	c.Check(name.GetAlias(common.BNBChain).Equals(bnbAddr), Equals, true)

	// happy path, update alt chain address
	bnbAddr = GetRandomBNBAddress()
	msg = NewMsgManageTHORName(tnName, common.BNBChain, bnbAddr, coin, 0, common.RuneAsset(), acc, acc)
	_, err = handler.handle(ctx, *msg)
	c.Assert(err, IsNil)
	name, err = mgr.Keeper().GetTHORName(ctx, tnName)
	c.Assert(err, IsNil)
	c.Check(name.GetAlias(common.BNBChain).Equals(bnbAddr), Equals, true)

	// happy path, release thorname back into the wild
	msg = NewMsgManageTHORName(tnName, common.THORChain, addr, common.NewCoin(common.RuneAsset(), cosmos.ZeroUint()), 1, common.RuneAsset(), acc, acc)
	_, err = handler.handle(ctx, *msg)
	c.Assert(err, IsNil)
	name, err = mgr.Keeper().GetTHORName(ctx, tnName)
	c.Assert(err, IsNil)
	c.Check(name.Owner.Empty(), Equals, true)
	c.Check(name.ExpireBlockHeight, Equals, int64(0))
}
