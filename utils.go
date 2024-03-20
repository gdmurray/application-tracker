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
	"google.golang.org/api/sheets/v4"
	"log"
	"os"
	"strings"
	"time"
)

type EmailContent struct {
	Subject string `json:"subject"`
	Sender  string `json:"sender"`
	Message string `json:"message"`
	Date    string `json:"date"`
}

func initializeOpenAIClient() (*openai.Client, error) {
	ctx := context.Background()
	secretName := "projects/tough-mechanic-417615/secrets/openai-api-key/versions/latest"
	apiKey, err := getSecret(ctx, secretName)
	if err != nil {
		return nil, fmt.Errorf("failed to get API key from Secret Manager: %v", err)
	}

	client := openai.NewClient(string(apiKey))
	return client, nil
}

type JobApplication struct {
	Classification string
	Company        string
	Role           string
	DateApplied    string
}

const (
	spreadsheetID = "1wV4x_1D1MptyUCHxO1tHPQTgAqf-s1TywzwgaGZChCU"
)

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
		MaxTokens:   256,
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

func isAllowedSender(sender string) bool {
	var excludedSenders = []string{
		"workspace@google.com",
		"workspace-noreply@google.com",
		"cloud-noreply@google.com",
		"no-reply@leetcode.com",
		"resumeworded.com",
		"google-workspace-alerts-noreply@google.com",
		"gtaanm@microsoft.com",
		"security-noreply@linkedin.com",
		"analytics-noreply@google.com",
		"jobscan.co",
		"noreply@glassdoor.com",
		"verify@crossover.com",
		"CloudPlatform-noreply@google.com",
		"PlatformNotifications-noreply@google.com",
		"info@glassdoor.com",
		"googlecloud@google.com",
		"no-reply@accounts.google.com",
	}

	for _, s := range excludedSenders {
		if strings.Contains(sender, s) {
			return false
		}
	}
	return true
}

func handleOpenAiResponse(classification string) JobApplication {
	// Split the response string by newline character
	lines := strings.Split(classification, "\n")

	// Initialize a new JobApplication object
	application := JobApplication{
		Classification: "Other",
		Company:        "N/A",
		Role:           "N/A",
		DateApplied:    "N/A",
	}

	// Parse each line and populate the JobApplication object
	for _, line := range lines {
		index := strings.Index(line, ":")
		if index != -1 {
			key := strings.TrimSpace(line[:index])
			value := strings.TrimSpace(line[index+1:])
			switch key {
			case "Classification":
				application.Classification = value
			case "Company":
				application.Company = value
			case "Role":
				application.Role = value
			case "Date Applied":
				application.DateApplied = value
			}
		}
	}

	return application
}

func getSheetsService(local bool) (*sheets.Service, error) {
	ctx := context.Background()

	credentials := getCredentials(local)

	// Authenticate using the service account key
	config, err := google.JWTConfigFromJSON(credentials, sheets.SpreadsheetsScope)
	if err != nil {
		log.Fatalf("Unable to parse service account key file to config: %v", err)
	}
	client := config.Client(ctx)

	return sheets.NewService(ctx, option.WithHTTPClient(client))
}
func insertApplicationIntoSpreadsheet(srv *sheets.Service, application *JobApplication) {

	var vr sheets.ValueRange
	myValues := []interface{}{application.Company, application.Role, application.DateApplied}
	vr.Values = append(vr.Values, myValues)

	// The range to append to, e.g., "Sheet1", and the input option
	rangeToAppend := "Applications" // Adjust the sheet name as necessary
	valueInputOption := "USER_ENTERED"

	// Append values to the spreadsheet
	_, err := srv.Spreadsheets.Values.Append(spreadsheetID, rangeToAppend, &vr).
		ValueInputOption(valueInputOption).Do()
	if err != nil {
		log.Fatalf("Unable to append data to the spreadsheet: %v", err)
	}

	log.Println("Data appended successfully.")
}

func getPreviousSheetValues(srv *sheets.Service) []JobApplication {
	readRange := "Applications!A2:C"

	// Use the Sheets API to fetch values
	resp, err := srv.Spreadsheets.Values.Get(spreadsheetID, readRange).Do()
	if err != nil {
		log.Fatalf("Unable to retrieve data from sheet: %v", err)
	}

	var applications []JobApplication

	if len(resp.Values) == 0 {
		fmt.Println("No data found.")
	} else {
		for _, row := range resp.Values {
			// Print columns A and B, which correspond to indices 0 and 1.
			// Check for length of row to avoid index out of range errors.
			applications = append(applications, JobApplication{
				Classification: "Application Response",
				Company:        row[0].(string),
				Role:           row[1].(string),
				DateApplied:    row[2].(string),
			})
		}
	}
	return applications
}

// applicationExists checks if an instance of JobApplication exists in the slice.
func applicationExists(applications []JobApplication, item JobApplication) bool {
	for _, v := range applications {
		if v.Company == item.Company &&
			v.Role == item.Role &&
			v.DateApplied == item.DateApplied {
			return true
		}
	}
	return false
}
func fetchEmailContent(gmailService *gmail.Service, userId, messageId string) (*EmailContent, error) {
	// Retrieve the email message
	msg, err := gmailService.Users.Messages.Get(userId, messageId).Do()
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve message: %v", err)
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
			return nil, fmt.Errorf("error decoding message body: %v", err)
		}
		messageBody = string(data)
	} else if len(msg.Payload.Parts) > 0 {
		// If the message body is in the payload's parts
		for _, part := range msg.Payload.Parts {
			if strings.HasPrefix(part.MimeType, "text/plain") {
				data, err := base64.URLEncoding.DecodeString(part.Body.Data)
				if err != nil {
					return nil, fmt.Errorf("error decoding message body: %v", err)
				}
				messageBody = string(data)
				break // Stop after finding the first plain text part
			}
		}
	}

	t := time.Unix(0, msg.InternalDate*int64(time.Millisecond))

	return &EmailContent{
		Subject: subject,
		Sender:  sender,
		Message: messageBody,
		Date:    t.Format(time.DateOnly),
	}, nil
}

func getCredentials(local bool) []byte {
	ctx := context.Background()
	if local == true {
		// Path to your service account key file
		serviceAccountFilePath := "./credentials/tough-mechanic-417615-0e0ea07e90d0.json"

		// Load the service account key from file
		jsonCredentials, err := os.ReadFile(serviceAccountFilePath)
		if err != nil {
			log.Fatalf("Unable to read service account key file: %v", err)
		}
		return jsonCredentials
	}

	secretName := "projects/tough-mechanic-417615/secrets/job-application-service-account/versions/latest"

	// Load the service account key from Secret Manager
	jsonCredentials, err := getSecret(ctx, secretName)
	if err != nil {
		log.Fatalf("Unable to read service account key from Secret Manager: %v", err)
	}

	return jsonCredentials

}
func getGmailService(userEmail string, local bool) (*gmail.Service, error) {
	ctx := context.Background()

	// Load the service account key from Secret Manager
	credentials := getCredentials(local)

	// Configure the JWT config for domain-wide delegation
	config, err := google.JWTConfigFromJSON(credentials, gmail.GmailModifyScope)

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
