package main

import (
	"encoding/json"
	"net/http"
)

type CalcDeckRequest struct {
	Description    string  `json:"description"`
	Length         float64 `json:"length"`
	Width          float64 `json:"width"`
	Height         float64 `json:"height"`
	Material       string  `json:"material"`
	RailMaterial   string  `json:"railMaterial"`
	RailInfill     string  `json:"railInfill"`
	StairWidth     float64 `json:"stairWidth"`
	StairRailCount float64 `json:"stairRailCount"`
	HasDemo        bool    `json:"hasDemo"`
	HasFascia      bool    `json:"hasFascia"`
	HasStairFascia bool    `json:"hasStairFascia"`
	HasStairTK     bool    `json:"hasStairTK"`
	CustomerState  string  `json:"customerState"`
}

type CalcDeckResponse struct {
	Description      string  `json:"description"`
	DeckCost         float64 `json:"deckCost"`
	DeckDescription  string  `json:"deckDescription"`
	DemoCost         float64 `json:"demoCost"`
	DemoDescription  string  `json:"demoDescription"`
	RailCost         float64 `json:"railCost"`
	RailDescription  string  `json:"railDescription"`
	RailFeet         float64 `json:"railFeet"`
	FasciaCost       float64 `json:"fasciaCost"`
	FasciaDescription string `json:"fasciaDescription"`
	FasciaFeet       float64 `json:"fasciaFeet"`
	StairCost        float64 `json:"stairCost"`
	StairDescription string  `json:"stairDescription"`
	StairRailCost    float64 `json:"stairRailCost"`
	StairRailDescription string `json:"stairRailDescription"`
	StairFasciaCost  float64 `json:"stairFasciaCost"`
	StairFasciaDescription string `json:"stairFasciaDescription"`
	StairToeKickCost float64 `json:"stairToeKickCost"`
	StairTKDescription string `json:"stairTKDescription"`
	Subtotal         float64 `json:"subtotal"`
	SalesTax         float64 `json:"salesTax"`
	TotalCost        float64 `json:"totalCost"`
	Error            string  `json:"error"`
}

func apiCalcDeckHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		json.NewEncoder(w).Encode(CalcDeckResponse{Error: "POST required"})
		return
	}

	var req CalcDeckRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		json.NewEncoder(w).Encode(CalcDeckResponse{Error: "Invalid request"})
		return
	}

	e := DeckEstimate{
		Desc:           req.Description,
		Length:         req.Length,
		Width:          req.Width,
		Height:         req.Height,
		Material:       req.Material,
		RailMaterial:   req.RailMaterial,
		RailInfill:     req.RailInfill,
		StairWidth:     req.StairWidth,
		StairRailCount: req.StairRailCount,
		HasDemo:        req.HasDemo,
		HasFascia:      req.HasFascia,
		HasStairFascia: req.HasStairFascia,
		HasStairTK:     req.HasStairTK,
		Customer:       Customer{State: req.CustomerState},
	}

	e.CalcAllCosts()

	resp := CalcDeckResponse{
		Description:            req.Description,
		DeckCost:               e.DeckCost,
		DeckDescription:        formatDeckDescription(e),
		DemoCost:               e.DemoCost,
		DemoDescription:        formatDemoDescription(e),
		RailCost:               e.RailCost,
		RailDescription:        formatRailDescription(e),
		RailFeet:               e.RailFeet,
		FasciaCost:             e.FasciaCost,
		FasciaDescription:      formatFasciaDescription(e),
		FasciaFeet:             e.FasciaFeet,
		StairCost:              e.StairCost,
		StairDescription:       formatStairDescription(e),
		StairRailCost:          e.StairRailCost,
		StairRailDescription:   formatStairRailDescription(e),
		StairFasciaCost:        e.StairFasciaCost,
		StairFasciaDescription: formatStairFasciaDescription(e),
		StairToeKickCost:       e.StairToeKickCost,
		StairTKDescription:     formatStairTKDescription(e),
		Subtotal:               e.Subtotal,
		SalesTax:               e.SalesTax,
		TotalCost:              e.TotalCost,
		Error:                  e.Error,
	}

	json.NewEncoder(w).Encode(resp)
}
