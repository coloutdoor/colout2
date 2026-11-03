package takeoff

import (
	"fmt"
	"math"
	"strings"
)

const (
	woodJoistSpacingIn      = 24.0
	compositeJoistSpacingIn = 16.0
	maxJoistLengthFt        = 20
	minLumberLengthFt       = 8
	maxStairRiseFt          = 12 // IBC: a flight may rise at most 12' between landings.
	stringerSpacingIn       = 16.0
	sd9ShortPerPiece        = 8 // SD9 1.5" per H2.5 and LUS.
	sd9LongPerHanger        = 4 // SD9 2.5" per LUS.
)

// isWood reports whether a material uses the wood rules. Every other material
// uses the composite rules, whatever its name.
func isWood(material string) bool {
	m := strings.ToLower(material)
	return strings.Contains(m, "cedar") || strings.Contains(m, "wood")
}

func joistSizeFor(projectionFt float64) string {
	switch {
	case projectionFt < 6:
		return "2x4"
	case projectionFt <= 8:
		return "2x6"
	case projectionFt <= 10:
		return "2x8"
	default:
		return "2x10"
	}
}

// joistLengthFor is longer than the projection, in 2' increments, from 8' to
// 20'. A 20' projection uses 20' joists.
func joistLengthFor(projectionFt float64) int {
	return min(maxJoistLengthFt, max(minLumberLengthFt, roundUp2(projectionFt)))
}

// joistCounts returns the joists the layout needs and the joists to order,
// which adds 10%, rounded down.
func joistCounts(widthFt float64, wood bool) (base, ordered int) {
	spacing := compositeJoistSpacingIn
	if wood {
		spacing = woodJoistSpacingIn
	}
	base = int(math.Ceil(widthFt*12/spacing)) + 1
	return base, base + base/10
}

// stringerCounts returns the stringers for a stair.
func stringerCounts(stairWidthFt, heightFt float64) (count, lengthFt int) {
	count = int(math.Ceil(stairWidthFt*12/stringerSpacingIn)) + 1
	return count, stringerLength(heightFt)
}

// stringerLength is deck height x 11 / 7 (a 7" rise and 11" run), rounded up
// to a 2' increment, minimum 8'.
func stringerLength(heightFt float64) int {
	return max(minLumberLengthFt, roundToNext2(heightFt*11/7))
}

// NeedsStairLanding reports whether stairs on a deck this high need a midspan
// landing. The IBC allows at most 12' of rise in one flight, so 12' is OK and
// anything higher needs one. The estimate pricing uses it too, so both follow
// one rule.
func NeedsStairLanding(heightFt float64) bool {
	return heightFt > maxStairRiseFt
}

// hangerSuffix turns a joist size into the Simpson code: 2x10 becomes 210.
func hangerSuffix(joistSize string) string {
	return strings.ReplaceAll(joistSize, "x", "")
}

// CalcFraming builds the framing list.
func CalcFraming(d Deck) List {
	ledgerBoards, ledgerLength := spanBoards(d.WidthFt)
	lus := d.JoistsOrdered - 2
	const luc, deckTies = 2, 2

	items := []Item{
		{d.JoistsOrdered, fmt.Sprintf("%s x %d' PT joist", d.JoistSize, d.JoistLength), "",
			fmt.Sprintf("%d + %d extra", d.JoistsBase, d.JoistsOrdered-d.JoistsBase)},
		{ledgerBoards, fmt.Sprintf("%s x %d' PT ledger", d.JoistSize, ledgerLength), "", ""},
		{ledgerBoards, fmt.Sprintf("%s x %d' PT rim joist", d.JoistSize, ledgerLength), "", ""},
	}

	var flags []string
	if d.HasStairs {
		if NeedsStairLanding(d.HeightFt) {
			flags = append(flags, "ENGINEERING REQUIRED: stairs need a midspan landing")
		} else {
			items = append(items,
				Item{d.Stringers, fmt.Sprintf("2x12 x %d' PT stringer", d.StringerLength), "", ""},
				Item{d.Stringers, "Simpson LSCZ", "Adjustable stair-stringer connector", "1 per stringer"},
			)
		}
	}

	suffix := hangerSuffix(d.JoistSize)
	items = append(items,
		Item{lus, "Simpson LUS" + suffix + "Z", "Joist hanger", "Joists ordered minus 2"},
		Item{luc, "Simpson LUC" + suffix + "Z", "Joist hanger", "Outside joists"},
		Item{d.JoistsOrdered, "Simpson H2.5", "Hurricane tie", "1 per joist ordered"},
		Item{deckTies, "Simpson DTT1Z", "Deck tie", "2 per deck"},
		Item{1, `21° x 0.131" HDG collated framing nails`, "Box", ""},
		Item{1, `6.5" Skill Saw blade`, "", ""},
		Item{1, `7.25" Skill Saw blade`, "", ""},
		Item{1, `FastenMaster LedgerLok Flathead 5", 50 count`, "Box", "Ledger"},
	)
	items = append(items, screwBoxes(`Simpson SD9 1.5"`, (d.JoistsOrdered+lus)*sd9ShortPerPiece)...)
	items = append(items, screwBoxes(`Simpson SD9 2.5"`, lus*sd9LongPerHanger)...)

	return List{Title: "Framing", Items: items, Flags: flags}
}
