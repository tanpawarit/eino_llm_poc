package src

import (
	"eino_llm_poc/src/model"
	"fmt"
	"os"

	"gopkg.in/yaml.v2"
)

type YAMLConfig struct {
	NLUConfig          model.NLUConfig          `yaml:"nlu"`
	ConversationConfig model.ConversationConfig `yaml:"conversation"`
}

func LoadConfig(filepath string) (*YAMLConfig, error) {
	data, err := os.ReadFile(filepath)
	if err != nil {
		return nil, fmt.Errorf("error reading config file: %v", err)
	}

	var config YAMLConfig
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return nil, fmt.Errorf("error parsing YAML: %v", err)
	}

	return &config, nil
}
