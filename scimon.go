package main

import (
	"bufio"
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
)

// ANSI color codes
const (
	colorGreen = "\033[32m"
	colorRed   = "\033[31m"
	colorReset = "\033[0m"
)

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
	processDOIFile(doiFilePath)
}

func processDOIFile(doiFilePath string) {
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
