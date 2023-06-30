package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/spf13/cobra"

	"gitlab.com/thorchain/thornode/common"
)

type SignedMsg struct {
	Msg       string `json:"msg"`
	Pubkey    string `json:"pubkey"`
	Signature string `json:"signature"`
}

func GetUtilCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:                        "util",
		Short:                      "Utility commands for the THORChain module",
		DisableFlagParsing:         true,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}

	cmd.AddCommand(GetCmdSignFile())
	cmd.AddCommand(GetCmdSignString())
	return cmd
}

func signAndPrint(blob []byte) error {
	signed, pubkey, err := common.SignBase64(blob)
	if err != nil {
		return err
	}

	result := SignedMsg{
		Msg:       string(blob),
		Pubkey:    pubkey,
		Signature: signed,
	}

	json, err := json.Marshal(result)
	if err != nil {
		return err
	}

	fmt.Println(string(json))

	return nil
}

func GetCmdSignFile() *cobra.Command {
	return &cobra.Command{
		Use:   "sign-file",
		Short: "Sign the contents of a file",
		Long:  `As with any signing capability, exercise caution with what messages you sign.`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			msg, err := os.ReadFile(args[0])
			if err != nil {
				return err
			}

			return signAndPrint(msg)
		},
	}
}

func GetCmdSignString() *cobra.Command {
	return &cobra.Command{
		Use:   "sign-string",
		Short: "Sign the provided string",
		Long:  `As with any signing capability, exercise caution with what messages you sign.`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			msg := []byte(args[0])

			return signAndPrint(msg)
		},
	}
}
