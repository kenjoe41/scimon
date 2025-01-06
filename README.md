# Scimon: DOI Availability Checker on Sci-Hub

![scihub](https://img.shields.io/badge/Sci--Hub-DOI%20Checker-blue)
![go-version](https://img.shields.io/badge/Go-1.23.3-green)
![open-source](https://img.shields.io/badge/Open%20Source-Yes-brightgreen)

## Overview

Scimon is a lightweight, command-line tool built in Golang to monitor and check the availability of DOI-based documents on [Sci-Hub](https://sci-hub.se). Using Scimon, researchers, students, and academics can track the availability of research papers by DOI without manually browsing Sci-Hub. This tool leverages automation and supports continuous monitoring via a cron job for seamless integration into a research workflow.

## Features

- **DOI Availability Check**: Quickly check if a DOI document is accessible on Sci-Hub.
- **Automatic DOI Management**: Track DOIs in a local file and monitor them regularly.
- **CLI Flags for Flexibility**:
  - `-check`: Check a DOI without adding it to the monitored list.
  - `-add`: Check a DOI and add it to the monitored list if available.
  - `-download`: Download the paper if it's available.
  - `-dir`: Specify the directory to download the papers to.
- **Colorized Output**: Green `[+]` indicates availability; red `[-]` indicates unavailability.
- **Hidden Directory for DOI List**: Stores your list of monitored DOIs in a hidden directory in your home folder.
- **Configuration with `config.json`**: Load settings like a Discord webhook for notifications.
- **Notification Support**: Send notifications to platforms such as Discord about the availability of DOIs.

## Requirements

- [Go 1.20](https://golang.org/doc/go1.20)
- Internet access to reach [Sci-Hub](https://sci-hub.se)

## Installation
To install Scimon directly from GitHub, use the go install command:

```bash
go install github.com/kenjoe41/scimon@latest
```

## Usage

### 1. Checking a DOI
Use the `-check` flag to verify the availability of a DOI on Sci-Hub without adding it to the monitored file.
```bash
scimon -check https://doi.org/10.1109/sp61157.2025.00016
```

### 2. Adding and Checking a DOI
Use the `-add` flag to check a DOI and add it to your monitored list if available.
```bash
scimon -add https://doi.org/10.1109/sp61157.2025.00016
```

### 3. Downloading a Paper
Use the `-download` flag along with the `-check` or `-add` flag to download the paper if it is available.
```bash
scimon -add https://doi.org/10.1109/sp61157.2025.00016 -download
```
Optionally, specify the directory to download the paper to using the `-dir` flag.
```bash
scimon -add https://doi.org/10.1109/sp61157.2025.00016 -download -dir "/path/to/directory"
```

### 4. Monitoring All DOIs
Run `scimon` without flags to check all DOIs stored in the monitored file.
```bash
scimon
```

### 5. Setting Up a Cron Job
To automate the monitoring process, set up a cron job:

```bash
crontab -e
```

Add the following line to check DOIs every day at midnight:
```cron
0 0 * * * /usr/local/bin/scimon
```

### Example Usage

**To Check All Monitored DOIs**:
```bash
$ scimon
[+] DOI: 10.1109/sp61157.2025.00016 is available
[-] DOI: 10.1000/j.journal.2024.01.01 is not available
```

**To Check and Add a New DOI**:
```bash
$ scimon -add 10.1234/example.2024.03
[+] DOI: 10.1234/example.2024.03 is available
DOI added to monitored file.
```

**To Check a Single DOI Without Adding**:
```bash
$ scimon -check 10.1234/test.2024.07
[-] DOI: 10.1234/test.2024.07 is not available
```

**To Check and Download a DOI**:
```bash
$ scimon -check 10.1234/example.2024.03 -download
[+] DOI: 10.1234/example.2024.03 is available
PDF downloaded successfully.
```

## Contributing

Contributions are welcome! Please fork the repository and submit a pull request.

## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.

## Support

For questions or support, open an issue on GitHub, pull requests are welcome, or reach out to the project maintainer directly.

## Acknowledgments

Special thanks to Sci-Hub for providing accessible academic resources.