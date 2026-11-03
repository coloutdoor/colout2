package takeoff

import (
	"reflect"
	"strings"
	"testing"
)

func TestNeedsStairLanding(t *testing.T) {
	tests := map[float64]bool{5: false, 12: false, 12.1: true, 13: true, 20: true}
	for heightFt, want := range tests {
		if got := NeedsStairLanding(heightFt); got != want {
			t.Errorf("NeedsStairLanding(%.2f) = %v, want %v", heightFt, got, want)
		}
	}
}

// landingRows returns the rows whose description starts with "Stair landing".
func landingRows(items []Item) []Item {
	var out []Item
	for _, item := range items {
		if strings.HasPrefix(item.Description, "Stair landing") {
			out = append(out, item)
		}
	}
	return out
}

// TestLanding1040 is the owner's 4' x 4' landing: 13' deck, 4' stairs, so the
// landing is at 6.5'.
func TestLanding1040(t *testing.T) {
	res, err := calculateFile(t, "estimate_1040.txt")
	if err != nil {
		t.Fatal(err)
	}
	footings := []Item{
		{4, landingPierBlock, "Stair landing", ""},
		{4, "4x4 x 8' PT post", "Stair landing", "Landing at 6.5'"},
		{4, "Simpson PC4Z", "Stair landing post cap", "1 per post"},
		{1, "4x6 x 8' PT beam", "Stair landing", "Cut into 2 beams"},
	}
	if got := landingRows(res.Lists[0].Items); !reflect.DeepEqual(got, footings) {
		t.Errorf("footings landing rows\n got: %+v\nwant: %+v", got, footings)
	}
	framing := []Item{
		{4, "2x6 x 8' PT", "Stair landing joists and rims", "4 joists 4', 2 rims 4', 1 spare"},
		{8, "Simpson H2.5", "Stair landing hurricane tie", "2 per landing joist"},
	}
	if got := landingRows(res.Lists[1].Items); !reflect.DeepEqual(got, framing) {
		t.Errorf("framing landing rows\n got: %+v\nwant: %+v", got, framing)
	}

	// SD9 1.5": 4 ABU + 4 PC6Z + 4 landing PC4Z = 12 pieces x 8 in footings;
	// 24 + 8 landing H2.5 + 22 LUS = 54 pieces x 8 in framing.
	wantScrews := map[int]string{0: "96 screws needed", 1: "432 screws needed"}
	for list, note := range wantScrews {
		items := res.Lists[list].Items
		found := false
		for _, item := range items {
			if item.Name == `Simpson SD9 1.5" screws, 100 count` || item.Name == `Simpson SD9 1.5" screws, 500 count` {
				found = item.Notes == note
			}
		}
		if !found {
			t.Errorf("list %d: no SD9 1.5\" row with %q", list, note)
		}
	}
}

// TestLandingWood checks c3: 19' deck, 3.5' Outdoor Wood stairs, landing at
// 9.5', so 10' posts and 16' stringers.
func TestLandingWood(t *testing.T) {
	res, err := calculateFile(t, "estimate_c3.txt")
	if err != nil {
		t.Fatal(err)
	}
	if got := landingRows(res.Lists[0].Items)[1]; got.Name != "4x4 x 10' PT post" || got.Notes != "Landing at 9.5'" {
		t.Errorf("landing post = %+v, want 4x4 x 10' at 9.5'", got)
	}
	want := []Item{
		{1, `CAMO EDGE 2-3/8" deck screws, 1750 count`, "Box", "274 sq ft with stair treads and landing"},
		{16, "2x6 x 16' Outdoor Wood stair tread board", "", "31 treads, 2 boards per tread"},
		{3, "2x6 x 16' Outdoor Wood deck board", "Stair landing", "9 rows cut to 3.5'"},
	}
	if got := res.Lists[2].Items[1:]; !reflect.DeepEqual(got, want) {
		t.Errorf("decking rows\n got: %+v\nwant: %+v", got, want)
	}
}

// The landing scales with stair width: 4' joists at 16" o.c. (2 per 8' board),
// 2 rims of the stair width, 1 spare board, and a 4x6 that covers 2 beams.
func TestLandingWidths(t *testing.T) {
	tests := []struct {
		widthFt    float64
		joists     int
		boards2x6  int
		beamLength string
	}{
		{3, 4, 4, "4x6 x 8' PT beam"},
		{4, 4, 4, "4x6 x 8' PT beam"},
		{5, 5, 6, "4x6 x 10' PT beam"},
		{6, 6, 6, "4x6 x 12' PT beam"},
	}
	for _, tt := range tests {
		d := Deck{StairWidthFt: tt.widthFt, LandingJoists: landingJoistCount(tt.widthFt), LandingPostLength: 8}
		if d.LandingJoists != tt.joists {
			t.Errorf("%g' stairs: %d joists, want %d", tt.widthFt, d.LandingJoists, tt.joists)
		}
		if boards, _ := landingFraming2x6(d); boards != tt.boards2x6 {
			t.Errorf("%g' stairs: %d 2x6 boards, want %d", tt.widthFt, boards, tt.boards2x6)
		}
		if got := landingFootingItems(d)[3].Name; got != tt.beamLength {
			t.Errorf("%g' stairs: beam %q, want %q", tt.widthFt, got, tt.beamLength)
		}
	}
}
