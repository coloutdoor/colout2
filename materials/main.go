package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/tmc/langchaingo/chains"
	//"github.com/tmc/langchaingo/llms/openai"   << Open AI
	"github.com/tmc/langchaingo/llms/googleai"
	"github.com/tmc/langchaingo/prompts"
)

func main() {
	ctx := context.Background()
	// Define the limit as a variable so we can get its address (&maxTokens)
	maxTokens := 8192
	temp := 0.1

	// 1. Initialize the LLM (e.g., OpenAI)
	// Make sure OPENAI_API_KEY is set in your environment variables
	// llm, err := openai.New() << Open AI
	llm, err := googleai.New(ctx,
		googleai.WithDefaultModel("gemini-3-flash-preview"),
		googleai.WithDefaultMaxTokens(maxTokens),
		googleai.WithDefaultTemperature(temp),
	)

	if err != nil {
		log.Fatal(err)
	}

	// 2. Define the Rules (System Prompt)
	// This is where you set your "persona" and constraints.
	ruleBytes, err := os.ReadFile("rules.txt")
	if err != nil {
		log.Fatalf("Failed to read rules.txt: %v", err)
	}
	systemRule := string(ruleBytes)

	// 2. Define a Prompt Template
	/*
		prompt := prompts.NewPromptTemplate(
			"What is a good name for a company that makes {{.product}}?",
			[]string{"product"},
		)
	*/

	// 3. Create a Chat Template
	// We combine the System Rule + The User Input variable
	prompt := prompts.NewChatPromptTemplate([]prompts.MessageFormatter{
		prompts.NewSystemMessagePromptTemplate(systemRule, nil),
		prompts.NewHumanMessagePromptTemplate("{{.estimate_text}}", []string{"estimate_text"}),
	})

	// 3. Create the Chain (Connecting the Prompt to the LLM)
	chain := chains.NewLLMChain(llm, prompt)

	// 4. Run the Chain
	// We pass the input variable "product" here
	/*
		output, err := chains.Call(ctx, chain, map[string]any{
			"product": "Decks and outdoor living.",
		})
	*/

	// 5. Run it
	// We simulate passing in the text from the PDF
	estimateBytes, err := os.ReadFile("estimate_1023.txt")
	if err != nil {
		log.Fatalf("Failed to read rules.txt: %v", err)
	}
	estimateText := string(estimateBytes)
	output, err := chains.Call(ctx, chain, map[string]any{
		//"estimate_text": "Deck size 20x20. TimberTech Dark Roast. 4ft stairs.",
		"estimate_text": estimateText,
	})

	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("LLM Answer:\n", output["text"])
}
