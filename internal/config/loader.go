package config

import (
	"fmt"
	"os"
	"eino_llm_poc/pkg"
	"eino_llm_poc/internal/core"
	"gopkg.in/yaml.v3"
)

// YAMLConfig represents the structure of config.yaml
type YAMLConfig struct {
	NLU struct {
		DefaultIntent       string  `yaml:"default_intent"`
		AdditionalIntent    string  `yaml:"additional_intent"`
		DefaultEntity       string  `yaml:"default_entity"`
		AdditionalEntity    string  `yaml:"additional_entity"`
		TupleDelimiter      string  `yaml:"tuple_delimiter"`
		RecordDelimiter     string  `yaml:"record_delimiter"`
		CompletionDelimiter string  `yaml:"completion_delimiter"`
		ImportanceThreshold float64 `yaml:"importance_threshold"`
	} `yaml:"nlu"`
}

// LoadConfig loads configuration from config.yaml
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

// BuildNLUConfig creates NLUConfig from YAML config and environment variables
func BuildNLUConfig(yamlConfig *YAMLConfig, apiKey string) pkg.NLUConfig {
	return pkg.NLUConfig{
		Model:               "openai/gpt-3.5-turbo",
		APIKey:              apiKey,
		BaseURL:             "https://openrouter.ai/api/v1",
		MaxTokens:           1500,
		Temperature:         0.1,
		ImportanceThreshold: yamlConfig.NLU.ImportanceThreshold,
		TupleDelimiter:      yamlConfig.NLU.TupleDelimiter,
		RecordDelimiter:     yamlConfig.NLU.RecordDelimiter,
		CompletionDelimiter: yamlConfig.NLU.CompletionDelimiter,
		DefaultIntent:       yamlConfig.NLU.DefaultIntent,
		AdditionalIntent:    yamlConfig.NLU.AdditionalIntent,
		DefaultEntity:       yamlConfig.NLU.DefaultEntity,
		AdditionalEntity:    yamlConfig.NLU.AdditionalEntity,
	}
}

// BuildCoreConfig creates core.Config with default values
func BuildCoreConfig(nluConfig pkg.NLUConfig) core.Config {
	return core.Config{
		NLU: nluConfig,
		Redis: core.RedisConfig{
			URL: "localhost:6379",
			TTL: 3600, // 1 hour
		},
		Storage: core.StorageConfig{
			LongtermDir: "data/longterm",
		},
		Graph: core.GraphConfig{
			DefaultFlow: core.GraphFlow{
				StartNode: "nlu",
				Edges: map[string][]core.GraphEdge{
					"nlu": {
						{To: "routing", Priority: 1},
					},
					"routing": {
						{To: "response", Priority: 1},
					},
					"response": {
						{To: "tools", Condition: map[string]any{"need_tools": true}, Priority: 1},
						{To: "complete", Priority: 2},
					},
					"tools": {
						{To: "complete", Priority: 1},
					},
				},
			},
			EnableParallel: false,
			MaxConcurrency: 3,
		},
	}
}