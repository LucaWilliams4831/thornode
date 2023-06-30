package cmd

import (
	"bufio"
	"fmt"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/input"
	"github.com/cosmos/cosmos-sdk/client/keys"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/spf13/cobra"
	"gitlab.com/thorchain/thornode/common/cosmos"
)

// GetPubKeyCmd cosmos sdk removed pubkey support recently , as a result of that , all pubkey will be print out in protobuf json format
// like{"@type":"/cosmos.crypto.secp256k1.PubKey","key":"AivZERqhB2H3l4JC7RYG3TeaaUwKf4N/mdxDqDXyZRpF"}
// THORChain need the pubkey in bech32 encoded format, this command is to convert the protobuf json back to bech32 encoded format
func GetPubKeyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pubkey",
		Short: "Convert Proto3 JSON encoded pubkey to bech32 format",
		Long:  ``,
		Args:  cobra.RangeArgs(0, 1),
		RunE:  convertPubKey,
	}
	f := cmd.Flags()
	f.String(keys.FlagBechPrefix, sdk.PrefixAccount, "The Bech32 prefix encoding for a key (acc|val|cons)")
	return cmd
}

// getPubKeyFromString decodes SDK PubKey using JSON marshaler.
func getPubKeyFromString(ctx client.Context, pkstr string) (cryptotypes.PubKey, error) {
	var pk cryptotypes.PubKey
	err := ctx.Codec.UnmarshalInterfaceJSON([]byte(pkstr), &pk)
	return pk, err
}

func convertPubKey(cmd *cobra.Command, args []string) error {
	clientCtx := client.GetClientContextFromCmd(cmd)
	var pkStr string
	if len(args) == 1 {
		pkStr = args[0]
	} else {
		// read it from input
		buf := bufio.NewReader(cmd.InOrStdin())
		readPubKey, err := input.GetString("Proto3 JSON encoded pubkey", buf)
		if err != nil {
			return fmt.Errorf("fail to get Proto3 JSON encoded pubkey,err: %w", err)
		}
		pkStr = readPubKey
	}

	pkey, err := getPubKeyFromString(clientCtx, pkStr)
	if err != nil {
		return err
	}
	prefix, _ := cmd.Flags().GetString(keys.FlagBechPrefix)
	switch prefix {
	case sdk.PrefixAccount:
		pubKey, err := cosmos.Bech32ifyPubKey(cosmos.Bech32PubKeyTypeAccPub, pkey)
		if err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), pubKey)
	case sdk.PrefixConsensus:
		pubKey, err := cosmos.Bech32ifyPubKey(cosmos.Bech32PubKeyTypeConsPub, pkey)
		if err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), pubKey)
	case sdk.PrefixValidator:
	}
	return nil
}
