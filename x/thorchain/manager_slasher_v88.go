package thorchain

import (
	"context"
	"fmt"
	"math/big"
	"strconv"
	"strings"

	"github.com/armon/go-metrics"
	"github.com/cosmos/cosmos-sdk/telemetry"
	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/crypto"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
	"gitlab.com/thorchain/thornode/x/thorchain/keeper"
)

// SlasherV88 is v88 implementation of slasher
type SlasherV88 struct {
	keeper   keeper.Keeper
	eventMgr EventManager
}

// newSlasherV88 create a new instance of Slasher
func newSlasherV88(keeper keeper.Keeper, eventMgr EventManager) *SlasherV88 {
	return &SlasherV88{keeper: keeper, eventMgr: eventMgr}
}

// BeginBlock called when a new block get proposed to detect whether there are duplicate vote
func (s *SlasherV88) BeginBlock(ctx cosmos.Context, req abci.RequestBeginBlock, constAccessor constants.ConstantValues) {
	// Iterate through any newly discovered evidence of infraction
	// Slash any validators (and since-unbonded liquidity within the unbonding period)
	// who contributed to valid infractions
	for _, evidence := range req.ByzantineValidators {
		switch evidence.Type {
		case abci.EvidenceType_DUPLICATE_VOTE:
			if err := s.HandleDoubleSign(ctx, evidence.Validator.Address, evidence.Height, constAccessor); err != nil {
				ctx.Logger().Error("fail to slash for double signing a block", "error", err)
			}
		default:
			ctx.Logger().Error("ignored unknown evidence type", "type", evidence.Type)
		}
	}
}

// HandleDoubleSign - slashes a validator for signing two blocks at the same
// block height
// https://blog.cosmos.network/consensus-compare-casper-vs-tendermint-6df154ad56ae
func (s *SlasherV88) HandleDoubleSign(ctx cosmos.Context, addr crypto.Address, infractionHeight int64, constAccessor constants.ConstantValues) error {
	// check if we're recent enough to slash for this behavior
	maxAge := constAccessor.GetInt64Value(constants.DoubleSignMaxAge)
	if (ctx.BlockHeight() - infractionHeight) > maxAge {
		ctx.Logger().Info("double sign detected but too old to be slashed", "infraction height", fmt.Sprintf("%d", infractionHeight), "address", addr.String())
		return nil
	}
	nas, err := s.keeper.ListActiveValidators(ctx)
	if err != nil {
		return err
	}

	for _, na := range nas {
		pk, err := cosmos.GetPubKeyFromBech32(cosmos.Bech32PubKeyTypeConsPub, na.ValidatorConsPubKey)
		if err != nil {
			return err
		}

		if addr.String() == pk.Address().String() {
			if na.Bond.IsZero() {
				return fmt.Errorf("found account to slash for double signing, but did not have any bond to slash: %s", addr)
			}
			// take 5% of the minimum bond, and put it into the reserve
			minBond, err := s.keeper.GetMimir(ctx, constants.MinimumBondInRune.String())
			if minBond < 0 || err != nil {
				minBond = constAccessor.GetInt64Value(constants.MinimumBondInRune)
			}
			slashAmount := cosmos.NewUint(uint64(minBond)).MulUint64(5).QuoUint64(100)
			if slashAmount.GT(na.Bond) {
				slashAmount = na.Bond
			}

			slashFloat, _ := new(big.Float).SetInt(slashAmount.BigInt()).Float32()
			telemetry.IncrCounterWithLabels(
				[]string{"thornode", "bond_slash"},
				slashFloat,
				[]metrics.Label{
					telemetry.NewLabel("address", addr.String()),
					telemetry.NewLabel("reason", "double_sign"),
				},
			)

			na.Bond = common.SafeSub(na.Bond, slashAmount)
			coin := common.NewCoin(common.RuneNative, slashAmount)
			if err := s.keeper.SendFromModuleToModule(ctx, BondName, ReserveName, common.NewCoins(coin)); err != nil {
				ctx.Logger().Error("fail to transfer funds from bond to reserve", "error", err)
				return fmt.Errorf("fail to transfer funds from bond to reserve: %w", err)
			}

			return s.keeper.SetNodeAccount(ctx, na)
		}
	}

	return fmt.Errorf("could not find node account with validator address: %s", addr)
}

