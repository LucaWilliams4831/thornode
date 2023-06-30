package thorchain

import (
	"errors"

	. "gopkg.in/check.v1"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/x/thorchain/keeper"
)

type HandlerObserveNetworkFeeSuite struct{}

var _ = Suite(&HandlerObserveNetworkFeeSuite{})

type KeeperObserveNetworkFeeTest struct {
	keeper.Keeper
	errFailListActiveNodeAccount   bool
	errFailGetObservedNetworkVoter bool
	errFailSaveNetworkFee          bool
}

func NewKeeperObserveNetworkFeeTest(k keeper.Keeper) KeeperObserveNetworkFeeTest {
	return KeeperObserveNetworkFeeTest{Keeper: k}
}

func (k KeeperObserveNetworkFeeTest) ListActiveValidators(ctx cosmos.Context) (NodeAccounts, error) {
	if k.errFailListActiveNodeAccount {
		return NodeAccounts{}, errKaboom
	}
	return k.Keeper.ListActiveValidators(ctx)
}

func (k KeeperObserveNetworkFeeTest) GetObservedNetworkFeeVoter(ctx cosmos.Context, height int64, chain common.Chain, rate int64) (ObservedNetworkFeeVoter, error) {
	if k.errFailGetObservedNetworkVoter {
		return ObservedNetworkFeeVoter{}, errKaboom
	}
	return k.Keeper.GetObservedNetworkFeeVoter(ctx, height, chain, rate)
}

func (k KeeperObserveNetworkFeeTest) SaveNetworkFee(ctx cosmos.Context, chain common.Chain, networkFee NetworkFee) error {
	if k.errFailSaveNetworkFee {
		return errKaboom
	}
	return k.Keeper.SaveNetworkFee(ctx, chain, networkFee)
}

func (h *HandlerObserveNetworkFeeSuite) TestHandlerObserveNetworkFee(c *C) {
	h.testHandlerObserveNetworkFeeWithVersion(c)
}

func (*HandlerObserveNetworkFeeSuite) testHandlerObserveNetworkFeeWithVersion(c *C) {
	ctx, keeper := setupKeeperForTest(c)
	activeNodeAccount := GetRandomValidatorNode(NodeActive)
	c.Assert(keeper.SetNodeAccount(ctx, activeNodeAccount), IsNil)
	handler := NewNetworkFeeHandler(NewDummyMgrWithKeeper(keeper))
	msg := NewMsgNetworkFee(1024, common.BNBChain, 256, 100, activeNodeAccount.NodeAddress)
	result, err := handler.Run(ctx, msg)
	c.Assert(err, IsNil)
	c.Assert(result, NotNil)

	// already signed not cause error
	result, err = handler.Run(ctx, msg)
	c.Assert(err, IsNil)
	c.Assert(result, NotNil)

	// already processed
	msg1 := NewMsgNetworkFee(1024, common.BNBChain, 256, 100, activeNodeAccount.NodeAddress)
	result, err = handler.Run(ctx, msg1)
	c.Assert(err, IsNil)
	c.Assert(result, NotNil)

	// fail list active node account should fail
	handler1 := NewNetworkFeeHandler(
		NewDummyMgrWithKeeper(KeeperObserveNetworkFeeTest{
			Keeper:                       keeper,
			errFailListActiveNodeAccount: true,
		}),
	)
	result, err = handler1.Run(ctx, msg)
	c.Assert(err, NotNil)
	c.Assert(result, IsNil)
	c.Assert(errors.Is(err, errInternal), Equals, true)

	// fail to get observed network fee voter should return an error
	handler2 := NewNetworkFeeHandler(
		NewDummyMgrWithKeeper(KeeperObserveNetworkFeeTest{
			Keeper:                         keeper,
			errFailGetObservedNetworkVoter: true,
		}),
	)
	result, err = handler2.Run(ctx, msg)
	c.Assert(err, NotNil)
	c.Assert(result, IsNil)

	// fail to save network fee should result in an error
	handler3 := NewNetworkFeeHandler(
		NewDummyMgrWithKeeper(KeeperObserveNetworkFeeTest{
			Keeper:                keeper,
			errFailSaveNetworkFee: true,
		}),
	)
	msg2 := NewMsgNetworkFee(2056, common.BNBChain, 200, 102, activeNodeAccount.NodeAddress)
	result, err = handler3.Run(ctx, msg2)
	c.Assert(err, NotNil)
	c.Assert(result, IsNil)

	// invalid message should return an error
	msg3 := NewMsgReserveContributor(GetRandomTx(), ReserveContributor{}, GetRandomBech32Addr())
	result, err = handler3.Run(ctx, msg3)
	c.Check(result, IsNil)
	c.Check(err, NotNil)
	c.Check(errors.Is(err, errInvalidMessage), Equals, true)
}
