package types

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"strings"

	"github.com/blang/semver"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
)

// Valid is to check whether the pool status is valid or not
func (x PoolStatus) Valid() error {
	if _, ok := PoolStatus_value[x.String()]; !ok {
		return errors.New("invalid pool status")
	}
	return nil
}

// MarshalJSON marshal PoolStatus to JSON in string form
func (x PoolStatus) MarshalJSON() ([]byte, error) {
	return json.Marshal(x.String())
}

// UnmarshalJSON convert string form back to PoolStatus
func (x *PoolStatus) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	*x = GetPoolStatus(s)
	return nil
}

// GetPoolStatus from string
func GetPoolStatus(ps string) PoolStatus {
	if val, ok := PoolStatus_value[ps]; ok {
		return PoolStatus(val)
	}
	return PoolStatus_Suspended
}

// Pools represent a list of pools
type Pools []Pool

// NewPool Returns a new Pool
func NewPool() Pool {
	return Pool{
		BalanceRune:         cosmos.ZeroUint(),
		BalanceAsset:        cosmos.ZeroUint(),
		SynthUnits:          cosmos.ZeroUint(),
		LPUnits:             cosmos.ZeroUint(),
		PendingInboundRune:  cosmos.ZeroUint(),
		PendingInboundAsset: cosmos.ZeroUint(),
		Status:              PoolStatus_Available,
	}
}

// Valid check whether the pool is valid or not, if asset is empty then it is not valid
func (m Pool) Valid() error {
	if m.IsEmpty() {
		return errors.New("pool asset cannot be empty")
	}
	return nil
}

func (m *Pool) GetPoolUnits() cosmos.Uint {
	return m.LPUnits.Add(m.SynthUnits)
}

func (m *Pool) CalcUnits(version semver.Version, s cosmos.Uint) cosmos.Uint {
	if version.GTE(semver.MustParse("0.80.0")) {
		return m.CalcUnitsV80(s)
	}
	return m.GetPoolUnits()
}

func (m *Pool) CalcUnitsV80(s cosmos.Uint) cosmos.Uint {
	// Calculate synth units
	// (L*S)/(2*A-S)
	// S := k.GetTotalSupply(ctx, p.Asset.GetSyntheticAsset())
	if m.BalanceAsset.IsZero() || m.Asset.IsVaultAsset() {
		m.SynthUnits = cosmos.ZeroUint()
	} else {
		numerator := m.LPUnits.Mul(s)
		denominator := common.SafeSub(m.BalanceAsset.MulUint64(2), s)
		if denominator.IsZero() {
			denominator = cosmos.OneUint()
		}
		m.SynthUnits = numerator.Quo(denominator)
	}
	return m.GetPoolUnits()
}

// IsAvailable check whether the pool is in Available status
func (m Pool) IsAvailable() bool {
	return m.Status == PoolStatus_Available
}

// IsEmpty will return true when the asset is empty
func (m Pool) IsEmpty() bool {
	return m.Asset.IsEmpty()
}

// String implement fmt.Stringer
func (m Pool) String() string {
	sb := strings.Builder{}
	sb.WriteString(fmt.Sprintln("rune-balance: " + m.BalanceRune.String()))
	sb.WriteString(fmt.Sprintln("asset-balance: " + m.BalanceAsset.String()))
	sb.WriteString(fmt.Sprintln("asset: " + m.Asset.String()))
	sb.WriteString(fmt.Sprintln("synth-units: " + m.SynthUnits.String()))
	sb.WriteString(fmt.Sprintln("lp-units: " + m.LPUnits.String()))
	sb.WriteString(fmt.Sprintln("pending-inbound-rune: " + m.PendingInboundRune.String()))
	sb.WriteString(fmt.Sprintln("pending-inbound-asset: " + m.PendingInboundAsset.String()))
	sb.WriteString(fmt.Sprintln("status: " + m.Status.String()))
	sb.WriteString(fmt.Sprintln("decimals:" + strconv.FormatInt(m.Decimals, 10)))
	return sb.String()
}

