package main

import (
	"bufio"
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/pinecone-io/go-pinecone/v4/pinecone"
	"github.com/srohatgi/health-comp/clients"
	"github.com/srohatgi/health-comp/config"
	"github.com/srohatgi/health-comp/constants"
	"github.com/srohatgi/health-comp/models"
	"github.com/srohatgi/health-comp/repositories"
)

// 2. The Tool Function: This is what gets executed when Ollama calls "query_daily_logs"
func queryDailyLogs(db *sql.DB, metricType string, daysBack int) string {
	// Calculate the date threshold
	threshold := time.Now().AddDate(0, 0, -daysBack).Format("2006-01-02")

	// Query the average value for that metric over the requested timeframe
	query := `
		SELECT AVG(value), COUNT(value), unit 
		FROM body_metrics 
		WHERE metric_type = ? AND timestamp >= ?
	`

	row := db.QueryRow(query, metricType, threshold)

	var avgValue sql.NullFloat64
	var count int
	var unit sql.NullString

	err := row.Scan(&avgValue, &count, &unit)
	if err != nil {
		return fmt.Sprintf("Error retrieving logs for %s: %v", metricType, err)
	}

	if count == 0 || !avgValue.Valid {
		return fmt.Sprintf("No logs found for %s in the last %d days.", metricType, daysBack)
	}

	// Return a clean string for the LLM to read
	return fmt.Sprintf("Found %d logs. The average %s over the last %d days is %.2f %s.",
		count, metricType, daysBack, avgValue.Float64, unit.String)
}

func main() {
	ctx := context.Background()
	cfg := config.Load()
	if cfg == nil {
		fmt.Println("Config Load Unsuccessful", nil)
	}

	fmt.Println("✅ SQLite Database created and populated.")

	// Initialize New Pinecone client
	pineconeClient, err := pinecone.NewClient(pinecone.NewClientParams{
		ApiKey: cfg.PineconeKey,
	})
	if err != nil {
		fmt.Println("Error Creating Pinecone client")
	}

	// Describe Index
	index, err := pineconeClient.DescribeIndex(ctx, constants.PineConeIndex)
	if err != nil {
		fmt.Print("Unable to describe the index")
	}

	idx, err := pineconeClient.Index(pinecone.NewIndexConnParams{Host: index.Host})
	if err != nil {
		fmt.Print("Unable to fetch the index")
	}

	fmt.Println("🤖 Health Assistant DB Connected! Type 'exit' to quit.")
	fmt.Println("-----------------------------------------------------")

	Process(ctx, idx)

	// question := "Sarthak Rohatgi, Swiggy, India, Java, GoLang, Finance"
	// searchReq := pinecone.SearchRecordsRequest{
	// 	Query: pinecone.SearchRecordsQuery{
	// 		Inputs: &map[string]interface{}{
	// 			"text": question,
	// 		},
	// 		TopK: 3,
	// 	},
	// }

	// searchResponse, err := idx.SearchRecords(ctx, &searchReq)
	// if err != nil {
	// 	fmt.Println("Unable to fetch the records")
	// }

	// fmt.Println("\n✅ Found the response:")
	// for i, match := range searchResponse.Result.Hits {
	// 	fmt.Printf("%d. (Score: %.2f) %s\n", i+1, match.Score, match.Fields["text"])
	// }

}

func initializeDB() *sql.DB {
	db, err := config.InitSqlite("app.db")
	if err != nil {
		log.Fatal(err)
	}

	if err := config.RunMigrations(db); err != nil {
		log.Fatal(err)
	}

	return db
}

func Process(ctx context.Context, defaultIndex *pinecone.IndexConnection) {
	// 1. Setup Local DB and Tools
	db := initializeDB()
	defer db.Close()
	myTools := clients.CreateToolMenu()
	reader := bufio.NewReader(os.Stdin)

	// This slice will maintain the context of the conversation
	var history []clients.Message

	fmt.Println("🤖 Welcome to Karetaker! Type 'exit' to quit.")
	fmt.Println("-----------------------------------------------------")

	for {
		fmt.Print("\nYou: ")
		userInput, _ := reader.ReadString('\n')
		userInput = strings.TrimSpace(userInput)

		if userInput == "exit" || userInput == "quit" {
			fmt.Println("Goodbye!")
			break
		}

		// Append user message to history
		history = append(history, clients.Message{Role: "user", Content: userInput})

		// --- STEP 1: First Pass (Request Tool Call) ---
		reqBody := clients.ChatRequest{
			Model:    "llama3.1",
			Stream:   false,
			Messages: history,
			Tools:    myTools,
		}

		chatResp, err := callOllama(reqBody)
		if err != nil {
			fmt.Println("Error calling Ollama:", err)
			continue
		}

		// --- STEP 2: Check for Tool Calls ---
		if len(chatResp.Message.ToolCalls) > 0 {
			// Add the Assistant's tool request to history
			history = append(history, chatResp.Message)

			fmt.Println("🤖 Thinking... (Using tools)")

			for _, toolCall := range chatResp.Message.ToolCalls {
				toolName := toolCall.Function.Name
				args := toolCall.Function.Arguments
				var toolOutput string

				switch toolName {
				case "add_weight":
					weightValue := args["weight_value"].(float64)
					repo := repositories.NewBodyMetricsRepo(db)
					err := repo.Create(&models.BodyMetrics{
						MetricName: "weight",
						Unit:       "kg",
						Value:      weightValue,
					})
					if err != nil {
						toolOutput = "Error saving weight to database."
					} else {
						toolOutput = fmt.Sprintf("Successfully recorded weight: %.2f kg", weightValue)
					}

				case "fetch_diet_plan":
					// Pinecone Fetch (v4 SDK)
					resp, err := defaultIndex.FetchVectors(ctx, []string{constants.DietPlanRecordId})
					if err != nil || len(resp.Vectors) == 0 {
						toolOutput = "Could not find a diet plan in the database."
					} else {
						// Extracting string from Pinecone Metadata Struct
						val := resp.Vectors[constants.DietPlanRecordId].Metadata.Fields["text"].GetStringValue()
						toolOutput = "User's Diet Plan: " + val
					}

				case "query_daily_logs":
					metricType := args["metric_type"].(string)
					days := int(args["days_back"].(float64))
					toolOutput = queryDailyLogs(db, metricType, days)

				default:
					toolOutput = "Tool not found."
				}

				// Append the tool's output to history with role "tool"
				history = append(history, clients.Message{
					Role:    "tool",
					Content: toolOutput,
				})
			}

			// --- STEP 3: Second Pass (Final Response) ---
			// Now that history has the tool output, Ollama will summarize it
			finalReq := clients.ChatRequest{
				Model:    "llama3.1",
				Stream:   false,
				Messages: history,
			}

			finalResp, err := callOllama(finalReq)
			if err != nil {
				fmt.Println("Error in final summary:", err)
				continue
			}

			fmt.Printf("\n🤖 Karetaker: %s\n", finalResp.Message.Content)
			history = append(history, finalResp.Message)

		} else {
			// No tools needed, just print the direct response
			fmt.Printf("\n🤖 Karetaker: %s\n", chatResp.Message.Content)
			history = append(history, chatResp.Message)
		}
	}
}

// Helper function to keep the loop clean
func callOllama(req clients.ChatRequest) (clients.ChatResponse, error) {
	var chatResp clients.ChatResponse
	jsonData, _ := json.Marshal(req)

	resp, err := http.Post("http://localhost:11434/api/chat", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return chatResp, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	err = json.Unmarshal(body, &chatResp)
	return chatResp, err
}
