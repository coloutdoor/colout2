package main

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
