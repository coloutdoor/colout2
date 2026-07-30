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
	DemoCost            float64            `yaml:"demo_cost"`
	FasciaCost          float64            `yaml:"fascia_cost"`
	PermitCostPerLevel  float64            `yaml:"permit_cost_per_level"`
	DiscountCodes       map[string]float64 `yaml:"discount_codes"`
}

// costs is the global pricing data, loaded at startup.
var costs Costs

// loadCosts reads and parses costs.yaml into the var.
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

// CalculateDeckCost computes the total price of the deck based on the provided material and labor costs.
func (estimate *DeckEstimate) CalculateDeckCost(costs Costs) {
	// Sum area across all sections; fall back to L×W for legacy single-section estimates
	var area float64
	if len(estimate.Sections) > 0 {
		for _, s := range estimate.Sections {
			area += s.Length * s.Width
		}
	} else {
		area = estimate.Length * estimate.Width
	}
	costPerSqFt, ok := costs.DeckMaterials[estimate.Material]
	if !ok {
		estimate.Error = "Please select a valid material for Deck"
		return
	}
	estimate.DeckArea = area
	baseCost := area * costPerSqFt

	if estimate.Height >= 20 {
		estimate.Error = "Decks 20 feet or higher will require additional engineering."
	} else if estimate.Height >= 5 {
		excessHeight := estimate.Height - 4
		multiplier := 1 + (excessHeight * 0.01)
		estimate.DeckCost = baseCost * multiplier
	} else {
		estimate.DeckCost = baseCost
	}
}

// CalculateDemoCost computes cost to demo and remove old structure.
// Uses rate from costs.yaml per square foot of deck area.
func (estimate *DeckEstimate) CalculateDemoCost(costs Costs) {
	if !estimate.HasDemo {
		estimate.DemoCost = 0.0
		return
	}

	deckArea := estimate.DeckArea // set by CalculateDeckCost
	railArea := 0.0
	stairArea := 0.0
	stairRailArea := 0.0

	if estimate.RailCost > 0.0 {
		railArea = estimate.RailFeet * 3
	}
	if estimate.StairCost > 0.0 {
		stairArea = estimate.Height * estimate.StairWidth * 1.5
		stairRailArea = stairArea // Something? for now ??
	}

	estimate.DemoCost = (deckArea + railArea + stairArea + stairRailArea) * costs.DemoCost

}

// CalculateFasciaCost computes fascia cost based on deck perimeter (2L + W).
// Uses rate from costs.yaml per linear foot.
func (estimate *DeckEstimate) CalculateFasciaCost(costs Costs) {
	estimate.FasciaFeet = 0.0
	estimate.FasciaCost = 0.0
	if estimate.HasFascia {
		estimate.FasciaFeet = estimate.RailFeet
		estimate.FasciaCost = estimate.FasciaFeet * costs.FasciaCost
	}
}

func (estimate *DeckEstimate) CalculateRailCost(costs Costs) {
	if estimate.RailMaterial == "" {
		estimate.RailInfill = ""
		estimate.RailCost = 0.0
		return
	}

	// Set to Baluster infill if not selected
	if estimate.RailInfill == "" {
		estimate.RailInfill = "balusters"
	}

	railMatCost := costs.RailMaterials[estimate.RailMaterial]
	railInfCost := costs.RailInfills[estimate.RailInfill]

	// Use manual override if set; otherwise calculate from sections
	if estimate.RailFeetOverride > 0 {
		estimate.RailFeet = estimate.RailFeetOverride
	} else if len(estimate.Sections) > 1 {
		total := estimate.Sections[0].Length + estimate.Sections[0].Width
		for i := 1; i < len(estimate.Sections); i++ {
			s, prev := estimate.Sections[i], estimate.Sections[i-1]
			total += s.Length + s.Width + math.Abs(s.Length-prev.Length)
		}
		estimate.RailFeet = total - estimate.StairWidth
	} else if len(estimate.Sections) == 1 {
		s := estimate.Sections[0]
		estimate.RailFeet = (2 * s.Length) + s.Width - estimate.StairWidth
	} else {
		estimate.RailFeet = (2 * estimate.Length) + estimate.Width - estimate.StairWidth
	}
	estimate.RailCost = estimate.RailFeet * (railMatCost + railInfCost)
}

