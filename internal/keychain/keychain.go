package keychain

import (
	"fmt"
	"os"

	gokeychain "github.com/keybase/go-keychain"
)

const (
	serviceName = "slack-cli"
	apiTokenKey = "api_token"
)

// GetAPIToken retrieves the Slack API token from keychain or environment
func GetAPIToken() (string, error) {
	// Try keychain first
	token, err := getFromKeychain(apiTokenKey)
	if err == nil && token != "" {
		return token, nil
	}

	// Fallback to environment variable
	token = os.Getenv("SLACK_API_TOKEN")
	if token != "" {
		return token, nil
	}

	return "", fmt.Errorf("no API token found - run 'slack-cli config set-token' or set SLACK_API_TOKEN")
}

// SetAPIToken stores the Slack API token in keychain
func SetAPIToken(token string) error {
	return setInKeychain(apiTokenKey, token)
}

// DeleteAPIToken removes the Slack API token from keychain
func DeleteAPIToken() error {
	return deleteFromKeychain(apiTokenKey)
}

func getFromKeychain(account string) (string, error) {
	query := gokeychain.NewItem()
	query.SetSecClass(gokeychain.SecClassGenericPassword)
	query.SetService(serviceName)
	query.SetAccount(account)
	query.SetMatchLimit(gokeychain.MatchLimitOne)
	query.SetReturnData(true)

	results, err := gokeychain.QueryItem(query)
	if err != nil {
		return "", err
	}

	if len(results) == 0 {
		return "", fmt.Errorf("no item found")
	}

	return string(results[0].Data), nil
}

func setInKeychain(account, value string) error {
	// First try to delete any existing item
	_ = deleteFromKeychain(account)

	item := gokeychain.NewItem()
	item.SetSecClass(gokeychain.SecClassGenericPassword)
	item.SetService(serviceName)
	item.SetAccount(account)
	item.SetData([]byte(value))
	item.SetSynchronizable(gokeychain.SynchronizableNo)
	item.SetAccessible(gokeychain.AccessibleWhenUnlocked)

	return gokeychain.AddItem(item)
}

func deleteFromKeychain(account string) error {
	item := gokeychain.NewItem()
	item.SetSecClass(gokeychain.SecClassGenericPassword)
	item.SetService(serviceName)
	item.SetAccount(account)

	return gokeychain.DeleteItem(item)
}
