package nutrition

import (
	"context"
	"encoding/json"
	"fmt"

	openai "github.com/sashabaranov/go-openai"
)

type MacroEstimate struct {
	Calories  int     `json:"calories"`
	ProteinG  float64 `json:"protein_g"`
	CarbsG    float64 `json:"carbs_g"`
	FatG      float64 `json:"fat_g"`
	MealName  string  `json:"meal_name"`
	RawJSON   []byte
}

const systemPrompt = `You are a nutrition analyst. Respond ONLY with JSON:
{"calories":int,"protein_g":float,"carbs_g":float,"fat_g":float,"meal_name":string}`

func AnalyzeMealPhoto(ctx context.Context, client *openai.Client, imageBase64 string) (*MacroEstimate, error) {
	resp, err := client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: openai.GPT4o,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: systemPrompt,
			},
			{
				Role: openai.ChatMessageRoleUser,
				MultiContent: []openai.ChatMessagePart{
					{
						Type: openai.ChatMessagePartTypeImageURL,
						ImageURL: &openai.ChatMessageImageURL{
							URL:    "data:image/jpeg;base64," + imageBase64,
							Detail: openai.ImageURLDetailAuto,
						},
					},
					{
						Type: openai.ChatMessagePartTypeText,
						Text: "Estimate macros for this meal",
					},
				},
			},
		},
		MaxTokens: 200,
	})
	if err != nil {
		return nil, fmt.Errorf("openai request: %w", err)
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}

	rawJSON := []byte(resp.Choices[0].Message.Content)
	est := &MacroEstimate{}
	if err := json.Unmarshal(rawJSON, est); err != nil {
		return nil, fmt.Errorf("parse macro json: %w", err)
	}
	est.RawJSON = rawJSON
	return est, nil
}
