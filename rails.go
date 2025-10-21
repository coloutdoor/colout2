package main

import "fmt"

func CalculateRailCost(length, width float64, railMaterial, railInfill string, stairWidth float64, costs Costs) (float64, error) {
	// Rails on 3 sides: 2 lengths + 1 width (house on one side) - stair opening
	perimeter := (2 * length) + width - stairWidth

	railMatCost := costs.RailMaterials[railMaterial] // 0.0 if not found
	railInfCost := costs.RailInfills[railInfill]     // 0.0 if not found

	if railMaterial == "" && railInfill != "" {
		return 0, fmt.Errorf("Rail infill requires a rail material")
	}

	return perimeter * (railMatCost + railInfCost), nil
}

// CalculateStairRailCost computes rail cost for stairs based on height and material.
// Assumes 2 sides, 1.6 steps/ft (length matches stair steps), 1.5x cost factor.
func CalculateStairRailCost(height float64, railMaterial string, stairRailSides float64, costs Costs) float64 {
	if railMaterial == "" {
		return 0 // No rails
	}
	if stairRailSides > 1.0 {
		stairRailSides = 2.0
	}
	stairRailLength := height * 1.6 // Matches stair steps
	railMatCost := costs.RailMaterials[railMaterial]
	stairCostFactor := 1.4
	return stairRailSides * stairRailLength * railMatCost * stairCostFactor
}
