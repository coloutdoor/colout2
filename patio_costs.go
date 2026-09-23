package main

// CalculatePatioCost computes the base cost of a patio cover from its footprint
// and PatioType. Finished dimensions add a 1 ft overhang on the two side edges
// and the outer edge (Width+2, Depth+1); the wall-attached edge has no overhang.
// Gutters and roofing are included in the per-sq-ft rate; finish-level extras
// (screens, electrical, etc.) are priced separately and not yet implemented.
func (estimate *PatioCoverEstimate) CalculatePatioCost(costs Costs) {
	rate, ok := costs.PatioMaterials[estimate.PatioType]
	if !ok {
		estimate.Error = "Please select a valid patio cover type"
		return
	}
	estimate.Area = (estimate.Width + 2) * (estimate.Depth + 1)
	estimate.BaseCost = estimate.Area * rate
}

// CalcPermitCost computes design/engineering/permit cost based on the selected
// tier, same formula and per-level rate as the deck estimate.
func (estimate *PatioCoverEstimate) CalcPermitCost(costs Costs) {
	estimate.PermitCost = float64(estimate.PermitLevel) * costs.PermitCostPerLevel
	if estimate.DIYMode == 1 || estimate.DIYMode == 2 {
		estimate.PermitCost += 500
	}
}
