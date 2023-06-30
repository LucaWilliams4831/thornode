package evm

import (
	evmtypes "gitlab.com/thorchain/thornode/bifrost/pkg/chainclients/shared/evm/types"
)

// BlockMetaAccessor define methods need to access block meta storage
type BlockMetaAccessor interface {
	GetBlockMetas() ([]*evmtypes.BlockMeta, error)
	GetBlockMeta(height int64) (*evmtypes.BlockMeta, error)
	SaveBlockMeta(height int64, block *evmtypes.BlockMeta) error
	PruneBlockMeta(height int64) error
	AddSignedTxItem(item evmtypes.SignedTxItem) error
	RemoveSignedTxItem(hash string) error
	GetSignedTxItems() ([]evmtypes.SignedTxItem, error)
}
