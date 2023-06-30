package ethereum

import (
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	ecommon "github.com/ethereum/go-ethereum/common"
	etypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"gitlab.com/thorchain/thornode/bifrost/thorclient/types"
	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	memo "gitlab.com/thorchain/thornode/x/thorchain/memo"
)

type (
	contractAddressValidator func(addr *ecommon.Address, includeWhitelist bool) bool
	assetResolver            func(token string) (common.Asset, error)
	tokenDecimalResolver     func(token string) int64
	amountConverter          func(token string, amt *big.Int) cosmos.Uint
)

type SmartContractLogParser struct {
	addressValidator contractAddressValidator
	assetResolver    assetResolver
	decimalResolver  tokenDecimalResolver
	amtConverter     amountConverter
	logger           zerolog.Logger
	vaultABI         *abi.ABI
}

func NewSmartContractLogParser(validator contractAddressValidator,
	resolver assetResolver,
	decimalResolver tokenDecimalResolver,
	amtConverter amountConverter,
	vaultABI *abi.ABI,
) SmartContractLogParser {
	return SmartContractLogParser{
		addressValidator: validator,
		assetResolver:    resolver,
		decimalResolver:  decimalResolver,
		vaultABI:         vaultABI,
		amtConverter:     amtConverter,
		logger:           log.Logger.With().Str("module", "SmartContractLogParser").Logger(),
	}
}

// vaultDepositEvent represent a vault deposit
type vaultDepositEvent struct {
	To     ecommon.Address
	Asset  ecommon.Address
	Amount *big.Int
	Memo   string
}

func (scp *SmartContractLogParser) parseDeposit(log etypes.Log) (vaultDepositEvent, error) {
	const DepositEventName = "Deposit"
	event := vaultDepositEvent{}
	if err := scp.unpackVaultLog(&event, DepositEventName, log); err != nil {
		return event, fmt.Errorf("fail to unpack event: %w", err)
	}
	return event, nil
}

// RouterCoin represent the coins transfer between vault
type RouterCoin struct {
	Asset  ecommon.Address
	Amount *big.Int
}

type routerVaultTransfer struct {
	OldVault ecommon.Address
	NewVault ecommon.Address
	Coins    []RouterCoin
	Memo     string
}

func (scp *SmartContractLogParser) parseVaultTransfer(log etypes.Log) (routerVaultTransfer, error) {
	const vaultTransferEventName = "VaultTransfer"
	event := routerVaultTransfer{}
	if err := scp.unpackVaultLog(&event, vaultTransferEventName, log); err != nil {
		return event, fmt.Errorf("fail to unpack event: %w", err)
	}
	return event, nil
}

func (scp *SmartContractLogParser) unpackVaultLog(out interface{}, event string, log etypes.Log) error {
	if len(log.Topics) == 0 {
		return errors.New("topics field in event log is empty")
	}
	if log.Topics[0] != scp.vaultABI.Events[event].ID {
		return errors.New("event signature mismatch")
	}
	if len(log.Data) > 0 {
		if err := scp.vaultABI.UnpackIntoInterface(out, event, log.Data); err != nil {
			return fmt.Errorf("fail to parse event: %w", err)
		}
	}
	var indexed abi.Arguments
	for _, arg := range scp.vaultABI.Events[event].Inputs {
		if arg.Indexed {
			indexed = append(indexed, arg)
		}
	}
	return abi.ParseTopics(out, indexed, log.Topics[1:])
}

type vaultTransferOutEvent struct {
	Vault  ecommon.Address
	To     ecommon.Address
	Asset  ecommon.Address
	Amount *big.Int
	Memo   string
}

func (scp *SmartContractLogParser) parseTransferOut(log etypes.Log) (vaultTransferOutEvent, error) {
	const TransferOutEventName = "TransferOut"
	event := vaultTransferOutEvent{}
	if err := scp.unpackVaultLog(&event, TransferOutEventName, log); err != nil {
		return event, fmt.Errorf("fail to parse transfer out event")
	}
	return event, nil
}

