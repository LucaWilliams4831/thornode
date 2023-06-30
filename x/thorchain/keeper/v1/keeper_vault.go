package keeperv1

import (
	"errors"
	"fmt"
	"sort"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	kvTypes "gitlab.com/thorchain/thornode/x/thorchain/keeper/types"
)

func (k KVStore) setVault(ctx cosmos.Context, key string, record Vault) {
	store := ctx.KVStore(k.storeKey)
	buf := k.cdc.MustMarshal(&record)
	if buf == nil {
		store.Delete([]byte(key))
	} else {
		store.Set([]byte(key), buf)
	}
}

func (k KVStore) getVault(ctx cosmos.Context, key string, record *Vault) (bool, error) {
	store := ctx.KVStore(k.storeKey)
	if !store.Has([]byte(key)) {
		return false, nil
	}

	bz := store.Get([]byte(key))
	if err := k.cdc.Unmarshal(bz, record); err != nil {
		return true, dbError(ctx, fmt.Sprintf("Unmarshal kvstore: (%T) %s", record, key), err)
	}
	return true, nil
}

// GetVaultIterator only iterate vault pools
func (k KVStore) GetVaultIterator(ctx cosmos.Context) cosmos.Iterator {
	return k.getIterator(ctx, prefixVault)
}

// GetMostSecure with given list of vaults, find the vault that is most secure
func (k KVStore) GetMostSecure(ctx cosmos.Context, vaults Vaults, signingTransPeriod int64) Vault {
	vaults = k.SortBySecurity(ctx, vaults, signingTransPeriod)
	if len(vaults) == 0 {
		return Vault{}
	}
	return vaults[len(vaults)-1]
}

// GetLeastSecure with given list of vaults, find the vault that is least secure
func (k KVStore) GetLeastSecure(ctx cosmos.Context, vaults Vaults, signingTransPeriod int64) Vault {
	vaults = k.SortBySecurity(ctx, vaults, signingTransPeriod)
	if len(vaults) == 0 {
		return Vault{}
	}
	return vaults[0]
}

// SortBySecurity sorts a list of vaults in an order by how close the total
// value of the vault is to the total bond of the members of that vault. Sorts
// by least secure to most secure.
func (k KVStore) SortBySecurity(ctx cosmos.Context, vaults Vaults, signingTransPeriod int64) Vaults {
	if len(vaults) <= 1 {
		return vaults
	}

	type VaultSecurity struct {
		Vault Vault
		Diff  int64
	}

	vaultSecurity := make([]VaultSecurity, len(vaults))

	for i, vault := range vaults {
		// get total bond
		totalBond := cosmos.ZeroUint()
		for _, pk := range vault.GetMembership() {
			na, err := k.GetNodeAccountByPubKey(ctx, pk)
			if err != nil {
				ctx.Logger().Error("failed to get node account by pubkey", "error", err)
				continue
			}
			if na.Status == NodeActive {
				totalBond = totalBond.Add(na.Bond)
			}
		}

		// get total value
		totalValue := cosmos.ZeroUint()
		for _, coin := range vault.Coins {
			if coin.Asset.IsRune() {
				continue
			} else {
				pool, err := k.GetPool(ctx, coin.Asset)
				if err != nil {
					ctx.Logger().Error("failed to get pool", "error", err)
					continue
				}
				totalValue = totalValue.Add(pool.AssetValueInRune(coin.Amount))
			}
		}

		// add recent unsent txout items to totalValue
		h := ctx.BlockHeight() - signingTransPeriod
		if h < 1 {
			h = 1
		}
		for height := h; height <= ctx.BlockHeight(); height += 1 {
			txOut, err := k.GetTxOut(ctx, height)
			if err != nil {
				ctx.Logger().Error("unable to get txout", "error", err)
				continue
			}
			for _, item := range txOut.TxArray {
				if item.OutHash.IsEmpty() {
					toAddress, err := vault.PubKey.GetAddress(item.Coin.Asset.GetChain())
					if err != nil {
						ctx.Logger().Error("failed to get address of chain", "error", err)
						continue
					}
					if item.VaultPubKey.Equals(vault.PubKey) {
						if item.Coin.Asset.IsRune() {
							totalValue = common.SafeSub(totalValue, item.Coin.Amount)
						} else {
							pool, err := k.GetPool(ctx, item.Coin.Asset)
							if err != nil {
								ctx.Logger().Error("failed to get pool", "error", err)
								continue
							}
							totalValue = common.SafeSub(totalValue, pool.AssetValueInRune(item.Coin.Amount))
						}
					} else if item.ToAddress.Equals(toAddress) {
						if item.Coin.Asset.IsRune() {
							totalValue = totalValue.Add(item.Coin.Amount)
						} else {
							pool, err := k.GetPool(ctx, item.Coin.Asset)
							if err != nil {
								ctx.Logger().Error("failed to get pool", "error", err)
								continue
							}
							totalValue = totalValue.Add(pool.AssetValueInRune(item.Coin.Amount))
						}
					}
				}
			}
		}

		if totalValue.GT(totalBond) {
			vaultSecurity[i] = VaultSecurity{
				Vault: vault,
				Diff:  -(int64(common.SafeSub(totalValue, totalBond).Uint64())),
			}
		} else {
			vaultSecurity[i] = VaultSecurity{
				Vault: vault,
				Diff:  int64(common.SafeSub(totalBond, totalValue).Uint64()),
			}
		}
	}

	// sort by how far total bond and total value are from each other
	sort.SliceStable(vaultSecurity, func(i, j int) bool {
		return vaultSecurity[i].Diff < vaultSecurity[j].Diff
	})

	final := make(Vaults, len(vaultSecurity))
	for i, v := range vaultSecurity {
		final[i] = v.Vault
	}

	return final
}

