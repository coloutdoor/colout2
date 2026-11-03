package takeoff

import (
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"
)

const testDir = "../materials/test/"

func calculateFile(t *testing.T, name string) (Result, error) {
	t.Helper()
	f, err := os.Open(testDir + name)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	in, err := ParseEstimateText(f)
	if err != nil {
		t.Fatal(err)
	}
	return Calculate(in)
}

// TestFootings1031 checks every row against values worked out by hand from
// footing_rules.txt: 15' projection x 30' wide, 5' high.
func TestFootings1031(t *testing.T) {
	res, err := calculateFile(t, "estimate_1031.txt")
	if err != nil {
		t.Fatal(err)
	}
	want := []Item{
		{8, "Footing", `16" x 16" x 12" deep`, ""},
		{24, "80# concrete bags", "", "3 per footing"},
		{8, "2x4 x 8' Doug Fir", "Footing frame", "1 per footing"},
		{8, "4x4 x 8' PT post", "", ""},
		{8, "Simpson ABU44Z", "Post base", ""},
		{8, "Simpson PC4Z", "Post cap", ""},
		{8, `1/2" x 5" wedge anchor`, "", "1 per post"},
		{4, "4x10 x 16' PT header", "", "Doubled for midspan header"},
		{1, `Simpson SD9 1.5" screws, 500 count`, "Box", "128 screws needed"},
	}
	got := res.Lists[0].Items
	if !reflect.DeepEqual(got, want) {
		t.Errorf("footings list\n got: %+v\nwant: %+v", got, want)
	}
}

// TestFootingsMatrix checks the single-section decks in the agreed test plan.
func TestFootingsMatrix(t *testing.T) {
	tests := []struct {
		id       string
		footings int
		pad      string
		bags     int
		post     string
	}{
		{"1031", 8, `16" x 16" x 12" deep`, 24, "4x4"},
		{"1039", 2, `16" x 16" x 12" deep`, 6, "4x4"},
		{"1040", 4, `24" x 24" x 12" deep`, 28, "6x6"},
		{"1052", 2, `16" x 16" x 12" deep`, 6, "4x4"},
		{"1054", 2, `16" x 16" x 12" deep`, 6, "4x4"},
		{"c1", 3, `24" x 24" x 12" deep`, 21, "6x6"},
		{"c3", 3, `24" x 24" x 12" deep`, 21, "6x6"},
		{"c4", 2, `20" x 20" x 12" deep`, 10, "4x6"},
		{"c5", 3, `20" x 20" x 12" deep`, 15, "6x6"},
		{"c7", 10, `16" x 16" x 12" deep`, 30, "4x4"},
		{"c8", 6, `20" x 20" x 12" deep`, 30, "4x6"},
	}
	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			res, err := calculateFile(t, "estimate_"+tt.id+".txt")
			if err != nil {
				t.Fatal(err)
			}
			items := res.Lists[0].Items
			if items[0].Count != tt.footings {
				t.Errorf("footings = %d, want %d", items[0].Count, tt.footings)
			}
			if items[0].Description != tt.pad {
				t.Errorf("pad = %s, want %s", items[0].Description, tt.pad)
			}
			if items[1].Count != tt.bags {
				t.Errorf("bags = %d, want %d", items[1].Count, tt.bags)
			}
			if !strings.HasPrefix(items[3].Name, tt.post+" ") {
				t.Errorf("post = %s, want %s", items[3].Name, tt.post)
			}
		})
	}
}

func TestFootingCount(t *testing.T) {
	tests := []struct {
		widthFt float64
		midspan bool
		want    int
	}{
		{3, false, 2},
		{5, false, 2},
		{14, false, 2},
		{14.5, false, 3},
		{24, false, 3},
		{24.5, false, 4},
		{34, false, 4},
		{44, false, 5},
		{30, true, 8},
		{14, true, 4},
	}
	for _, tt := range tests {
		if got := footingCount(tt.widthFt, tt.midspan); got != tt.want {
			t.Errorf("footingCount(%g, %v) = %d, want %d", tt.widthFt, tt.midspan, got, tt.want)
		}
	}
}

func TestHeightRanges(t *testing.T) {
	tests := []struct {
		heightFt float64
		pad      int
		bags     int
		post     string
	}{
		{5.9, 16, 3, "4x4"},
		{6, 20, 5, "4x6"},
		{10, 20, 5, "4x6"},
		{10.5, 20, 5, "6x6"},
		{12, 20, 5, "6x6"},
		{12.5, 24, 7, "6x6"},
		{20, 24, 7, "6x6"},
	}
	for _, tt := range tests {
		pad, post := padFor(tt.heightFt), postFor(tt.heightFt)
		if pad.Inches != tt.pad || pad.Bags != tt.bags || post.Size != tt.post {
			t.Errorf("height %g: got %d\"/%d bags/%s, want %d\"/%d bags/%s",
				tt.heightFt, pad.Inches, pad.Bags, post.Size, tt.pad, tt.bags, tt.post)
		}
	}
}

func TestSpanBoards(t *testing.T) {
	tests := []struct {
		widthFt     float64
		count, feet int
	}{
		{5, 1, 8},
		{15, 1, 16},
		{20, 1, 20},
		{24, 2, 12},
		{27, 2, 14},
		{30, 2, 16},
	}
	for _, tt := range tests {
		count, feet := spanBoards(tt.widthFt)
		if count != tt.count || feet != tt.feet {
			t.Errorf("spanBoards(%g) = %d x %d', want %d x %d'", tt.widthFt, count, feet, tt.count, tt.feet)
		}
	}
}

func TestRoundUp2(t *testing.T) {
	tests := map[float64]int{5: 6, 6: 8, 13: 14, 16: 18}
	for feet, want := range tests {
		if got := roundUp2(feet); got != want {
			t.Errorf("roundUp2(%g) = %d, want %d", feet, got, want)
		}
	}
}

func TestPostLength(t *testing.T) {
	tests := map[float64]int{3: 8, 4: 8, 5: 8, 6: 8, 7: 8, 8: 10, 13: 14, 19: 20}
	for heightFt, want := range tests {
		if got := postLength(heightFt); got != want {
			t.Errorf("postLength(%g) = %d, want %d", heightFt, got, want)
		}
	}
}

func TestFewestBoxes(t *testing.T) {
	tests := []struct {
		need, small, large int
	}{
		{64, 1, 0},
		{100, 1, 0},
		{104, 0, 1},
		{500, 0, 1},
		{550, 1, 1},
		{1000, 0, 2},
	}
	for _, tt := range tests {
		small, large := fewestBoxes(tt.need, 100, 500)
		if small != tt.small || large != tt.large {
			t.Errorf("fewestBoxes(%d) = %d small, %d large, want %d, %d", tt.need, small, large, tt.small, tt.large)
		}
	}
}

func TestEngineeringStops(t *testing.T) {
	for _, name := range []string{"estimate_c2.txt", "estimate_c6.txt"} {
		res, err := calculateFile(t, name)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasPrefix(res.Stop, "ENGINEERING REQUIRED") || len(res.Lists) != 0 {
			t.Errorf("%s: got stop %q and %d lists", name, res.Stop, len(res.Lists))
		}
	}
}

func TestMultiSectionNotSupported(t *testing.T) {
	for _, id := range []string{"1028", "1035", "1036", "1037", "1055"} {
		if _, err := calculateFile(t, "estimate_"+id+".txt"); !errors.Is(err, ErrMultiSection) {
			t.Errorf("%s: got %v, want ErrMultiSection", id, err)
		}
	}
}
