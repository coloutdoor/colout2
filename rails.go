package main

import "fmt"

func CalculateRailCost(length, width float64, railMaterial, railInfill string, costs Costs) (float64, error) {
	// Rails on 3 sides: 2 lengths + 1 width (house on one side)
	perimeter := (2 * length) + width
	railMatCost := costs.RailMaterials[railMaterial] // 0.0 if not found
	railInfCost := costs.RailInfills[railInfill]     // 0.0 if not found

	if railMaterial == "" && railInfill != "" {
		return 0, fmt.Errorf("rail infill requires a rail material")
	}

	return perimeter * (railMatCost + railInfCost), nil
}
