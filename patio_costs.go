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

// roofSlopeRatePerSqFt is the extra cost per sq ft (total finished area, incl.
// overhangs) for each 1:12 of roof slope beyond the 4:12 baseline.
var roofSlopeRatePerSqFt = map[string]float64{
	"truss":       1.0,
	"timberframe": 2.0,
}

// CalculateRoofSlopeCost computes the extra cost for a roof slope steeper than
// the 4:12 baseline. Pergola and lean-to covers are fixed at 2:12 with no edit
// option and no extra cost. Truss and timberframe default to 4:12 and price
// each additional 1:12 of slope at roofSlopeRatePerSqFt, applied against the
// total finished square footage (including overhangs).
func (estimate *PatioCoverEstimate) CalculateRoofSlopeCost() {
	if estimate.PatioType == "pergola" || estimate.PatioType == "leanto" {
		estimate.RoofSlope = 2
		estimate.RoofSlopeCost = 0
		return
	}
	if estimate.RoofSlope < 4 {
		estimate.RoofSlope = 4
	}
	rate, ok := roofSlopeRatePerSqFt[estimate.PatioType]
	if !ok {
		estimate.RoofSlopeCost = 0
		return
	}
	estimate.RoofSlopeCost = float64(estimate.RoofSlope-4) * rate * estimate.Area
}

// CalcPermitCost computes design/engineering/permit cost based on the selected
// tier, same formula and per-level rate as the deck estimate.
func (estimate *PatioCoverEstimate) CalcPermitCost(costs Costs) {
	estimate.PermitCost = float64(estimate.PermitLevel) * costs.PermitCostPerLevel
	if estimate.DIYMode == 1 || estimate.DIYMode == 2 {
		estimate.PermitCost += 500
	}
}
