package keeperv1

import (
	. "gopkg.in/check.v1"

	"gitlab.com/thorchain/thornode/common"
	cosmos "gitlab.com/thorchain/thornode/common/cosmos"
)

type KeeperLoanSuite struct{}

var _ = Suite(&KeeperLoanSuite{})

func (mas *KeeperLoanSuite) SetUpSuite(c *C) {
	SetupConfigForTest()
}

func (s *KeeperLoanSuite) TestLoan(c *C) {
	ctx, k := setupKeeperForTest(c)
	asset := common.BNBAsset

	loan, err := k.GetLoan(ctx, asset, GetRandomRUNEAddress())
	c.Assert(err, IsNil)
	c.Check(loan.CollateralUp, NotNil)
	c.Check(loan.CollateralDown, NotNil)

	loan.DebtUp = cosmos.NewUint(12)
	k.SetLoan(ctx, loan)
	loan, err = k.GetLoan(ctx, asset, loan.Owner)
	c.Assert(err, IsNil)
	c.Check(loan.Asset.Equals(asset), Equals, true)
	c.Check(loan.DebtUp.Equal(cosmos.NewUint(12)), Equals, true)
	iter := k.GetLoanIterator(ctx, common.BNBAsset)
	c.Check(iter, NotNil)
	iter.Close()
	k.RemoveLoan(ctx, loan)
}

func (s *KeeperLoanSuite) TestLoanTotalCollateral(c *C) {
	ctx, k := setupKeeperForTest(c)
	asset := common.BNBAsset

	amt := cosmos.NewUint(104)
	k.SetTotalCollateral(ctx, asset, amt)

	amt, err := k.GetTotalCollateral(ctx, asset)
	c.Assert(err, IsNil)
	c.Check(amt.Uint64(), Equals, uint64(104))
}
