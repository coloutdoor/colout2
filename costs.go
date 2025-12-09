package main

import (
	"fmt"
	"math"
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
	data, err := os.ReadFile("static/costs.yaml")
	if err != nil {
		return fmt.Errorf("failed to read costs.yaml: %v", err)
	}
	if err := yaml.Unmarshal(data, &costs); err != nil {
		return fmt.Errorf("failed to parse costs.yaml: %v", err)
	}
	return nil
}

// Calculate Deck Costs
func (e *DeckEstimate) CalculateDeckCost(costs Costs) {
	area := e.Length * e.Width
	costPerSqFt, ok := costs.DeckMaterials[e.Material]
	if !ok {
		e.Error = "Please select a valid material for Deck"
		return
	}
	baseCost := area * costPerSqFt

	if e.Height >= 20 {
		e.Error = "Decks 20 feet or higher will require additional engineering."
	} else if e.Height >= 5 {
		excessHeight := e.Height - 4
		multiplier := 1 + (excessHeight * 0.01)
		e.DeckCost = baseCost * multiplier
	} else {
		e.DeckCost = baseCost
	}
}

// CalculateDemoCost computes cost to demo and remove old structure.
// Uses rate from costs.yaml per square foot of deck area.
func (e *DeckEstimate) CalculateDemoCost(costs Costs) {
	if !e.HasDemo {
		e.DemoCost = 0.0
		return
	}

	deckArea := e.Length * e.Width
	railArea := 0.0
	stairArea := 0.0
	stairRailArea := 0.0

	if e.RailCost > 0.0 {
		railArea = e.RailFeet * 3
	}
	if e.StairCost > 0.0 {
		stairArea = e.Height * e.StairWidth * 1.5
		stairRailArea = stairArea // Something? for now ??
	}

	e.DemoCost = (deckArea + railArea + stairArea + stairRailArea) * costs.DemoCost

}

// CalculateFasciaCost computes fascia cost based on deck perimeter (2L + W).
// Uses rate from costs.yaml per linear foot.
func (e *DeckEstimate) CalculateFasciaCost(costs Costs) {
	e.FasciaFeet = 0.0
	e.FasciaCost = 0.0
	if e.HasFascia {
		e.FasciaFeet = (2 * e.Length) + e.Width // Matches rail calc
		e.FasciaCost = e.FasciaFeet * costs.FasciaCost
	}
}

func (e *DeckEstimate) CalculateRailCost(Costs) {
	if e.RailMaterial == "" {
		e.RailInfill = ""
		e.RailCost = 0.0
		return
	}

	// Set to Baluster infill if not selected
	if e.RailInfill == "" {
		e.RailInfill = "balusters"
	}

	// Rails on 3 sides: 2 lengths + 1 width (house on one side) - stair opening
	railMatCost := costs.RailMaterials[e.RailMaterial] // 0.0 if not found
	railInfCost := costs.RailInfills[e.RailInfill]     // 0.0 if not found
	e.RailFeet = (2 * e.Length) + e.Width - e.StairWidth
	e.RailCost = e.RailFeet * (railMatCost + railInfCost)
}

// CalculateStairRailCost computes rail cost for stairs based on height and material.
// Assumes 2 sides, 1.6 steps/ft (length matches stair steps), 1.5x cost factor.
func (e *DeckEstimate) CalculateStairRailCost(costs Costs) {
	if e.RailMaterial == "" {
		e.StairRailCost = 0
		return
	}

	if e.StairRailCount > 1.0 {
		e.StairRailCount = 2.0
	}
	stairRailLength := e.Height * 1.6 // Matches stair steps
	railMatCost := costs.RailMaterials[e.RailMaterial]
	stairCostFactor := 1.4
	e.StairRailCost = e.StairRailCount * stairRailLength * railMatCost * stairCostFactor
}

var stairAdjustCost = 1.5 // Adjust the stairs by 1.5X vs deck costs
var stepToHeight = 1.6    // 1.6 steps/foot
//   - CalculateStairCost computes stair cost based on height, width, and deck material cost.
//
// Assumes 7-inch rise (~1.6 steps per ft of height), 3 ft min width, 1.5x material cost adjustment.
// Returns 0 if stairWidth is 0 (no stairs). Errors if width < 3 ft and > 0.
// func CalculateStairCost(height, stairWidth, materialCost float64) (float64, error) {
func (e *DeckEstimate) CalcStairCost(cost Costs) {
	materialCost := costs.DeckMaterials[e.Material]

	if e.StairWidth == 0 {
		e.StairCost = 0 // No stairs
	} else if e.StairWidth > 0 && e.StairWidth < 3 {
		e.Error = "stair width must be at least 3 ft if specified"
		e.StairCost = 0
	} else {
		steps := math.Ceil(e.Height * stepToHeight) // ~1.6 steps/ft, round up
		e.StairCost = materialCost * steps * e.StairWidth * stairAdjustCost
	}
}

// - CalculateStairFasciaCost computes stair cost based on height, width, and deck material cost.
func (e *DeckEstimate) CalcStairFasciaCost(cost Costs) {
	if e.StairWidth == 0 || !e.HasStairFascia {
		e.StairFasciaCost = 0
	} else {
		length := math.Ceil(e.Height * 1.6)                                // ~1.6 steps/ft, round up
		stairAdjustCost := 1.5                                             // 12" fascia required for stairs
		e.StairFasciaCost = length * cost.FasciaCost * stairAdjustCost * 2 // Fascia 2 sides
	}
}

// - CalculateStairFasciaCost computes stair cost based on height, width, and deck material cost.
func (e *DeckEstimate) CalcStairToeKickCost(cost Costs) {
	if e.StairWidth == 0 || !e.HasStairTK {
		// No stairs or No Toe Kicks on Stairs
		e.StairToeKickCost = 0
	} else {
		steps := math.Ceil(e.Height * 1.6)                          // ~1.6 steps/ft, round up
		e.StairToeKickCost = steps * e.StairWidth * cost.FasciaCost // Fascia 2 sides
	}
}

// CalculateSalesTax applies WA sales tax to the subtotal (deck + rail costs).
// Currently hardcoded at 10% (6.5% state + 3.5% local, e.g., Seattle).
// Future: Replace with dynamic lookup based on address.
func CalculateSalesTax(subtotal float64) float64 {
	const taxRate = 0.087 // 8.7% total WA sales tax
	return subtotal * taxRate
}
