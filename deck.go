package main

import "fmt"

func CalculateDeckCost(length, width, height float64, material string, costs Costs) (float64, error) {
	area := length * width
	costPerSqFt, ok := costs.DeckMaterials[material]
	if !ok {
		return 0, fmt.Errorf("please select a valid material")
	}
	baseCost := area * costPerSqFt

	if height >= 20 {
		return 0, fmt.Errorf("we can't build decks 20 feet or higher without additional information")
	} else if height > 4 {
		excessHeight := height - 4
		multiplier := 1 + (excessHeight * 0.01)
		return baseCost * multiplier, nil
	}
	return baseCost, nil
}
