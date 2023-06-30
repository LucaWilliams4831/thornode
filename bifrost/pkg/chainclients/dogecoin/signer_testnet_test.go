//go:build testnet
// +build testnet

package dogecoin

import (
	"github.com/eager7/dogd/chaincfg"
	"gitlab.com/thorchain/thornode/bifrost/pkg/chainclients/shared/utxo"
	stypes "gitlab.com/thorchain/thornode/bifrost/thorclient/types"
	"gitlab.com/thorchain/thornode/bifrost/tss"
	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	. "gopkg.in/check.v1"
)

func (s *DogecoinSignerSuite) TestGetChainCfg(c *C) {
	param := s.client.getChainCfg()
	c.Assert(param, Equals, &chaincfg.TestNet3Params)
}

func (s *DogecoinSignerSuite) TestSignTxWithTSS(c *C) {
	pubkey, err := common.NewPubKey("tthorpub1addwnpepqwznsrgk2t5vn2cszr6ku6zned6tqxknugzw3vhdcjza284d7djp5rql6vn")
	c.Assert(err, IsNil)
	addr, err := pubkey.GetAddress(common.DOGEChain)
	c.Assert(err, IsNil)
	txOutItem := stypes.TxOutItem{
		Chain:       common.DOGEChain,
		ToAddress:   addr,
		VaultPubKey: "tthorpub1addwnpepqtvzm6wa6ezgjj9l4sdvzcf64wf0wzs8x9mgjfhjp6tkzcvkyfyqg9a9p8e",
		Coins: common.Coins{
			common.NewCoin(common.DOGEAsset, cosmos.NewUint(10)),
		},
		MaxGas: common.Gas{
			common.NewCoin(common.DOGEAsset, cosmos.NewUint(1000)),
		},
		InHash:  "",
		OutHash: "",
	}

	thorKeyManager := &tss.MockThorchainKeyManager{}
	s.client.ksWrapper, err = NewKeySignWrapper(s.client.privateKey, thorKeyManager)
	txHash := "66d2d6b5eb564972c59e4797683a1225a02515a41119f0a8919381236b63e948"
	c.Assert(err, IsNil)
	// utxo := NewUnspentTransactionOutput(*txHash, 0, 0.00018, 100, txOutItem.VaultPubKey)
	blockMeta := utxo.NewBlockMeta("000000000000008a0da55afa8432af3b15c225cc7e04d32f0de912702dd9e2ae",
		100,
		"0000000000000068f0710c510e94bd29aa624745da43e32a1de887387306bfda")
	blockMeta.AddCustomerTransaction(txHash)
	c.Assert(s.client.temporalStorage.SaveBlockMeta(blockMeta.Height, blockMeta), IsNil)
	buf, _, _, err := s.client.SignTx(txOutItem, 1)
	c.Assert(err, IsNil)
	c.Assert(buf, NotNil)
}
