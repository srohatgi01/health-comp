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
	_ = idx

	fmt.Println("🤖 Health Assistant DB Connected! Type 'exit' to quit.")
	fmt.Println("-----------------------------------------------------")

	Process()

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

func Process() {

	// Initialize the database connection
	db := initializeDB()
	defer db.Close()

	// create tool menu
	myTools := clients.CreateToolMenu()

	reader := bufio.NewReader(os.Stdin)

	fmt.Println("🤖 Welcome to Karetaker! Type 'exit' to quit.")
	fmt.Println("-----------------------------------------------------")

	// 2. Start the Chat Loop
	for {
		fmt.Print("\nYou: ")

		// Wait for the user to type something and press Enter
		userInput, _ := reader.ReadString('\n')
		userInput = strings.TrimSpace(userInput)

		// Allow the user to quit the program
		if userInput == "exit" || userInput == "quit" {
			fmt.Println("Goodbye!")
			break
		}

		// create olamma request
		reqBody := clients.ChatRequest{
			Model:  "llama3.1",
			Stream: false,
			Messages: []clients.Message{
				{Role: "user", Content: userInput},
			},
			Tools: myTools,
		}

		jsonData, _ := json.Marshal(reqBody)

		// Step C: Send to your local Ollama server
		resp, err := http.Post("http://localhost:11434/api/chat", "application/json", bytes.NewBuffer(jsonData))
		if err != nil {
			log.Fatalf("Failed to connect to Ollama. Is it running? Error: %v", err)
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)

		var chatResp clients.ChatResponse
		json.Unmarshal(body, &chatResp)

		fmt.Println("🤖 The AI decided to use the following tools:")
		fmt.Println("---------------------------------------------")

		var finalDatabaseResult string
		for _, toolCall := range chatResp.Message.ToolCalls {
			toolName := toolCall.Function.Name
			args := toolCall.Function.Arguments // This is a map[string]interface{}

			fmt.Printf("-> Intercepted tool request: %s\n", toolName)

			// THE DISPATCHER: Route the string name to the actual Go function
			switch toolName {

			case "add_weight":
				weightValue := args["weight_value"].(float64)

				fmt.Printf("Extracted weight %f from the chat. Adding to the SQLite Database", weightValue)

				bodyMetricsRepo := repositories.NewBodyMetricsRepo(db)
				err := bodyMetricsRepo.Create(&models.BodyMetrics{
					MetricName: "weight",
					Unit:       "kg",
					Value:      weightValue,
				})
				if err != nil {
					fmt.Print("Could not add weight to the database: ", err)
					continue
				}

				fmt.Println("Added Weight successfully to the database")

			case "query_daily_logs":
				// 1. Extract and cast the arguments from the JSON map
				// Note: JSON numbers always decode as float64 in Go interfaces, so we cast to int
				metricType := args["metric_type"].(string)
				daysBackFloat := args["days_back"].(float64)
				daysBack := int(daysBackFloat)

				fmt.Printf("-> Executing SQLite search for %s over %d days...\n", metricType, daysBack)

				// 2. CALL THE ACTUAL FUNCTION
				// (Assuming you initialized your 'db' connection earlier in main)
				finalDatabaseResult = queryDailyLogs(db, metricType, daysBack)

			case "query_health_rules":
				// 1. Extract the argument
				foodName := args["food_name"].(string)

				fmt.Printf("-> Executing Pinecone search for %s...\n", foodName)

				// 2. CALL THE ACTUAL FUNCTION
				// (This would be your Pinecone SearchRecords function we wrote earlier)
				// finalDatabaseResult = queryPineconeRules(idx, foodName)

				finalDatabaseResult = "Simulated Pinecone result: " + foodName + " is safe."

			default:
				fmt.Printf("-> AI tried to call an unknown tool: %s\n", toolName)
			}
		}

		fmt.Println("Database returned:", finalDatabaseResult)
	}
}