// EnsureValidPoolStatus make sure the pool is in a valid status otherwise it return an error
func (m Pool) EnsureValidPoolStatus(msg cosmos.Msg) error {
	switch m.Status {
	case PoolStatus_Available:
		return nil
	case PoolStatus_Staged:
		_, ok := msg.(*MsgSwap)
		if ok {
			return errors.New("pool is in staged status, can't swap")
		}
		return nil
	case PoolStatus_Suspended:
		return errors.New("pool suspended")
	default:
		return fmt.Errorf("unknown pool status,%s", m.Status)
	}
}

// AssetValueInRune convert a specific amount of asset amt into its rune value
func (m Pool) AssetValueInRune(amt cosmos.Uint) cosmos.Uint {
	if m.BalanceRune.IsZero() || m.BalanceAsset.IsZero() {
		return cosmos.ZeroUint()
	}
	return common.GetUncappedShare(m.BalanceRune, m.BalanceAsset, amt)
}

// RuneReimbursementForAssetWithdrawal returns the equivalent amount of rune for a
// given amount of asset withdrawn from the pool, taking slip into account. When
// this amount is added to the pool, the constant product of depths rule is
// preserved.
func (m Pool) RuneReimbursementForAssetWithdrawal(amt cosmos.Uint) cosmos.Uint {
	if m.BalanceRune.IsZero() || m.BalanceAsset.IsZero() {
		return cosmos.ZeroUint()
	}
	denom := common.SafeSub(m.BalanceAsset, amt)
	if denom.IsZero() {
		// With slip, as the amount approaches the entire asset balance of the pool
		// the equivalent rune value approaches infinity. Return 0 in the limiting
		// case.
		return cosmos.ZeroUint()
	}
	return common.GetUncappedShare(m.BalanceRune, denom, amt)
}

// RuneValueInAsset convert a specific amount of rune amt into its asset value
func (m Pool) RuneValueInAsset(amt cosmos.Uint) cosmos.Uint {
	if m.BalanceRune.IsZero() || m.BalanceAsset.IsZero() {
		return cosmos.ZeroUint()
	}
	assetAmt := common.GetUncappedShare(m.BalanceAsset, m.BalanceRune, amt)
	return cosmos.RoundToDecimal(assetAmt, m.Decimals)
}

func (m Pools) Get(asset common.Asset) (Pool, bool) {
	for _, p := range m {
		if p.Asset.Equals(asset) {
			return p, true
		}
	}
	return NewPool(), false
}

func (m Pools) Set(pool Pool) Pools {
	for i, p := range m {
		if p.Asset.Equals(pool.Asset) {
			m[i] = pool
		}
	}
	m = append(m, pool)
	return m
}

// RuneDisbursementForAssetAdd returns the equivalent amount of rune for a
// given amount of asset added to the pool, taking slip into account. When this
// amount is withdrawn from the pool, the constant product of depths rule is
// preserved.
func (m Pool) RuneDisbursementForAssetAdd(amt cosmos.Uint) cosmos.Uint {
	if m.BalanceRune.IsZero() || m.BalanceAsset.IsZero() {
		return cosmos.ZeroUint()
	}
	denom := m.BalanceAsset.Add(amt)
	return common.GetUncappedShare(m.BalanceRune, denom, amt)
}

// AssetDisbursementForRuneAdd returns the equivalent amount of asset for a
// given amount of rune added to the pool, taking slip into account. When this
// amount is withdrawn from the pool, the constant product of depths rule is
// preserved.
func (m Pool) AssetDisbursementForRuneAdd(amt cosmos.Uint) cosmos.Uint {
	if m.BalanceRune.IsZero() || m.BalanceAsset.IsZero() {
		return cosmos.ZeroUint()
	}
	denom := m.BalanceRune.Add(amt)
	outAmt := common.GetUncappedShare(m.BalanceAsset, denom, amt)
	return cosmos.RoundToDecimal(outAmt, m.Decimals)
}

func (m Pool) GetLUVI() cosmos.Uint {
	bigInt := &big.Int{}
	balRune := m.BalanceRune.MulUint64(1e12).BigInt()
	balAsset := m.BalanceAsset.MulUint64(1e12).BigInt()
	num := bigInt.Mul(balRune, balAsset)
	num = bigInt.Sqrt(num)
	denom := m.GetPoolUnits().BigInt()
	if len(denom.Bits()) == 0 {
		return cosmos.ZeroUint()
	}
	result := bigInt.Quo(num, denom)
	return cosmos.NewUintFromBigInt(result)
}
