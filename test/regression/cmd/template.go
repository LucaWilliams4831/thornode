package main

import (
	"fmt"
	"sync"
	"text/template"

	"gitlab.com/thorchain/thornode/constants"
)

////////////////////////////////////////////////////////////////////////////////////////
// Templates
////////////////////////////////////////////////////////////////////////////////////////

// nativeTxIDs are scoped to the routine and contain the native txids for all sent txs
var (
	nativeTxIDs   = map[int][]string{}
	nativeTxIDsMu = sync.Mutex{}
)

// templates contain all base templates referenced in tests
var templates *template.Template

// funcMap is a map of functions that can be used in all templates and tests
var funcMap = template.FuncMap{
	"observe_txid": func(i int) string {
		return fmt.Sprintf("%064x", i) // padded 64-bit hex string
	},
	"native_txid": func(i int) string {
		// this will get double-rendered
		return fmt.Sprintf("{{ native_txid %d }}", i)
	},
	"version": func() string {
		return constants.Version
	},
	"store_version": func() uint64 {
		return constants.SWVersion.Minor
	},
	"addr_module_thorchain": func() string {
		return ModuleAddrThorchain
	},
	"addr_module_asgard": func() string {
		return ModuleAddrAsgard
	},
	"addr_module_bond": func() string {
		return ModuleAddrBond
	},
	"addr_module_transfer": func() string {
		return ModuleAddrTransfer
	},
	"addr_module_reserve": func() string {
		return ModuleAddrReserve
	},
	"addr_module_fee_collector": func() string {
		return ModuleAddrFeeCollector
	},
	"addr_module_lending": func() string {
		return ModuleAddrLending
	},
}

////////////////////////////////////////////////////////////////////////////////////////
// Functions
////////////////////////////////////////////////////////////////////////////////////////

func init() {
	// register template names for all keys
	for k, v := range templateAddress {
		vv := v // copy
		funcMap[k] = func() string {
			return vv
		}
	}
	for k, v := range templatePubKey {
		vv := v // copy
		funcMap[k] = func() string {
			return vv
		}
	}

	// parse all templates with custom functions
	templates = template.Must(
		template.New("").Funcs(funcMap).ParseGlob("templates/*.yaml"),
	)
}
