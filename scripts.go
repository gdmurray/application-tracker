package main

import (
	"fmt"
	"github.com/sashabaranov/go-openai"
	"log"
	"os"
	"time"
)

func testClassification() {
	// Get the value of the environment variable
	apiKey := os.Getenv("OPENAI_API_KEY")

	// Email address of the user to impersonate
	userEmail := "greg@gregmurray.dev"

	// Create the Gmail service
	gmailService, err := getGmailService(userEmail, true)
	if err != nil {
		log.Fatalf("Unable to create Gmail client: %v", err)
	}

	// Fetch the user's email messages
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

	srv, err := getSheetsService(true)
	if err != nil {
		log.Fatalf("Unable to retrieve Sheets client: %v", err)
	}

	if classification.Classification == "Application Response" {
		insertApplicationIntoSpreadsheet(srv, &classification)
	}
}

func scanPastJobApplications() {
	//Get the value of the environment variable
	apiKey := os.Getenv("OPENAI_API_KEY")

	//Email address of the user to impersonate
	userEmail := "greg@gregmurray.dev"

	// Create the Gmail service
	gmailService, err := getGmailService(userEmail, true)
	if err != nil {
		log.Fatalf("Unable to create Gmail client: %v", err)
	}

	// Fetch the user's email messages
	messages, err := gmailService.Users.Messages.List(userEmail).Q("in:inbox").Do()
	if err != nil {
		log.Fatalf("Unable to retrieve messages: %v", err)
	}

	// Check if any messages were found
	if len(messages.Messages) == 0 {
		fmt.Println("No messages found.")
		return
	}

	oaiClient := openai.NewClient(apiKey)

	srv, err := getSheetsService(true)
	if err != nil {
		log.Fatalf("Unable to retrieve Sheets client: %v", err)
	}

	applications := getPreviousSheetValues(srv)
	fmt.Printf("Found %v applications\n", len(applications))

	for _, part := range messages.Messages {
		emailContent, err := fetchEmailContent(gmailService, userEmail, part.Id)
		if err != nil {
			log.Fatalf("Unable to retrieve message: %v", err)
		}
		if isAllowedSender(emailContent.Sender) == false {
			continue
		}
		fmt.Printf("Message Info \n%v\n%v\n\n", emailContent.Sender, emailContent.Subject)
		fullMessage := fmt.Sprintf("Subject: %s\nFrom: %s\n%sTime: %s\n", emailContent.Subject, emailContent.Sender, emailContent.Message, emailContent.Date)
		response, err := classifyEmail(oaiClient, fullMessage)
		if err != nil {
			log.Fatalf("Error classifying email: %v", err)
		}

		classification := handleOpenAiResponse(response)

		if classification.DateApplied == "N/A" {
			classification.DateApplied = emailContent.Date
		}

		fmt.Printf("Classification: %v\n", classification)

		if applicationExists(applications, classification) == false && classification.Classification == "Application Response" {
			insertApplicationIntoSpreadsheet(srv, &classification)
			applications = append(applications, classification)
		}

		fmt.Printf("---------Sleeping for 3s----------\n")
		time.Sleep(3 * time.Second)
	}
}
