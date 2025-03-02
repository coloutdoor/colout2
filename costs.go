package main

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Costs holds pricing data loaded from costs.yaml.
type Costs struct {
	DeckMaterials map[string]float64 `yaml:"deck_materials"`
	RailMaterials map[string]float64 `yaml:"rail_materials"`
	RailInfills   map[string]float64 `yaml:"rail_infills"`
	DemoCost      float64            `yaml:"demo_cost"`
	FasciaCost    float64            `yaml:"fascia_cost"`
}

// costs is the global pricing data, loaded at startup.
var costs Costs

// loadCosts reads and parses costs.yaml into the costs var.
func loadCosts() error {
	data, err := os.ReadFile("costs.yaml")
	if err != nil {
		return fmt.Errorf("failed to read costs.yaml: %v", err)
	}
	if err := yaml.Unmarshal(data, &costs); err != nil {
		return fmt.Errorf("failed to parse costs.yaml: %v", err)
	}
	return nil
}
