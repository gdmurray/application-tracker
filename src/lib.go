package main

import (
	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	"context"
	"encoding/base64"
	"fmt"
	"github.com/sashabaranov/go-openai"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
	"log"
	"strings"
)

func initializeOpenAIClient() (*openai.Client, error) {
	ctx := context.Background()
	secretName := "projects/your-project-id/secrets/openai-api-key/versions/latest"
	apiKey, err := getSecret(ctx, secretName)
	if err != nil {
		return nil, fmt.Errorf("failed to get API key from Secret Manager: %v", err)
	}

	client := openai.NewClient(string(apiKey))
	return client, nil
}
func classifyEmail(client *openai.Client, emailContent string) (string, error) {
	ctx := context.Background()
	messages := []openai.ChatCompletionMessage{
		{
			Role: "system",
			Content: "You are a system who can classify the subject, sender, " +
				"and content of an email and be able to classify whether the email is a job application response email." +
				"These emails are ones companies send out when you have applied for a job. You will respond in two sections, " +
				"and the format must be consistent and parseable. The first section is the classification, which will look like this\n" +
				"Classification: (Application Response | Other).\n" +
				"The second section is lines of application details, which all will just say N/A if the email is not an application response email." +
				"The information I'd like you to try to extract from the email is: Company, Role, and Date Applied." +
				"Always list that information on a new line in that order, even if you can't find the information, then put down N/A.\n " +
				"Example: Classification: Application Response\nCompany: Google\nRole: Software Engineer\nDate Applied: 2022-01-01\n" +
				"Example 2: Classification: Other\nCompany: N/A\nRole: N/A\nDate Applied: N/A\n",
		},
		{
			Role:    "user",
			Content: emailContent,
		},
		// Add more messages as needed to provide context for the conversation
	}
	response, err := client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model:       openai.GPT3Dot5Turbo,
		MaxTokens:   100,
		Temperature: 0.5,
		Messages:    messages,
	})

	if err != nil {
		return "", fmt.Errorf("error creating chat completion: %w", err)
	}

	if len(response.Choices) > 0 && response.Choices[0].Message.Content != "" {
		return response.Choices[0].Message.Content, nil
	}

	return "", fmt.Errorf("no completion choices returned")
}
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
