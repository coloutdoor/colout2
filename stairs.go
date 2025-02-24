package main

import (
	"fmt"
	"math"
)

// / CalculateStairCost computes stair cost based on height, width, and deck material cost.
// Assumes 7-inch rise (~1.6 steps per ft of height), 3 ft min width, 1.5x material cost adjustment.
// Returns 0 if stairWidth is 0 (no stairs). Errors if width < 3 ft and > 0.
func CalculateStairCost(height, stairWidth, materialCost float64) (float64, error) {
	steps := math.Ceil(height * 1.6) // ~1.6 steps/ft, round up
	if stairWidth > 0 && stairWidth < 3 {
		return 0, fmt.Errorf("stair width must be at least 3 ft if specified")
	}
	if stairWidth == 0 {
		return 0, nil // No stairs
	}
	stairAdjustCost := 1.5
	baseCost := materialCost * steps * stairWidth * stairAdjustCost
	return baseCost, nil
}