// LackObserving Slash node accounts that didn't observe a single inbound txn
func (s *SlasherV88) LackObserving(ctx cosmos.Context, constAccessor constants.ConstantValues) error {
	signingTransPeriod := constAccessor.GetInt64Value(constants.SigningTransactionPeriod)
	height := ctx.BlockHeight()
	if height < signingTransPeriod {
		return nil
	}
	heightToCheck := height - signingTransPeriod
	tx, err := s.keeper.GetTxOut(ctx, heightToCheck)
	if err != nil {
		return fmt.Errorf("fail to get txout for block height(%d): %w", heightToCheck, err)
	}
	// no txout , return
	if tx == nil || tx.IsEmpty() {
		return nil
	}
	for _, item := range tx.TxArray {
		if item.InHash.IsEmpty() {
			continue
		}
		if item.InHash.Equals(common.BlankTxID) {
			continue
		}
		if err := s.slashNotObserving(ctx, item.InHash, constAccessor); err != nil {
			ctx.Logger().Error("fail to slash not observing", "error", err)
		}
	}

	return nil
}

func (s *SlasherV88) slashNotObserving(ctx cosmos.Context, txHash common.TxID, constAccessor constants.ConstantValues) error {
	voter, err := s.keeper.GetObservedTxInVoter(ctx, txHash)
	if err != nil {
		return fmt.Errorf("fail to get observe txin voter (%s): %w", txHash.String(), err)
	}

	if len(voter.Txs) == 0 {
		return nil
	}

	nodes, err := s.keeper.ListActiveValidators(ctx)
	if err != nil {
		return fmt.Errorf("unable to get list of active accounts: %w", err)
	}
	if len(voter.Txs) > 0 {
		tx := voter.Tx
		if !tx.IsEmpty() && len(tx.Signers) > 0 {
			height := voter.Height
			if tx.IsFinal() {
				height = voter.FinalisedHeight
			}
			// as long as the node has voted one of the tx , regardless finalised or not , it should not be slashed
			var allSigners []cosmos.AccAddress
			for _, item := range voter.Txs {
				allSigners = append(allSigners, item.GetSigners()...)
			}
			s.checkSignerAndSlash(ctx, nodes, height, allSigners, constAccessor)
		}
	}
	return nil
}

func (s *SlasherV88) checkSignerAndSlash(ctx cosmos.Context, nodes NodeAccounts, blockHeight int64, signers []cosmos.AccAddress, constAccessor constants.ConstantValues) {
	for _, na := range nodes {
		// the node is active after the tx finalised
		if na.ActiveBlockHeight > blockHeight {
			continue
		}
		found := false
		for _, addr := range signers {
			if na.NodeAddress.Equals(addr) {
				found = true
				break
			}
		}
		// this na is not found, therefore it should be slashed
		if !found {
			lackOfObservationPenalty := constAccessor.GetInt64Value(constants.LackOfObservationPenalty)
			slashCtx := ctx.WithContext(context.WithValue(ctx.Context(), constants.CtxMetricLabels, []metrics.Label{
				telemetry.NewLabel("reason", "not_observing"),
			}))
			if err := s.keeper.IncNodeAccountSlashPoints(slashCtx, na.NodeAddress, lackOfObservationPenalty); err != nil {
				ctx.Logger().Error("fail to inc slash points", "error", err)
			}
		}
	}
}

