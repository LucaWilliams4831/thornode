package tokenlist

import (
	"encoding/json"

	"github.com/blang/semver"
	"gitlab.com/thorchain/thornode/common/tokenlist/bsctokens"
)

var bscTokenListV111 EVMTokenList

func init() {
	if err := json.Unmarshal(bsctokens.BSCTokenListRawV111, &bscTokenListV111); err != nil {
		panic(err)
	}
}

func GetBSCTokenList(version semver.Version) EVMTokenList {
	return bscTokenListV111
}
