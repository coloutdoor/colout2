package main

// CalculateDemoCost computes cost to demo and remove old structure.
// Uses rate from costs.yaml per square foot of deck area.
func CalculateDemoCost(deckArea float64, costs Costs) float64 {
	return deckArea * costs.DemoCost
}