// LackSigning slash account that fail to sign tx
func (s *SlasherV88) LackSigning(ctx cosmos.Context, mgr Manager) error {
	var resultErr error
	signingTransPeriod := mgr.GetConstants().GetInt64Value(constants.SigningTransactionPeriod)
	if ctx.BlockHeight() < signingTransPeriod {
		return nil
	}
	height := ctx.BlockHeight() - signingTransPeriod
	txs, err := s.keeper.GetTxOut(ctx, height)
	if err != nil {
		return fmt.Errorf("fail to get txout from block height(%d): %w", height, err)
	}
	for i, tx := range txs.TxArray {
		if !common.CurrentChainNetwork.SoftEquals(tx.ToAddress.GetNetwork(mgr.GetVersion(), tx.Chain)) {
			continue // skip this transaction
		}
		if tx.OutHash.IsEmpty() {
			// Slash node account for not sending funds
			vault, err := s.keeper.GetVault(ctx, tx.VaultPubKey)
			if err != nil {
				// in some edge cases, when a txout item had been schedule to be send out by an yggdrasil vault
				// however the node operator decide to quit by sending a leave command, which will result in the vault get removed
				// if that happen , txout item should be scheduled to send out using asgard, thus when if fail to get vault , just
				// log the error, and continue
				ctx.Logger().Error("Unable to get vault", "error", err, "vault pub key", tx.VaultPubKey.String())
			}
			// slash if its a yggdrasil vault, and the chain isn't halted
			if vault.IsYggdrasil() && !isChainHalted(ctx, mgr, tx.Chain) {
				na, err := s.keeper.GetNodeAccountByPubKey(ctx, tx.VaultPubKey)
				if err != nil {
					ctx.Logger().Error("Unable to get node account", "error", err, "vault pub key", tx.VaultPubKey.String())
					continue
				}
				slashPoints := signingTransPeriod * 2

				slashCtx := ctx.WithContext(context.WithValue(ctx.Context(), constants.CtxMetricLabels, []metrics.Label{
					telemetry.NewLabel("reason", "not_signing"),
				}))
				if err := s.keeper.IncNodeAccountSlashPoints(slashCtx, na.NodeAddress, slashPoints); err != nil {
					ctx.Logger().Error("fail to inc slash points", "error", err, "node addr", na.NodeAddress.String())
				}
				if err := mgr.EventMgr().EmitEvent(ctx, NewEventSlashPoint(na.NodeAddress, slashPoints, fmt.Sprintf("fail to sign out tx after %d blocks", signingTransPeriod))); err != nil {
					ctx.Logger().Error("fail to emit slash point event")
				}
				releaseHeight := ctx.BlockHeight() + (signingTransPeriod * 2)
				reason := "fail to send yggdrasil transaction"
				if err := s.keeper.SetNodeAccountJail(ctx, na.NodeAddress, releaseHeight, reason); err != nil {
					ctx.Logger().Error("fail to set node account jail", "node address", na.NodeAddress, "reason", reason, "error", err)
				}
			}

			memo, _ := ParseMemoWithTHORNames(ctx, s.keeper, tx.Memo) // ignore err
			if memo.IsInternal() {
				// there is a different mechanism for rescheduling outbound
				// transactions for migration transactions
				continue
			}
			var voter ObservedTxVoter
			if !memo.IsType(TxRagnarok) {
				voter, err = s.keeper.GetObservedTxInVoter(ctx, tx.InHash)
				if err != nil {
					ctx.Logger().Error("fail to get observed tx voter", "error", err)
					resultErr = fmt.Errorf("failed to get observed tx voter: %w", err)
					continue
				}
			}

			maxOutboundAttempts := fetchConfigInt64(ctx, mgr, constants.MaxOutboundAttempts)
			if maxOutboundAttempts > 0 {
				age := ctx.BlockHeight() - voter.FinalisedHeight
				attempts := age / signingTransPeriod
				if attempts >= maxOutboundAttempts {
					ctx.Logger().Info("txn dropped, too many attempts", "hash", tx.InHash)
					continue
				}
			}

			active, err := s.keeper.GetAsgardVaultsByStatus(ctx, ActiveVault)
			if err != nil {
				return fmt.Errorf("fail to get active asgard vaults: %w", err)
			}
			available := active.Has(tx.Coin).SortBy(tx.Coin.Asset)
			if len(available) == 0 {
				// we need to give it somewhere to send from, even if that
				// asgard doesn't have enough funds. This is because if we
				// don't the transaction will just be dropped on the floor,
				// which is bad. Instead it may try to send from an asgard that
				// doesn't have enough funds, fail, and then get rescheduled
				// again later. Maybe by then the network will have enough
				// funds to satisfy.
				// TODO add split logic to send it out from multiple asgards in
				// this edge case.
				ctx.Logger().Error("unable to determine asgard vault to send funds, trying first asgard")
				if len(active) > 0 {
					vault = active[0]
				}
			} else {
				// each time we reschedule a transaction, we take the age of
				// the transaction, and move it to an vault that has less funds
				// than last time. This is here to ensure that if an asgard
				// vault becomes unavailable, the network will reschedule the
				// transaction on a different asgard vault.
				age := ctx.BlockHeight() - voter.FinalisedHeight
				if vault.IsYggdrasil() {
					// since the last attempt was a yggdrasil vault, lets
					// artificially inflate the age to ensure that the first
					// attempt is the largest asgard vault with funds
					age -= signingTransPeriod
					if age < 0 {
						age = 0
					}
				}
				rep := int(age / signingTransPeriod)
				if vault.PubKey.Equals(available[rep%len(available)].PubKey) {
					// looks like the new vault is going to be the same as the
					// old vault, increment rep to ensure a differ asgard is
					// chosen (if there is more than one option)
					rep++
				}
				vault = available[rep%len(available)]
			}
			if !memo.IsType(TxRagnarok) {
				// update original tx action in observed tx
				// check observedTx has done status. Skip if it does already.
				voterTx := voter.GetTx(NodeAccounts{})
				if voterTx.IsDone(len(voter.Actions)) {
					if len(voterTx.OutHashes) > 0 && len(voterTx.GetOutHashes()) > 0 {
						txs.TxArray[i].OutHash = voterTx.GetOutHashes()[0]
					}
					continue
				}

				// update the actions in the voter with the new vault pubkey
				for i, action := range voter.Actions {
					if action.Equals(tx) {
						voter.Actions[i].VaultPubKey = vault.PubKey
					}
				}
				s.keeper.SetObservedTxInVoter(ctx, voter)

			}
			// Save the tx to as a new tx, select Asgard to send it this time.
			tx.VaultPubKey = vault.PubKey

			// update max gas
			maxGas, err := mgr.GasMgr().GetMaxGas(ctx, tx.Chain)
			if err != nil {
				ctx.Logger().Error("fail to get max gas", "error", err)
			} else {
				tx.MaxGas = common.Gas{maxGas}
				// Update MaxGas in ObservedTxVoter action as well
				if err := updateTxOutGas(ctx, s.keeper, tx, common.Gas{maxGas}); err != nil {
					ctx.Logger().Error("Failed to update MaxGas of action in ObservedTxVoter", "hash", tx.InHash, "error", err)
				}
			}
			tx.GasRate = int64(mgr.GasMgr().GetGasRate(ctx, tx.Chain).Uint64())

			// if a pool with the asset name doesn't exist, skip rescheduling
			if !tx.Coin.Asset.IsRune() && !s.keeper.PoolExist(ctx, tx.Coin.Asset) {
				ctx.Logger().Error("fail to add outbound tx", "error", "coin is not rune and does not have an associated pool")
				continue
			}

			err = mgr.TxOutStore().UnSafeAddTxOutItem(ctx, mgr, tx)
			if err != nil {
				ctx.Logger().Error("fail to add outbound tx", "error", err)
				resultErr = fmt.Errorf("failed to add outbound tx: %w", err)
				continue
			}
			// because the txout item has been rescheduled, thus mark the replaced tx out item as already send out, even it is not
			// in this way bifrost will not send it out again cause node to be slashed
			txs.TxArray[i].OutHash = common.BlankTxID
		}
	}
	if !txs.IsEmpty() {
		if err := s.keeper.SetTxOut(ctx, txs); err != nil {
			return fmt.Errorf("fail to save tx out : %w", err)
		}
	}

	return resultErr
}

