package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/anthropics/anthropic-sdk-go"
	resend "github.com/resend/resend-go/v2"
)

type materialsSection struct {
	title     string
	rulesFile string
}

var materialsSections = []materialsSection{
	{"Footings & Posts", "materials/rules/footing_rules.txt"},
	{"Framing", "materials/rules/framing_rules.txt"},
	{"Decking & Trim", "materials/rules/decking_rules.txt"},
	{"Rails", "materials/rules/rails_rules.txt"},
}

func materialsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userAuth := getUserAuth(r, w)
	if !isAdminUser(userAuth.Email) && userAuth.Role != "contractor" {
		http.Error(w, "Unauthorized", http.StatusForbidden)
		return
	}

	idStr := r.PathValue("estimateID")
	idInt, err := strconv.Atoi(idStr)
	if err != nil || idInt <= 0 {
		http.Error(w, "Invalid estimate ID", http.StatusBadRequest)
		return
	}

	de := getEstimate(idInt)
	if de.Error != "" {
		json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": de.Error})
		return
	}

	if de.DeckArea == 0 {
		de.CalcAllCosts()
	}

	estimateText := formatEstimateText(de)
	markdown, err := generateMaterialsList(estimateText)
	if err != nil {
		log.Printf("materialsHandler: Claude error: %v", err)
		json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": "Failed to generate materials list."})
		return
	}

	if err := emailMaterialsList(de, estimateText, markdown); err != nil {
		log.Printf("materialsHandler: email error: %v", err)
		json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": "Generated but failed to send email."})
		return
	}

	log.Printf("materialsHandler: sent materials for estimate %d", idInt)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "sent", "message": "Materials list emailed to support@columbiaoutdoor.com"})
}

func formatEstimateText(e DeckEstimate) string {
	var sb strings.Builder

	// Customer info
	sb.WriteString(fmt.Sprintf("%s %s\n", e.Customer.FirstName, e.Customer.LastName))
	sb.WriteString(fmt.Sprintf("%s\n", e.Customer.Address))
	sb.WriteString(fmt.Sprintf("%s, %s %s\n", e.Customer.City, e.Customer.State, e.Customer.Zip))
	if e.Customer.PhoneNumber != "" {
		sb.WriteString(fmt.Sprintf("%s\n", e.Customer.PhoneNumber))
	}
	if e.Customer.Email != "" {
		sb.WriteString(fmt.Sprintf("%s\n", e.Customer.Email))
	}

	sb.WriteString(fmt.Sprintf("\nEstimate ID: %d", e.EstimateID))
	if !e.ExpirationDate.IsZero() {
		sb.WriteString(fmt.Sprintf("  Expires: %s", e.ExpirationDate.Format("2006-01-02")))
	}
	sb.WriteString("\n\n")

	// Deck dimensions — the critical block Claude uses for all calculations
	material := materialLabel(e.Material)
	if len(e.Sections) > 0 {
		sb.WriteString(fmt.Sprintf("Deck: %.1f sq ft of %s, %.1f ft high\n", e.DeckArea, material, e.Height))
		for _, s := range e.Sections {
			sb.WriteString(fmt.Sprintf("  Section %q: %.1f ft (projection) x %.1f ft (width) = %.1f sq ft\n",
				s.Label, s.Length, s.Width, s.Length*s.Width))
		}
	} else {
		sb.WriteString(fmt.Sprintf("Deck: Supply and install %.1f sq ft of %s deck. "+
			"Deck size approximately %.1f x %.1f ft, %.1f ft high.\n",
			e.DeckArea, material, e.Length, e.Width, e.Height))
	}

	// Rails
	if e.RailCost > 0 {
		sb.WriteString(fmt.Sprintf("Rail: %s rail with %s infill, %.1f lineal ft\n",
			e.RailMaterial, e.RailInfill, e.RailFeet))
	} else {
		sb.WriteString("Rail: not included\n")
	}

	// Fascia
	if e.HasFascia && e.FasciaCost > 0 {
		sb.WriteString(fmt.Sprintf("Fascia: %.1f lineal ft, match deck material\n", e.FasciaFeet))
	} else {
		sb.WriteString("Fascia: not included\n")
	}

	// Stairs
	if e.StairCost > 0 {
		sb.WriteString(fmt.Sprintf("Stairs: %.1f ft wide, total rise %.1f ft, %s treads\n",
			e.StairWidth, e.Height, material))
		if e.StairRailCost > 0 {
			sb.WriteString(fmt.Sprintf("Stair Rails: %s rail with %s infill, both sides\n",
				e.RailMaterial, e.RailInfill))
		}
		if e.HasStairFascia && e.StairFasciaCost > 0 {
			sb.WriteString("Stair Fascia: included\n")
		}
		if e.HasStairTK && e.StairToeKickCost > 0 {
			sb.WriteString("Stair Toe Kicks: included\n")
		}
	} else {
		sb.WriteString("Stairs: not included\n")
	}

	// Demo
	if e.HasDemo && e.DemoCost > 0 {
		sb.WriteString(fmt.Sprintf("Demo: remove existing structure, %.1f sq ft deck\n", e.DeckArea))
	}

	// Costs summary
	sb.WriteString(fmt.Sprintf("\nSubtotal: $%.2f\n", e.Subtotal))
	sb.WriteString(fmt.Sprintf("Sales Tax (%s): $%.2f\n", e.Customer.State, e.SalesTax))
	sb.WriteString(fmt.Sprintf("Total: $%.2f\n", e.TotalCost))

	return sb.String()
}

