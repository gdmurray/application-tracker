# Job Application Tracker

## Overview

This repository contains the source code for a Job Application Tracker, a project designed to automate the tracking of job application responses via email using Google's Gmail API, Google Sheets API, and OpenAI's GPT-3. The project utilizes Go for its core logic, orchestrated by Docker for easy deployment and cloud functions for handling Gmail notifications.

## Features

- **Automated Email Classification**: Classifies emails to identify job application responses using OpenAI's GPT-3.
- **Gmail Integration**: Watches for new emails and processes them automatically.
- **Google Sheets Integration**: Inserts classified job application responses into a Google Sheet for easy tracking.

## Prerequisites

- Go 1.22
- A Google Cloud Project with the Gmail API and Google Sheets API enabled.
- An OpenAI API key for utilizing GPT-3.

## Dependencies

The project relies on several Go modules for interacting with Google Cloud services, Gmail, Google Sheets, and OpenAI's GPT-3 API. Key dependencies include:

- `cloud.google.com/go/secretmanager`
- `github.com/sashabaranov/go-openai`
- `golang.org/x/oauth2`
- `google.golang.org/api`

(See `go.mod` for the full list of dependencies)

## Setup

1. **Google Cloud Project Setup**: Ensure you have a Google Cloud project with the Gmail API and Google Sheets API enabled. Also, set up a service account with domain-wide delegation if necessary and enable the Secret Manager API.

2. **OpenAI API Key**: Secure an API key from OpenAI for accessing GPT-3 services.

3. **Service Account and Secrets**: Store your service account JSON key and OpenAI API key in Google Cloud Secret Manager for secure access within the application.

4. **Google Sheets**: Prepare a Google Sheet for tracking the job applications and note its ID.

5. **Environment Variables**: Set up necessary environment variables or adjust the source code to load from Google Cloud Secret Manager directly.

6. **Deployment**: The application can be deployed using Docker, as described in the `Dockerfile`. Additionally, Google Cloud Build and Google Cloud Run configurations are provided in `cloudbuild.yaml` for CI/CD.

## Usage

- To monitor Gmail for new job application emails and classify them, deploy the application to a server or run it locally.
- Classified job application responses will be automatically inserted into the specified Google Sheet.

## Development

- `main.go`: Entry point of the application, setting up webhooks and processing emails.
- `scripts.go`: Contains logic for handling command line scripts to do tasks such as testing classification and scanning previous emails
- `utils.go`: Includes utility functions for interacting with Gmail, Google Sheets, OpenAI, and handling classified data.
- `Dockerfile` and `cloudbuild.yaml`: For building and deploying the application.

## Contributing

Contributions are welcome! Please feel free to submit pull requests or create issues for bugs and feature requests.
