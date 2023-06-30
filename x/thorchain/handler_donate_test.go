package thorchain

import (
	"errors"

	se "github.com/cosmos/cosmos-sdk/types/errors"
	. "gopkg.in/check.v1"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/x/thorchain/keeper"
)

type HandlerDonateSuite struct{}

var _ = Suite(&HandlerDonateSuite{})

type HandlerDonateTestHelper struct {
	keeper.Keeper
	failToGetPool  bool
	failToSavePool bool
}

func NewHandlerDonateTestHelper(k keeper.Keeper) *HandlerDonateTestHelper {
	return &HandlerDonateTestHelper{
		Keeper: k,
	}
}

func (h *HandlerDonateTestHelper) GetPool(ctx cosmos.Context, asset common.Asset) (Pool, error) {
	if h.failToGetPool {
		return NewPool(), errKaboom
	}
	return h.Keeper.GetPool(ctx, asset)
}

func (h *HandlerDonateTestHelper) SetPool(ctx cosmos.Context, p Pool) error {
	if h.failToSavePool {
		return errKaboom
	}
	return h.Keeper.SetPool(ctx, p)
}

func (HandlerDonateSuite) TestDonate(c *C) {
	w := getHandlerTestWrapper(c, 1, true, true)
	// happy path
	prePool, err := w.keeper.GetPool(w.ctx, common.BNBAsset)
	c.Assert(err, IsNil)
	mgr := NewDummyMgrWithKeeper(w.keeper)
	donateHandler := NewDonateHandler(mgr)
	msg := NewMsgDonate(GetRandomTx(), common.BNBAsset, cosmos.NewUint(common.One*5), cosmos.NewUint(common.One*5), w.activeNodeAccount.NodeAddress)
	_, err = donateHandler.Run(w.ctx, msg)
	c.Assert(err, IsNil)
	afterPool, err := w.keeper.GetPool(w.ctx, common.BNBAsset)
	c.Assert(err, IsNil)
	c.Assert(afterPool.BalanceRune.String(), Equals, prePool.BalanceRune.Add(msg.RuneAmount).String())
	c.Assert(afterPool.BalanceAsset.String(), Equals, prePool.BalanceAsset.Add(msg.AssetAmount).String())

	msgBan := NewMsgBan(GetRandomBech32Addr(), w.activeNodeAccount.NodeAddress)
	result, err := donateHandler.Run(w.ctx, msgBan)
	c.Check(err, NotNil)
	c.Check(errors.Is(err, errInvalidMessage), Equals, true)
	c.Check(result, IsNil)

	testKeeper := NewHandlerDonateTestHelper(w.keeper)
	testKeeper.failToGetPool = true
	donateHandler1 := NewDonateHandler(NewDummyMgrWithKeeper(testKeeper))
	result, err = donateHandler1.Run(w.ctx, msg)
	c.Check(err, NotNil)
	c.Check(errors.Is(err, errInternal), Equals, true)
	c.Check(result, IsNil)

	testKeeper = NewHandlerDonateTestHelper(w.keeper)
	testKeeper.failToSavePool = true
	donateHandler2 := NewDonateHandler(NewDummyMgrWithKeeper(testKeeper))
	result, err = donateHandler2.Run(w.ctx, msg)
	c.Check(err, NotNil)
	c.Check(errors.Is(err, errInternal), Equals, true)
	c.Check(result, IsNil)
}

func (HandlerDonateSuite) TestHandleMsgDonateValidation(c *C) {
	w := getHandlerTestWrapper(c, 1, true, false)
	testCases := []struct {
		name        string
		msg         *MsgDonate
		expectedErr error
	}{
		{
			name:        "invalid signer address should fail",
			msg:         NewMsgDonate(GetRandomTx(), common.BNBAsset, cosmos.NewUint(common.One*5), cosmos.NewUint(common.One*5), cosmos.AccAddress{}),
			expectedErr: se.ErrInvalidAddress,
		},
		{
			name:        "empty asset should fail",
			msg:         NewMsgDonate(GetRandomTx(), common.Asset{}, cosmos.NewUint(common.One*5), cosmos.NewUint(common.One*5), w.activeNodeAccount.NodeAddress),
			expectedErr: se.ErrUnknownRequest,
		},
		{
			name:        "pool doesn't exist should fail",
			msg:         NewMsgDonate(GetRandomTx(), common.BNBAsset, cosmos.NewUint(common.One*5), cosmos.NewUint(common.One*5), w.activeNodeAccount.NodeAddress),
			expectedErr: se.ErrUnknownRequest,
		},
		{
			name:        "synth asset should fail",
			msg:         NewMsgDonate(GetRandomTx(), common.BNBAsset.GetSyntheticAsset(), cosmos.NewUint(common.One*5), cosmos.NewUint(common.One*5), w.activeNodeAccount.NodeAddress),
			expectedErr: errInvalidMessage,
		},
	}

	donateHandler := NewDonateHandler(NewDummyMgrWithKeeper(w.keeper))
	for _, item := range testCases {
		_, err := donateHandler.Run(w.ctx, item.msg)
		c.Check(errors.Is(err, item.expectedErr), Equals, true, Commentf("name:%s", item.name))
	}
}
