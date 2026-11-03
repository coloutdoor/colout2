package takeoff

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func deckingItems(t *testing.T, id string) []Item {
	t.Helper()
	res, err := calculateFile(t, "estimate_"+id+".txt")
	if err != nil {
		t.Fatal(err)
	}
	return res.Lists[2].Items
}

// TestDecking1031 checks every row against decking_rules.txt: 15' x 30' cedar
// deck, 450 sq ft, no fascia, no stairs.
func TestDecking1031(t *testing.T) {
	want := []Item{
		{66, "2x6 x 16' Cedar deck board", "", "33 rows x 2 per row"},
		{1, `CAMO EDGE 2-3/8" deck screws, 1750 count`, "Box", "450 sq ft"},
	}
	if got := deckingItems(t, "1031"); !reflect.DeepEqual(got, want) {
		t.Errorf("decking list\n got: %+v\nwant: %+v", got, want)
	}
}

// TestDecking1040 is the composite list with every stair option: 8' x 27',
// 13' high, 4' stairs, 39 LF of fascia.
func TestDecking1040(t *testing.T) {
	want := []Item{
		{38, "16' TimberTech Pro Legacy grooved deck board", "", "19 rows x 2 per row"},
		{1, "Hidden deck clips, 450 pack", "Grooved boards", "216 sq ft"},
		{1, "12' TimberTech Pro Legacy S4S divider board", "", "Deck over 20' wide"},
		{11, "16' TimberTech Pro Legacy S4S stair tread board", "", "21 treads, 2 boards per tread"},
		{1, "TimberTech Cortex color match, 300 LF", "S4S fasteners", "188 LF of S4S"},
		{5, `12" x 12' TimberTech Pro Legacy fascia`, "", "39 LF"},
		{4, `12" x 12' TimberTech Pro Legacy stair side fascia`, "", "44 LF, 2 sides"},
		{7, `7" x 12' TimberTech Pro Legacy toe kick`, "", "21 treads, 3 per board"},
		{2, "Starborn Fascia System screws, 100 count", "Box", "167 LF of fascia"},
	}
	if got := deckingItems(t, "1040"); !reflect.DeepEqual(got, want) {
		t.Errorf("decking list\n got: %+v\nwant: %+v", got, want)
	}
}

// TestDeckingWoodStairs checks the cedar trim rows: 16' x 22', 9' high, 4'
// stairs, 50 LF of fascia, stair fascia, no toe kicks.
func TestDeckingWoodStairs(t *testing.T) {
	want := []Item{
		{70, "2x6 x 12' Cedar deck board", "", "35 rows x 2 per row"},
		{1, `CAMO EDGE 2-3/8" deck screws, 1750 count`, "Box", "407 sq ft with stair treads"},
		{6, "1x10 x 12' #2 Tight Knot cedar fascia", "", "50 LF, matches rim joist"},
		{8, "2x6 x 16' Cedar stair tread board", "", "15 treads, 2 boards per tread"},
		{3, "1x12 x 12' #2 Tight Knot cedar stair side fascia", "", "32 LF, 2 sides"},
		{1, `#7 x 2" Simpson Strong-Tie #1 Square Drive Trim Head Deck Screws, 5 lb box (770)`, "Box", "82 LF of 1x cedar, 164 screws"},
	}
	if got := deckingItems(t, "c8"); !reflect.DeepEqual(got, want) {
		t.Errorf("decking list\n got: %+v\nwant: %+v", got, want)
	}
}

