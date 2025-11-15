package main

import (
	"math"
)

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
