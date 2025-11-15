package main

import (
	"fmt"
	"math"
)

//   - CalculateStairCost computes stair cost based on height, width, and deck material cost.
//
// Assumes 7-inch rise (~1.6 steps per ft of height), 3 ft min width, 1.5x material cost adjustment.
// Returns 0 if stairWidth is 0 (no stairs). Errors if width < 3 ft and > 0.
// func CalculateStairCost(height, stairWidth, materialCost float64) (float64, error) {
func (e *DeckEstimate) CalcStairCost(materialCost float64) (float64, error) {
	steps := math.Ceil(e.Height * 1.6) // ~1.6 steps/ft, round up
	if e.StairWidth > 0 && e.StairWidth < 3 {
		return 0, fmt.Errorf("stair width must be at least 3 ft if specified")
	}
	if e.StairWidth == 0 {
		return 0, nil // No stairs
	}
	stairAdjustCost := 1.5
	baseCost := materialCost * steps * e.StairWidth * stairAdjustCost
	return baseCost, nil
}

// - CalculateStairFasciaCost computes stair cost based on height, width, and deck material cost.
func (e *DeckEstimate) CalcStairFasciaCost(cost Costs) (float64, error) {
	if e.StairWidth == 0 {
		return 0, nil // No stairs
	}
	length := math.Ceil(e.Height * 1.6)                               // ~1.6 steps/ft, round up
	stairAdjustCost := 1.5                                            // 12" fascia required for stairs
	stairFasciaCost := length * cost.FasciaCost * stairAdjustCost * 2 // Fascia 2 sides
	return stairFasciaCost, nil
}

// - CalculateStairFasciaCost computes stair cost based on height, width, and deck material cost.
func (e *DeckEstimate) CalcStairToeKickCost(cost Costs) (float64, error) {
	if e.StairWidth == 0 || !e.HasStairTK {
		return 0, nil // No stairs or No Toe Kicks on Stairs
	}
	steps := math.Ceil(e.Height * 1.6)                    // ~1.6 steps/ft, round up
	stairTKCost := steps * e.StairWidth * cost.FasciaCost // Fascia 2 sides
	return stairTKCost, nil
}
