package main

import (
	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	"context"
	"encoding/base64"
	"fmt"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
	"log"
	"strings"
)

func fetchEmailContent(gmailService *gmail.Service, userId, messageId string) (string, error) {
	// Retrieve the email message
	msg, err := gmailService.Users.Messages.Get(userId, messageId).Do()
	if err != nil {
		return "", fmt.Errorf("unable to retrieve message: %v", err)
	}

	// Extract subject and sender
	var subject, sender string
	for _, header := range msg.Payload.Headers {
		switch header.Name {
		case "Subject":
			subject = header.Value
		case "From":
			sender = header.Value
		}
	}

	// Extract message body
	var messageBody string
	if msg.Payload.Body.Data != "" {
		// If the message body is in the payload's data field (base64 encoded)
		data, err := base64.URLEncoding.DecodeString(msg.Payload.Body.Data)
		if err != nil {
			return "", fmt.Errorf("error decoding message body: %v", err)
		}
		messageBody = string(data)
	} else if len(msg.Payload.Parts) > 0 {
		// If the message body is in the payload's parts
		for _, part := range msg.Payload.Parts {
			if strings.HasPrefix(part.MimeType, "text/plain") {
				data, err := base64.URLEncoding.DecodeString(part.Body.Data)
				if err != nil {
					return "", fmt.Errorf("error decoding message body: %v", err)
				}
				messageBody = string(data)
				break // Stop after finding the first plain text part
			}
		}
	}

	// Combine subject, sender, and message body
	fullMessage := fmt.Sprintf("Subject: %s\nFrom: %s\n\n%s", subject, sender, messageBody)

	return fullMessage, nil
}

func getGmailService() (*gmail.Service, error) {
	ctx := context.Background()
	secretName := "projects/tough-mechanic-417615/secrets/job-application-service-account/versions/latest"

	// Email address of the user to impersonate
	userEmail := "greg@gregmurray.dev"

	// Load the service account key from Secret Manager
	jsonCredentials, err := getSecret(ctx, secretName)
	if err != nil {
		log.Fatalf("Unable to read service account key from Secret Manager: %v", err)
	}

	// Configure the JWT config for domain-wide delegation
	config, err := google.JWTConfigFromJSON(jsonCredentials, gmail.GmailModifyScope)

	if err != nil {
		log.Fatalf("Unable to parse service account key to config: %v", err)
	}
	config.Subject = userEmail

	// Create an HTTP client using the configured config and the context
	ts := config.TokenSource(ctx)
	client := oauth2.NewClient(ctx, ts)

	return gmail.NewService(ctx, option.WithHTTPClient(client))
}

func getSecret(ctx context.Context, secretName string) ([]byte, error) {
	// Create the Secret Manager client.
	client, err := secretmanager.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to setup secret manager client: %w", err)
	}
	defer client.Close()

	// Build the request.
	req := &secretmanagerpb.AccessSecretVersionRequest{
		Name: secretName,
	}

	// Call the API.
	result, err := client.AccessSecretVersion(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to access secret version: %w", err)
	}

	return result.Payload.Data, nil
}
