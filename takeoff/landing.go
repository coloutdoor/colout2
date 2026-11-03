package takeoff

import (
	"fmt"
	"math"
)

// A midspan stair landing is 4' deep x the stair width, at half the deck
// height, and splits the stairs into 2 flights. It stands on its own pier
// blocks and 4x4 posts, with 2 4x6 beams and 2x6 joists and rims.
const (
	maxStairRiseFt        = 12 // IBC: a flight may rise at most 12' between landings.
	landingDepthFt        = 4.0
	landingPosts          = 4
	landingJoistSpacingIn = 16.0
	landingBoardFt        = 8 // 2x6 joists and rims are cut from 8' boards.
	landingSpareBoards    = 1
	landingTiesPerJoist   = 2 // H2.5 ties, one at each beam.
	landingPierBlock      = `11" x 11-1/2" x 11-1/2" concrete pier block with metal bracket`
)

// NeedsStairLanding reports whether stairs on a deck this high need a midspan
// landing. The IBC allows at most 12' of rise in one flight, so 12' is OK and
// anything higher needs one. The estimate pricing uses it too, so both follow
// one rule.
func NeedsStairLanding(heightFt float64) bool {
	return heightFt > maxStairRiseFt
}

// landingJoistCount is the 4' joists across the stair width at 16" o.c.,
// including both outside joists.
func landingJoistCount(stairWidthFt float64) int {
	return int(math.Ceil(stairWidthFt*12/landingJoistSpacingIn)) + 1
}

// landingFraming2x6 is the 2x6 x 8' boards for the landing: 2 4' joists per
// board, 2 rims of the stair width, and 1 spare board.
func landingFraming2x6(d Deck) (boards int, note string) {
	joistBoards := ceilDiv(d.LandingJoists, landingBoardFt/int(landingDepthFt))
	rimsPerBoard := max(1, int(math.Floor(landingBoardFt/d.StairWidthFt)))
	rimBoards := ceilDiv(2, rimsPerBoard)
	note = fmt.Sprintf("%d joists 4', 2 rims %g', %d spare", d.LandingJoists, d.StairWidthFt, landingSpareBoards)
	return joistBoards + rimBoards + landingSpareBoards, note
}

// landingFootingItems are the landing's rows in the footings list, before the
// screw rows. Its PC4Z caps add to the SD9 count there.
func landingFootingItems(d Deck) []Item {
	beamLength := max(minLumberLengthFt, roundToNext2(2*d.StairWidthFt))
	return []Item{
		{landingPosts, landingPierBlock, "Stair landing", ""},
		{landingPosts, fmt.Sprintf("4x4 x %d' PT post", d.LandingPostLength), "Stair landing",
			fmt.Sprintf("Landing at %g'", d.LandingHeightFt)},
		{landingPosts, "Simpson PC4Z", "Stair landing post cap", "1 per post"},
		{1, fmt.Sprintf("4x6 x %d' PT beam", beamLength), "Stair landing", "Cut into 2 beams"},
	}
}

// landingFramingItems are the landing's joist, rim, and tie rows in the
// framing list. Its H2.5 ties add to the SD9 count there.
func landingFramingItems(d Deck) []Item {
	boards, note := landingFraming2x6(d)
	return []Item{
		{boards, fmt.Sprintf("2x6 x %d' PT", landingBoardFt), "Stair landing joists and rims", note},
		{d.LandingJoists * landingTiesPerJoist, "Simpson H2.5", "Stair landing hurricane tie", "2 per landing joist"},
	}
}

// landingDeckBoards covers the landing with 16' boards cut to the stair width,
// the same stock as the stair treads.
func landingDeckBoards(d Deck, coverageIn float64) (boards, rows int) {
	rows = int(math.Ceil(landingDepthFt * 12 / coverageIn))
	return ceilDiv(rows, piecesPerBoard(treadBoardFt, d.StairWidthFt)), rows
}

// landingAreaSqFt is the landing deck area, added to the screw and clip
// coverage.
func landingAreaSqFt(d Deck) float64 {
	if !d.Landing {
		return 0
	}
	return landingDepthFt * d.StairWidthFt
}
