package main

// CalculateDemoCost computes cost to demo and remove old structure.
// Uses rate from costs.yaml per square foot of deck area.
func CalculateDemoCost(estimate DeckEstimate, costs Costs) float64 {
	deckArea := estimate.Length * estimate.Width
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

	return (deckArea + railArea + stairArea + stairRailArea) * costs.DemoCost
}
