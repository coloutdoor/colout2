package main

import "fmt"

func CalculateRailCost(length, width float64, railMaterial, railInfill string, costs Costs) (float64, error) {
	perimeter := (length + width) * 2
	railMatCost := costs.RailMaterials[railMaterial] // 0.0 if not found
	railInfCost := costs.RailInfills[railInfill]     // 0.0 if not found

	if railMaterial == "" && railInfill != "" {
		return 0, fmt.Errorf("rail infill requires a rail material")
	}

	return perimeter * (railMatCost + railInfCost), nil
}
