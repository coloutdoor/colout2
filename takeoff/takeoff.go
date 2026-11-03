// Package takeoff calculates a deck Materials Take Off list from an estimate.
// It has no database, web, or LLM dependencies, so every calculation can be
// tested directly and always gives the same answer.
package takeoff

import (
	"errors"
	"fmt"
	"math"
)

// ErrMultiSection is returned for estimates with more than one deck section.
var ErrMultiSection = errors.New("multi-section decks are not supported yet")

const (
	maxHeightFt     = 20.0
	maxProjectionFt = 20.0
	midspanOverFt   = 13.0
)

// Section is one rectangle of a deck.
type Section struct {
	Label        string
	ProjectionFt float64
	WidthFt      float64
}

// Input holds the estimate details the calculations need.
type Input struct {
	EstimateID string
	Material   string // Repeated in the output exactly as the estimate names it.
	HeightFt   float64
	Sections   []Section

	FasciaLF     float64 // 0 means no fascia.
	StairWidthFt float64 // 0 means no stairs.
	StairFascia  bool
	ToeKicks     bool
}

// Deck is one rectangle plus the values several lists share, calculated once.
type Deck struct {
	Material     string
	ProjectionFt float64
	WidthFt      float64
	HeightFt     float64
	Midspan      bool // Projection over 13' doubles footings and headers.
	Footings     int
	Pad          PadSpec
	Post         PostSpec

	Wood          bool
	JoistSize     string
	JoistLength   int
	JoistsBase    int
	JoistsOrdered int

	AreaSqFt float64
	FasciaLF float64

	HasStairs      bool
	StairWidthFt   float64
	Stringers      int // Per flight.
	StringerLength int
	Treads         int
	StairFascia    bool
	ToeKicks       bool

	// A deck over 12' with stairs gets a midspan landing and 2 flights.
	Landing           bool
	LandingHeightFt   float64
	LandingPostLength int
	LandingJoists     int
}

// Item is one row of a materials list.
type Item struct {
	Count       int
	Name        string
	Description string
	Notes       string
}

// List is one titled section of the take off.
type List struct {
	Title string
	Items []Item
}

// Result is the finished take off. When Stop is set the deck needs engineering
// and no lists are produced.
type Result struct {
	Deck  Deck
	Stop  string
	Lists []List
}

// NewDeck works out the shared values for a single-section estimate.
func NewDeck(in Input) (Deck, error) {
	if len(in.Sections) == 0 {
		return Deck{}, errors.New("estimate has no deck sections")
	}
	if len(in.Sections) > 1 {
		return Deck{}, ErrMultiSection
	}
	s := in.Sections[0]
	if s.ProjectionFt <= 0 || s.WidthFt <= 0 || in.HeightFt <= 0 {
		return Deck{}, fmt.Errorf("invalid deck size: %g x %g, %g high", s.ProjectionFt, s.WidthFt, in.HeightFt)
	}

	midspan := s.ProjectionFt > midspanOverFt
	wood := isWood(in.Material)
	base, ordered := joistCounts(s.WidthFt, wood)
	d := Deck{
		Material:      in.Material,
		ProjectionFt:  s.ProjectionFt,
		WidthFt:       s.WidthFt,
		HeightFt:      in.HeightFt,
		Midspan:       midspan,
		Footings:      footingCount(s.WidthFt, midspan),
		Pad:           padFor(in.HeightFt),
		Post:          postFor(in.HeightFt),
		Wood:          wood,
		JoistSize:     joistSizeFor(s.ProjectionFt),
		JoistLength:   joistLengthFor(s.ProjectionFt),
		JoistsBase:    base,
		JoistsOrdered: ordered,
		AreaSqFt:      s.ProjectionFt * s.WidthFt,
		FasciaLF:      in.FasciaLF,
	}
	if in.StairWidthFt > 0 {
		d.HasStairs = true
		d.StairWidthFt = in.StairWidthFt
		flightRise := in.HeightFt
		if NeedsStairLanding(in.HeightFt) {
			flightRise = in.HeightFt / 2
			d.Landing = true
			d.LandingHeightFt = flightRise
			d.LandingPostLength = postLength(flightRise)
			d.LandingJoists = landingJoistCount(in.StairWidthFt)
		}
		d.Stringers, d.StringerLength = stringerCounts(in.StairWidthFt, flightRise)
		d.Treads = treadCount(in.HeightFt)
		d.StairFascia = in.StairFascia
		d.ToeKicks = in.ToeKicks
	}
	return d, nil
}

