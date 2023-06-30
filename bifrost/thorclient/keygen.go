package thorclient

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	btypes "gitlab.com/thorchain/thornode/bifrost/blockscanner/types"
	"gitlab.com/thorchain/thornode/x/thorchain/types"
)

// GetKeygenBlock retrieves keygen request for the given block height from thorchain
func (b *thorchainBridge) GetKeygenBlock(blockHeight int64, pk string) (types.KeygenBlock, error) {
	path := fmt.Sprintf("%s/%d/%s", KeygenEndpoint, blockHeight, pk)
	body, status, err := b.getWithPath(path)
	if err != nil {
		if status == http.StatusNotFound {
			return types.KeygenBlock{}, btypes.ErrUnavailableBlock
		}
		return types.KeygenBlock{}, fmt.Errorf("failed to get keygen for a block height: %w", err)
	}
	var query types.QueryKeygenBlock
	if err := json.Unmarshal(body, &query); err != nil {
		return types.KeygenBlock{}, fmt.Errorf("failed to unmarshal Keygen: %w", err)
	}

	if query.Signature == "" {
		return types.KeygenBlock{}, errors.New("invalid keygen signature: empty")
	}

	buf, err := json.Marshal(query.KeygenBlock)
	if err != nil {
		return types.KeygenBlock{}, fmt.Errorf("fail to marshal keygen block to json: %w", err)
	}

	pubKey := b.keys.GetSignerInfo().GetPubKey()
	s, err := base64.StdEncoding.DecodeString(query.Signature)
	if err != nil {
		return types.KeygenBlock{}, errors.New("invalid keygen signature: cannot decode signature")
	}
	if !pubKey.VerifySignature(buf, s) {
		return types.KeygenBlock{}, errors.New("invalid keygen signature: bad signature")
	}

	return query.KeygenBlock, nil
}
