package takeoff

import (
	"fmt"
	"math"
	"strings"
)

const (
	woodBoardCoverageIn      = 5.5
	compositeBoardCoverageIn = 5.25
	treadRiseIn              = 7.5
	treadBoardFt             = 16
	boardsPerTread           = 2
	fasciaBoardFt            = 12
	stairFasciaSides         = 2
	woodToeKicksPerBoard     = 2
	fasciaScrewsLFPerBox     = 100
	trimScrewsPerLF          = 2
	cedarGrade               = "#2 Tight Knot"
)

var (
	cedarLengths     = []int{8, 10, 12, 16, 20}
	compositeLengths = []int{12, 16, 20}
)

// treadCount is the number of treads for a deck height.
func treadCount(heightFt float64) int {
	return int(math.Ceil(heightFt * 12 / treadRiseIn))
}

// piecesPerBoard is how many stair-width pieces one board gives.
func piecesPerBoard(boardFt int, stairWidthFt float64) int {
	return max(1, int(math.Floor(float64(boardFt)/stairWidthFt)))
}

// lengthAtLeast returns the shortest stock length that covers feet, or the
// longest stock length if none does.
func lengthAtLeast(lengths []int, feet float64) int {
	for _, length := range lengths {
		if float64(length) >= feet {
			return length
		}
	}
	return lengths[len(lengths)-1]
}

// boardRun is the deck board order for a deck.
type boardRun struct {
	Boards   int
	LengthFt int
	Note     string
}

// woodBoards uses 1 board per row up to 20' wide and 2 boards per row wider.
func woodBoards(projectionFt, widthFt float64) boardRun {
	rows := int(math.Ceil(projectionFt * 12 / woodBoardCoverageIn))
	perRow := int(math.Ceil(widthFt / 20))
	return boardRun{
		Boards:   rows * perRow,
		LengthFt: lengthAtLeast(cedarLengths, widthFt/float64(perRow)),
		Note:     fmt.Sprintf("%d rows x %d per row", rows, perRow),
	}
}

// compositeBoards picks the one board length with the least waste per board.
// A tie goes to the fewest boards.
func compositeBoards(projectionFt, widthFt float64) boardRun {
	rows := int(math.Ceil(projectionFt * 12 / compositeBoardCoverageIn))
	var best boardRun
	bestWaste := math.Inf(1)
	for _, length := range compositeLengths {
		feet := float64(length)
		var run boardRun
		var waste float64
		if feet >= widthFt {
			perBoard := piecesPerBoard(length, widthFt)
			waste = feet - float64(perBoard)*widthFt
			run = boardRun{Boards: ceilDiv(rows, perBoard), LengthFt: length,
				Note: fmt.Sprintf("%d rows, %d per board", rows, perBoard)}
		} else {
			perRow := int(math.Ceil(widthFt / feet))
			waste = feet - widthFt/float64(perRow)
			run = boardRun{Boards: rows * perRow, LengthFt: length,
				Note: fmt.Sprintf("%d rows x %d per row", rows, perRow)}
		}
		if waste < bestWaste || (waste == bestWaste && run.Boards < best.Boards) {
			best, bestWaste = run, waste
		}
	}
	return best
}

// dividerLength is the shortest composite board longer than the projection.
// A 20' projection uses a 20' board, the same as joists.
func dividerLength(projectionFt float64) int {
	for _, length := range compositeLengths {
		if float64(length) > projectionFt {
			return length
		}
	}
	return compositeLengths[len(compositeLengths)-1]
}

// fasciaBoards is the board count for a fascia run, using 12' boards.
func fasciaBoards(lf float64) int {
	return int(math.Ceil(lf/fasciaBoardFt)) + 1
}

// woodFasciaSize matches the rim joist: a 2x8 rim uses 1x8 fascia.
func woodFasciaSize(joistSize string) string {
	return strings.Replace(joistSize, "2x", "1x", 1)
}