// SlashVault thorchain keep monitoring the outbound tx from asgard pool
// and yggdrasil pool, usually the txout is triggered by thorchain itself by
// adding an item into the txout array, refer to TxOutItem for the detail, the
// TxOutItem contains a specific coin and amount.  if somehow thorchain
// discover signer send out fund more than the amount specified in TxOutItem,
// it will slash the node account who does that by taking 1.5 * extra fund from
// node account's bond and subsidise the pool that actually lost it.
func (s *SlasherV88) SlashVault(ctx cosmos.Context, vaultPK common.PubKey, coins common.Coins, mgr Manager) error {
	if coins.IsEmpty() {
		return nil
	}

	vault, err := s.keeper.GetVault(ctx, vaultPK)
	if err != nil {
		return fmt.Errorf("fail to get slash vault (pubkey %s), %w", vaultPK, err)
	}
	membership := vault.GetMembership()

	// sum the total bond of membership of the vault
	totalBond := cosmos.ZeroUint()
	for _, member := range membership {
		na, err := s.keeper.GetNodeAccountByPubKey(ctx, member)
		if err != nil {
			ctx.Logger().Error("fail to get node account bond", "pk", member, "error", err)
			continue
		}
		totalBond = totalBond.Add(na.Bond)
	}

	metricLabels, _ := ctx.Context().Value(constants.CtxMetricLabels).([]metrics.Label)

	for _, coin := range coins {
		if coin.IsEmpty() {
			continue
		}

		// rune value is the value in RUNE of the missing funds
		var runeValue cosmos.Uint
		if coin.Asset.IsRune() {
			runeValue = coin.Amount
		} else {
			runeValue = s.adjustPoolForSlashedAsset(ctx, coin, mgr)
		}
		if runeValue.IsZero() {
			continue
		}

		// total slash amount is 1.5x the RUNE value of the missing funds
		totalSlashAmountInRune := runeValue.MulUint64(3).QuoUint64(2)

		pauseOnSlashThreshold := fetchConfigInt64(ctx, mgr, constants.PauseOnSlashThreshold)
		if pauseOnSlashThreshold > 0 && totalSlashAmountInRune.GTE(cosmos.NewUint(uint64(pauseOnSlashThreshold))) {
			// set mimirs to pause the chain and ygg funding
			s.keeper.SetMimir(ctx, mimirStopFundYggdrasil, ctx.BlockHeight())
			mimirEvent := NewEventSetMimir(strings.ToUpper(mimirStopFundYggdrasil), strconv.FormatInt(ctx.BlockHeight(), 10))
			if err := mgr.EventMgr().EmitEvent(ctx, mimirEvent); err != nil {
				ctx.Logger().Error("fail to emit set_mimir event", "error", err)
			}

			key := fmt.Sprintf("Halt%sChain", coin.Asset.Chain)
			s.keeper.SetMimir(ctx, key, ctx.BlockHeight())
			mimirEvent = NewEventSetMimir(strings.ToUpper(key), strconv.FormatInt(ctx.BlockHeight(), 10))
			if err := mgr.EventMgr().EmitEvent(ctx, mimirEvent); err != nil {
				ctx.Logger().Error("fail to emit set_mimir event", "error", err)
			}
		}

		for _, member := range membership {
			na, err := s.keeper.GetNodeAccountByPubKey(ctx, member)
			if err != nil {
				ctx.Logger().Error("fail to get node account for slash", "pk", member, "error", err)
				continue
			}
			if na.Bond.IsZero() {
				ctx.Logger().Info("validator's bond is zero, can't be slashed", "node address", na.NodeAddress.String())
				continue
			}
			slashAmountRune := common.GetSafeShare(na.Bond, totalBond, totalSlashAmountInRune)
			if slashAmountRune.GT(na.Bond) {
				ctx.Logger().Info("slash amount is larger than bond", "slash amount", slashAmountRune, "bond", na.Bond)
				slashAmountRune = na.Bond
			}
			ctx.Logger().Info("slash node account", "node address", na.NodeAddress.String(), "amount", slashAmountRune.String(), "total slash amount", totalSlashAmountInRune)
			na.Bond = common.SafeSub(na.Bond, slashAmountRune)

			tx := common.Tx{}
			tx.ID = common.BlankTxID
			tx.FromAddress = na.BondAddress
			bondEvent := NewEventBond(slashAmountRune, BondCost, tx)
			if err := s.eventMgr.EmitEvent(ctx, bondEvent); err != nil {
				return fmt.Errorf("fail to emit bond event: %w", err)
			}

			slashAmountRuneFloat, _ := new(big.Float).SetInt(slashAmountRune.BigInt()).Float32()
			telemetry.IncrCounterWithLabels(
				[]string{"thornode", "bond_slash"},
				slashAmountRuneFloat,
				append(
					metricLabels,
					telemetry.NewLabel("address", na.NodeAddress.String()),
					telemetry.NewLabel("coin_symbol", coin.Asset.Symbol.String()),
					telemetry.NewLabel("coin_chain", string(coin.Asset.Chain)),
					telemetry.NewLabel("vault_type", vault.Type.String()),
					telemetry.NewLabel("vault_status", vault.Status.String()),
				),
			)

			// Ban the node account. Ensure we don't ban more than 1/3rd of any
			// given active or retiring vault
			if vault.IsYggdrasil() {
				// TODO: temporally disabling banning for the theft of funds. This
				// is to give the code time to prove itself reliable before the it
				// starts booting nodes out of the system
				toBan := false // TODO flip this to true
				if na.Bond.IsZero() {
					toBan = true
				}
				for _, vaultPk := range na.GetSignerMembership() {
					vault, err := s.keeper.GetVault(ctx, vaultPk)
					if err != nil {
						ctx.Logger().Error("fail to get vault", "error", err)
						continue
					}
					if !(vault.Status == ActiveVault || vault.Status == RetiringVault) {
						continue
					}
					activeMembers := 0
					for _, pk := range vault.GetMembership() {
						member, _ := s.keeper.GetNodeAccountByPubKey(ctx, pk)
						if member.Status == NodeActive {
							activeMembers++
						}
					}
					if !HasSuperMajority(activeMembers, len(vault.GetMembership())) {
						toBan = false
						break
					}
				}
				if toBan {
					na.ForcedToLeave = true
					na.LeaveScore = 1 // Set Leave Score to 1, which means the nodes is bad
				}
			}

			if err := s.keeper.SetNodeAccount(ctx, na); err != nil {
				ctx.Logger().Error("fail to save node account for slash", "error", err)
			}
		}

		//  2/3 of the total slashed RUNE value to asgard
		//  1/3 of the total slashed RUNE value to reserve
		runeToAsgard := runeValue
		runeToReserve := common.SafeSub(totalSlashAmountInRune, runeToAsgard)

		if !runeToReserve.IsZero() {
			if err := s.keeper.SendFromModuleToModule(ctx, BondName, ReserveName, common.NewCoins(common.NewCoin(common.RuneAsset(), runeToReserve))); err != nil {
				ctx.Logger().Error("fail to send slash funds to reserve module", "pk", vaultPK, "error", err)
			}
		}
		if !runeToAsgard.IsZero() {
			if err := s.keeper.SendFromModuleToModule(ctx, BondName, AsgardName, common.NewCoins(common.NewCoin(common.RuneAsset(), runeToAsgard))); err != nil {
				ctx.Logger().Error("fail to send slash fund to asgard module", "pk", vaultPK, "error", err)
			}
		}

	}

	return nil
}

