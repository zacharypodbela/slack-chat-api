package cmd

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"time"

	"github.com/piekstra/slack-cli/internal/keychain"
	"github.com/spf13/cobra"
)

const (
	defaultPort    = 8085
	callbackPath   = "/callback"
	slackAuthURL   = "https://slack.com/oauth/v2/authorize"
	slackTokenURL  = "https://slack.com/api/oauth.v2.access"
)

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Authenticate with Slack",
	Long: `Authenticate with Slack using OAuth or manual token entry.

OAuth login (recommended):
  slack-cli auth login

Manual token:
  slack-cli config set-token`,
}

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Login via OAuth (opens browser)",
	Long: `Authenticate with Slack using OAuth.

This will:
1. Open your browser to Slack's authorization page
2. After you authorize, Slack redirects back to this CLI
3. The token is automatically saved to your Keychain

Prerequisites:
- A Slack app with OAuth configured
- Redirect URL set to: http://localhost:8085/callback
- Client ID and Client Secret from your app settings

On first run, you'll be prompted to enter your Client ID and Secret.
These are stored securely in your Keychain for future logins.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		clientID, _ := cmd.Flags().GetString("client-id")
		clientSecret, _ := cmd.Flags().GetString("client-secret")
		port, _ := cmd.Flags().GetInt("port")

		// Try to get stored credentials if not provided
		if clientID == "" {
			clientID, _ = keychain.GetClientID()
		}
		if clientSecret == "" {
			clientSecret, _ = keychain.GetClientSecret()
		}

		// Prompt for missing credentials
		if clientID == "" {
			fmt.Print("Enter Slack Client ID: ")
			fmt.Scanln(&clientID)
			if clientID == "" {
				return fmt.Errorf("client ID is required")
			}
			if err := keychain.SetClientID(clientID); err != nil {
				fmt.Printf("Warning: couldn't save Client ID to keychain: %v\n", err)
			}
		}

		if clientSecret == "" {
			fmt.Print("Enter Slack Client Secret: ")
			fmt.Scanln(&clientSecret)
			if clientSecret == "" {
				return fmt.Errorf("client secret is required")
			}
			if err := keychain.SetClientSecret(clientSecret); err != nil {
				fmt.Printf("Warning: couldn't save Client Secret to keychain: %v\n", err)
			}
		}

		return doOAuthLogin(clientID, clientSecret, port)
	},
}

var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Remove stored credentials",
	RunE: func(cmd *cobra.Command, args []string) error {
		all, _ := cmd.Flags().GetBool("all")

		if err := keychain.DeleteAPIToken(); err != nil {
			fmt.Println("No API token was stored")
		} else {
			fmt.Println("API token removed")
		}

		if all {
			keychain.DeleteClientID()
			keychain.DeleteClientSecret()
			fmt.Println("OAuth credentials removed")
		}

		return nil
	},
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show authentication status",
	RunE: func(cmd *cobra.Command, args []string) error {
		token, err := keychain.GetAPIToken()
		if err != nil || token == "" {
			fmt.Println("Not authenticated")
			fmt.Println("\nTo authenticate:")
			fmt.Println("  slack-cli auth login      # OAuth (recommended)")
			fmt.Println("  slack-cli config set-token  # Manual token")
			return nil
		}

		// Mask token for display
		masked := token[:8] + "..." + token[len(token)-4:]
		fmt.Printf("Authenticated: %s\n", masked)

		// Try to get workspace info
		c, err := clientFromToken(token)
		if err == nil {
			if team, err := c.GetTeamInfo(); err == nil {
				fmt.Printf("Workspace: %s (%s.slack.com)\n", team.Name, team.Domain)
			}
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(authCmd)

	authCmd.AddCommand(loginCmd)
	loginCmd.Flags().String("client-id", "", "Slack app Client ID")
	loginCmd.Flags().String("client-secret", "", "Slack app Client Secret")
	loginCmd.Flags().Int("port", defaultPort, "Local port for OAuth callback")

	authCmd.AddCommand(logoutCmd)
	logoutCmd.Flags().Bool("all", false, "Also remove stored OAuth credentials")

	authCmd.AddCommand(statusCmd)
}

func doOAuthLogin(clientID, clientSecret string, port int) error {
	// Generate state for CSRF protection
	state, err := generateState()
	if err != nil {
		return fmt.Errorf("failed to generate state: %w", err)
	}

	redirectURI := fmt.Sprintf("http://localhost:%d%s", port, callbackPath)

	// Build authorization URL
	authURL, err := url.Parse(slackAuthURL)
	if err != nil {
		return err
	}

	params := url.Values{}
	params.Set("client_id", clientID)
	params.Set("scope", "channels:read,channels:write,chat:write,users:read,reactions:write,search:read,team:read,groups:read,im:read,mpim:read")
	params.Set("redirect_uri", redirectURI)
	params.Set("state", state)
	authURL.RawQuery = params.Encode()

	// Channel to receive the auth code
	codeChan := make(chan string, 1)
	errChan := make(chan error, 1)

	// Start local server
	server := &http.Server{Addr: fmt.Sprintf(":%d", port)}

	http.HandleFunc(callbackPath, func(w http.ResponseWriter, r *http.Request) {
		// Verify state
		if r.URL.Query().Get("state") != state {
			errChan <- fmt.Errorf("state mismatch - possible CSRF attack")
			http.Error(w, "State mismatch", http.StatusBadRequest)
			return
		}

		// Check for error
		if errParam := r.URL.Query().Get("error"); errParam != "" {
			errChan <- fmt.Errorf("OAuth error: %s", errParam)
			http.Error(w, "Authorization failed", http.StatusBadRequest)
			return
		}

		code := r.URL.Query().Get("code")
		if code == "" {
			errChan <- fmt.Errorf("no code in callback")
			http.Error(w, "No code received", http.StatusBadRequest)
			return
		}

		// Send success page
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<!DOCTYPE html>
<html>
<head><title>Success</title></head>
<body style="font-family: -apple-system, sans-serif; display: flex; justify-content: center; align-items: center; height: 100vh; margin: 0;">
<div style="text-align: center;">
<h1>Authentication Successful</h1>
<p>You can close this window and return to the terminal.</p>
</div>
</body>
</html>`)

		codeChan <- code
	})

	// Start server in background
	go func() {
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			errChan <- err
		}
	}()

	// Open browser
	fmt.Printf("Opening browser to authorize...\n")
	fmt.Printf("If browser doesn't open, visit:\n%s\n\n", authURL.String())

	if err := openBrowser(authURL.String()); err != nil {
		fmt.Printf("Couldn't open browser: %v\n", err)
	}

	fmt.Println("Waiting for authorization...")

	// Wait for callback or timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	var code string
	select {
	case code = <-codeChan:
		// Success
	case err := <-errChan:
		server.Shutdown(ctx)
		return err
	case <-ctx.Done():
		server.Shutdown(ctx)
		return fmt.Errorf("authorization timed out")
	}

	// Shutdown server
	server.Shutdown(ctx)

	// Exchange code for token
	fmt.Println("Exchanging code for token...")

	token, err := exchangeCodeForToken(clientID, clientSecret, code, redirectURI)
	if err != nil {
		return fmt.Errorf("failed to exchange code: %w", err)
	}

	// Store token
	if err := keychain.SetAPIToken(token); err != nil {
		return fmt.Errorf("failed to store token: %w", err)
	}

	fmt.Println("Successfully authenticated!")

	return nil
}