// CalcDecking builds the deck boards and trim list.
func CalcDecking(d Deck) List {
	if d.Wood {
		return List{Title: "Decking & Trim", Items: woodDecking(d)}
	}
	return List{Title: "Decking & Trim", Items: compositeDecking(d)}
}

func woodDecking(d Deck) []Item {
	run := woodBoards(d.ProjectionFt, d.WidthFt)
	items := []Item{{run.Boards, fmt.Sprintf("2x6 x %d' %s deck board", run.LengthFt, d.Material), "", run.Note}}

	// CAMO boxes are sold by coverage: 100, 200, and 500 sq ft.
	// Wood stair treads use the same screws as the deck boards.
	sqft := int(math.Ceil(d.AreaSqFt))
	note := fmt.Sprintf("%d sq ft", sqft)
	if d.HasStairs {
		treadArea := float64(d.Treads) * d.StairWidthFt * boardsPerTread * woodBoardCoverageIn / 12
		sqft = int(math.Ceil(d.AreaSqFt + treadArea))
		note = fmt.Sprintf("%d sq ft with stair treads", sqft)
	}
	boxes := coverPacks(sqft, []int{100, 200, 500})
	for i, count := range []string{"350", "700", "1750"} {
		if boxes[i] > 0 {
			items = append(items, Item{boxes[i], `CAMO EDGE 2-3/8" deck screws, ` + count + " count", "Box", note})
		}
	}

	if d.FasciaLF > 0 {
		size := woodFasciaSize(d.JoistSize)
		items = append(items, Item{fasciaBoards(d.FasciaLF),
			fmt.Sprintf("%s x %d' %s cedar fascia", size, fasciaBoardFt, cedarGrade), "",
			fmt.Sprintf("%g LF, matches rim joist", d.FasciaLF)})
	}
	if !d.HasStairs {
		return append(items, trimScrews(d.FasciaLF)...)
	}

	boards := ceilDiv(d.Treads*boardsPerTread, piecesPerBoard(treadBoardFt, d.StairWidthFt))
	items = append(items, Item{boards, fmt.Sprintf("2x6 x %d' %s stair tread board", treadBoardFt, d.Material), "",
		fmt.Sprintf("%d treads, 2 boards per tread", d.Treads)})
	cedarLF := d.FasciaLF
	if d.StairFascia {
		lf := stairFasciaLF(d.StringerLength)
		cedarLF += lf
		items = append(items, Item{stairFasciaBoards(lf),
			fmt.Sprintf("1x12 x %d' %s cedar stair side fascia", fasciaBoardFt, cedarGrade), "",
			fmt.Sprintf("%g LF, 2 sides", lf)})
	}
	if d.ToeKicks {
		cedarLF += float64(d.Treads) * d.StairWidthFt
		length := lengthAtLeast(cedarLengths, 2*d.StairWidthFt)
		items = append(items, Item{ceilDiv(d.Treads, woodToeKicksPerBoard),
			fmt.Sprintf("1x8 x %d' %s cedar toe kick", length, cedarGrade), "",
			fmt.Sprintf("%d treads, 2 per board", d.Treads)})
	}
	return append(items, trimScrews(cedarLF)...)
}

// trimScrews returns the rows for fastening 1x cedar: deck fascia, stair side
// fascia, and toe kicks.
func trimScrews(cedarLF float64) []Item {
	if cedarLF <= 0 {
		return nil
	}
	need := int(math.Ceil(cedarLF * trimScrewsPerLF))
	small, large := fewestBoxes(need, 150, 770)
	const name = `#7 x 2" Simpson Strong-Tie #1 Square Drive Trim Head Deck Screws`
	note := fmt.Sprintf("%g LF of 1x cedar, %d screws", cedarLF, need)
	var items []Item
	if small > 0 {
		items = append(items, Item{small, name + ", 1 lb box (150)", "Box", note})
	}
	if large > 0 {
		items = append(items, Item{large, name + ", 5 lb box (770)", "Box", note})
	}
	return items
}

