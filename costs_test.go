package main

import (
	"math"
	"testing"
)

// testCosts mirrors static/costs.yaml for use in tests without file I/O.
var testCosts = Costs{
	DeckMaterials: map[string]float64{
		"outdoorWood":          30.0,
		"cedar":                39.0,
		"timberTechPrime":      39.0,
		"timberTechProReserve": 49.0,
		"timberTechProLegacy":  59.0,
	},
	RailMaterials: map[string]float64{
		"wood":      95.0,
		"aluminum":  130.0,
		"composite": 150.0,
	},
	RailInfills: map[string]float64{
		"balusters": 10.0,
		"cable":     40.0,
		"glass":     109.0,
	},
	DemoCost:   5.0,
	FasciaCost: 21.0,
}

func approxEqual(a, b float64) bool {
	return math.Abs(a-b) < 0.001
}

// --- CalculateDeckCost ---

func TestCalculateDeckCost(t *testing.T) {
	tests := []struct {
		name      string
		length    float64
		width     float64
		height    float64
		material  string
		wantCost  float64
		wantError bool
	}{
		{"outdoorWood ground level", 10, 10, 3, "outdoorWood", 3000.0, false},
		{"cedar ground level", 10, 10, 3, "cedar", 3900.0, false},
		{"timberTechProLegacy ground level", 10, 10, 3, "timberTechProLegacy", 5900.0, false},
		{"height 5 adds 1% per excess foot", 10, 10, 5, "outdoorWood", 3000.0 * 1.01, false},
		{"height 6 adds 2%", 10, 10, 6, "outdoorWood", 3000.0 * 1.02, false},
		{"height 19 adds 15%", 10, 10, 19, "outdoorWood", 3000.0 * 1.15, false},
		{"height 20 sets error", 10, 10, 20, "outdoorWood", 0.0, true},
		{"invalid material sets error", 10, 10, 3, "bamboo", 0.0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := DeckEstimate{Length: tt.length, Width: tt.width, Height: tt.height, Material: tt.material}
			e.CalculateDeckCost(testCosts)
			if tt.wantError {
				if e.Error == "" {
					t.Errorf("expected an error, got none")
				}
				return
			}
			if e.Error != "" {
				t.Errorf("unexpected error: %s", e.Error)
			}
			if !approxEqual(e.DeckCost, tt.wantCost) {
				t.Errorf("DeckCost = %.4f, want %.4f", e.DeckCost, tt.wantCost)
			}
		})
	}
}

// --- CalculateRailCost ---

func TestCalculateRailCost(t *testing.T) {
	tests := []struct {
		name        string
		length      float64
		width       float64
		stairWidth  float64
		railMat     string
		railInfill  string
		wantCost    float64
		wantFeet    float64
		wantInfill  string
	}{
		{
			"no material clears infill and cost",
			10, 10, 0, "", "cable",
			0.0, 0.0, "",
		},
		{
			"wood balusters 3-sided rail",
			10, 10, 0, "wood", "balusters",
			30 * (95 + 10), 30.0, "balusters",
		},
		{
			"aluminum cable",
			10, 10, 0, "aluminum", "cable",
			30 * (130 + 40), 30.0, "cable",
		},
		{
			"composite glass",
			10, 10, 0, "composite", "glass",
			30 * (150 + 109), 30.0, "glass",
		},
		{
			"stair opening reduces rail feet",
			10, 10, 4, "wood", "balusters",
			26 * (95 + 10), 26.0, "balusters",
		},
		{
			"empty infill defaults to balusters",
			10, 10, 0, "wood", "",
			30 * (95 + 10), 30.0, "balusters",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := DeckEstimate{
				Length: tt.length, Width: tt.width,
				StairWidth: tt.stairWidth,
				RailMaterial: tt.railMat, RailInfill: tt.railInfill,
			}
			e.CalculateRailCost(testCosts)
			if !approxEqual(e.RailCost, tt.wantCost) {
				t.Errorf("RailCost = %.2f, want %.2f", e.RailCost, tt.wantCost)
			}
			if !approxEqual(e.RailFeet, tt.wantFeet) {
				t.Errorf("RailFeet = %.2f, want %.2f", e.RailFeet, tt.wantFeet)
			}
			if e.RailInfill != tt.wantInfill {
				t.Errorf("RailInfill = %q, want %q", e.RailInfill, tt.wantInfill)
			}
		})
	}
}

// --- CalcStairCost ---

