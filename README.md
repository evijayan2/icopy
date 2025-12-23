# iCopy

A command-line utility written in Go for scanning, copying, and organizing image and video files based on metadata (creation date), with optional checksum generation and source cleanup.

---

## Features

* Scan directories and generate MD5 checksums
* Read image or video creation timestamp metadata
* Organize copied files using configurable directory formats
* Recursive directory traversal
* Force copy and overwrite handling
* Optional removal of source files after copying
* Console logging and file logging (`custom.log`)

---

## Supported Formats

### Video Formats

| Format | Processing Method | Description |
| :--- | :--- | :--- |
| `mp4`, `mov`, `m4v`, `3gp` | Metadata Parsing | Extracts creation date from atoms/boxes (e.g., `mvhd`). |
| `qt`, `3g2`, `f4v`, `f4p`, `f4a`, `f4b` | Metadata Parsing | Extracts creation date from atoms/boxes. |
| `mpg`, `vob` | Header Parsing | Extracts timestamp from MPEG Program Stream packs. |
| `wmv`, `avi`, `mkv`, `webm` | File Modification Time | Uses filesystem `ModTime` as a fallback. |
| `flv`, `ts`, `mts`, `m2ts` | File Modification Time | Uses filesystem `ModTime` as a fallback. |
| `ogg`, `yuv`, `rm`, `rmvb`, `viv` | File Modification Time | Uses filesystem `ModTime` as a fallback. |
| `asf`, `amv`, `svi`, `mxf` | File Modification Time | Uses filesystem `ModTime` as a fallback. |
| `roq`, `nsv` | File Modification Time | Uses filesystem `ModTime` as a fallback. |

### Image Formats

| Format | Processing Method | Description |
| :--- | :--- | :--- |
| `jpg`, `jpeg` | EXIF Parsing | Extracts Date/Time Original from EXIF data. |
| `tiff`, `tif` | EXIF Parsing | Extracts creation date from TIFF metadata. |
| `cr2`, `nef`, `arw`, `dng`, `orf`, `rw2`, `raf` | EXIF Parsing | Attempts to parse embedded EXIF; falls back to ModTime if failed. |
| `png`, `gif`, `bmp` | File Modification Time | Uses filesystem `ModTime` as a fallback. |
| `heic` | EXIF Parsing | Extracts EXIF from ISOBMFF metadata box. |
| `webp`, `svg`, `psd`, `ai` | File Modification Time | Uses filesystem `ModTime` as a fallback. |
| `cr3` | File Modification Time | Uses fallback (CR3 structure is complex/ISOBMFF). |

---

## Prerequisites

* Go installed (compatible with this repository)
* macOS, Linux, or Windows
* Terminal or shell access

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
## Build

To build the project locally, run:

```bash
make build
```

To verify the installation:

```bash
make verify
```

## Release

To create release binaries manually:

```bash
make release
```

Binaries will be available in the `build/` directory.

---

## Usage

Run directly using Go:

```bash
go run main.go [options]
```

Or using the compiled binary:

```bash
./icopy [options]
```

---

## Command-Line Flags

The following flags are defined **exactly as implemented in `main.go`**, including defaults and behavior.

| Flag            | Type   | Default | Description                                           |
| --------------- | ------ | ------- | ----------------------------------------------------- |
| `-scan`         | bool   | `false` | Scan files and generate MD5 checksum files only       |
| `-video`        | bool   | `false` | Read video creation date metadata                     |
| `-image`        | bool   | `false` | Read image creation date metadata                     |
| `-removesource` | bool   | `false` | Remove source files after successful copy             |
| `-dirformat`    | string | `"NOF"` | Output directory format (`DATE`, `YEAR-MONTH`, `NOF`) |
| `-out`          | string | `"."`   | Output directory                                      |
| `-in`           | string | `""`    | Input directory (required)                            |
| `-recursive`    | bool   | `false` | Recursively process subdirectories                    |
| `-force`        | bool   | `false` | Force copy of files                                   |
| `-overwrite`    | string | `"no"`  | Overwrite existing files (`yes`, `no`, `ask`)         |

---

## Flag Behavior Notes

* Exactly one of `-image` or `-video` should be enabled.
* If `-in` is not provided, the program exits with an error.
* When `-scan=true`, files are scanned and validated but **not copied**.
* `-force` overrides duplicate and conflict checks.

---

## Directory Format Options

* **DATE** – Organize files as `YYYY-MM-DD/`
* **YEAR-MONTH** – Organize files as `YYYY-MM/`
* **NOF** – No folder organization (default)

---

## Examples

### Scan and Generate MD5 Checksums

```bash
./icopy \
  -scan=true \
  -in=/path/to/media \
  -out=/path/to/output
```

---

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

### Copy and Remove Source Files

```bash
./icopy \
  -image=true \
  -in=/path/to/media \
  -out=/organized \
  -recursive=true \
  -removesource=true
```

> **Warning:** Source files are permanently deleted after a successful copy.

---

### Force Copy with Overwrite

```bash
./icopy \
  -image=true \
  -in=/src \
  -out=/dest \
  -force=true \
  -overwrite=yes
```

---

## Logging

* Logs are written to the console
* A detailed log file is created as `custom.log`
* Errors, skipped files, and copied files are summarized

---

## Best Practices

* Run with `-scan=true` before copying large datasets
* Avoid `-removesource` until results are verified
* Use `YEAR-MONTH` format for large photo libraries
* Redirect output to a file for auditing

```bash
./icopy ...options... > icopy.log 2>&1
```

---

## Typical Use Cases

* Organizing phone or camera photos
* Backing up large video collections
* Cleaning and structuring legacy media archives

---

## License

Refer to the repository for license information.
