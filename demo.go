package main

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
