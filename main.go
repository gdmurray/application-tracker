package main

import (
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"google.golang.org/api/gmail/v1"
	"log"
	"net/http"
)

// PubSubMessage is the payload of a Pub/Sub event.
type PubSubMessage struct {
	Message struct {
		Data       string            `json:"data,omitempty"`
		Attributes map[string]string `json:"attributes,omitempty"`
		MessageID  string            `json:"messageId,omitempty"`
	} `json:"message,omitempty"`
	Subscription string `json:"subscription,omitempty"`
}

type EmailData struct {
	EmailAddress string `json:"emailAddress"`
	HistoryID    uint64 `json:"historyId"`
}

func main() {
	// Define a flag for script runs with default value false
	runTestClassification := flag.Bool("T", false, "Run the script")
	runInboxScan := flag.Bool("S", false, "Scan the inbox for past emails")
	// Parse command-line flags
	flag.Parse()

	// If the flag "runScript" is provided, execute the script
	if *runTestClassification {
		testClassification()
		return
	}

	if *runInboxScan {
		scanPastJobApplications()
		return
	}

	http.HandleFunc("/message", handlePubSubMessage)
	http.HandleFunc("/refresh", handleTokenRefresh)

	// Start the server
	fmt.Println("Starting server...")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		fmt.Printf("Failed to start server: %v\n", err)
	}
}

func handlePubSubMessage(w http.ResponseWriter, r *http.Request) {
	var m PubSubMessage
	if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	// Email address of the user to impersonate
	userEmail := "greg@gregmurray.dev"

	data, err := base64.StdEncoding.DecodeString(m.Message.Data)
	if err != nil {
		http.Error(w, "Error decoding message", http.StatusBadRequest)
		return
	}

	fmt.Printf("\nReceived message: %s with attributes: %v\n", data, m.Message.Attributes)
	// Here, you can process the message (e.g., update a database, perform an operation, etc.)

	var emailData EmailData
	jsonError := json.Unmarshal(data, &emailData)
	if jsonError != nil {
		http.Error(w, "Error UnMarshalling message", http.StatusBadRequest)
	}

	// Create the Gmail service using the client
	gmailService, err := getGmailService(userEmail, false)
	if err != nil {
		log.Fatalf("Unable to create Gmail client: %v", err)
	}

	historyListCall := gmailService.Users.History.List(userEmail).StartHistoryId(emailData.HistoryID)
	historyList, err := historyListCall.Do()
	if err != nil {
		log.Fatalf("Unable to retrieve history: %v", err)
	}

	openAiClient, err := initializeOpenAIClient()
	if err != nil {
		log.Fatalf("Unable to initialize openAI Client: %v", err)
	}

	srv, err := getSheetsService(false)
	if err != nil {
		log.Fatalf("Unable to retrieve Sheets client: %v", err)
	}

	fmt.Printf("Found %v history items: %v\n", len(historyList.History), historyList.History)
	for _, history := range historyList.History {
		fmt.Printf("Found %v messages in history\n", len(history.Messages))
		for _, msg := range history.Messages {
			// Fetch each message using its ID
			emailContent, emailError := fetchEmailContent(gmailService, userEmail, msg.Id)
			if emailError != nil {
				log.Fatalf("Unable to retrieve message %v: %v", msg.Id, err)
			}

			if isAllowedSender(emailContent.Sender) == false {
				fmt.Printf("Ignoring email from %v\n", emailContent.Sender)
				continue
			}

			fullMessage := fmt.Sprintf("Subject: %s\nFrom: %s\n\n%s", emailContent.Subject, emailContent.Sender, emailContent.Message)

			// Process the message, e.g., read its content
			response, err := classifyEmail(openAiClient, fullMessage)
			if err != nil {
				fmt.Printf("Error classifying email from %v: %v", emailContent.Sender, err)
				w.WriteHeader(http.StatusNoContent)
				return
			}

			classification := handleOpenAiResponse(response)

			if classification.DateApplied == "N/A" {
				classification.DateApplied = emailContent.Date
			}

			fmt.Printf("Classification: %v\n", classification)

			if classification.Classification == "Application Response" {
				insertApplicationIntoSpreadsheet(srv, &classification)
			}
		}
	}

	// Acknowledge the message by responding with a success status code
	w.WriteHeader(http.StatusOK)
}

func handleTokenRefresh(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	fmt.Printf("Received request to refresh token\n")

	// Email address of the user to impersonate
	userEmail := "greg@gregmurray.dev"

	// Create the Gmail service using the client
	gmailService, err := getGmailService(userEmail, false)
	if err != nil {
		log.Fatalf("Unable to create Gmail client: %v", err)
	}

	// Set up the watch request on the user's Gmail account
	watchRequest := &gmail.WatchRequest{
		TopicName:         "projects/tough-mechanic-417615/topics/gmail-notifications",
		LabelIds:          []string{"INBOX"},
		LabelFilterAction: "include",
	}
	user := "me" // 'me' can be used here as we're acting on behalf of the impersonated user
	watchResponse, err := gmailService.Users.Watch(user, watchRequest).Do()
	if err != nil {
		log.Fatalf("Unable to set up watch on user's Gmail account: %v", err)
	}

	fmt.Printf("Watch established on user's Gmail account: %v\n", watchResponse)
	w.WriteHeader(http.StatusOK)
}
