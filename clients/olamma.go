package clients

type ChatRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	Stream   bool      `json:"stream"`
	Tools    []Tool    `json:"tools"`
}

type Message struct {
	Role      string     `json:"role"`
	Content   string     `json:"content"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"` // Added this!
}

type ToolCall struct {
	Function struct {
		Name      string                 `json:"name"`
		Arguments map[string]interface{} `json:"arguments"`
	} `json:"function"`
}

type Tool struct {
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
}

type ToolFunction struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  ToolParameters `json:"parameters"`
}

type ToolParameters struct {
	Type       string              `json:"type"`
	Properties map[string]Property `json:"properties"`
	Required   []string            `json:"required"`
}

type Property struct {
	Type        string `json:"type"`
	Description string `json:"description"`
}

// 2. Define the response structure to catch the AI's decision
type ChatResponse struct {
	Message Message `json:"message"` // Use the unified Message struct here
}

func CreateToolMenu() []Tool {
	return []Tool{
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "query_date_time_details",
				Description: "Get System Date/Time/Weekday if asked for now, today, tonight, right now, etc",
			},
		},
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "add_weight",
				Description: "Whenever user asks to add/log/track/record body weight, add it to the sqllite database",
				Parameters: ToolParameters{
					Type: "object",
					Properties: map[string]Property{
						"weight_value": {Type: "float", Description: "The body weight that needs to be added (e.g. 75.43, 89.43, 90)"},
					},
					Required: []string{"weight_value"},
				},
			},
		},
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "fetch_diet_plan",
				Description: "Fetch the user's diet plan whenever asked or when user what's to know what to eat or what is for dinner/lunch/breakfast, snack etc.",
			},
		},
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "query_health_rules",
				Description: "Search the Pinecone database for dietary rules or nutrition facts about specific foods.",
				Parameters: ToolParameters{
					Type: "object",
					Properties: map[string]Property{
						"food_name": {Type: "string", Description: "The specific food to look up (e.g., 'lentils', 'beef liver')"},
					},
					Required: []string{"food_name"},
				},
			},
		},
	}
}
