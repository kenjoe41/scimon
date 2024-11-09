package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
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
		isAvailable := checkDOI(doi)
		printStatus(doi, isAvailable)
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
		isAvailable := checkDOI(doi)
		printStatus(doi, isAvailable)
		if !isAvailable {
			// Append DOI to the file if it's available
			if err := addDOIToFile(doiFilePath, doi); err != nil {
				fmt.Fprintf(os.Stderr, "Error adding DOI to file: %v\n", err)
			} else {
				fmt.Fprintf(os.Stderr, "DOI added to monitored file.\n")
			}
		} else {
			fmt.Fprintf(os.Stderr, "DOI not available; not added to monitored file.\n")
		}

		return
	}

	// If no flags are provided, process all DOIs in the file
	processDOIFile(doiFilePath, config.DiscordWebhook)
}

func processDOIFile(doiFilePath, discordWebhook string) {
	// Open or create the DOI file
	doiFile, err := os.OpenFile(doiFilePath, os.O_RDONLY|os.O_CREATE, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening DOI file: %v\n", err)
		return
	}
	defer doiFile.Close()

	// Process each DOI and check availability on Sci-Hub
	scanner := bufio.NewScanner(doiFile)
	for scanner.Scan() {
		doi := scanner.Text()
		if doi == "" {
			continue
		}
		isAvailable := checkDOI(doi)
		printStatus(doi, isAvailable)

		// Send Discord notification
		sendDiscordNotification(discordWebhook, doi, isAvailable)
	}
}

func checkDOI(doi string) bool {
	// Construct Sci-Hub URL
	sciHubURL := fmt.Sprintf("https://sci-hub.se/%s", doi)
	resp, err := http.Get(sciHubURL)
	if err != nil || resp.StatusCode != http.StatusOK {
		return false
	}
	defer resp.Body.Close()

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false
	}

	// Check for the indicator text in the HTML
	if strings.Contains(string(body), `Unfortunately, Sci-Hub doesn't have the requested document`) {
		return false
	}

	return true
}

func printStatus(doi string, available bool) {
	if available {
		fmt.Printf("[%s+%s] DOI: %s is available\n", colorGreen, colorReset, doi)
	} else {
		fmt.Fprintf(os.Stderr, "[%s-%s] DOI: %s is not available\n", colorRed, colorReset, doi)
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

func sendDiscordNotification(webhookURL, doi string, available bool) {
	// Prepare the message
	status := "not available"
	if available {
		status = "available"
	}

	message := fmt.Sprintf("DOI: %s is %s", doi, status)

	// Create the JSON payload
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