func TestCalcStairCost(t *testing.T) {
	tests := []struct {
		name      string
		height    float64
		stairWidth float64
		material  string
		wantCost  float64
		wantError bool
	}{
		{"no stairs", 8, 0, "outdoorWood", 0.0, false},
		{"width too narrow", 8, 2, "outdoorWood", 0.0, true},
		{"4ft wide 8ft high outdoorWood", 8, 4, "outdoorWood", 30 * math.Ceil(8*1.6) * 4 * 1.5, false},
		{"4ft wide 4ft high cedar", 4, 4, "cedar", 39 * math.Ceil(4*1.6) * 4 * 1.5, false},
		{"minimum 3ft width", 8, 3, "outdoorWood", 30 * math.Ceil(8*1.6) * 3 * 1.5, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := DeckEstimate{Height: tt.height, StairWidth: tt.stairWidth, Material: tt.material}
			e.CalcStairCost(testCosts)
			if tt.wantError {
				if e.Error == "" {
					t.Errorf("expected an error, got none")
				}
				return
			}
			if e.Error != "" {
				t.Errorf("unexpected error: %s", e.Error)
			}
			if !approxEqual(e.StairCost, tt.wantCost) {
				t.Errorf("StairCost = %.4f, want %.4f", e.StairCost, tt.wantCost)
			}
		})
	}
}

// --- CalculateFasciaCost ---

func TestCalculateFasciaCost(t *testing.T) {
	tests := []struct {
		name      string
		length    float64
		width     float64
		hasFascia bool
		wantCost  float64
		wantFeet  float64
	}{
		{"no fascia", 10, 10, false, 0.0, 0.0},
		{"10x10 fascia", 10, 10, true, 30 * 21.0, 30.0},
		{"12x16 fascia", 12, 16, true, (2*12 + 16) * 21.0, 40.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := DeckEstimate{Length: tt.length, Width: tt.width, HasFascia: tt.hasFascia}
			e.CalculateFasciaCost(testCosts)
			if !approxEqual(e.FasciaCost, tt.wantCost) {
				t.Errorf("FasciaCost = %.2f, want %.2f", e.FasciaCost, tt.wantCost)
			}
			if !approxEqual(e.FasciaFeet, tt.wantFeet) {
				t.Errorf("FasciaFeet = %.2f, want %.2f", e.FasciaFeet, tt.wantFeet)
			}
		})
	}
}

// --- CalculateDemoCost ---

func TestCalculateDemoCost(t *testing.T) {
	tests := []struct {
		name      string
		setup     func() DeckEstimate
		wantCost  float64
	}{
		{
			"no demo",
			func() DeckEstimate {
				e := DeckEstimate{Length: 10, Width: 10, HasDemo: false, Material: "outdoorWood"}
				e.CalculateDeckCost(testCosts)
				return e
			},
			0.0,
		},
		{
			"deck only demo",
			func() DeckEstimate {
				e := DeckEstimate{Length: 10, Width: 10, HasDemo: true, Material: "outdoorWood"}
				e.CalculateDeckCost(testCosts)
				return e
			},
			10 * 10 * 5.0,
		},
		{
			"deck with rails",
			func() DeckEstimate {
				e := DeckEstimate{Length: 10, Width: 10, HasDemo: true, Material: "outdoorWood", RailMaterial: "wood", RailInfill: "balusters"}
				e.CalculateDeckCost(testCosts)
				e.CalculateRailCost(testCosts)
				return e
			},
			(100 + 30*3) * 5.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := tt.setup()
			e.CalculateDemoCost(testCosts)
			if !approxEqual(e.DemoCost, tt.wantCost) {
				t.Errorf("DemoCost = %.2f, want %.2f", e.DemoCost, tt.wantCost)
			}
		})
	}
}

// --- CalculateSalesTax ---

func TestCalculateSalesTax(t *testing.T) {
	tests := []struct {
		subtotal float64
		state    string
		want     float64
	}{
		{0, "WA", 0},
		{1000, "WA", 87.0},
		{10000, "WA", 870.0},
		{13680, "WA", 1190.16},
		{1000, "OR", 0.0},
		{1000, "ID", 60.0},
		{1000, "", 87.0},  // unknown state defaults to WA
	}

	for _, tt := range tests {
		got := CalculateSalesTax(tt.subtotal, tt.state)
		if !approxEqual(got, tt.want) {
			t.Errorf("CalculateSalesTax(%.2f, %q) = %.4f, want %.4f", tt.subtotal, tt.state, got, tt.want)
		}
	}
}
