package thorchain

import (
	"errors"

	se "github.com/cosmos/cosmos-sdk/types/errors"
	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	. "gopkg.in/check.v1"
)

type HandlerSolvencyTestSuite struct{}

var _ = Suite(&HandlerSolvencyTestSuite{})

func (s *HandlerSolvencyTestSuite) TestValidate(c *C) {
	ctx, mgr := setupManagerForTest(c)

	handler := NewSolvencyHandler(mgr)
	// msgSolvency signed by  none active node should be rejected
	msgSolvency, err := NewMsgSolvency(common.ETHChain,
		GetRandomPubKey(),
		common.NewCoins(
			common.NewCoin(common.ETHAsset, cosmos.NewUint(1024*common.One)),
		),
		1024,
		GetRandomBech32Addr())
	c.Assert(err, IsNil)
	c.Assert(handler.validate(ctx, *msgSolvency), NotNil)
	// active node
	var activeNodes [4]NodeAccount
	for i := 0; i < 4; i++ {
		node := GetRandomValidatorNode(NodeActive)
		activeNodes[i] = node
		c.Assert(mgr.Keeper().SetNodeAccount(ctx, node), IsNil)
	}
	// msgSolvency signed by active node should be accepted
	msgSolvency.Signer = activeNodes[0].NodeAddress

	c.Assert(err, IsNil)
	c.Assert(handler.validate(ctx, *msgSolvency), IsNil)

	result, err := handler.Run(ctx, msgSolvency)
	c.Assert(err, IsNil)
	c.Assert(result, NotNil)
	// solvency voter should have been created
	voter, err := mgr.Keeper().GetSolvencyVoter(ctx, msgSolvency.Id, msgSolvency.Chain)
	c.Assert(err, IsNil)
	c.Assert(voter.Empty(), Equals, false)

	asgard := NewVault(1024, ActiveVault, AsgardVault, msgSolvency.PubKey, []string{
		common.ETHChain.String(),
		common.BTCChain.String(),
		common.BNBChain.String(),
		common.LTCChain.String(),
		common.BCHChain.String(),
	}, nil)
	asgard.AddFunds(common.NewCoins(common.NewCoin(common.ETHAsset, cosmos.NewUint(1024*common.One))))
	c.Assert(mgr.Keeper().SetVault(ctx, asgard), IsNil)

	// second active node report solvency , it should be accepted
	// reach consensus , vault is solvent , everything continues
	msgSolvency.Signer = activeNodes[1].NodeAddress
	result, err = handler.Run(ctx, msgSolvency)
	c.Assert(err, IsNil)
	c.Assert(result, NotNil)

	// third active node report solvency , it should be accepted
	msgSolvency.Signer = activeNodes[2].NodeAddress
	result, err = handler.Run(ctx, msgSolvency)
	c.Assert(err, IsNil)
	c.Assert(result, NotNil)

	// vault suppose to have 1024 ETH, however only 100 left , vault is insolvent , chain should stop
	msgSolvency1, err := NewMsgSolvency(common.ETHChain,
		asgard.PubKey,
		common.NewCoins(
			common.NewCoin(common.ETHAsset, cosmos.NewUint(100*common.One)),
		),
		1024,
		GetRandomBech32Addr())
	c.Assert(err, IsNil)
	msgSolvency1.Signer = activeNodes[0].NodeAddress
	result, err = handler.Run(ctx, msgSolvency1)
	c.Assert(err, IsNil)
	c.Assert(result, NotNil)

	// minority , so the second voter should reach consensus
	msgSolvency1.Signer = activeNodes[1].NodeAddress
	result, err = handler.Run(ctx, msgSolvency1)
	c.Assert(err, IsNil)
	c.Assert(result, NotNil)
	halt, err := mgr.Keeper().GetMimir(ctx, "SolvencyHaltETHChain")
	c.Assert(err, IsNil)
	c.Assert(halt, Equals, ctx.BlockHeight())
	c.Assert(mgr.Keeper().DeleteMimir(ctx, "SolvencyHaltETHChain"), IsNil)

	// vault suppose to have 1024 ETH, however only 1000 left , but there are 20 ETH in the pending outbound queue
	// chain should not stopped
	txOut := NewTxOut(ctx.BlockHeight())
	txOut.TxArray = []TxOutItem{
		{
			Chain:       common.ETHChain,
			ToAddress:   "0x3a196410a0f5facd08fd7880a4b8551cd085c031",
			VaultPubKey: asgard.PubKey,
			Coin:        common.NewCoin(common.ETHAsset, cosmos.NewUint(20*common.One)),
			Memo:        "OUT:693c3337193b1185fb0a36d8b7ec3f612ad57e599fd25e7ad6ec887aae43b291",
			InHash:      "693c3337193b1185fb0a36d8b7ec3f612ad57e599fd25e7ad6ec887aae43b291",
		},
	}
	c.Assert(mgr.Keeper().SetTxOut(ctx, txOut), IsNil)
	ctx = ctx.WithBlockHeight(ctx.BlockHeight() + 100)
	msgSolvency2, err := NewMsgSolvency(common.ETHChain,
		asgard.PubKey,
		common.NewCoins(
			common.NewCoin(common.ETHAsset, cosmos.NewUint(1000*common.One)),
		),
		1024,
		GetRandomBech32Addr())

	c.Assert(err, IsNil)
	msgSolvency2.Signer = activeNodes[0].NodeAddress
	result, err = handler.Run(ctx, msgSolvency2)
	c.Assert(err, IsNil)
	c.Assert(result, NotNil)

	msgSolvency2.Signer = activeNodes[1].NodeAddress
	result, err = handler.Run(ctx, msgSolvency2)
	c.Assert(err, IsNil)
	c.Assert(result, NotNil)
	halt, err = mgr.Keeper().GetMimir(ctx, "SolvencyHaltETHChain")
	c.Assert(err, IsNil)
	c.Assert(halt, Equals, int64(-1))

	// tampered MsgSolvency should be rejected
	msgSolvency2.Coins = common.NewCoins(
		common.NewCoin(common.ETHAsset, cosmos.NewUint(1024*common.One)),
	)
	result, err = handler.Run(ctx, msgSolvency2)
	c.Assert(err, NotNil)
	c.Assert(errors.Is(err, se.ErrUnknownRequest), Equals, true)
	c.Assert(result, IsNil)
}
