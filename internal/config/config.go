package config

import (
	"errors"
	"os"
	"strings"

	"github.com/ricelines/chat/matrix-mcp-go/internal/scopes"
)

const (
	envListenAddr        = "MATRIX_MCP_LISTEN_ADDR"
	envHomeserverURL     = "MATRIX_HOMESERVER_URL"
	envUsername          = "MATRIX_USERNAME"
	envPassword          = "MATRIX_PASSWORD"
	envRegistrationToken = "MATRIX_REGISTRATION_TOKEN"
	envScopes            = "MATRIX_MCP_SCOPES"

	defaultListenAddr = ":8080"
)

type Config struct {
	ListenAddr        string
	HomeserverURL     string
	Username          string
	Password          string
	RegistrationToken string
	Scopes            scopes.Set
}

func FromEnv() (Config, error) {
	parsedScopes, err := scopes.Parse(strings.TrimSpace(os.Getenv(envScopes)))
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		ListenAddr:        strings.TrimSpace(os.Getenv(envListenAddr)),
		HomeserverURL:     strings.TrimSpace(os.Getenv(envHomeserverURL)),
		Username:          strings.TrimSpace(os.Getenv(envUsername)),
		Password:          strings.TrimSpace(os.Getenv(envPassword)),
		RegistrationToken: strings.TrimSpace(os.Getenv(envRegistrationToken)),
		Scopes:            parsedScopes,
	}
	if cfg.ListenAddr == "" {
		cfg.ListenAddr = defaultListenAddr
	}
	return cfg, cfg.Validate()
}

func (c Config) Validate() error {
	var problems []string
	if c.ListenAddr == "" {
		problems = append(problems, "listen addr must not be empty")
	}
	if c.HomeserverURL == "" {
		problems = append(problems, envHomeserverURL+" is required")
	}
	if c.Username == "" {
		problems = append(problems, envUsername+" is required")
	}
	if c.Password == "" {
		problems = append(problems, envPassword+" is required")
	}
	if len(problems) > 0 {
		return errors.New(strings.Join(problems, "; "))
	}
	return nil
}
