package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

const (
	hiddenDir   = ".scimon"
	doiFileName = "doi_urls.txt"
	configFile  = "config.json"
)

// ANSI color codes
const (
	colorGreen = "\033[32m"
	colorRed   = "\033[31m"
	colorReset = "\033[0m"
)

type Config struct {
	DiscordWebhook string `json:"discord_webhook"`
}

func main() {
	// Parse command-line arguments
	checkFlag := flag.String("check", "", "Check DOI without adding to file")
	addFlag := flag.String("add", "", "Check and add DOI to monitored file")
	flag.Parse()

	// Handle the `-check` flag
	if *checkFlag != "" {
		doi := *checkFlag
		isAvailable, pdfLink := checkDOI(doi)
		printStatus(doi, isAvailable, pdfLink)
		return

	}

	// Setup hidden directory and files
	homeDir, _ := os.UserHomeDir()
	scimonDir := filepath.Join(homeDir, hiddenDir)
	if err := os.MkdirAll(scimonDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating hidden directory: %v\n", err)
		return
	}

	// Load config
	config, err := loadConfig()
	if err != nil {
		// Check if the error indicates the file does not exist by comparing the error message
		if strings.Contains(err.Error(), "no such file or directory") {
			fmt.Fprintf(os.Stderr, "Warning: Config file not found. An example config file has been created at %s. Please populate it with the necessary arguments.\n", filepath.Join(scimonDir, configFile))

			// Create an example config file at the recommended path
			exampleConfig := `{
	"discord_webhook": "https://discord.com/api/webhooks/YOUR_WEBHOOK_URL"
}`

			err = os.WriteFile(filepath.Join(scimonDir, configFile), []byte(exampleConfig), 0644)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error creating example config file: %v\n", err)
				return
			}

			// Try loading the config again after creating the example
			config, err = loadConfig()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error loading config after creation: %v\n", err)
				return
			}

			// Ensure the loaded config is not nil (optional, depends on your loadConfig implementation)
			if config == nil {
				fmt.Fprintf(os.Stderr, "Error: Config is nil after loading.\n")
				return
			}

		} else {
			fmt.Fprintf(os.Stderr, "Error loading configs: %v\n", err)
			return
		}
	}

	// Continue with the rest of your program...

	// File path for monitored DOIs
	doiFilePath := filepath.Join(scimonDir, doiFileName)

	// Handle the `-add` flag
	if *addFlag != "" {
		doi := *addFlag
		isAvailable, pdfLink := checkDOI(doi)
		printStatus(doi, isAvailable, pdfLink)
		if !isAvailable {
			// Append DOI to the file if it's available
			if err := addDOIToFile(doiFilePath, doi); err != nil {
				fmt.Fprintf(os.Stderr, "Error adding DOI to file: %v\n", err)
			} else {
				fmt.Fprintf(os.Stderr, "DOI added to monitored file.\n")
			}
		}

		return
	}

	// If no flags are provided, process all DOIs in the file
	processDOIFile(doiFilePath, config.DiscordWebhook)
}

func processDOIFile(doiFilePath, discordWebhook string) {
	// Open or create the DOI file
	doiFile, err := os.OpenFile(doiFilePath, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening DOI file: %v\n", err)
		return
	}
	defer doiFile.Close()

	// Read all DOIs from the file
	var doIs []string
	scanner := bufio.NewScanner(doiFile)
	for scanner.Scan() {
		doi := scanner.Text()
		if doi != "" {
			doIs = append(doIs, doi)
		}
	}

	// Check DOI availability and remove available DOIs
	var updatedDOIs []string
	for _, doi := range doIs {
		isAvailable, pdfLink := checkDOI(doi)
		printStatus(doi, isAvailable, pdfLink)

		// Send Discord notification
		sendDiscordNotification(discordWebhook, doi, isAvailable, pdfLink)

		// If the DOI is not available, keep it in the list for later
		if !isAvailable {
			updatedDOIs = append(updatedDOIs, doi)
		}
	}

	// Overwrite the DOI file with the updated list (without available DOIs)
	err = doiFile.Truncate(0) // Clear the file
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error clearing DOI file: %v\n", err)
		return
	}

	_, err = doiFile.Seek(0, 0) // Rewind to the beginning of the file
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error seeking to the beginning of the DOI file: %v\n", err)
		return
	}

	// Write the updated DOIs back into the file
	for _, doi := range updatedDOIs {
		_, err := doiFile.WriteString(doi + "\n")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error writing DOI to file: %v\n", err)
			return
		}
	}
}

func checkDOI(doi string) (bool, string) {
	// Construct Sci-Hub URL
	sciHubURL := fmt.Sprintf("https://sci-hub.se/%s", doi)
	resp, err := http.Get(sciHubURL)
	if err != nil || resp.StatusCode != http.StatusOK {
		return false, ""
	}
	defer resp.Body.Close()

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, ""
	}

	// Check for the indicator text in the HTML
	if strings.Contains(string(body), `Unfortunately, Sci-Hub doesn't have the requested document`) {
		return false, ""
	}

	pdfURL, _ := extractPDFLink(string(body))

	return true, pdfURL
}

func extractPDFLink(body string) (string, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(body))
	if err != nil {
		return "", err
	}

	pdfLink, found := doc.Find("embed[src]").Attr("src")
	if !found {
		return "", nil // No PDF link found, but not an error
	}

	// Split the link at the "#" symbol and return the first part
	parts := strings.SplitN(pdfLink, "#", 2)
	pdfLink = parts[0]

	// Parse the URL
	u, err := url.Parse(pdfLink)
	if err != nil {
		return "", err
	}

	// Ensure the scheme is https
	u.Scheme = "https"

	return u.String(), nil
}

func printStatus(doi string, available bool, pdfLink string) {
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

func addDOIToFile(doiFilePath, doi string) error {
	doiFile, err := os.OpenFile(doiFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer doiFile.Close()

	_, err = doiFile.WriteString(doi + "\n")
	return err
}

func loadConfig() (*Config, error) {
	// Load the configuration file
	userHomeDir, _ := os.UserHomeDir()
	configFilePath := filepath.Join(userHomeDir, hiddenDir, configFile)
	file, err := os.Open(configFilePath)
	if err != nil {
		return nil, fmt.Errorf("could not open config file: %v", err)
	}
	defer file.Close()

	var config Config
	decoder := json.NewDecoder(file)
	err = decoder.Decode(&config)
	if err != nil {
		return nil, fmt.Errorf("could not decode config file: %v", err)
	}

	return &config, nil
}

func sendDiscordNotification(webhookURL, doi string, available bool, pdfLink string) {
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
