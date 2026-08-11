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
	var desc string
	switch de.DIYMode {
	case 1:
		desc = fmt.Sprintf("DIY materials only — concrete footings, pressure treated framing, and %.1f sq ft of %s decking. %.1f ft high.",
			de.DeckArea, material, de.Height)
	case 2:
		desc = fmt.Sprintf("Plans only — professional design drawings for %.1f sq ft of %s deck at %.1f ft high. Materials and installation not included.",
			de.DeckArea, material, de.Height)
	default:
		desc = fmt.Sprintf("Supply and install concrete footings with premium pressure treated lumber. "+
			"Supply and install %.1f sq ft of %s deck. %.1f ft high.",
			de.DeckArea, material, de.Height)
	}
	for _, s := range de.Sections {
		desc += fmt.Sprintf("\n  — %s: %.1f proj × %.1f ft wide (%.1f sf)",
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
	if de.DIYMode == 1 || de.DIYMode == 2 {
		return "Homeowner is responsible for demo and disposal of any existing structure."
	}
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
	if de.RailMaterial != "" {
		switch de.DIYMode {
		case 1:
			desc = fmt.Sprintf("DIY materials only — %s rail posts, top rail, and %s infill. Approximately %.1f lineal ft.",
				de.RailMaterial, de.RailInfill, de.RailFeet)
		case 2:
			desc = fmt.Sprintf("Plans include %s rail with %s infill, approximately %.1f lineal ft. Materials not included.",
				de.RailMaterial, de.RailInfill, de.RailFeet)
		default:
			if de.RailCost > 0 {
				desc = fmt.Sprintf("Supply and install %s rail posts and top rail with %s infill. Rails approximately %.1f lineal ft",
					de.RailMaterial, de.RailInfill, de.RailFeet)
			}
		}
	}
	return desc
}

// formatStairDescription
func formatStairDescription(de DeckEstimate) string {
	desc := "Stairs not included"
	if de.StairWidth > 0 {
		switch de.DIYMode {
		case 1:
			desc = fmt.Sprintf(`DIY materials only — pressure treated stair framing, %.1f ft wide with matching %s treads. Total rise %.1f ft.`,
				de.StairWidth, de.Material, de.Height)
		case 2:
			desc = fmt.Sprintf("Plans include stairs at %.1f ft wide with %s treads, %.1f ft total rise. Materials not included.",
				de.StairWidth, de.Material, de.Height)
		default:
			if de.StairCost > 0 {
				desc = fmt.Sprintf(`Supply and install premium pressure treated stair framing at %.1f ft wide.
                        Stair treads approximately 11" per step with matching %s decking on treads
                        Total rise of stairs is %.1f ft.`, de.StairWidth, de.Material, de.Height)
			}
		}
	}
	return desc
}

// formatFasciaDescription
func formatFasciaDescription(de DeckEstimate) string {
	desc := "Deck fascia not included"
	if de.HasFascia {
		switch de.DIYMode {
		case 1:
			desc = fmt.Sprintf("DIY materials only — fascia to match deck material, approximately %.1f lineal ft.", de.FasciaFeet)
		case 2:
			desc = fmt.Sprintf("Plans include fascia to match deck material, approximately %.1f lineal ft. Materials not included.", de.FasciaFeet)
		default:
			if de.FasciaCost > 0 {
				desc = fmt.Sprintf("Supply and install fascia to match deck material approximately %.1f lineal ft", de.FasciaFeet)
			}
		}
	}
	return desc
}

// formatStairRailDescription
func formatStairRailDescription(de DeckEstimate) string {
	desc := "Stair Rails not included"
	if de.StairWidth > 0 && de.RailMaterial != "" {
		side := "one side only"
		sidePhrase := "matching stair rail - one side only"
		if de.StairRailCount > 1.0 {
			side = "both sides"
			sidePhrase = "matching stair rails on both sides"
		}
		switch de.DIYMode {
		case 1:
			desc = fmt.Sprintf("DIY materials only — %s rail posts, top rail, and %s infill. Stair rails %s.",
				de.RailMaterial, de.RailInfill, side)
		case 2:
			desc = fmt.Sprintf("Plans include stair rails %s — %s with %s infill. Materials not included.",
				side, de.RailMaterial, de.RailInfill)
		default:
			if de.StairRailCost > 0 {
				desc = fmt.Sprintf("Supply and install %s with %s rail posts and top rail with %s infill.",
					sidePhrase, de.RailMaterial, de.RailInfill)
			}
		}
	}
	return desc
}

// formatStairFasciaDescription
func formatStairFasciaDescription(de DeckEstimate) string {
	desc := "Stair fascia not included"
	if de.HasStairFascia {
		switch de.DIYMode {
		case 1:
			desc = "DIY materials only — matching stair fascia."
		case 2:
			desc = "Plans include matching stair fascia. Materials not included."
		default:
			desc = "Add matching stair fascia to stairs"
		}
	}
	return desc
}

// formatStairTKDescription
func formatStairTKDescription(de DeckEstimate) string {
	desc := "No toe kicks.  Open."
	if de.HasStairTK {
		switch de.DIYMode {
		case 1:
			desc = "DIY materials only — matching toe kicks."
		case 2:
			desc = "Plans include matching toe kicks. Materials not included."
		default:
			desc = "Add matching toe kicks to stairs"
		}
	}
	return desc
}

// formatPermitDescription returns a scope description based on the selected permit tier.
func formatPermitDescription(de DeckEstimate) string {
	switch de.PermitLevel {
	case 1:
		return "Professional architectural design and material takeoff list included. Engineering not included but may be required for your project."
	case 2:
		return "Professional architectural design, structural engineering, and material takeoff list included. Permits not included but may be required for your project."
	case 3:
		return "Professional architectural design, structural engineering, permit application, and material takeoff list included."
	default:
		if de.DIYMode == 1 || de.DIYMode == 2 {
			return "Complete material takeoff list included. Design, engineering, and permits not included."
		}
		return "Design, engineering, and permits not included. May be required for your project."
	}
}