type vaultTransferAllowanceEvent struct {
	OldVault ecommon.Address
	NewVault ecommon.Address
	Asset    ecommon.Address
	Amount   *big.Int
	Memo     string
}

func (scp *SmartContractLogParser) parseTransferAllowanceEvent(log etypes.Log) (vaultTransferAllowanceEvent, error) {
	const TransferAllowanceEventName = "TransferAllowance"
	event := vaultTransferAllowanceEvent{}
	if err := scp.unpackVaultLog(&event, TransferAllowanceEventName, log); err != nil {
		return event, fmt.Errorf("fail to parse transfer allowance event")
	}
	return event, nil
}

// THORChainRouterTransferOutAndCall represents a TransferOutAndCall event raised by the THORChainRouter contract.
type THORChainRouterTransferOutAndCall struct {
	Vault        ecommon.Address
	Target       ecommon.Address
	Amount       *big.Int
	FinalAsset   ecommon.Address
	To           ecommon.Address
	AmountOutMin *big.Int
	Memo         string
}

// ParseTransferOutAndCall is a log parse operation binding the contract event 0xbda904e26adea40cc083dc36e80fde1641dfdd8b9a035c44022a43e713f73d36.
func (scp *SmartContractLogParser) ParseTransferOutAndCall(log etypes.Log) (*THORChainRouterTransferOutAndCall, error) {
	const TransferOutAndCallEventName = "TransferOutAndCall"
	event := new(THORChainRouterTransferOutAndCall)
	if err := scp.unpackVaultLog(event, TransferOutAndCallEventName, log); err != nil {
		return nil, err
	}
	return event, nil
}

