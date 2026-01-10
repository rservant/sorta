# Sorta

A file organization utility that declutters directories by moving files into structured folders based on filename prefixes and embedded ISO dates.

## How It Works

Sorta scans source directories for files matching this naming pattern:

```
<prefix> <YYYY-MM-DD> <description>.<ext>
```

For example:
- `Invoice 2024-03-15 Acme Corp.pdf` → moves to `Invoices/2024 Invoice/`
- `receipt 2023-11-20 Amazon order.pdf` → moves to `Receipts/2023 Receipt/`

Files that don't match any prefix rule go to a `for-review` subdirectory within each source directory.

## Installation

### Download Pre-built Binaries

Download the latest release for your platform from the [Releases page](https://github.com/rservant/sorta/releases).

**macOS:**
```bash
# Download the binary for your architecture (arm64 for Apple Silicon, amd64 for Intel)
curl -LO https://github.com/rservant/sorta/releases/latest/download/sorta-darwin-arm64

# Remove quarantine attribute (required for unsigned binaries)
xattr -d com.apple.quarantine sorta-darwin-arm64

# Make executable and move to PATH
chmod +x sorta-darwin-arm64
sudo mv sorta-darwin-arm64 /usr/local/bin/sorta
```

**Linux:**
```bash
curl -LO https://github.com/rservant/sorta/releases/latest/download/sorta-linux-amd64
chmod +x sorta-linux-amd64
sudo mv sorta-linux-amd64 /usr/local/bin/sorta
```

**Windows:**
Download `sorta-windows-amd64.exe` from the releases page. You may see a SmartScreen warning on first run—click "More info" then "Run anyway".

### Build from Source

```bash
go build -o sorta ./cmd/sorta
```

## Usage

Sorta uses subcommands for different operations:

### Run Organization

```bash
# Use default config (sorta-config.json)
./sorta run

# Use custom config file
./sorta -c myconfig.json run
./sorta --config myconfig.json run
```

### View Configuration

```bash
./sorta config
./sorta -c myconfig.json config
```

### Add Source Directory

```bash
./sorta add-source /path/to/directory
```

Creates the config file if it doesn't exist.

### Auto-Discover Prefix Rules

```bash
./sorta discover /path/to/organized/files
```

Scans a directory to automatically detect prefix rules from existing file organization. For example, if you have:

```
/Documents/
├── Invoices/
│   └── Invoice 2024-01-15 Acme.pdf
└── Receipts/
    └── Receipt 2024-02-20 Amazon.pdf
```

Running `./sorta discover /Documents` will detect and add rules for "Invoice" and "Receipt" prefixes.

## Configuration

Sorta uses `sorta-config.json` by default, or specify a custom path with `-c`/`--config`.

```json
{
  "sourceDirectories": [
    "/Users/me/Downloads",
    "/Users/me/Desktop"
  ],
  "prefixRules": [
    { "prefix": "Invoice", "targetDirectory": "/Users/me/Documents/Invoices" },
    { "prefix": "Receipt", "targetDirectory": "/Users/me/Documents/Receipts" },
    { "prefix": "Statement", "targetDirectory": "/Users/me/Documents/Statements" }
  ]
}
```

### Configuration Fields

| Field | Description |
|-------|-------------|
| `sourceDirectories` | Directories to scan for files |
| `prefixRules` | List of prefix-to-target mappings |

Note: The `forReviewDirectory` field is no longer used. Unclassified files are placed in a `for-review` subdirectory within each source directory.

## Matching Rules

- **Case-insensitive**: `INVOICE`, `Invoice`, and `invoice` all match the same rule
- **Longest prefix wins**: If you have rules for both "Invoice" and "Invoice Tax", a file starting with "Invoice Tax" matches the longer prefix
- **Space delimiter required**: The prefix must be followed by a single space, then the date
- **Valid ISO date required**: Date must be YYYY-MM-DD format with valid month/day values

## Output Structure

Classified files are organized into year-prefix subfolders:

```
/Documents/Invoices/
├── 2023 Invoice/
│   ├── Invoice 2023-01-15 Client A.pdf
│   └── Invoice 2023-06-20 Client B.pdf
└── 2024 Invoice/
    └── Invoice 2024-03-10 Client C.pdf
```

The prefix in the filename is normalized to match the canonical casing from your config.

### Unclassified Files

Files that don't match any prefix rule are moved to a `for-review` subdirectory within their source directory:

```
/Users/me/Downloads/
└── for-review/
    ├── random-file.txt
    └── document-without-date.pdf
```

### Duplicate Handling

When a file would overwrite an existing file at the destination, Sorta renames it:

- First duplicate: `filename_duplicate.pdf`
- Second duplicate: `filename_duplicate_2.pdf`
- And so on...

## Running Tests

```bash
go test ./... -v
```

All property-based tests run 100 iterations by default using the `gopter` library.

## License

MIT