// IncSlashPoints will increase the given account's slash points
func (s *SlasherV88) IncSlashPoints(ctx cosmos.Context, point int64, addresses ...cosmos.AccAddress) {
	for _, addr := range addresses {
		if err := s.keeper.IncNodeAccountSlashPoints(ctx, addr, point); err != nil {
			ctx.Logger().Error("fail to increase node account slash point", "error", err, "address", addr.String())
		}
	}
}

// DecSlashPoints will decrease the given account's slash points
func (s *SlasherV88) DecSlashPoints(ctx cosmos.Context, point int64, addresses ...cosmos.AccAddress) {
	for _, addr := range addresses {
		if err := s.keeper.DecNodeAccountSlashPoints(ctx, addr, point); err != nil {
			ctx.Logger().Error("fail to decrease node account slash point", "error", err, "address", addr.String())
		}
	}
}

// adjustPoolForSlashedAsset - deduct the asset coin amount from the pool and
// reimburse with the RUNE value of the deducted amount. Returns the RUNE value
// but does not transfer RUNE to the Asgard module.
func (s *SlasherV88) adjustPoolForSlashedAsset(ctx cosmos.Context, coin common.Coin, mgr Manager) cosmos.Uint {
	pool, err := s.keeper.GetPool(ctx, coin.Asset)
	if err != nil {
		ctx.Logger().Error("fail to get pool for slash", "asset", coin.Asset, "error", err)
		return cosmos.ZeroUint()
	}
	// THORChain doesn't even have a pool for the asset
	if pool.IsEmpty() {
		ctx.Logger().Error("cannot slash for an empty pool", "asset", coin.Asset)
		return cosmos.ZeroUint()
	}
	coinAmount := coin.Amount
	if coinAmount.GT(pool.BalanceAsset) {
		coinAmount = pool.BalanceAsset
	}
	runeValue := pool.RuneReimbursementForAssetWithdrawal(coinAmount)
	pool.BalanceAsset = common.SafeSub(pool.BalanceAsset, coinAmount)
	pool.BalanceRune = pool.BalanceRune.Add(runeValue)
	if err := s.keeper.SetPool(ctx, pool); err != nil {
		ctx.Logger().Error("fail to save pool for slash", "asset", coin.Asset, "error", err)
		return cosmos.ZeroUint()
	}
	poolSlashAmt := []PoolAmt{
		{
			Asset:  pool.Asset,
			Amount: 0 - int64(coinAmount.Uint64()),
		},
		{
			Asset:  common.RuneAsset(),
			Amount: int64(runeValue.Uint64()),
		},
	}
	eventSlash := NewEventSlash(pool.Asset, poolSlashAmt)
	if err := mgr.EventMgr().EmitEvent(ctx, eventSlash); err != nil {
		ctx.Logger().Error("fail to emit slash event", "error", err)
	}
	return runeValue
}