// stopReason returns the engineering message for a deck over the limits.
func stopReason(d Deck) string {
	switch {
	case d.HeightFt > maxHeightFt:
		return fmt.Sprintf("ENGINEERING REQUIRED: deck height %g' is over %g'", d.HeightFt, maxHeightFt)
	case d.ProjectionFt > maxProjectionFt:
		return fmt.Sprintf("ENGINEERING REQUIRED: deck projection %g' is over %g'", d.ProjectionFt, maxProjectionFt)
	}
	return ""
}

// Calculate builds the take off. The rails list doesn't exist yet.
// Landing materials are mixed into the footings, framing, and decking lists.
func Calculate(in Input) (Result, error) {
	d, err := NewDeck(in)
	if err != nil {
		return Result{}, err
	}
	if stop := stopReason(d); stop != "" {
		return Result{Deck: d, Stop: stop}, nil
	}
	return Result{Deck: d, Lists: []List{CalcFootings(d), CalcFraming(d), CalcDecking(d)}}, nil
}

// roundUp2 returns the next 2' increment strictly greater than feet, so a post
// on a 6' deck is 8', not 6'.
func roundUp2(feet float64) int {
	return (int(math.Floor(feet/2)) + 1) * 2
}

// roundToNext2 rounds up to a 2' increment, leaving exact increments alone.
func roundToNext2(feet float64) int {
	return int(math.Ceil(feet/2)) * 2
}

// spanBoards returns the boards needed to cover a width: one board up to 20',
// otherwise equal boards. Lengths are multiples of 2', from 8' to 20'.
// Headers, ledgers, and rim joists all use it.
func spanBoards(widthFt float64) (count, lengthFt int) {
	count = int(math.Ceil(widthFt / 20))
	return count, max(minLumberLengthFt, roundToNext2(widthFt/float64(count)))
}

// ceilDiv divides, rounding up.
func ceilDiv(a, b int) int {
	return (a + b - 1) / b
}

// coverPacks covers need with the fewest packs. Sizes are ascending, and the
// result holds a count for each size: as many of the largest size as fit, then
// one pack of the smallest size that covers the rest.
func coverPacks(need int, sizes []int) []int {
	counts := make([]int, len(sizes))
	last := len(sizes) - 1
	counts[last] = need / sizes[last]
	rest := need % sizes[last]
	if rest == 0 {
		return counts
	}
	for i, size := range sizes {
		if rest <= size {
			counts[i]++
			break
		}
	}
	return counts
}

// fewestBoxes covers need with the fewest boxes of the two sizes sold.
func fewestBoxes(need, small, large int) (smallBoxes, largeBoxes int) {
	counts := coverPacks(need, []int{small, large})
	return counts[0], counts[1]
}

// screwBoxes returns the rows for a screw sold in 100 and 500 count boxes.
func screwBoxes(name string, need int) []Item {
	smallBoxes, largeBoxes := fewestBoxes(need, 100, 500)
	note := fmt.Sprintf("%d screws needed", need)
	var items []Item
	if smallBoxes > 0 {
		items = append(items, Item{smallBoxes, name + " screws, 100 count", "Box", note})
	}
	if largeBoxes > 0 {
		items = append(items, Item{largeBoxes, name + " screws, 500 count", "Box", note})
	}
	return items
}
