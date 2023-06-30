package types

import (
	. "gopkg.in/check.v1"

	"github.com/cosmos/cosmos-sdk/store"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	paramstypes "github.com/cosmos/cosmos-sdk/x/params/types"
	"github.com/tendermint/tendermint/libs/log"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	dbm "github.com/tendermint/tm-db"
	cosmos "gitlab.com/thorchain/thornode/common/cosmos"
)

type JailSuite struct{}

var _ = Suite(&JailSuite{})

func (s JailSuite) TestNewJail(c *C) {
	addr := GetRandomBech32Addr()
	jail := NewJail(addr)
	c.Check(jail.NodeAddress.Equals(addr), Equals, true)
	c.Check(jail.ReleaseHeight, Equals, int64(0))
	c.Check(jail.Reason, Equals, "")
}

func (s JailSuite) TestIsJailed(c *C) {
	addr := GetRandomBech32Addr()
	jail := NewJail(addr)

	keyAcc := cosmos.NewKVStoreKey(authtypes.StoreKey)
	keyParams := cosmos.NewKVStoreKey(paramstypes.StoreKey)
	tkeyParams := cosmos.NewTransientStoreKey(paramstypes.TStoreKey)

	db := dbm.NewMemDB()
	ms := store.NewCommitMultiStore(db)
	ms.MountStoreWithDB(keyAcc, cosmos.StoreTypeIAVL, db)
	ms.MountStoreWithDB(keyParams, cosmos.StoreTypeIAVL, db)
	ms.MountStoreWithDB(cosmos.NewKVStoreKey("thorchain"), cosmos.StoreTypeIAVL, db)
	ms.MountStoreWithDB(tkeyParams, cosmos.StoreTypeTransient, db)
	err := ms.LoadLatestVersion()
	c.Assert(err, IsNil)

	ctx := cosmos.NewContext(ms, tmproto.Header{ChainID: "thorchain"}, false, log.NewNopLogger())
	ctx = ctx.WithBlockHeight(100)

	c.Check(jail.IsJailed(ctx), Equals, false)
	jail.ReleaseHeight = 100
	c.Check(jail.IsJailed(ctx), Equals, false)
	jail.ReleaseHeight = 101
	c.Check(jail.IsJailed(ctx), Equals, true)
}
