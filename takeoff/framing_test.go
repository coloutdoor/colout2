package takeoff

import (
	"reflect"
	"strings"
	"testing"
)

// TestFraming1031 checks every row against values worked out by hand from
// framing_rules.txt: 15' projection x 30' wide cedar deck, no stairs.
func TestFraming1031(t *testing.T) {
	res, err := calculateFile(t, "estimate_1031.txt")
	if err != nil {
		t.Fatal(err)
	}
	want := List{
		Title: "Framing",
		Items: []Item{
			{17, "2x10 x 16' PT joist", "", "16 + 1 extra"},
			{2, "2x10 x 16' PT ledger", "", ""},
			{2, "2x10 x 16' PT rim joist", "", ""},
			{15, "Simpson LUS210Z", "Joist hanger", "Joists ordered minus 2"},
			{2, "Simpson LUC210Z", "Joist hanger", "Outside joists"},
			{17, "Simpson H2.5", "Hurricane tie", "1 per joist ordered"},
			{2, "Simpson DTT1Z", "Deck tie", "2 per deck"},
			{1, `21° x 0.131" HDG collated framing nails`, "Box", ""},
			{1, `6.5" Skill Saw blade`, "", ""},
			{1, `7.25" Skill Saw blade`, "", ""},
			{1, `FastenMaster LedgerLok Flathead 5", 50 count`, "Box", "Ledger"},
			{1, `Simpson SD9 1.5" screws, 500 count`, "Box", "256 screws needed"},
			{1, `Simpson SD9 2.5" screws, 100 count`, "Box", "60 screws needed"},
		},
	}
	if got := res.Lists[1]; !reflect.DeepEqual(got, want) {
		t.Errorf("framing list\n got: %+v\nwant: %+v", got, want)
	}
}

// TestFramingMatrix checks the single-section decks in the agreed test plan.
func TestFramingMatrix(t *testing.T) {
	const noStairs, stairStop = "", "stop"
	tests := []struct {
		id        string
		joists    string
		count     int
		stringer  string // Name row, noStairs, or stairStop.
		stringers int
	}{
		{"1031", "2x10 x 16' PT joist", 17, noStairs, 0},
		{"1039", "2x8 x 12' PT joist", 4, "2x12 x 8' PT stringer", 4},
		{"1040", "2x6 x 10' PT joist", 24, stairStop, 0},
		{"1052", "2x8 x 12' PT joist", 9, "2x12 x 8' PT stringer", 6},
		{"1054", "2x8 x 12' PT joist", 5, "2x12 x 8' PT stringer", 4},
		{"c1", "2x10 x 14' PT joist", 17, stairStop, 0},
		{"c3", "2x8 x 12' PT joist", 9, stairStop, 0},
		{"c4", "2x10 x 14' PT joist", 8, "2x12 x 10' PT stringer", 4},
		{"c5", "2x10 x 14' PT joist", 20, "2x12 x 20' PT stringer", 5},
		{"c7", "2x10 x 16' PT joist", 30, noStairs, 0},
		{"c8", "2x10 x 18' PT joist", 13, "2x12 x 16' PT stringer", 4},
	}
	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			res, err := calculateFile(t, "estimate_"+tt.id+".txt")
			if err != nil {
				t.Fatal(err)
			}
			list := res.Lists[1]
			if got := list.Items[0]; got.Name != tt.joists || got.Count != tt.count {
				t.Errorf("joists = %d %s, want %d %s", got.Count, got.Name, tt.count, tt.joists)
			}

			var stringer *Item
			for i := range list.Items {
				if strings.Contains(list.Items[i].Name, "stringer") {
					stringer = &list.Items[i]
				}
			}
			switch tt.stringer {
			case noStairs, stairStop:
				if stringer != nil {
					t.Errorf("unexpected stringer row: %+v", *stringer)
				}
				if flagged := len(list.Flags) > 0; flagged != (tt.stringer == stairStop) {
					t.Errorf("flags = %v", list.Flags)
				}
			default:
				if stringer == nil || stringer.Name != tt.stringer || stringer.Count != tt.stringers {
					t.Errorf("stringer = %+v, want %d %s", stringer, tt.stringers, tt.stringer)
				}
				if len(list.Flags) != 0 {
					t.Errorf("flags = %v", list.Flags)
				}
			}
		})
	}
}

func TestJoistSizes(t *testing.T) {
	tests := []struct {
		projectionFt float64
		size         string
		lengthFt     int
	}{
		{3, "2x4", 8},
		{5.9, "2x4", 8},
		{6, "2x6", 8},
		{8, "2x6", 10},
		{8.5, "2x8", 10},
		{10, "2x8", 12},
		{10.5, "2x10", 12},
		{18, "2x10", 20},
		{19, "2x10", 20},
		{20, "2x10", 20},
	}
	for _, tt := range tests {
		size, length := joistSizeFor(tt.projectionFt), joistLengthFor(tt.projectionFt)
		if size != tt.size || length != tt.lengthFt {
			t.Errorf("projection %g: got %s x %d', want %s x %d'", tt.projectionFt, size, length, tt.size, tt.lengthFt)
		}
	}
}

func TestJoistCounts(t *testing.T) {
	tests := []struct {
		widthFt       float64
		wood          bool
		base, ordered int
	}{
		{30, true, 16, 17},
		{27, false, 22, 24},
		{5, true, 4, 4},
		{10, false, 9, 9},
		{36, false, 28, 30},
	}
	for _, tt := range tests {
		base, ordered := joistCounts(tt.widthFt, tt.wood)
		if base != tt.base || ordered != tt.ordered {
			t.Errorf("joistCounts(%g, %v) = %d, %d, want %d, %d", tt.widthFt, tt.wood, base, ordered, tt.base, tt.ordered)
		}
	}
}

// Stringers are deck height x 11 / 7, rounded up to a 2' increment.
func TestStringerLengths(t *testing.T) {
	tests := map[float64]int{4: 8, 5: 8, 6: 10, 7: 12, 9: 16, 12: 20}
	for heightFt, want := range tests {
		if _, got := stringerCounts(4, heightFt); got != want {
			t.Errorf("%.2f' deck: stringer %d', want %d'", heightFt, got, want)
		}
	}
}

func TestNeedsStairLanding(t *testing.T) {
	tests := map[float64]bool{5: false, 12: false, 12.1: true, 13: true, 20: true}
	for heightFt, want := range tests {
		if got := NeedsStairLanding(heightFt); got != want {
			t.Errorf("NeedsStairLanding(%.2f) = %v, want %v", heightFt, got, want)
		}
	}
}

func TestStringerConnector(t *testing.T) {
	res, err := calculateFile(t, "estimate_1039.txt")
	if err != nil {
		t.Fatal(err)
	}
	want := Item{4, "Simpson LSCZ", "Adjustable stair-stringer connector", "1 per stringer"}
	for _, item := range res.Lists[1].Items {
		if item == want {
			return
		}
	}
	t.Errorf("no row %+v in %+v", want, res.Lists[1].Items)
}

func TestIsWood(t *testing.T) {
	tests := map[string]bool{
		"Cedar": true, "Outdoor Wood": true, "TimberTech Prime": false,
		"Trex Transcend": false, "Azek": false, "TimberTech AZEK (cellular PVC)": false,
	}
	for material, want := range tests {
		if got := isWood(material); got != want {
			t.Errorf("isWood(%q) = %v, want %v", material, got, want)
		}
	}
}
