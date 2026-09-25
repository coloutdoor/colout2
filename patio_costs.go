package main

import "math"

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

// CalculatePostCount computes the number of posts needed to support the patio
// cover: posts spaced a maximum of 12 ft apart along the width, minimum 2.
func (estimate *PatioCoverEstimate) CalculatePostCount() {
	spans := int(math.Ceil(estimate.Width / 12))
	count := spans + 1
	if count < 2 {
		count = 2
	}
	estimate.PostCount = count
}

// CalculatePostWrapCost prices the post wrap add-on: costs.PatioPostWrapPerSqFt
// per finished sq ft (including overhangs) plus costs.PatioPostWrapPerPost per
// post. Zero when the option is off.
func (estimate *PatioCoverEstimate) CalculatePostWrapCost(costs Costs) {
	if !estimate.HasPostWrap {
		estimate.PostWrapCost = 0
		return
	}
	estimate.PostWrapCost = costs.PatioPostWrapPerSqFt*estimate.Area + costs.PatioPostWrapPerPost*float64(estimate.PostCount)
}

// CalculateRoofSlopeCost computes the extra cost for a roof slope steeper than
// the 4:12 baseline. Pergola and lean-to covers are fixed at 2:12 with no edit
// option and no extra cost. Truss and timberframe default to 4:12 and price
// each additional 1:12 of slope at costs.PatioRoofSlopePerSqFt[PatioType],
// applied against the total finished square footage (including overhangs).
func (estimate *PatioCoverEstimate) CalculateRoofSlopeCost(costs Costs) {
	if estimate.PatioType == "pergola" || estimate.PatioType == "leanto" {
		estimate.RoofSlope = 2
		estimate.RoofSlopeCost = 0
		return
	}
	if estimate.RoofSlope < 4 {
		estimate.RoofSlope = 4
	}
	rate, ok := costs.PatioRoofSlopePerSqFt[estimate.PatioType]
	if !ok {
		estimate.RoofSlopeCost = 0
		return
	}
	estimate.RoofSlopeCost = float64(estimate.RoofSlope-4) * rate * estimate.Area
}

// CalculateFinishCeilingCost prices the finish ceiling add-on. Pergola has no
// ceiling to finish (not available); timberframe leaves its beams exposed by
// design, included in the base price at no extra cost. Lean-to and truss are
// optional: costs.PatioFinishCeilingPerSqFt/sq ft when on, $0 when off.
func (estimate *PatioCoverEstimate) CalculateFinishCeilingCost(costs Costs) {
	if estimate.PatioType == "pergola" || estimate.PatioType == "timberframe" {
		estimate.HasFinishCeiling = false
		estimate.FinishCeilingCost = 0
		return
	}
	if estimate.HasFinishCeiling {
		estimate.FinishCeilingCost = costs.PatioFinishCeilingPerSqFt * estimate.Area
	} else {
		estimate.FinishCeilingCost = 0
	}
}

// CalculatePaintStainCost prices the paint/stain add-on:
// costs.PatioPaintStainPerSqFt/sq ft when on, $0 when off.
func (estimate *PatioCoverEstimate) CalculatePaintStainCost(costs Costs) {
	if estimate.HasPaintStain {
		estimate.PaintStainCost = costs.PatioPaintStainPerSqFt * estimate.Area
	} else {
		estimate.PaintStainCost = 0
	}
}

// CalculateFinishHardwareCost prices the finish hardware upgrade:
// costs.PatioFinishHardwarePerSqFt/sq ft when on, $0 when off (standard
// galvanized hardware, included).
func (estimate *PatioCoverEstimate) CalculateFinishHardwareCost(costs Costs) {
	if estimate.HasFinishHardware {
		estimate.FinishHardwareCost = costs.PatioFinishHardwarePerSqFt * estimate.Area
	} else {
		estimate.FinishHardwareCost = 0
	}
}

// CalculateElectricalCost prices the electrical add-on: a flat trip charge
// (costs.PatioElectricalBase) plus costs.PatioElectricalPerItem for each
// canned light, ceiling fan, switch, and outlet combined. $0 when nothing is
// selected.
func (estimate *PatioCoverEstimate) CalculateElectricalCost(costs Costs) {
	totalItems := estimate.ElectricalLights + estimate.ElectricalFans + estimate.ElectricalSwitches + estimate.ElectricalOutlets
	estimate.HasElectrical = totalItems > 0
	if !estimate.HasElectrical {
		estimate.ElectricalCost = 0
		return
	}
	estimate.ElectricalCost = costs.PatioElectricalBase + costs.PatioElectricalPerItem*float64(totalItems)
}

// CalcPermitCost computes design/engineering/permit cost based on the selected
// tier, same formula and per-level rate as the deck estimate.
func (estimate *PatioCoverEstimate) CalcPermitCost(costs Costs) {
	estimate.PermitCost = float64(estimate.PermitLevel) * costs.PermitCostPerLevel
	if estimate.DIYMode == 1 || estimate.DIYMode == 2 {
		estimate.PermitCost += 500
	}
}
