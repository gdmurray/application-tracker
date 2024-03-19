package main

import (
	"context"
	"fmt"
	"github.com/sashabaranov/go-openai"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
	"log"
	"os"
)

func testClassification() {
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	// Print the current working directory
	fmt.Println("Current working directory:", cwd)

	// Get the value of the environment variable
	apiKey := os.Getenv("OPENAI_API_KEY")

	ctx := context.Background()

	// Path to your service account key file
	serviceAccountFilePath := "./credentials/tough-mechanic-417615-0e0ea07e90d0.json"

	// Email address of the user to impersonate
	userEmail := "greg@gregmurray.dev"

	// Load the service account key from file
	jsonCredentials, err := os.ReadFile(serviceAccountFilePath)
	if err != nil {
		log.Fatalf("Unable to read service account key file: %v", err)
	}

	// Configure the JWT config for domain-wide delegation
	config, err := google.JWTConfigFromJSON(jsonCredentials, gmail.GmailModifyScope)
	if err != nil {
		log.Fatalf("Unable to parse service account key file to config: %v", err)
	}
	config.Subject = userEmail

	// Create an HTTP client using the configured config and the context
	ts := config.TokenSource(ctx)
	client := oauth2.NewClient(ctx, ts)

	// Create the Gmail service using the client
	gmailService, err := gmail.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		log.Fatalf("Unable to create Gmail client: %v", err)
	}

	// Fetch the user's email messages
	//user := "me" // "me" represents the authenticated user
	messages, err := gmailService.Users.Messages.List(userEmail).Q("in:inbox").Do()
	if err != nil {
		log.Fatalf("Unable to retrieve messages: %v", err)
	}

	// Check if any messages were found
	if len(messages.Messages) == 0 {
		fmt.Println("No messages found.")
		return
	}

	// Get the ID of the last message
	lastMessageID := messages.Messages[1].Id

	// Fetch the last message
	emailContent, err := fetchEmailContent(gmailService, userEmail, lastMessageID)
	if err != nil {
		log.Fatalf("Unable to retrieve last message: %v", err)
	}

	fullMessage := fmt.Sprintf("Subject: %s\nFrom: %s\n%sTime: %s\n", emailContent.Subject, emailContent.Sender, emailContent.Message, emailContent.Date)
	fmt.Printf("Full Message: %s\n", fullMessage)

	fmt.Printf("API Key: %s\n", apiKey)
	oaiClient := openai.NewClient(apiKey)

	response, err := classifyEmail(oaiClient, fullMessage)
	if err != nil {
		log.Fatalf("Error classifying email: %v", err)
	}

	//fmt.Printf("Classification Response: %s\n", response)
	classification := handleOpenAiResponse(response)

	if classification.DateApplied == "N/A" {
		classification.DateApplied = emailContent.Date
	}

	fmt.Printf("Classification: %v\n", classification)

	if classification.Classification == "Application Response" {
		insertApplicationIntoSpreadsheet(&classification)
	}
}