// TestDeckingMatrix checks the single-section decks in the agreed test plan.
func TestDeckingMatrix(t *testing.T) {
	tests := []struct {
		id          string
		boards      int
		lengthFt    int
		treadBoards int // 0 means no stairs.
	}{
		{"1031", 66, 16, 0},
		{"1039", 22, 8, 3},
		{"1040", 38, 16, 11},
		{"1052", 12, 20, 8},
		{"1054", 6, 20, 4},
		{"c1", 28, 20, 13},
		{"c3", 22, 16, 16},
		{"c4", 27, 16, 5},
		{"c5", 60, 12, 14},
		{"c7", 96, 12, 0},
		{"c8", 70, 12, 8},
	}
	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			items := deckingItems(t, tt.id)
			length := fmt.Sprintf(" %d' ", tt.lengthFt)
			if got := items[0]; got.Count != tt.boards || !strings.Contains(" "+got.Name, length) {
				t.Errorf("deck boards = %d %q, want %d x %d'", got.Count, got.Name, tt.boards, tt.lengthFt)
			}
			treadBoards := 0
			for _, item := range items {
				if strings.Contains(item.Name, "stair tread board") {
					treadBoards = item.Count
				}
			}
			if treadBoards != tt.treadBoards {
				t.Errorf("tread boards = %d, want %d", treadBoards, tt.treadBoards)
			}
		})
	}
}

// TestCompositeBoardExamples uses the worked examples in decking_rules.txt.
func TestCompositeBoardExamples(t *testing.T) {
	tests := []struct {
		widthFt  float64
		lengthFt int
		perRow   string
	}{
		{10, 20, "2 per board"},
		{14, 16, "1 per board"},
		{22, 12, "2 per row"},
		{25, 16, "2 per row"},
		{27, 16, "2 per row"},
	}
	for _, tt := range tests {
		run := compositeBoards(10, tt.widthFt)
		if run.LengthFt != tt.lengthFt || !strings.HasSuffix(run.Note, tt.perRow) {
			t.Errorf("width %g: got %d', %q, want %d', %q", tt.widthFt, run.LengthFt, run.Note, tt.lengthFt, tt.perRow)
		}
	}
}

func TestCoverPacks(t *testing.T) {
	camo := []int{100, 200, 500}
	tests := []struct {
		need int
		want []int
	}{
		{50, []int{1, 0, 0}},
		{168, []int{0, 1, 0}},
		{450, []int{0, 0, 1}},
		{500, []int{0, 0, 1}},
		{510, []int{1, 0, 1}},
		{1000, []int{0, 0, 2}},
	}
	for _, tt := range tests {
		if got := coverPacks(tt.need, camo); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("coverPacks(%d) = %v, want %v", tt.need, got, tt.want)
		}
	}
}

func TestTreadsAndToeKicks(t *testing.T) {
	if got := treadCount(5); got != 8 {
		t.Errorf("treadCount(5) = %d, want 8", got)
	}
	if got := treadCount(13); got != 21 {
		t.Errorf("treadCount(13) = %d, want 21", got)
	}
	tests := []struct {
		boardFt int
		width   float64
		want    int
	}{
		{16, 3, 5},
		{16, 3.5, 4},
		{16, 4, 4},
		{16, 6, 2},
		{12, 3.5, 3},
		{12, 4, 3},
		{12, 6, 2},
		{12, 20, 1},
	}
	for _, tt := range tests {
		if got := piecesPerBoard(tt.boardFt, tt.width); got != tt.want {
			t.Errorf("piecesPerBoard(%d, %g) = %d, want %d", tt.boardFt, tt.width, got, tt.want)
		}
	}
}

// A 9' deck has 16' stringers, so 32 LF of fascia, which takes 3 12' boards.
func TestStairFasciaExample(t *testing.T) {
	lf := stairFasciaLF(16)
	if lf != 32 || stairFasciaBoards(lf) != 3 {
		t.Errorf("got %g LF, %d boards, want 32 LF, 3 boards", lf, stairFasciaBoards(lf))
	}
}

func TestDividerLength(t *testing.T) {
	tests := map[float64]int{8: 12, 12: 16, 13: 16, 14: 16, 16: 20, 20: 20}
	for projectionFt, want := range tests {
		if got := dividerLength(projectionFt); got != want {
			t.Errorf("dividerLength(%g) = %d, want %d", projectionFt, got, want)
		}
	}
}
