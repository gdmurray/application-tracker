package main

import (
	"context"
	"fmt"
	"golang.org/x/oauth2"
	"log"
	"os"

	"golang.org/x/oauth2/google"
	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
)

func setup_gmail_watch() {
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
	config, err := google.JWTConfigFromJSON(jsonCredentials, gmail.GmailReadonlyScope)
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

	// Set up the watch request on the user's Gmail account
	watchRequest := &gmail.WatchRequest{
		TopicName: "projects/tough-mechanic-417615/topics/gmail-notifications",
		LabelIds:  []string{"INBOX"},
	}
	user := "me" // 'me' can be used here as we're acting on behalf of the impersonated user
	watchResponse, err := gmailService.Users.Watch(user, watchRequest).Do()
	if err != nil {
		log.Fatalf("Unable to set up watch on user's Gmail account: %v", err)
	}

	fmt.Printf("Watch established on user's Gmail account: %v\n", watchResponse)
}
