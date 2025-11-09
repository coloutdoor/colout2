package main

import (
	"fmt"
	"html/template"
	"strconv"
	"strings"
)

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
	return fmt.Sprintf("Supply and install concrete footings "+
		"with premium pressure treated lumber. "+
		"Supply and install %.1f sq ft of %s deck. "+
		"Deck size apprimately  %.1f x %.1f ft, %.1f ft high.", de.DeckArea, material,
		de.Length, de.Width, de.Height)
}

// ***************************************************************************************************
// Format Demo Description
//
// * This returns a Template
// ***************************************************************************************************
func formatDemoDescription(de DeckEstimate) template.HTML {
	if de.DemoCost <= 0.0 {
		return template.HTML("Demo and removal of existing structure is not included.")
	}

	demodesc := "<p>" + template.HTMLEscapeString("Remove and dispose of the exsisting structures:") + "</p>"
	demodesc = fmt.Sprintf("%s "+" <p> * Wood or composite deck and wood frame %.1f sq ft </p>", demodesc, de.DeckArea)

	if de.RailCost <= 0.0 {
		demodesc = fmt.Sprintf("%s "+"<p> * Rail demo not included. </p>", demodesc)
	} else {
		demodesc = fmt.Sprintf("%s "+"<p> * Rail demo %.1f ln ft. </p>", demodesc, de.RailFeet)
	}

	if de.StairCost <= 0.0 {
		demodesc = fmt.Sprintf("%s "+"<p> * Stair demo not included. </p>", demodesc)
	} else {
		demodesc = fmt.Sprintf("%s "+"<p> * Stair and Rail demo %.1f ft high. </p>", demodesc, de.Height)
	}

	return template.HTML(demodesc)
}
