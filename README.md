# iCopy

A command-line utility written in Go to scan, copy, and organize image and video files using metadata (creation date) with optional checksum generation and source cleanup.

---

## Features

* Scan directories containing images and/or videos
* Read creation date metadata from media files
* Organize copied files into structured directories
* Generate MD5 checksums
* Handle duplicates and overwrite behavior
* Optional removal of source files after copy
* Recursive directory traversal

---

## Prerequisites

* Go installed (compatible with `go.mod` in this repository)
* macOS, Linux, or Windows
* Basic familiarity with command-line usage

---

## Installation

Clone the repository:

```bash
git clone https://github.com/evijayan2/icopy.git
cd icopy
```

Download dependencies:

```bash
go mod tidy
```

Build the binary:

```bash
go build -o icopy main.go
```

---

## Usage

Run directly with Go:

```bash
go run main.go [options]
```

Or using the compiled binary:

```bash
./icopy [options]
```

---

## Command-Line Options

| Flag            | Description                                          |
| --------------- | ---------------------------------------------------- |
| `-in`           | Input directory containing media files               |
| `-out`          | Output directory where files will be copied          |
| `-image`        | Enable image metadata processing (`true/false`)      |
| `-video`        | Enable video metadata processing (`true/false`)      |
| `-scan`         | Scan and generate MD5 checksums only (`true/false`)  |
| `-recursive`    | Recursively process subdirectories (`true/false`)    |
| `-dirformat`    | Output directory format: `DATE`, `YEAR-MONTH`, `NOF` |
| `-overwrite`    | Overwrite handling: `yes`, `no`, `ask`               |
| `-force`        | Force copy regardless of conflicts (`true/false`)    |
| `-removesource` | Delete source files after copy (`true/false`)        |

---

## Directory Formats

* **DATE**: `YYYY-MM-DD/`
* **YEAR-MONTH**: `YYYY-MM/`
* **NOF**: No subfolder organization

---

## Examples

### Copy Images Organized by Year and Month

```bash
./icopy \
  -image=true \
  -in=/path/to/photos \
  -out=/backup/photos \
  -dirformat=YEAR-MONTH \
  -recursive=true
```

---

### Copy Videos Organized by Date

```bash
./icopy \
  -video=true \
  -in=/path/to/videos \
  -out=/backup/videos \
  -dirformat=DATE \
  -recursive=true
```

---

### Scan and Generate MD5 Checksums Only

```bash
./icopy \
  -scan=true \
  -in=/path/to/media \
  -out=/path/to/output
```

---

### Copy and Remove Source Files

```bash
./icopy \
  -image=true \
  -video=true \
  -in=/path/to/media \
  -out=/organized \
  -recursive=true \
  -removesource=true
```

> **Warning:** This permanently deletes original files after successful copy.

---

## Duplicate and Overwrite Handling

* `yes` – Always overwrite existing files
* `no` – Skip files that already exist
* `ask` – Prompt or apply interactive logic where applicable

---

## Best Practices

* Run without `-removesource` during initial testing
* Keep a backup of original media
* Use `YEAR-MONTH` for large photo libraries
* Redirect logs to a file for auditing

```bash
./icopy ...options... > icopy.log 2>&1
```

---

## Typical Use Cases

* Organizing phone photos and videos
* Backing up camera media
* Deduplicating and structuring large media libraries
* Preparing media archives for long-term storage

---

## Notes

* Metadata availability depends on file type and source
* Files without valid metadata may fall back to filesystem timestamps
* All operations are controlled via flags; there is no interactive UI

---

## License

Refer to the repository for licensing details.