func compositeDecking(d Deck) []Item {
	run := compositeBoards(d.ProjectionFt, d.WidthFt)
	items := []Item{{run.Boards, fmt.Sprintf("%d' %s grooved deck board", run.LengthFt, d.Material), "", run.Note}}

	sqft := int(math.Ceil(d.AreaSqFt))
	clips450, clips900 := fewestBoxes(sqft, 250, 500)
	note := fmt.Sprintf("%d sq ft", sqft)
	if clips450 > 0 {
		items = append(items, Item{clips450, "Hidden deck clips, 450 pack", "Grooved boards", note})
	}
	if clips900 > 0 {
		items = append(items, Item{clips900, "Hidden deck clips, 900 pack", "Grooved boards", note})
	}

	s4sLF := 0
	if d.WidthFt > 20 {
		length := dividerLength(d.ProjectionFt)
		s4sLF += length
		items = append(items, Item{1, fmt.Sprintf("%d' %s S4S divider board", length, d.Material), "", "Deck over 20' wide"})
	}
	if d.HasStairs {
		boards := ceilDiv(d.Treads*boardsPerTread, piecesPerBoard(treadBoardFt, d.StairWidthFt))
		s4sLF += boards * treadBoardFt
		items = append(items, Item{boards, fmt.Sprintf("%d' %s S4S stair tread board", treadBoardFt, d.Material), "",
			fmt.Sprintf("%d treads, 2 boards per tread", d.Treads)})
	}
	if s4sLF > 0 {
		small, large := fewestBoxes(s4sLF, 100, 300)
		cortexNote := fmt.Sprintf("%d LF of S4S", s4sLF)
		if small > 0 {
			items = append(items, Item{small, "TimberTech Cortex color match, 100 LF", "S4S fasteners", cortexNote})
		}
		if large > 0 {
			items = append(items, Item{large, "TimberTech Cortex color match, 300 LF", "S4S fasteners", cortexNote})
		}
	}

	fasciaLF := d.FasciaLF
	if d.FasciaLF > 0 {
		items = append(items, Item{fasciaBoards(d.FasciaLF),
			fmt.Sprintf(`12" x %d' %s fascia`, fasciaBoardFt, d.Material), "", fmt.Sprintf("%g LF", d.FasciaLF)})
	}
	if d.HasStairs && d.StairFascia {
		lf := stairFasciaLF(d.StringerLength)
		fasciaLF += lf
		items = append(items, Item{stairFasciaBoards(lf),
			fmt.Sprintf(`12" x %d' %s stair side fascia`, fasciaBoardFt, d.Material), "",
			fmt.Sprintf("%g LF, 2 sides", lf)})
	}
	if d.HasStairs && d.ToeKicks {
		fasciaLF += float64(d.Treads) * d.StairWidthFt
		perBoard := piecesPerBoard(fasciaBoardFt, d.StairWidthFt)
		items = append(items, Item{ceilDiv(d.Treads, perBoard),
			fmt.Sprintf(`7" x %d' %s toe kick`, fasciaBoardFt, d.Material), "",
			fmt.Sprintf("%d treads, %d per board", d.Treads, perBoard)})
	}
	if fasciaLF > 0 {
		items = append(items, Item{int(math.Ceil(fasciaLF / fasciaScrewsLFPerBox)),
			"Starborn Fascia System screws, 100 count", "Box", fmt.Sprintf("%g LF of fascia", fasciaLF)})
	}
	return items
}

// stairFasciaLF is the stringer length on each side of the stair.
func stairFasciaLF(stringerLengthFt int) float64 {
	return float64(stringerLengthFt * stairFasciaSides)
}

// stairFasciaBoards covers the stair fascia LF with 12' boards, with no extra
// board, unlike deck fascia.
func stairFasciaBoards(lf float64) int {
	return int(math.Ceil(lf / fasciaBoardFt))
}