// SetVault save the Vault object to store
func (k KVStore) SetVault(ctx cosmos.Context, vault Vault) error {
	if vault.IsAsgard() {
		if err := k.addAsgardIndex(ctx, vault.PubKey); err != nil {
			return err
		}
	}

	k.setVault(ctx, k.GetKey(ctx, prefixVault, vault.PubKey.String()), vault)
	return nil
}

// VaultExists check whether the given pubkey is associated with a vault
func (k KVStore) VaultExists(ctx cosmos.Context, pk common.PubKey) bool {
	return k.has(ctx, k.GetKey(ctx, prefixVault, pk.String()))
}

// GetVault get Vault with the given pubkey from data store
func (k KVStore) GetVault(ctx cosmos.Context, pk common.PubKey) (Vault, error) {
	record := Vault{
		BlockHeight: ctx.BlockHeight(),
		PubKey:      pk,
	}
	ok, err := k.getVault(ctx, k.GetKey(ctx, prefixVault, pk.String()), &record)
	if !ok {
		return record, fmt.Errorf("vault with pubkey(%s) doesn't exist: %w", pk, kvTypes.ErrVaultNotFound)
	}
	if record.PubKey.IsEmpty() {
		record.PubKey = pk
	}
	return record, err
}

// HasValidVaultPools check the data store to see whether we have a valid vault
func (k KVStore) HasValidVaultPools(ctx cosmos.Context) (bool, error) {
	iterator := k.GetVaultIterator(ctx)
	defer iterator.Close()
	for ; iterator.Valid(); iterator.Next() {
		var vault Vault
		if err := k.cdc.Unmarshal(iterator.Value(), &vault); err != nil {
			return false, dbError(ctx, "fail to unmarshal vault", err)
		}
		if vault.HasFunds() {
			return true, nil
		}
	}
	return false, nil
}

func (k KVStore) getAsgardIndex(ctx cosmos.Context) (common.PubKeys, error) {
	record := make([]string, 0)
	_, err := k.getStrings(ctx, k.GetKey(ctx, prefixVaultAsgardIndex, ""), &record)
	if err != nil {
		return nil, err
	}
	pks := make(common.PubKeys, len(record))
	for i, s := range record {
		pks[i], err = common.NewPubKey(s)
		if err != nil {
			return nil, err
		}
	}
	return pks, nil
}

func (k KVStore) addAsgardIndex(ctx cosmos.Context, pubkey common.PubKey) error {
	pks, err := k.getAsgardIndex(ctx)
	if err != nil {
		return err
	}
	for _, pk := range pks {
		if pk.Equals(pubkey) {
			return nil
		}
	}
	pks = append(pks, pubkey)
	k.setStrings(ctx, k.GetKey(ctx, prefixVaultAsgardIndex, ""), pks.Strings())
	return nil
}

func (k KVStore) RemoveFromAsgardIndex(ctx cosmos.Context, pubkey common.PubKey) error {
	pks, err := k.getAsgardIndex(ctx)
	if err != nil {
		return err
	}

	newPks := common.PubKeys{}
	for _, pk := range pks {
		if !pk.Equals(pubkey) {
			newPks = append(newPks, pk)
		}
	}

	k.setStrings(ctx, k.GetKey(ctx, prefixVaultAsgardIndex, ""), newPks.Strings())
	return nil
}

// GetAsgardVaults return all asgard vaults
func (k KVStore) GetAsgardVaults(ctx cosmos.Context) (Vaults, error) {
	pks, err := k.getAsgardIndex(ctx)
	if err != nil {
		return nil, err
	}

	var asgards Vaults
	for _, pk := range pks {
		vault, err := k.GetVault(ctx, pk)
		if err != nil {
			return nil, err
		}
		if vault.IsAsgard() {
			asgards = append(asgards, vault)
		}
	}

	return asgards, nil
}

// GetAsgardVaultsByStatus get all the asgard vault that have the given status
func (k KVStore) GetAsgardVaultsByStatus(ctx cosmos.Context, status VaultStatus) (Vaults, error) {
	all, err := k.GetAsgardVaults(ctx)
	if err != nil {
		return nil, err
	}

	var asgards Vaults
	for _, vault := range all {
		if vault.Status == status {
			asgards = append(asgards, vault)
		}
	}

	return asgards, nil
}

// DeleteVault remove the given vault from data store
func (k KVStore) DeleteVault(ctx cosmos.Context, pubkey common.PubKey) error {
	vault, err := k.GetVault(ctx, pubkey)
	if err != nil {
		if errors.Is(err, kvTypes.ErrVaultNotFound) {
			return nil
		}
		return err
	}

	if vault.HasFunds() {
		return errors.New("unable to delete vault: it still contains funds")
	}

	if vault.IsAsgard() {
		if err := k.RemoveFromAsgardIndex(ctx, pubkey); err != nil {
			ctx.Logger().Error("fail to remove vault from asgard index", "error", err)
		}
	}
	// delete the actual vault
	k.del(ctx, k.GetKey(ctx, prefixVault, vault.PubKey.String()))
	return nil
}
