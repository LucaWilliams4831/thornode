package common

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/gogo/protobuf/jsonpb"
)

var (
	// EmptyAsset empty asset, not valid
	EmptyAsset = Asset{Chain: EmptyChain, Symbol: "", Ticker: "", Synth: false}
	// LUNAAsset LUNA
	LUNAAsset = Asset{Chain: TERRAChain, Symbol: "LUNA", Ticker: "LUNA", Synth: false}
	// ATOMAsset ATOM
	ATOMAsset = Asset{Chain: GAIAChain, Symbol: "ATOM", Ticker: "ATOM", Synth: false}
	// BNBAsset BNB
	BNBAsset = Asset{Chain: BNBChain, Symbol: "BNB", Ticker: "BNB", Synth: false}
	// BNBBEP20Asset BNB
	BNBBEP20Asset = Asset{Chain: BSCChain, Symbol: "BNB", Ticker: "BNB", Synth: false}
	// BTCAsset BTC
	BTCAsset = Asset{Chain: BTCChain, Symbol: "BTC", Ticker: "BTC", Synth: false}
	// LTCAsset BTC
	LTCAsset = Asset{Chain: LTCChain, Symbol: "LTC", Ticker: "LTC", Synth: false}
	// BCHAsset BCH
	BCHAsset = Asset{Chain: BCHChain, Symbol: "BCH", Ticker: "BCH", Synth: false}
	// DOGEAsset DOGE
	DOGEAsset = Asset{Chain: DOGEChain, Symbol: "DOGE", Ticker: "DOGE", Synth: false}
	// ETHAsset ETH
	ETHAsset = Asset{Chain: ETHChain, Symbol: "ETH", Ticker: "ETH", Synth: false}
	// AVAXAsset AVAX
	AVAXAsset = Asset{Chain: AVAXChain, Symbol: "AVAX", Ticker: "AVAX", Synth: false}
	// Rune67CAsset RUNE on Binance test net
	Rune67CAsset = Asset{Chain: BNBChain, Symbol: "RUNE-67C", Ticker: "RUNE", Synth: false} // testnet asset on binance ganges
	// RuneB1AAsset RUNE on Binance main net
	RuneB1AAsset = Asset{Chain: BNBChain, Symbol: "RUNE-B1A", Ticker: "RUNE", Synth: false} // mainnet
	// RuneNative RUNE on thorchain
	RuneNative            = Asset{Chain: THORChain, Symbol: "RUNE", Ticker: "RUNE", Synth: false}
	RuneERC20Asset        = Asset{Chain: ETHChain, Symbol: "RUNE-0x3155ba85d5f96b2d030a4966af206230e46849cb", Ticker: "RUNE", Synth: false}
	RuneERC20TestnetAsset = Asset{Chain: ETHChain, Symbol: "RUNE-0xd601c6A3a36721320573885A8d8420746dA3d7A0", Ticker: "RUNE", Synth: false}
	TOR                   = Asset{Chain: THORChain, Symbol: "TOR", Ticker: "TOR", Synth: false}
	THORBTC               = Asset{Chain: THORChain, Symbol: "BTC", Ticker: "BTC", Synth: false}
)

// NewAsset parse the given input into Asset object
func NewAsset(input string) (Asset, error) {
	var err error
	var asset Asset
	var sym string
	var parts []string
	if strings.Count(input, "/") > 0 {
		parts = strings.SplitN(input, "/", 2)
		asset.Synth = true
	} else {
		parts = strings.SplitN(input, ".", 2)
		asset.Synth = false
	}
	if len(parts) == 1 {
		asset.Chain = THORChain
		sym = parts[0]
	} else {
		asset.Chain, err = NewChain(parts[0])
		if err != nil {
			return EmptyAsset, err
		}
		sym = parts[1]
	}

	asset.Symbol, err = NewSymbol(sym)
	if err != nil {
		return EmptyAsset, err
	}

	parts = strings.SplitN(sym, "-", 2)
	asset.Ticker, err = NewTicker(parts[0])
	if err != nil {
		return EmptyAsset, err
	}

	return asset, nil
}

// Equals determinate whether two assets are equivalent
func (a Asset) Equals(a2 Asset) bool {
	return a.Chain.Equals(a2.Chain) && a.Symbol.Equals(a2.Symbol) && a.Ticker.Equals(a2.Ticker) && a.Synth == a2.Synth
}

func (a Asset) GetChain() Chain {
	if a.Synth {
		return THORChain
	}
	return a.Chain
}

// Get layer1 asset version
func (a Asset) GetLayer1Asset() Asset {
	if !a.IsSyntheticAsset() {
		return a
	}
	return Asset{
		Chain:  a.Chain,
		Symbol: a.Symbol,
		Ticker: a.Ticker,
		Synth:  false,
	}
}

