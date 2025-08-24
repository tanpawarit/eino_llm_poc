package src

import (
	"eino_llm_poc/src/model"
	"fmt"

	"github.com/kelseyhightower/envconfig"
)

type Config struct {
	LogConfig          model.LogConfig          `envconfig:""`
	NLUConfig          model.NLUConfig          `envconfig:""`
	ConversationConfig model.ConversationConfig `envconfig:""`
}

func LoadConfig() (*Config, error) {
	var config Config
	err := envconfig.Process("", &config)
	if err != nil {
		return nil, fmt.Errorf("error processing environment configuration: %v", err)
	}

	return &config, nil
}
