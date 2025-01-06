package notification

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
)

// ANSI color codes
const (
	colorGreen = "\033[32m"
	colorRed   = "\033[31m"
	colorReset = "\033[0m"
)

type DiscordMessage struct {
	Content string `json:"content"`
}

func SendDiscordNotification(webhookURL, doi string, available bool, pdfLink string) {
	// Prepare the message
	status := "not available"
	if available {
		status = "available"
	}

	// Start the message with DOI status
	message := fmt.Sprintf("DOI: %s is %s on SciHub.", doi, status)

	// If a PDF link is available, append it to the message
	if pdfLink != "" {
		message = fmt.Sprintf("%s\nPDF Link: %s", message, pdfLink)
	}

	// Create the JSON payload for Discord
	payload := map[string]interface{}{
		"content": message,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating Discord message payload: %v\n", err)
		return
	}

	// Send the request to the Discord webhook
	resp, err := http.Post(webhookURL, "application/json", strings.NewReader(string(payloadBytes)))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error sending Discord notification: %v\n", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != 204 {
		fmt.Fprintf(os.Stderr, "Error sending Discord notification, status code: %d\n", resp.StatusCode)
	}
}

func PrintStatus(doi string, available bool, pdfLink string) {
	if available {
		message := fmt.Sprintf("[%s+%s] DOI: %s is available on SciHub.", colorGreen, colorReset, doi)

		// Append the PDF link if it's available
		if pdfLink != "" {
			message = fmt.Sprintf("%s Get it at %s", message, pdfLink)
		}

		// Print the message to standard output (console)
		fmt.Println(message)
	} else {
		// Print the error message to standard error (console)
		fmt.Fprintf(os.Stderr, "[%s-%s] DOI: %s is not available on SciHub yet.\n", colorRed, colorReset, doi)
	}
}
