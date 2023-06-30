package thorchain

import (
	cosmos "gitlab.com/thorchain/thornode/common/cosmos"
)

type DummyYggManager struct{}

func NewDummyYggManger() *DummyYggManager {
	return &DummyYggManager{}
}

func (DummyYggManager) Fund(ctx cosmos.Context, mgr Manager) error {
	return errKaboom
}
