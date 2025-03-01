package main

// CalculateDemoCost computes cost to demo and remove old structure.
// Fixed rate: $5 per square foot of deck area.
func CalculateDemoCost(deckArea float64) float64 {
	demoRate := 5.0 // $5/sq ft
	return deckArea * demoRate
}
