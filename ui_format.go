package main

import (
	"fmt"
	"strconv"
	"strings"
)

type LineItem struct {
	Name          string // e.g., "Decking", "Rails", "Demo"
	Description   string
	Cost          float64
	FormattedCost string
}

// Update the helper to accept the new 'name' argument
func newLineItem(name, desc string, cost float64) LineItem {
	return LineItem{
		Name:          name,
		Description:   desc,
		Cost:          cost,
		FormattedCost: formatCost(cost),
	}
}

// formatCost formats a float64 cost with commas and $ prefix (e.g., $13,680.00).
func formatCost(cost float64) string {
	str := strconv.FormatFloat(cost, 'f', 2, 64) // e.g., "13680.00"
	parts := strings.Split(str, ".")
	intPart := parts[0]
	decPart := parts[1]
	var withCommas string
	for i, digit := range intPart {
		if i > 0 && (len(intPart)-i)%3 == 0 {
			withCommas += ","
		}
		withCommas += string(digit)
	}
	return "$" + withCommas + "." + decPart
}

// formatDeckDescription formats the deck description from DeckEstimate fields.
func formatDeckDescription(de DeckEstimate) string {
	material := ""
	switch de.Material {
	case "outdoorWood":
		material = "Outdoor Wood"
	case "cedar":
		material = "Cedar"
	case "timberTechPrime":
		material = "TimberTech Prime"
	case "timberTechProReserve":
		material = "TimberTech Pro Reserve"
	case "timberTechProLegacy":
		material = "TimberTech Pro Legacy"
	}
	desc := fmt.Sprintf("Supply and install concrete footings with premium pressure treated lumber. "+
		"Supply and install %.1f sq ft of %s deck. %.1f ft high.",
		de.DeckArea, material, de.Height)
	for _, s := range de.Sections {
		desc += fmt.Sprintf("\n  — %s: %.1f × %.1f ft (%.1f sf)",
			s.Label, s.Length, s.Width, s.Length*s.Width)
	}
	return desc
}

// ***************************************************************************************************
// Format Demo Description
//
// * This returns a Template
// ***************************************************************************************************
func formatDemoDescription(de DeckEstimate) string {
	if de.DemoCost <= 0.0 {
		return "Demo and removal of existing structure is not included."
	}

	demodesc := "Remove and dispose of the existing structures."
	demodesc = fmt.Sprintf("%s "+" * Wood or composite deck and wood frame %.1f sq ft", demodesc, de.DeckArea)

	if de.RailCost <= 0.0 {
		demodesc = fmt.Sprintf("%s "+" Rail demo not included.", demodesc)
	} else {
		demodesc = fmt.Sprintf("%s "+" Rail demo %.1f ln ft. ", demodesc, de.RailFeet)
	}

	if de.StairCost <= 0.0 {
		demodesc = fmt.Sprintf("%s "+" Stair demo not included. ", demodesc)
	} else {
		demodesc = fmt.Sprintf("%s "+" Stair and Rail demo %.1f ft high.", demodesc, de.Height)
	}

	return demodesc
}

// formatRailDescription
func formatRailDescription(de DeckEstimate) string {
	desc := "Deck rails not included"
	if de.RailCost > 0.0 {
		desc = fmt.Sprintf("Supply and install %s rail posts and top rail with %s infill. Rails approximately %.1f lineal ft",
			de.RailMaterial, de.RailInfill, de.RailFeet)
	}
	return desc
}

// formatStairDescription
func formatStairDescription(de DeckEstimate) string {
	desc := "Stairs not included"
	if de.StairCost > 0.0 {
		desc = fmt.Sprintf(`Supply and install premium pressure treated stair framing at %.1f ft wide. 
                        Stair treads approximately 11" per step with matching %s decking on treads
                        Total rise of stairs is %.1f ft.`, de.StairWidth, de.Material, de.Height)
	}
	return desc
}

// formatFasciaDescription
func formatFasciaDescription(de DeckEstimate) string {
	desc := "Deck fascia not included"
	if de.FasciaCost > 0.0 {
		desc = fmt.Sprintf("Supply and install fascia to match deck material approximately %.1f lineal ft", de.FasciaFeet)
	}
	return desc
}

// formatStairRailDescription
func formatStairRailDescription(de DeckEstimate) string {
	desc := "Stair Rails not included"
	if de.StairRailCost > 0.0 {
		// 1. Determine the text based on the count first
		railSideText := "matching stair rail - one side only"
		if de.StairRailCount > 1.0 {
			railSideText = "matching stair rails on both sides"
		}

		// 2. Build the final string
		desc = fmt.Sprintf("Supply and install %s with %s rail posts and top rail with %s infill.",
			railSideText,
			de.RailMaterial,
			de.RailInfill,
		)

	}
	return desc
}

// formatStairFasciaDescription
func formatStairFasciaDescription(de DeckEstimate) string {
	// Default to the negative case
	desc := "Stair fascia not included"

	// If cost is non-zero (Go template 'if' treats 0 as false)
	if de.StairFasciaCost > 0.0 {
		desc = "Add matching stair fascia to stairs"
	}
	return desc
}

// formatStairTKDescription
func formatStairTKDescription(de DeckEstimate) string {
	desc := "No toe kicks.  Open."
	if de.HasStairTK {
		desc = "Add matching toe kicks to stairs"
	}
	return desc
}
