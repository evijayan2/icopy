# iCopy

iCopy is a command-line tool for scanning, copying, and managing image and video files based on their metadata.

## Features

- Scan directories and generate MD5 checksum files.
- Read image and video creation date metadata.
- Copy files to a specified output directory with various formatting options.
- Remove source files after copying.
- Handle errors and log activities.

## Installation

To install the dependencies, run:

```sh
go mod tidy
```

## Usage
To execute the program, use the following command:

```sh
go run main.go [options]
```

### Command Line Options
`-scan`: Scan and generate MD5 checksum files. (true/false)
`-video`: Read video creation date time metadata. (true/false)
`-image`: Read image creation date time metadata. (true/false)
`-removesource`: Remove source files after copying. (true/false)
`-dirformat`: Output directory format. Options: DATE, YEAR-MONTH, NOF (No Format/Preserve Original)
`-out`: Output directory.
`-in`: Input directory.
`-recursive`: Recursively copy files. (true/false)
`-force`: Force copy of files. (true/false)
`-overwrite`: Overwrite existing files. Options: yes, no, ask

### Examples
Scan and generate MD5 checksum files

```sh
go run main.go -scan=true -in=/path/to/input -out=/path/to/output
```

Read image creation date metadata and copy files

```sh
go run main.go -image=true -in=/path/to/input -out=/path/to/output -dirformat=YEAR-MONTH
```

Read video creation date metadata and copy files

```sh
go run main.go -video=true -in=/path/to/input -out=/path/to/output -dirformat=DATE
```

Remove source files after copying

```sh
go run main.go -image=true -in=/path/to/input -out=/path/to/output -removesource=true
```