func (scp *SmartContractLogParser) getTxInItem(logs []*etypes.Log, txInItem *types.TxInItem) (bool, error) {
	if len(logs) == 0 {
		return false, nil
	}
	isVaultTransfer := false
	for _, item := range logs {
		// only events produced by THORChain router is processed
		if !scp.addressValidator(&item.Address, false) {
			continue
		}
		earlyExit := false
		switch item.Topics[0].String() {
		case depositEvent:
			// router contract , deposit function has re-entrance protection
			depositEvt, err := scp.parseDeposit(*item)
			if err != nil {
				scp.logger.Err(err).Msg("fail to parse deposit event")
				continue
			}
			scp.logger.Info().Msgf("deposit:%+v", depositEvt)
			if len(depositEvt.Amount.Bits()) == 0 {
				scp.logger.Info().Msg("deposit amount is 0, ignore")
				continue
			}
			if len(txInItem.To) > 0 && !strings.EqualFold(txInItem.To, depositEvt.To.String()) {
				return false, fmt.Errorf("multiple events in the same transaction, have different to addresses , ignore")
			}
			if len(txInItem.Memo) > 0 && !strings.EqualFold(txInItem.Memo, depositEvt.Memo) {
				return false, fmt.Errorf("multiple events in the same transaction , have different memo , ignore")
			}
			asset, err := scp.assetResolver(depositEvt.Asset.String())
			if err != nil {
				scp.logger.Err(err).Msgf("fail to get asset from token address: %s", depositEvt.Asset)
				continue
			}
			if asset.IsEmpty() {
				continue
			}
			txInItem.To = depositEvt.To.String()
			txInItem.Memo = depositEvt.Memo
			decimals := scp.decimalResolver(depositEvt.Asset.String())
			scp.logger.Info().Msgf("token:%s,decimals:%d", depositEvt.Asset, decimals)
			txInItem.Coins = append(txInItem.Coins,
				common.NewCoin(asset, scp.amtConverter(depositEvt.Asset.String(), depositEvt.Amount)).WithDecimals(decimals))
			isVaultTransfer = false
		case transferOutEvent:
			// it is not legal to have multiple transferOut event , transferOut event should be final
			transferOutEvt, err := scp.parseTransferOut(*item)
			if err != nil {
				scp.logger.Err(err).Msg("fail to parse transfer out event")
				continue
			}
			scp.logger.Info().Msgf("transfer out: %+v", transferOutEvt)
			m, err := memo.ParseMemo(common.LatestVersion, transferOutEvt.Memo)
			if err != nil {
				scp.logger.Err(err).Msgf("fail to parse memo: %s", transferOutEvt.Memo)
				continue
			}
			if !m.IsOutbound() && !m.IsType(memo.TxMigrate) && !m.IsType(memo.TxYggdrasilFund) {
				scp.logger.Error().Msgf("%s is not correct memo to use transfer out", transferOutEvt.Memo)
				continue
			}
			asset, err := scp.assetResolver(transferOutEvt.Asset.String())
			if err != nil {
				return false, fmt.Errorf("fail to get asset from token address: %w", err)
			}
			if asset.IsEmpty() {
				return false, nil
			}
			txInItem.To = transferOutEvt.To.String()
			txInItem.Memo = transferOutEvt.Memo
			decimals := scp.decimalResolver(transferOutEvt.Asset.String())
			txInItem.Coins = common.NewCoins(
				common.NewCoin(asset, scp.amtConverter(transferOutEvt.Asset.String(), transferOutEvt.Amount)).WithDecimals(decimals),
			)
			earlyExit = true
			isVaultTransfer = false
		case transferAllowanceEvent:
			// there is no circumstance , router will emit multiple transferAllowance event
			// if that does happen , it means something dodgy happened
			transferAllowanceEvt, err := scp.parseTransferAllowanceEvent(*item)
			if err != nil {
				scp.logger.Err(err).Msg("fail to parse transfer allowance event")
				continue
			}
			scp.logger.Info().Msgf("transfer allowance: %+v", transferAllowanceEvt)
			if len(transferAllowanceEvt.Amount.Bits()) == 0 {
				scp.logger.Error().Msg("transfer allowance event with amount 0, ignore")
				continue
			}
			if len(txInItem.Sender) > 0 && !strings.EqualFold(txInItem.Sender, transferAllowanceEvt.OldVault.String()) {
				scp.logger.Error().Msg("transfer allowance event , vault address is not the same as sender, ignore")
				continue
			}
			if len(txInItem.To) > 0 && !strings.EqualFold(txInItem.To, transferAllowanceEvt.NewVault.String()) {
				scp.logger.Error().Msg("multiple transfer allowance events , have different to addresses , ignore")
				continue
			}
			if len(txInItem.Memo) > 0 && !strings.EqualFold(txInItem.Memo, transferAllowanceEvt.Memo) {
				scp.logger.Error().Msg("multiple events in the same transaction , have different memo , ignore")
				continue
			}
			m, err := memo.ParseMemo(common.LatestVersion, transferAllowanceEvt.Memo)
			if err != nil {
				scp.logger.Err(err).Msgf("fail to parse memo: %s", transferAllowanceEvt.Memo)
				continue
			}
			if !(m.IsType(memo.TxMigrate) || m.IsType(memo.TxYggdrasilFund)) {
				scp.logger.Error().Msgf("%s is not correct memo to use transfer allowance", transferAllowanceEvt.Memo)
				continue
			}
			asset, err := scp.assetResolver(transferAllowanceEvt.Asset.String())
			if err != nil {
				scp.logger.Err(err).Msgf("fail to get asset from token address")
				continue
			}
			if asset.IsEmpty() {
				continue
			}
			txInItem.To = transferAllowanceEvt.NewVault.String()
			txInItem.Memo = transferAllowanceEvt.Memo
			decimals := scp.decimalResolver(transferAllowanceEvt.Asset.String())
			txInItem.Coins = common.NewCoins(
				common.NewCoin(asset, scp.amtConverter(transferAllowanceEvt.Asset.String(), transferAllowanceEvt.Amount)).WithDecimals(decimals),
			)
			isVaultTransfer = false
		case vaultTransferEvent:
			transferEvent, err := scp.parseVaultTransfer(*item)
			if err != nil {
				scp.logger.Err(err).Msg("fail to parse vault transfer event")
				continue
			}
			scp.logger.Info().Msgf("vault transfer: %+v", transferEvent)
			if len(txInItem.Sender) > 0 && !strings.EqualFold(txInItem.Sender, transferEvent.OldVault.String()) {
				scp.logger.Error().Msg("vault transfer event , vault address is not the same as sender, ignore")
				continue
			}
			if len(txInItem.To) > 0 && !strings.EqualFold(txInItem.To, transferEvent.NewVault.String()) {
				scp.logger.Error().Msg("multiple vaultTransfer events , have different to addresses , ignore")
				continue
			}
			if len(txInItem.Memo) > 0 && !strings.EqualFold(txInItem.Memo, transferEvent.Memo) {
				scp.logger.Error().Msg("multiple events in the same transaction , have different memo , ignore")
				continue
			}
			m, err := memo.ParseMemo(common.LatestVersion, transferEvent.Memo)
			if err != nil {
				scp.logger.Err(err).Msgf("fail to parse memo: %s", transferEvent.Memo)
				continue
			}
			if !m.IsType(memo.TxYggdrasilReturn) {
				scp.logger.Error().Msgf("%s is not yggdrasil return memo", transferEvent.Memo)
				continue
			}
			txInItem.To = transferEvent.NewVault.String()
			txInItem.Memo = transferEvent.Memo
			var totalCoins common.Coins
			for _, item := range transferEvent.Coins {
				asset, err := scp.assetResolver(item.Asset.String())
				if err != nil {
					scp.logger.Err(err).Msg("fail to get asset from token address")
					continue
				}
				if asset.IsEmpty() {
					continue
				}
				decimals := scp.decimalResolver(item.Asset.String())
				totalCoins = append(totalCoins, common.NewCoin(asset, scp.amtConverter(item.Asset.String(), item.Amount)).WithDecimals(decimals))
			}
			txInItem.Coins = totalCoins
			isVaultTransfer = true
		case transferOutAndCallEvent:
			transferOutAndCall, err := scp.ParseTransferOutAndCall(*item)
			if err != nil {
				scp.logger.Err(err).Msg("fail to parse transferOutAndCall event")
				continue
			}
			scp.logger.Info().Msgf("transferOutAndCall: %+v", transferOutAndCall)
			m, err := memo.ParseMemo(common.LatestVersion, transferOutAndCall.Memo)
			if err != nil {
				scp.logger.Err(err).Msgf("fail to parse memo: %s", transferOutAndCall.Memo)
				continue
			}
			if !m.IsType(memo.TxOutbound) {
				scp.logger.Error().Msgf("%s is not an outbound memo", transferOutAndCall.Memo)
				continue
			}
			decimals := scp.decimalResolver(ethToken)
			txInItem.Coins = common.Coins{
				common.NewCoin(common.ETHAsset, scp.amtConverter(ethToken, transferOutAndCall.Amount)).WithDecimals(decimals),
			}
			aggregatorAddr, err := common.NewAddress(transferOutAndCall.Target.String())
			if err != nil {
				scp.logger.Err(err).Str("aggregator_address", transferOutAndCall.Target.String()).Msg("fail to parse aggregator address")
				continue
			}
			aggregatorTargetAddr, err := common.NewAddress(transferOutAndCall.FinalAsset.String())
			if err != nil {
				scp.logger.Err(err).Str("final_asset", transferOutAndCall.FinalAsset.String()).Msg("fail to parse aggregator target address")
				continue
			}

			txInItem.To = transferOutAndCall.To.String()
			txInItem.Memo = transferOutAndCall.Memo
			txInItem.Sender = transferOutAndCall.Vault.String()
			txInItem.Aggregator = aggregatorAddr.String()
			txInItem.AggregatorTarget = aggregatorTargetAddr.String()
			if transferOutAndCall.AmountOutMin != nil {
				limit := cosmos.NewUintFromBigInt(transferOutAndCall.AmountOutMin)
				if !limit.IsZero() {
					txInItem.AggregatorTargetLimit = &limit
				}
			}
		}
		if earlyExit {
			break
		}
	}
	return isVaultTransfer, nil
}
