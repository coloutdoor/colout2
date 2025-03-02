package main

// CalculateFasciaCost computes fascia cost based on deck perimeter (2L + W).
// Uses rate from costs.yaml per linear foot.
func CalculateFasciaCost(length, width float64, costs Costs) float64 {
	perimeter := (2 * length) + width // Matches rail calc
	return perimeter * costs.FasciaCost
}
