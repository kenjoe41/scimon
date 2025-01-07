package doi

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/flytam/filenamify"
	"github.com/hashicorp/go-retryablehttp"
	"github.com/kenjoe41/scimon/internal/notification"
)

func CheckDOI(client *retryablehttp.Client, doi string) (bool, string, error) {
	url := fmt.Sprintf("https://sci-hub.se/%s", doi)
	resp, err := client.Get(url)
	if err != nil || resp.StatusCode != http.StatusOK {
		return false, "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, "", err
	}

	if strings.Contains(string(body), `Unfortunately, Sci-Hub doesn't have the requested document`) {
		return false, "", nil
	}

	pdfLink, err := extractPDFLink(string(body))
	if err != nil || !isValidPDFLink(client, pdfLink) {
		// If the extracted PDF link is invalid, try constructing the fallback URL
		doiPath := strings.TrimPrefix(doi, "https://doi.org/")
		fallbackLink := fmt.Sprintf("https://sci.bban.top/pdf/%s.pdf", doiPath)
		if isValidPDFLink(client, fallbackLink) {
			return true, fallbackLink, nil
		}
		return false, "", err
	}
	return true, pdfLink, nil
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

	parts := strings.SplitN(pdfLink, "#", 2)
	pdfLink = parts[0]

	u, err := url.Parse(pdfLink)
	if err != nil {
		return "", err
	}

	u.Scheme = "https"
	if u.Host == "" || !strings.Contains(u.Host, "sci-hub.se") {
		u.Host = "sci-hub.se"
	}

	return u.String(), nil
}

func isValidPDFLink(client *retryablehttp.Client, link string) bool {
	resp, err := client.Head(link)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}

func ProcessDOIFile(doiFilePath, discordWebhook string, downloadFlag bool, dirFlag string, client *retryablehttp.Client) {
	// Open or create the DOI file
	doiFile, err := os.OpenFile(doiFilePath, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening DOI file: %v\n", err)
		return
	}
	defer doiFile.Close()

	// Read all DOIs from the file
	var dois []string
	scanner := bufio.NewScanner(doiFile)
	for scanner.Scan() {
		doi := scanner.Text()
		if doi != "" {
			dois = append(dois, doi)
		}
	}

	// Check DOI availability and remove available DOIs
	var updatedDOIs []string
	for _, doiURL := range dois {
		isAvailable, pdfLink, _ := CheckDOI(client, doiURL)
		notification.PrintStatus(doiURL, isAvailable, pdfLink)

		// Send Discord notification
		if isAvailable {
			notification.SendDiscordNotification(discordWebhook, doiURL, isAvailable, pdfLink)
		}

		// Download PDF if available and link is valid
		if isAvailable && pdfLink != "" && downloadFlag {
			err := DownloadPDF(pdfLink, dirFlag)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error downloading PDF for DOI %s: %v\n", doiURL, err)
			}
		}

		// If the DOI is not available, keep it in the list for later
		if !isAvailable {
			updatedDOIs = append(updatedDOIs, doiURL)
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

func DownloadPDF(pdfLink, baseDir string) error {
	// Ensure the base directory exists
	if baseDir == "" {
		baseDir = "."
	}
	err := os.MkdirAll(baseDir, 0755)
	if err != nil {
		return fmt.Errorf("failed to create base directory: %w", err)
	}

	// File path to save the PDF
	urlFilename, _ := getFileNameFromURL(pdfLink)
	filePath := filepath.Join(baseDir, urlFilename)

	// Download the PDF
	resp, err := http.Get(pdfLink)
	if err != nil {
		return fmt.Errorf("failed to download PDF: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download PDF, status code: %d", resp.StatusCode)
	}

	// Save the file
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	_, err = io.Copy(file, resp.Body)
	if err != nil {
		return fmt.Errorf("failed to save PDF: %w", err)
	}

	fmt.Printf("PDF downloaded successfully: %s\n", filePath)
	return nil
}

// getFileNameFromURL extracts the filename from a given URL
func getFileNameFromURL(fileURL string) (string, error) {
	parsedURL, err := url.Parse(fileURL)
	if err != nil {
		return "", fmt.Errorf("invalid URL: %w", err)
	}

	// Get the base name of the path component
	baseName := path.Base(parsedURL.Path)
	safePath, _ := filenamify.Filenamify(baseName, filenamify.Options{})
	return safePath, nil
}

func AddDOIToFile(doiFilePath, doi string) error {
	// Open the DOI file for reading and writing
	doiFile, err := os.OpenFile(doiFilePath, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	defer doiFile.Close()

	// Read existing DOIs into a map to ensure uniqueness
	doiSet := make(map[string]struct{})
	scanner := bufio.NewScanner(doiFile)
	for scanner.Scan() {
		doiSet[scanner.Text()] = struct{}{} // Empty struct{} is used to save memory
	}

	// Check if the DOI is already in the set
	if _, exists := doiSet[doi]; exists {
		// DOI already exists, no need to add it again
		return nil
	}

	// If the DOI doesn't exist, append it to the file
	_, err = doiFile.WriteString(doi + "\n")
	return err
}