func exchangeCodeForToken(clientID, clientSecret, code, redirectURI string) (string, error) {
	data := url.Values{}
	data.Set("client_id", clientID)
	data.Set("client_secret", clientSecret)
	data.Set("code", code)
	data.Set("redirect_uri", redirectURI)

	resp, err := http.PostForm(slackTokenURL, data)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result struct {
		OK          bool   `json:"ok"`
		Error       string `json:"error"`
		AccessToken string `json:"access_token"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	if !result.OK {
		return "", fmt.Errorf("token exchange failed: %s", result.Error)
	}

	return result.AccessToken, nil
}

func generateState() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func openBrowser(url string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	default:
		return fmt.Errorf("unsupported platform")
	}

	return cmd.Start()
}

// clientFromToken creates a client with a specific token (for status check)
func clientFromToken(token string) (*statusClient, error) {
	return &statusClient{token: token}, nil
}

type statusClient struct {
	token string
}

func (c *statusClient) GetTeamInfo() (*teamInfo, error) {
	req, _ := http.NewRequest("GET", "https://slack.com/api/team.info", nil)
	req.Header.Set("Authorization", "Bearer "+c.token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		OK   bool `json:"ok"`
		Team struct {
			Name   string `json:"name"`
			Domain string `json:"domain"`
		} `json:"team"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &teamInfo{Name: result.Team.Name, Domain: result.Team.Domain}, nil
}

type teamInfo struct {
	Name   string
	Domain string
}
