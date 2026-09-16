package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/joho/godotenv"
)

const maxAlpacaCredentialsFileSize = 8 << 10

var alpacaCredentialEnvironmentKeys = []string{"ALPACA_API_KEY_ID", "ALPACA_API_SECRET_KEY"}

type AlpacaCredentials struct {
	APIKeyID     string
	APISecretKey string
}

func captureAlpacaCredentialEnvironment() map[string]environmentValue {
	values := make(map[string]environmentValue, len(alpacaCredentialEnvironmentKeys))
	for _, key := range alpacaCredentialEnvironmentKeys {
		value, set := os.LookupEnv(key)
		values[key] = environmentValue{value: value, set: set}
	}
	return values
}

func restoreAlpacaCredentialEnvironment(values map[string]environmentValue) {
	for _, key := range alpacaCredentialEnvironmentKeys {
		value := values[key]
		if value.set {
			_ = os.Setenv(key, value.value)
		} else {
			_ = os.Unsetenv(key)
		}
	}
}

func DefaultAlpacaCredentialsFile() string {
	if base, err := os.UserConfigDir(); err == nil && strings.TrimSpace(base) != "" {
		return filepath.Join(base, "TradingAgents", "alpaca.env")
	}
	if home, err := os.UserHomeDir(); err == nil && strings.TrimSpace(home) != "" {
		return filepath.Join(home, ".tradingagents", "alpaca.env")
	}
	return ""
}

// LoadAlpacaCredentials reads a protected file outside Git. Process
// environment values override the file so deployments need no local file.
func LoadAlpacaCredentials() (AlpacaCredentials, error) {
	environment := AlpacaCredentials{
		APIKeyID: strings.TrimSpace(os.Getenv("ALPACA_API_KEY_ID")), APISecretKey: strings.TrimSpace(os.Getenv("ALPACA_API_SECRET_KEY")),
	}
	if (environment.APIKeyID == "") != (environment.APISecretKey == "") {
		return AlpacaCredentials{}, errors.New("Alpaca process credentials require both key ID and secret key")
	}
	if environment.APIKeyID != "" && environment.APISecretKey != "" {
		return environment, nil
	}
	credentials := AlpacaCredentials{}
	path := strings.TrimSpace(os.Getenv("STOCKGOD_ALPACA_CREDENTIALS_FILE"))
	if path == "" {
		path = DefaultAlpacaCredentialsFile()
	}
	if path != "" {
		values, err := readAlpacaCredentialsFile(path)
		if err != nil {
			return AlpacaCredentials{}, err
		}
		credentials.APIKeyID = strings.TrimSpace(values["ALPACA_API_KEY_ID"])
		credentials.APISecretKey = strings.TrimSpace(values["ALPACA_API_SECRET_KEY"])
		if (credentials.APIKeyID == "") != (credentials.APISecretKey == "") {
			return AlpacaCredentials{}, errors.New("Alpaca credentials file requires both key ID and secret key")
		}
	}
	return credentials, nil
}

func readAlpacaCredentialsFile(path string) (map[string]string, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, errors.New("Alpaca credentials path is invalid")
	}
	info, err := os.Lstat(absPath)
	if errors.Is(err, os.ErrNotExist) {
		return map[string]string{}, nil
	}
	if err != nil {
		return nil, errors.New("Alpaca credentials file is unreadable")
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return nil, errors.New("Alpaca credentials path must be a regular non-symlink file")
	}
	if info.Mode().Perm() != 0o600 {
		return nil, fmt.Errorf("Alpaca credentials file permissions must be 0600, got %04o", info.Mode().Perm())
	}
	if info.Size() > maxAlpacaCredentialsFileSize {
		return nil, errors.New("Alpaca credentials file exceeds the size limit")
	}
	inside, err := pathInsideGitRepository(absPath)
	if err != nil {
		return nil, errors.New("Alpaca credentials path cannot be verified")
	}
	if inside {
		return nil, errors.New("Alpaca credentials file must be outside the Git repository")
	}
	values, err := godotenv.Read(absPath)
	if err != nil {
		return nil, errors.New("Alpaca credentials file contains invalid env syntax")
	}
	return values, nil
}
