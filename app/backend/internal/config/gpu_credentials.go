package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const maxGPUCredentialsFileSize = 16 << 10

var gpuCredentialEnvironmentKeys = []string{"RUNPOD_API_KEY", "MODAL_TOKEN_ID", "MODAL_TOKEN_SECRET", "LAMBDA_API_KEY", "VAST_API_KEY"}

type environmentValue struct {
	value string
	set   bool
}

func captureGPUCredentialEnvironment() map[string]environmentValue {
	values := make(map[string]environmentValue, len(gpuCredentialEnvironmentKeys))
	for _, key := range gpuCredentialEnvironmentKeys {
		value, set := os.LookupEnv(key)
		values[key] = environmentValue{value: value, set: set}
	}
	return values
}

func restoreGPUCredentialEnvironment(values map[string]environmentValue) {
	for _, key := range gpuCredentialEnvironmentKeys {
		value := values[key]
		if value.set {
			_ = os.Setenv(key, value.value)
		} else {
			_ = os.Unsetenv(key)
		}
	}
}

type GPUProviderCredentials struct {
	RunPodAPIKey     string `json:"runpod_api_key"`
	ModalTokenID     string `json:"modal_token_id"`
	ModalTokenSecret string `json:"modal_token_secret"`
	LambdaAPIKey     string `json:"lambda_api_key"`
	VastAPIKey       string `json:"vast_api_key"`
}

func DefaultGPUCredentialsFile() string {
	if base, err := os.UserConfigDir(); err == nil && strings.TrimSpace(base) != "" {
		return filepath.Join(base, "TradingAgents", "gpu-provider-credentials.json")
	}
	if home, err := os.UserHomeDir(); err == nil && strings.TrimSpace(home) != "" {
		return filepath.Join(home, ".tradingagents", "gpu-provider-credentials.json")
	}
	return ""
}

// ReloadGPUProviderCredentials replaces only GPU credentials. Values present in
// the process environment take precedence over the protected local file.
func ReloadGPUProviderCredentials(cfg *Config) error {
	if cfg == nil {
		return errors.New("GPU credential reload requires config")
	}
	credentials, err := readGPUProviderCredentials(cfg.GPUCredentialsFile)
	if err != nil {
		clearGPUProviderCredentials(cfg)
		applyGPUCredentialEnvironment(cfg)
		return err
	}
	cfg.RunPodAPIKey = strings.TrimSpace(credentials.RunPodAPIKey)
	cfg.ModalTokenID = strings.TrimSpace(credentials.ModalTokenID)
	cfg.ModalTokenSecret = strings.TrimSpace(credentials.ModalTokenSecret)
	cfg.LambdaAPIKey = strings.TrimSpace(credentials.LambdaAPIKey)
	cfg.VastAPIKey = strings.TrimSpace(credentials.VastAPIKey)
	applyGPUCredentialEnvironment(cfg)
	return nil
}

func readGPUProviderCredentials(path string) (GPUProviderCredentials, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return GPUProviderCredentials{}, nil
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return GPUProviderCredentials{}, errors.New("GPU credentials path is invalid")
	}
	info, err := os.Lstat(absPath)
	if errors.Is(err, os.ErrNotExist) {
		return GPUProviderCredentials{}, nil
	}
	if err != nil {
		return GPUProviderCredentials{}, errors.New("GPU credentials file is unreadable")
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return GPUProviderCredentials{}, errors.New("GPU credentials path must be a regular non-symlink file")
	}
	if info.Mode().Perm() != 0o600 {
		return GPUProviderCredentials{}, fmt.Errorf("GPU credentials file permissions must be 0600, got %04o", info.Mode().Perm())
	}
	if info.Size() > maxGPUCredentialsFileSize {
		return GPUProviderCredentials{}, errors.New("GPU credentials file exceeds the size limit")
	}
	inside, err := pathInsideGitRepository(absPath)
	if err != nil {
		return GPUProviderCredentials{}, err
	}
	if inside {
		return GPUProviderCredentials{}, errors.New("GPU credentials file must be outside the Git repository")
	}
	file, err := os.Open(absPath)
	if err != nil {
		return GPUProviderCredentials{}, errors.New("GPU credentials file is unreadable")
	}
	defer file.Close()
	decoder := json.NewDecoder(io.LimitReader(file, maxGPUCredentialsFileSize))
	decoder.DisallowUnknownFields()
	var credentials GPUProviderCredentials
	if err := decoder.Decode(&credentials); err != nil {
		return GPUProviderCredentials{}, errors.New("GPU credentials file contains invalid JSON")
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return GPUProviderCredentials{}, errors.New("GPU credentials file must contain one JSON object")
	}
	return credentials, nil
}

func pathInsideGitRepository(path string) (bool, error) {
	realPath, err := filepath.EvalSymlinks(path)
	if err != nil {
		return false, errors.New("GPU credentials path cannot be resolved")
	}
	dir := filepath.Dir(realPath)
	for current := dir; ; current = filepath.Dir(current) {
		if _, err := os.Stat(filepath.Join(current, ".git")); err == nil {
			relative, relErr := filepath.Rel(current, realPath)
			return relErr == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)), relErr
		}
		parent := filepath.Dir(current)
		if parent == current {
			return false, nil
		}
	}
}

func clearGPUProviderCredentials(cfg *Config) {
	cfg.RunPodAPIKey = ""
	cfg.ModalTokenID = ""
	cfg.ModalTokenSecret = ""
	cfg.LambdaAPIKey = ""
	cfg.VastAPIKey = ""
}

func applyGPUCredentialEnvironment(cfg *Config) {
	for key, target := range map[string]*string{
		"RUNPOD_API_KEY": &cfg.RunPodAPIKey, "MODAL_TOKEN_ID": &cfg.ModalTokenID,
		"MODAL_TOKEN_SECRET": &cfg.ModalTokenSecret, "LAMBDA_API_KEY": &cfg.LambdaAPIKey,
		"VAST_API_KEY": &cfg.VastAPIKey,
	} {
		if value, ok := os.LookupEnv(key); ok && strings.TrimSpace(value) != "" {
			*target = strings.TrimSpace(value)
		}
	}
}
