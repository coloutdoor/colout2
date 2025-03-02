package main

import (
	"log"
	"strconv"
	"strings"
)

// formatCost formats a float64 cost with commas and $ prefix (e.g., $13,680.00).
func formatCost(cost float64) string {
	log.Printf("Debug formatCost is: %.2f", cost)
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
