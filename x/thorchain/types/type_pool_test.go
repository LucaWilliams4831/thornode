package types

import (
	"encoding/json"

	. "gopkg.in/check.v1"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
)

type PoolTestSuite struct{}

var _ = Suite(&PoolTestSuite{})

func (PoolTestSuite) TestPool(c *C) {
	p := NewPool()
	c.Check(p.IsEmpty(), Equals, true)
	p.Asset = common.BNBAsset
	c.Check(p.IsEmpty(), Equals, false)
	p.BalanceRune = cosmos.NewUint(100 * common.One)
	p.BalanceAsset = cosmos.NewUint(50 * common.One)
	c.Check(p.AssetValueInRune(cosmos.NewUint(25*common.One)).Equal(cosmos.NewUint(50*common.One)), Equals, true)
	c.Check(p.RuneValueInAsset(cosmos.NewUint(50*common.One)).Equal(cosmos.NewUint(25*common.One)), Equals, true)

	signer := GetRandomBech32Addr()
	bnbAddress := GetRandomBNBAddress()
	txID := GetRandomTxHash()

	tx := common.NewTx(
		txID,
		GetRandomBNBAddress(),
		GetRandomBNBAddress(),
		common.Coins{
			common.NewCoin(common.BNBAsset, cosmos.NewUint(1)),
		},
		BNBGasFeeSingleton,
		"",
	)
	m := NewMsgSwap(tx, common.BNBAsset, bnbAddress, cosmos.NewUint(2), common.NoAddress, cosmos.ZeroUint(), "", "", nil, 0, signer)

	c.Check(p.EnsureValidPoolStatus(m), IsNil)
	msgNoop := NewMsgNoOp(GetRandomObservedTx(), signer, "")
	c.Check(p.EnsureValidPoolStatus(msgNoop), IsNil)
	p.Status = PoolStatus_Available
	c.Check(p.EnsureValidPoolStatus(m), IsNil)
	p.Status = PoolStatus(100)
	c.Check(p.EnsureValidPoolStatus(msgNoop), NotNil)

	p.Status = PoolStatus_Suspended
	c.Check(p.EnsureValidPoolStatus(msgNoop), NotNil)
	p1 := NewPool()
	c.Check(p1.Valid(), NotNil)
	p1.Asset = common.BNBAsset
	c.Check(p1.AssetValueInRune(cosmos.NewUint(100)).Uint64(), Equals, cosmos.ZeroUint().Uint64())
	c.Check(p1.RuneValueInAsset(cosmos.NewUint(100)).Uint64(), Equals, cosmos.ZeroUint().Uint64())
	p1.BalanceRune = cosmos.NewUint(100 * common.One)
	p1.BalanceAsset = cosmos.NewUint(50 * common.One)
	c.Check(p1.Valid(), IsNil)

	c.Check(p1.IsAvailable(), Equals, true)

	// When Pool is in staged status, it can't swap
	p2 := NewPool()
	p2.Status = PoolStatus_Staged
	msgSwap := NewMsgSwap(GetRandomTx(), common.BNBAsset, GetRandomBNBAddress(), cosmos.NewUint(1000), common.NoAddress, cosmos.ZeroUint(), "", "", nil, 0, GetRandomBech32Addr())
	c.Check(p2.EnsureValidPoolStatus(msgSwap), NotNil)
	c.Check(p2.EnsureValidPoolStatus(msgNoop), IsNil)
}

func (PoolTestSuite) TestPoolStatus(c *C) {
	inputs := []string{
		"Available", "Staged", "Suspended", "whatever",
	}
	for _, item := range inputs {
		ps := GetPoolStatus(item)
		c.Assert(ps.Valid(), IsNil)
	}
	var ps PoolStatus
	err := json.Unmarshal([]byte(`"Available"`), &ps)
	c.Assert(err, IsNil)
	c.Check(ps == PoolStatus_Available, Equals, true)
	err = json.Unmarshal([]byte(`{asdf}`), &ps)
	c.Assert(err, NotNil)

	buf, err := json.Marshal(ps)
	c.Check(err, IsNil)
	c.Check(buf, NotNil)
}

