package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/piekstra/slack-cli/internal/keychain"
	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Configure slack-cli",
}

var setTokenCmd = &cobra.Command{
	Use:   "set-token [token]",
	Short: "Set the Slack API token",
	Long: `Set the Slack API token to be stored securely in the macOS Keychain.

If no token is provided as an argument, you will be prompted to enter it.
The token is stored securely and is not visible in process listings or shell history.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var token string

		if len(args) > 0 {
			token = args[0]
		} else {
			fmt.Print("Enter Slack API token: ")
			reader := bufio.NewReader(os.Stdin)
			input, err := reader.ReadString('\n')
			if err != nil {
				return fmt.Errorf("failed to read input: %w", err)
			}
			token = strings.TrimSpace(input)
		}

		if token == "" {
			return fmt.Errorf("token cannot be empty")
		}

		if err := keychain.SetAPIToken(token); err != nil {
			return fmt.Errorf("failed to store token: %w", err)
		}

		fmt.Println("API token stored securely in Keychain")
		return nil
	},
}

var deleteTokenCmd = &cobra.Command{
	Use:   "delete-token",
	Short: "Delete the stored Slack API token",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := keychain.DeleteAPIToken(); err != nil {
			return fmt.Errorf("failed to delete token: %w", err)
		}

		fmt.Println("API token deleted from Keychain")
		return nil
	},
}

var showConfigCmd = &cobra.Command{
	Use:   "show",
	Short: "Show current configuration status",
	RunE: func(cmd *cobra.Command, args []string) error {
		token, err := keychain.GetAPIToken()
		if err != nil {
			fmt.Println("API Token: Not configured")
			fmt.Println("\nRun 'slack-cli config set-token' to configure")
			return nil
		}

		// Mask the token for display
		masked := token[:8] + strings.Repeat("*", len(token)-12) + token[len(token)-4:]
		fmt.Printf("API Token: %s (from Keychain)\n", masked)

		return nil
	},
}

func init() {
	rootCmd.AddCommand(configCmd)
	configCmd.AddCommand(setTokenCmd)
	configCmd.AddCommand(deleteTokenCmd)
	configCmd.AddCommand(showConfigCmd)
}
