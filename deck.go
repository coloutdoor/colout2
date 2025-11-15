package main

// func (e *DeckEstimate) CalcStairToeKickCost(cost Costs) (float64, error) {
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
