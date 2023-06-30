package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/client/tx"
	"github.com/spf13/cobra"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"

	"gitlab.com/thorchain/thornode/x/thorchain/types"
)

func GetTxCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:                        types.ModuleName,
		Short:                      "THORChain transaction subcommands",
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}

	cmd.AddCommand(GetCmdSetNodeKeys())
	cmd.AddCommand(GetCmdSetVersion())
	cmd.AddCommand(GetCmdSetIPAddress())
	cmd.AddCommand(GetCmdBan())
	cmd.AddCommand(GetCmdMimir())
	cmd.AddCommand(GetCmdNodePauseChain())
	cmd.AddCommand(GetCmdNodeResumeChain())
	cmd.AddCommand(GetCmdDeposit())
	cmd.AddCommand(GetCmdSend())
	for _, subCmd := range cmd.Commands() {
		flags.AddTxFlagsToCmd(subCmd)
	}
	return cmd
}

// GetCmdDeposit command to send a native transaction
func GetCmdDeposit() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "deposit [amount] [coin] [memo]",
		Short: "sends a deposit transaction",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}
			amt, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid amount (must be an integer): %w", err)
			}

			asset, err := common.NewAsset(args[1])
			if err != nil {
				return fmt.Errorf("invalid asset: %w", err)
			}

			coin := common.NewCoin(asset, cosmos.NewUint(uint64(amt)))

			msg := types.NewMsgDeposit(common.Coins{coin}, args[2], clientCtx.GetFromAddress())
			if err := msg.ValidateBasic(); err != nil {
				return err
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}
	return cmd
}

// GetCmdSend command to send funds
func GetCmdSend() *cobra.Command {
	return &cobra.Command{
		Use:   "send [to_address] [coins]",
		Short: "sends funds",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}
			toAddr, err := cosmos.AccAddressFromBech32(args[0])
			if err != nil {
				return fmt.Errorf("invalid address: %w", err)
			}

			coins, err := cosmos.ParseCoins(args[1])
			if err != nil {
				return fmt.Errorf("invalid coins: %w", err)
			}

			msg := types.NewMsgSend(clientCtx.GetFromAddress(), toAddr, coins)
			if err := msg.ValidateBasic(); err != nil {
				return err
			}
			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}
}

// GetCmdMimir command to change a mimir attribute
func GetCmdMimir() *cobra.Command {
	return &cobra.Command{
		Use:   "mimir [key] [value]",
		Short: "updates a mimir attribute (admin only)",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			val, err := strconv.ParseInt(args[1], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid value (must be an integer): %w", err)
			}

			msg := types.NewMsgMimir(strings.ToUpper(args[0]), val, clientCtx.GetFromAddress())
			if err := msg.ValidateBasic(); err != nil {
				return err
			}
			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}
}

// GetCmdNodePauseChain command to change node pause chain
func GetCmdNodePauseChain() *cobra.Command {
	return &cobra.Command{
		Use:   "pause-chain",
		Short: "globally pause chain (NOs only)",
		Args:  cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			msg := types.NewMsgNodePauseChain(int64(1), clientCtx.GetFromAddress())
			if err := msg.ValidateBasic(); err != nil {
				return err
			}
			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}
}

// GetCmdNodeResumeChain command to change node resume chain
func GetCmdNodeResumeChain() *cobra.Command {
	return &cobra.Command{
		Use:   "resume-chain",
		Short: "globally resume chain (NOs only)",
		Args:  cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			msg := types.NewMsgNodePauseChain(int64(-1), clientCtx.GetFromAddress())
			if err := msg.ValidateBasic(); err != nil {
				return err
			}
			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}
}

// GetCmdBan command to ban a node accounts
func GetCmdBan() *cobra.Command {
	return &cobra.Command{
		Use:   "ban [node address]",
		Short: "votes to ban a node address (caution: costs 0.1% of minimum bond)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			addr, err := cosmos.AccAddressFromBech32(args[0])
			if err != nil {
				return fmt.Errorf("invalid node address: %w", err)
			}

			msg := types.NewMsgBan(addr, clientCtx.GetFromAddress())
			if err := msg.ValidateBasic(); err != nil {
				return err
			}
			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}
}

// GetCmdSetIPAddress command to set a node accounts IP Address
func GetCmdSetIPAddress() *cobra.Command {
	return &cobra.Command{
		Use:   "set-ip-address [ip address]",
		Short: "update registered ip address",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			msg := types.NewMsgSetIPAddress(args[0], clientCtx.GetFromAddress())
			if err := msg.ValidateBasic(); err != nil {
				return err
			}
			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}
}

// GetCmdSetVersion command to set an admin config
func GetCmdSetVersion() *cobra.Command {
	return &cobra.Command{
		Use:   "set-version",
		Short: "update registered version",
		Args:  cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			msg := types.NewMsgSetVersion(constants.SWVersion.String(), clientCtx.GetFromAddress())
			if err := msg.ValidateBasic(); err != nil {
				return err
			}
			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}
}

// GetCmdSetNodeKeys command to add a node keys
func GetCmdSetNodeKeys() *cobra.Command {
	return &cobra.Command{
		Use:   "set-node-keys  [secp256k1] [ed25519] [validator_consensus_pub_key]",
		Short: "set node keys, the account use to sign this tx has to be whitelist first",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			secp256k1Key, err := common.NewPubKey(args[0])
			if err != nil {
				return fmt.Errorf("fail to parse secp256k1 pub key ,err:%w", err)
			}
			ed25519Key, err := common.NewPubKey(args[1])
			if err != nil {
				return fmt.Errorf("fail to parse ed25519 pub key ,err:%w", err)
			}
			pk := common.NewPubKeySet(secp256k1Key, ed25519Key)
			validatorConsPubKey, err := cosmos.GetPubKeyFromBech32(cosmos.Bech32PubKeyTypeConsPub, args[2])
			if err != nil {
				return fmt.Errorf("fail to parse validator consensus public key: %w", err)
			}
			validatorConsPubKeyStr, err := cosmos.Bech32ifyPubKey(cosmos.Bech32PubKeyTypeConsPub, validatorConsPubKey)
			if err != nil {
				return fmt.Errorf("fail to convert public key to string: %w", err)
			}
			msg := types.NewMsgSetNodeKeys(pk, validatorConsPubKeyStr, clientCtx.GetFromAddress())
			err = msg.ValidateBasic()
			if err != nil {
				return err
			}
			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}
}