// Get synthetic asset of asset
func (a Asset) GetSyntheticAsset() Asset {
	if a.IsSyntheticAsset() {
		return a
	}
	return Asset{
		Chain:  a.Chain,
		Symbol: a.Symbol,
		Ticker: a.Ticker,
		Synth:  true,
	}
}

// Get derived asset of asset
func (a Asset) GetDerivedAsset() Asset {
	return Asset{
		Chain:  THORChain,
		Symbol: a.Symbol,
		Ticker: a.Ticker,
		Synth:  false,
	}
}

// Check if asset is a pegged asset
func (a Asset) IsSyntheticAsset() bool {
	return a.Synth
}

func (a Asset) IsVaultAsset() bool {
	return a.IsSyntheticAsset()
}

// Check if asset is a derived asset
func (a Asset) IsDerivedAsset() bool {
	return !a.Synth && a.GetChain().IsTHORChain() && !a.IsRune()
}

// Native return native asset, only relevant on THORChain
func (a Asset) Native() string {
	if a.IsRune() {
		return "rune"
	}
	if a.Equals(TOR) {
		return "tor"
	}
	return strings.ToLower(a.String())
}

// IsEmpty will be true when any of the field is empty, chain,symbol or ticker
func (a Asset) IsEmpty() bool {
	return a.Chain.IsEmpty() || a.Symbol.IsEmpty() || a.Ticker.IsEmpty()
}

// String implement fmt.Stringer , return the string representation of Asset
func (a Asset) String() string {
	div := "."
	if a.Synth {
		div = "/"
	}
	return fmt.Sprintf("%s%s%s", a.Chain.String(), div, a.Symbol.String())
}

// IsGasAsset check whether asset is base asset used to pay for gas
func (a Asset) IsGasAsset() bool {
	gasAsset := a.GetChain().GetGasAsset()
	if gasAsset.IsEmpty() {
		return false
	}
	return a.Equals(gasAsset)
}

// IsRune is a helper function ,return true only when the asset represent RUNE
func (a Asset) IsRune() bool {
	return a.Equals(BEP2RuneAsset()) || a.Equals(RuneNative) || a.Equals(ERC20RuneAsset())
}

// IsNativeRune is a helper function, return true only when the asset represent NATIVE RUNE
func (a Asset) IsNativeRune() bool {
	return a.IsRune() && a.Chain.IsTHORChain()
}

// IsNative is a helper function, returns true when the asset is a native
// asset to THORChain (ie rune, a synth, etc)
func (a Asset) IsNative() bool {
	return a.GetChain().IsTHORChain()
}

// IsBNB is a helper function, return true only when the asset represent BNB
func (a Asset) IsBNB() bool {
	return a.Equals(BNBAsset)
}

// MarshalJSON implement Marshaler interface
func (a Asset) MarshalJSON() ([]byte, error) {
	return json.Marshal(a.String())
}

// UnmarshalJSON implement Unmarshaler interface
func (a *Asset) UnmarshalJSON(data []byte) error {
	var err error
	var assetStr string
	if err := json.Unmarshal(data, &assetStr); err != nil {
		return err
	}
	if assetStr == "." {
		*a = EmptyAsset
		return nil
	}
	*a, err = NewAsset(assetStr)
	return err
}

// MarshalJSONPB implement jsonpb.Marshaler
func (a Asset) MarshalJSONPB(*jsonpb.Marshaler) ([]byte, error) {
	return a.MarshalJSON()
}

// UnmarshalJSONPB implement jsonpb.Unmarshaler
func (a *Asset) UnmarshalJSONPB(unmarshal *jsonpb.Unmarshaler, content []byte) error {
	return a.UnmarshalJSON(content)
}

// RuneAsset return RUNE Asset depends on different environment
func RuneAsset() Asset {
	return RuneNative
}

// BEP2RuneAsset is RUNE on BEP2
func BEP2RuneAsset() Asset {
	if strings.EqualFold(os.Getenv("NET"), "testnet") || strings.EqualFold(os.Getenv("NET"), "mocknet") {
		return Rune67CAsset
	}
	return RuneB1AAsset
}

// ERC20RuneAsset is RUNE on ETH
func ERC20RuneAsset() Asset {
	if strings.EqualFold(os.Getenv("NET"), "testnet") || strings.EqualFold(os.Getenv("NET"), "mocknet") {
		// On testnet/mocknet, return  ERC20_RUNE_CONTRACT if it is explicitly set
		if os.Getenv("ERC20_RUNE_CONTRACT") != "" {
			return Asset{
				Chain:  ETHChain,
				Symbol: Symbol(fmt.Sprintf("RUNE-%s", os.Getenv("ERC20_RUNE_CONTRACT"))),
				Ticker: "RUNE",
				Synth:  false,
			}
		}
		// Default to hardcoded address
		return RuneERC20TestnetAsset
	}
	return RuneERC20Asset
}

// Replace pool name "." with a "-" for Mimir key checking.
func (a Asset) MimirString() string {
	return a.Chain.String() + "-" + a.Symbol.String()
}