// CalculateStairRailCost computes rail cost for stairs based on height and material.
// Assumes 2 sides, 1.6 steps/ft (length matches stair steps), 1.5x cost factor.
func (estimate *DeckEstimate) CalculateStairRailCost(costs Costs) {
	if estimate.RailMaterial == "" {
		estimate.StairRailCost = 0
		return
	}

	if estimate.StairRailCount > 1.0 {
		estimate.StairRailCount = 2.0
	}
	stairRailLength := estimate.Height * 1.6 // Matches stair steps
	railMatCost := costs.RailMaterials[estimate.RailMaterial]
	stairCostFactor := 1.4
	estimate.StairRailCost = estimate.StairRailCount * stairRailLength * railMatCost * stairCostFactor
}

var stairAdjustCost = 1.5 // Adjust the stairs by 1.5X vs deck costs
var stepToHeight = 1.6    // 1.6 steps/foot

// CalcStairCost computes stair cost based on height, width, and deck material cost.
// Assumes 7-inch rise (~1.6 steps per ft of height), 3 ft min width, 1.5x material cost adjustment.
// Returns 0 if stairWidth is 0 (no stairs). Errors if width < 3 ft and > 0.
// func CalculateStairCost(height, stairWidth, materialCost float64) (float64, error) {
func (estimate *DeckEstimate) CalcStairCost(costs Costs) {
	materialCost := costs.DeckMaterials[estimate.Material]

	if estimate.StairWidth == 0 {
		estimate.StairCost = 0 // No stairs
	} else if estimate.StairWidth > 0 && estimate.StairWidth < 3 {
		estimate.Error = "stair width must be at least 3 ft if specified"
		estimate.StairCost = 0
	} else {
		steps := math.Ceil(estimate.Height * stepToHeight) // ~1.6 steps/ft, round up
		estimate.StairCost = materialCost * steps * estimate.StairWidth * stairAdjustCost
	}
}

// CalcStairFasciaCost computes stair cost based on height, width, and deck material cost.
func (estimate *DeckEstimate) CalcStairFasciaCost(costs Costs) {
	if estimate.StairWidth == 0 || !estimate.HasStairFascia {
		estimate.StairFasciaCost = 0
	} else {
		length := math.Ceil(estimate.Height * 1.6)                                 // ~1.6 steps/ft, round up
		stairAdjustCost := 1.5                                                     // 12" fascia required for stairs
		estimate.StairFasciaCost = length * costs.FasciaCost * stairAdjustCost * 2 // Fascia 2 sides
	}
}

// CalcStairToeKickCost computes stair cost based on height, width, and deck material cost.
func (estimate *DeckEstimate) CalcStairToeKickCost(cost Costs) {
	if estimate.StairWidth == 0 || !estimate.HasStairTK {
		// No stairs or No Toe Kicks on Stairs
		estimate.StairToeKickCost = 0
	} else {
		steps := math.Ceil(estimate.Height * 1.6)                                 // ~1.6 steps/ft, round up
		estimate.StairToeKickCost = steps * estimate.StairWidth * cost.FasciaCost // Fascia 2 sides
	}
}

// CalcPermitCost computes design/engineering/permit cost based on the selected tier.
// Level 0 = none, 1 = design, 2 = design+engineering, 3 = design+engineering+permits.
func (estimate *DeckEstimate) CalcPermitCost(costs Costs) {
	estimate.PermitCost = float64(estimate.PermitLevel) * costs.PermitCostPerLevel
}

var stateTaxRates = map[string]float64{
	"WA": 0.087,
}

// CalculateSalesTax applies sales tax based on the customer's state.
// Defaults to WA rate if state is unrecognized or empty.
func CalculateSalesTax(subtotal float64, state string) float64 {
	rate, ok := stateTaxRates[state]
	if !ok {
		rate = stateTaxRates["WA"]
	}
	return subtotal * rate
}
