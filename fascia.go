package main

import "log"

// CalculateFasciaCost computes fascia cost based on deck perimeter (2L + W).
// Uses rate from costs.yaml per linear foot.
func (e *DeckEstimate) CalculateFasciaCost(costs Costs) {
	e.FasciaFeet = 0.0
	e.FasciaCost = 0.0
	if e.HasFascia {
		e.FasciaFeet = (2 * e.Length) + e.Width // Matches rail calc
		e.FasciaCost = e.FasciaFeet * costs.FasciaCost

		log.Printf("Fascia is set to %.1f for %.1f ft.", e.FasciaCost, e.FasciaFeet)

	}
}
