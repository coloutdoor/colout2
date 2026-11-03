package takeoff

import (
	"fmt"
	"math"
)

const (
	maxFootingGapFt = 10.0
	overhangFt      = 2.0 // Maximum header overhang at each end.
	screwsPerPiece  = 8
)

// PadSpec is the concrete footing for a deck height.
type PadSpec struct {
	Inches int // Square pad size. All pads are 12" deep.
	Bags   int // 80# bags per footing.
}

// PostSpec is the post and matching hardware for a deck height.
type PostSpec struct {
	Size   string
	Base   string
	Cap    string
	Anchor string
	Header string
}

// footingCount spaces footings a maximum of 10' apart with up to 2' of overhang
// at each end, and doubles them for the midspan header.
func footingCount(widthFt float64, midspan bool) int {
	count := int(math.Ceil((widthFt-2*overhangFt)/maxFootingGapFt)) + 1
	if count < 2 {
		count = 2
	}
	if midspan {
		count *= 2
	}
	return count
}

func padFor(heightFt float64) PadSpec {
	switch {
	case heightFt < 6:
		return PadSpec{Inches: 16, Bags: 3}
	case heightFt <= 12:
		return PadSpec{Inches: 20, Bags: 5}
	default:
		return PadSpec{Inches: 24, Bags: 7}
	}
}

func postFor(heightFt float64) PostSpec {
	switch {
	case heightFt < 6:
		return PostSpec{Size: "4x4", Base: "ABU44Z", Cap: "PC4Z", Anchor: `1/2" x 5"`, Header: "4x10"}
	case heightFt <= 10:
		return PostSpec{Size: "4x6", Base: "ABU46Z", Cap: "PC4Z", Anchor: `1/2" x 5"`, Header: "4x10"}
	default:
		return PostSpec{Size: "6x6", Base: "ABU66Z", Cap: "PC6Z", Anchor: `5/8" x 5"`, Header: "6x10"}
	}
}

// postLength is longer than the deck height, in 2' increments, minimum 8'.
func postLength(heightFt float64) int {
	return max(8, roundUp2(heightFt))
}

// CalcFootings builds the footings and posts list.
func CalcFootings(d Deck) List {
	headerBoards, headerLength := spanBoards(d.WidthFt)
	headerNote := ""
	if d.Midspan {
		headerBoards *= 2
		headerNote = "Doubled for midspan header"
	}

	screws := (d.Footings + d.Footings) * screwsPerPiece // One base and one cap per post.

	items := []Item{
		{d.Footings, "Footing", fmt.Sprintf(`%d" x %d" x 12" deep`, d.Pad.Inches, d.Pad.Inches), ""},
		{d.Footings * d.Pad.Bags, "80# concrete bags", "", fmt.Sprintf("%d per footing", d.Pad.Bags)},
		{d.Footings, "2x4 x 8' Doug Fir", "Footing frame", "1 per footing"},
		{d.Footings, fmt.Sprintf("%s x %d' PT post", d.Post.Size, postLength(d.HeightFt)), "", ""},
		{d.Footings, "Simpson " + d.Post.Base, "Post base", ""},
		{d.Footings, "Simpson " + d.Post.Cap, "Post cap", ""},
		{d.Footings, d.Post.Anchor + " wedge anchor", "", "1 per post"},
		{headerBoards, fmt.Sprintf("%s x %d' PT header", d.Post.Header, headerLength), "", headerNote},
	}
	items = append(items, screwBoxes(`Simpson SD9 1.5"`, screws)...)
	return List{Title: "Footings & Posts", Items: items}
}
