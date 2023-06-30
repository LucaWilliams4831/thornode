package ethereum

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/util"

	"gitlab.com/thorchain/thornode/bifrost/pkg/chainclients/ethereum/types"
)

const (
	// prefixTokenMeta declares prefix to use in leveldb to avoid conflicts
	// #nosec G101 this is just a prefix
	prefixTokenMeta = `eth-tokenmeta-`
)

// LevelDBTokenMeta struct
type LevelDBTokenMeta struct {
	db *leveldb.DB
}

// NewLevelDBTokenMeta creates a new level db backed TokenMeta
func NewLevelDBTokenMeta(db *leveldb.DB) (*LevelDBTokenMeta, error) {
	return &LevelDBTokenMeta{db: db}, nil
}

func (t *LevelDBTokenMeta) getTokenMetaKey(address string) string {
	return fmt.Sprintf("%s%s", prefixTokenMeta, strings.ToUpper(address))
}

// GetTokenMeta for given token address
func (t *LevelDBTokenMeta) GetTokenMeta(address string) (types.TokenMeta, error) {
	key := t.getTokenMetaKey(address)
	exist, err := t.db.Has([]byte(key), nil)
	if err != nil {
		return types.TokenMeta{}, fmt.Errorf("fail to check whether token meta(%s) exist: %w", key, err)
	}
	if !exist {
		return types.TokenMeta{}, nil
	}
	v, err := t.db.Get([]byte(key), nil)
	if err != nil {
		return types.TokenMeta{}, fmt.Errorf("fail to get token meta(%s) from storage: %w", key, err)
	}
	var tm types.TokenMeta
	if err := json.Unmarshal(v, &tm); err != nil {
		return types.TokenMeta{}, fmt.Errorf("fail to unmarshal token meta from json: %w", err)
	}
	return tm, nil
}

// SaveTokenMeta persistent the given TokenMeta into storage
func (t *LevelDBTokenMeta) SaveTokenMeta(symbol, address string, decimals uint64) error {
	key := t.getTokenMetaKey(address)
	tokenMeta := types.NewTokenMeta(symbol, address, decimals)
	buf, err := json.Marshal(tokenMeta)
	if err != nil {
		return fmt.Errorf("fail to marshal token meta to json: %w", err)
	}
	return t.db.Put([]byte(key), buf, nil)
}

// GetTokens returns all the token metas in storage
func (t *LevelDBTokenMeta) GetTokens() ([]*types.TokenMeta, error) {
	tokenMetas := make([]*types.TokenMeta, 0)
	iterator := t.db.NewIterator(util.BytesPrefix([]byte(prefixTokenMeta)), nil)
	defer iterator.Release()
	for iterator.Next() {
		buf := iterator.Value()
		if len(buf) == 0 {
			continue
		}
		var tokenMeta types.TokenMeta
		if err := json.Unmarshal(buf, &tokenMeta); err != nil {
			return nil, fmt.Errorf("fail to unmarshal token meta: %w", err)
		}
		found := false
		for _, item := range tokenMetas {
			if strings.EqualFold(item.Address, tokenMeta.Address) &&
				strings.EqualFold(item.Symbol, tokenMeta.Symbol) {
				found = true
				break
			}
		}
		if !found {
			tokenMetas = append(tokenMetas, &tokenMeta)
		}
	}
	return tokenMetas, nil
}