func (PoolTestSuite) TestPools(c *C) {
	pools := make(Pools, 0)
	bnb := NewPool()
	bnb.Asset = common.BNBAsset
	btc := NewPool()
	btc.Asset = common.BTCAsset
	btc.BalanceRune = cosmos.NewUint(10)

	pools = pools.Set(bnb)
	pools = pools.Set(btc)
	c.Assert(pools, HasLen, 2)

	pool, ok := pools.Get(common.BNBAsset)
	c.Check(ok, Equals, true)
	c.Check(pool.Asset.Equals(common.BNBAsset), Equals, true)

	pool, ok = pools.Get(common.BTCAsset)
	c.Check(ok, Equals, true)
	c.Check(pool.Asset.Equals(common.BTCAsset), Equals, true)
	c.Check(pool.BalanceRune.Uint64(), Equals, uint64(10))

	pool.BalanceRune = cosmos.NewUint(20)
	pools = pools.Set(pool)
	pool, ok = pools.Get(common.BTCAsset)
	c.Check(ok, Equals, true)
	c.Check(pool.Asset.Equals(common.BTCAsset), Equals, true)
	c.Check(pool.BalanceRune.Uint64(), Equals, uint64(20))
}

func (PoolTestSuite) TestCalcUnits(c *C) {
	version := GetCurrentVersion()

	pool := NewPool()
	pool.Asset = common.BNBAsset
	pool.LPUnits = cosmos.NewUint(100)

	// no asset balance
	synthSupply := cosmos.NewUint(100)
	units := pool.CalcUnits(version, synthSupply)
	c.Assert(pool.SynthUnits.Uint64(), Equals, uint64(0),
		Commentf("pool without asset balance should have zero synth units"))
	c.Assert(units.Uint64(), Equals, uint64(100),
		Commentf("pool without asset balance should return LPUnits"))

	// asset balance <= synthSupply / 2
	pool.BalanceAsset = cosmos.NewUint(100)
	pool.BalanceRune = cosmos.NewUint(100)
	pool.LPUnits = cosmos.NewUint(100)
	synthSupply = cosmos.NewUint(200)
	units = pool.CalcUnits(version, synthSupply)
	c.Assert(pool.SynthUnits.Uint64(), Equals, uint64(20_000))
	c.Assert(units.Uint64(), Equals, uint64(20_100))

	// normal case
	pool.BalanceAsset = cosmos.NewUint(1_000)
	pool.BalanceRune = cosmos.NewUint(1_000)
	synthSupply = cosmos.NewUint(100)
	units = pool.CalcUnits(version, synthSupply)
	c.Assert(pool.SynthUnits.Uint64(), Equals, uint64(5))
	c.Assert(units.Uint64(), Equals, uint64(105))
}

func (PoolTestSuite) TestReimbursementAndDisbursement(c *C) {
	p := NewPool()
	c.Check(p.IsEmpty(), Equals, true)
	p.Asset = common.BNBAsset
	c.Check(p.IsEmpty(), Equals, false)
	p.BalanceRune = cosmos.NewUint(100 * common.One)
	p.BalanceAsset = cosmos.NewUint(50 * common.One)
	c.Check(p.RuneDisbursementForAssetAdd(cosmos.NewUint(150*common.One)).Equal(cosmos.NewUint(75*common.One)), Equals, true)
	c.Check(p.AssetDisbursementForRuneAdd(cosmos.NewUint(900*common.One)).Equal(cosmos.NewUint(45*common.One)), Equals, true)

	p.BalanceRune = cosmos.NewUint(0)
	c.Check(p.RuneDisbursementForAssetAdd(cosmos.NewUint(1*common.One)).Equal(cosmos.NewUint(0*common.One)), Equals, true)
	c.Check(p.AssetDisbursementForRuneAdd(cosmos.NewUint(1*common.One)).Equal(cosmos.NewUint(0*common.One)), Equals, true)

	p.BalanceRune = cosmos.NewUint(100 * common.One)
	p.BalanceAsset = cosmos.NewUint(0)
	c.Check(p.RuneDisbursementForAssetAdd(cosmos.NewUint(1*common.One)).Equal(cosmos.NewUint(0*common.One)), Equals, true)
	c.Check(p.AssetDisbursementForRuneAdd(cosmos.NewUint(1*common.One)).Equal(cosmos.NewUint(0*common.One)), Equals, true)
}

func (PoolTestSuite) TestLUVI(c *C) {
	p := NewPool()
	p.BalanceRune = cosmos.NewUint(100)
	p.BalanceAsset = cosmos.NewUint(50)
	p.LPUnits = cosmos.NewUint(75)
	p.SynthUnits = cosmos.NewUint(12)
	c.Check(p.GetLUVI().String(), Equals, "812766415156")
}
