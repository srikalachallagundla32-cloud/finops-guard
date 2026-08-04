package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

const defaultFailOnCostThreshold = 50.00

type Settings struct {
	Currency            string  `yaml:"currency"`
	FailOnCostThreshold float64 `yaml:"fail_on_cost_threshold"`
}

type Rule struct {
	ID         string   `yaml:"id"`
	Name       string   `yaml:"name"`
	Severity   string   `yaml:"severity"`
	TargetAPIs []string `yaml:"target_apis"`
}

type Config struct {
	Version  string   `yaml:"version"`
	Settings Settings `yaml:"settings"`
	Rules    []Rule   `yaml:"rules"`
}

// Load reads and parses a .finops-guard.yml file.
func Load(filePath string) (*Config, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// FailOnCostThreshold reads settings.fail_on_cost_threshold from filePath,
// falling back to $50.00 if the file doesn't exist or can't be parsed.
func FailOnCostThreshold(filePath string) float64 {
	cfg, err := Load(filePath)
	if err != nil {
		return defaultFailOnCostThreshold
	}
	return cfg.Settings.FailOnCostThreshold
}
