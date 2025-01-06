package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hashicorp/go-retryablehttp"
	"github.com/kenjoe41/scimon/internal/config"
	"github.com/kenjoe41/scimon/internal/doi"
	"github.com/kenjoe41/scimon/internal/notification"
)

const (
	hiddenDir   = ".scimon"
	doiFileName = "doi_urls.txt"
	configFile  = "config.json"
)

func main() {
	// Parse command-line arguments
	checkFlag := flag.String("check", "", "Check DOI without adding to file")
	addFlag := flag.String("add", "", "Check and add DOI to monitored file")
	downloadFlag := flag.Bool("download", false, "Download paper if it's available")
	dirFlag := flag.String("dir", "", "Directory to download papers to.")
	flag.Parse()

	client := retryablehttp.NewClient()
	client.RetryMax = 3 // Set the number of retries
	client.Logger = nil // Set the logger to output nothing

	// Handle the `-check` flag
	if *checkFlag != "" {
		isAvailable, pdfLink, err := doi.CheckDOI(client, *checkFlag)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error checking DOI: %v\n", err)
			return
		}
		notification.PrintStatus(*checkFlag, isAvailable, pdfLink)

		// Download PDF if available and link is valid
		if isAvailable && pdfLink != "" && *downloadFlag {
			err := doi.DownloadPDF(pdfLink, *dirFlag)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error downloading PDF for DOI %s: %v\n", *checkFlag, err)
			}
		}

		return
	}

	// Setup hidden directory and files
	homeDir, _ := os.UserHomeDir()
	scimonDir := filepath.Join(homeDir, hiddenDir)
	if err := os.MkdirAll(scimonDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating hidden directory: %v\n", err)
		return
	}

	configFilePath := filepath.Join(scimonDir, configFile)

	cfg, err := config.LoadConfig(configFilePath)
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
			cfg, err := config.LoadConfig(configFilePath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error loading config after creation: %v\n", err)
				return
			}

			// Ensure the loaded config is not nil (optional, depends on your loadConfig implementation)
			if cfg == nil {
				fmt.Fprintf(os.Stderr, "Error: Config is nil after loading.\n")
				return
			}

		} else {
			fmt.Fprintf(os.Stderr, "Error loading configs: %v\n", err)
			return
		}
	}

	// File path for monitored DOIs
	doiFilePath := filepath.Join(scimonDir, doiFileName)

	// Handle the `-add` flag
	if *addFlag != "" {
		doiURL := *addFlag
		isAvailable, pdfLink, _ := doi.CheckDOI(client, doiURL)
		notification.PrintStatus(doiURL, isAvailable, pdfLink)
		if !isAvailable {
			// Append DOI to the file if it's available
			if err := doi.AddDOIToFile(doiFilePath, doiURL); err != nil {
				fmt.Fprintf(os.Stderr, "Error adding DOI to file: %v\n", err)
			} else {
				fmt.Fprintf(os.Stderr, "DOI added to monitored file.\n")
			}
		} else if isAvailable && pdfLink != "" && *downloadFlag {
			err := doi.DownloadPDF(pdfLink, *dirFlag)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error downloading PDF for DOI %s: %v\n", doiURL, err)
			}
		}

		return
	}

	// If no flags are provided, process all DOIs in the file
	doi.ProcessDOIFile(doiFilePath, cfg.DiscordWebhook, *downloadFlag, *dirFlag, client)
}