func materialLabel(m string) string {
	switch m {
	case "outdoorWood":
		return "Outdoor Wood"
	case "cedar":
		return "Cedar"
	case "timberTechPrime":
		return "TimberTech Prime"
	case "timberTechProReserve":
		return "TimberTech Pro Reserve"
	case "timberTechProLegacy":
		return "TimberTech Pro Legacy"
	default:
		return m
	}
}

func generateMaterialsList(estimateText string) (string, error) {
	ctx := context.Background()
	client := anthropic.NewClient()

	type sectionResult struct {
		index  int
		title  string
		output string
		err    error
	}

	results := make([]sectionResult, len(materialsSections))
	var wg sync.WaitGroup

	for i, s := range materialsSections {
		wg.Add(1)
		go func(i int, s materialsSection) {
			defer wg.Done()
			rulesBytes, err := os.ReadFile(s.rulesFile)
			if err != nil {
				results[i] = sectionResult{index: i, title: s.title, err: fmt.Errorf("read %s: %w", s.rulesFile, err)}
				return
			}
			msg, err := client.Messages.New(ctx, anthropic.MessageNewParams{
				Model:     anthropic.ModelClaudeSonnet4_6,
				MaxTokens: 4096,
				System:    []anthropic.TextBlockParam{{Text: string(rulesBytes)}},
				Messages:  []anthropic.MessageParam{
					anthropic.NewUserMessage(anthropic.NewTextBlock(estimateText)),
				},
			})
			if err != nil {
				results[i] = sectionResult{index: i, title: s.title, err: err}
				return
			}
			results[i] = sectionResult{index: i, title: s.title, output: msg.Content[0].Text}
		}(i, s)
	}

	wg.Wait()

	var sb strings.Builder
	for _, r := range results {
		if r.err != nil {
			sb.WriteString(fmt.Sprintf("## %s\n\nERROR: %v\n\n", r.title, r.err))
			continue
		}
		sb.WriteString(fmt.Sprintf("## %s\n\n%s\n\n", r.title, r.output))
	}
	return sb.String(), nil
}

func emailMaterialsList(e DeckEstimate, estimateText, markdown string) error {
	apiKey := os.Getenv("RESEND_API_KEY")
	if apiKey == "" {
		return fmt.Errorf("RESEND_API_KEY not set")
	}

	subject := fmt.Sprintf("Materials List – Estimate #%d (%s %s)", e.EstimateID, e.Customer.FirstName, e.Customer.LastName)

	client := resend.NewClient(apiKey)
	params := &resend.SendEmailRequest{
		From:    "Columbia Outdoor <support@columbiaoutdoor.com>",
		To:      []string{"support@columbiaoutdoor.com"},
		Subject: subject,
		Html:    "<p>Materials list and source estimate attached.</p>",
		Attachments: []*resend.Attachment{
			{
				Filename:    fmt.Sprintf("estimate_%d.txt", e.EstimateID),
				Content:     []byte(estimateText),
				ContentType: "text/plain",
			},
			{
				Filename:    fmt.Sprintf("materials_%d.md", e.EstimateID),
				Content:     []byte(markdown),
				ContentType: "text/markdown",
			},
		},
	}

	_, err := client.Emails.Send(params)
	return err
}
