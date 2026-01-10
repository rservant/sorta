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

### Audit Trail Commands

Sorta maintains a complete audit trail of all file operations, enabling review and undo of any run.

```bash
# List all runs with summary statistics
./sorta audit list

# Show detailed events for a specific run
./sorta audit show <run-id>

# Filter events by type (MOVE, SKIP, ERROR, etc.)
./sorta audit show <run-id> --type MOVE

# Export a run's audit data to a file
./sorta audit export <run-id> --output audit-export.json
```

### Undo Operations

Undo any previous run to restore files to their original locations:

```bash
# Undo the most recent run
./sorta undo

# Undo a specific run by ID
./sorta undo <run-id>

# Preview what would be undone without making changes
./sorta undo --preview
./sorta undo <run-id> --preview

# Cross-machine undo with path mapping
./sorta undo --path-mapping "/old/path:/new/path"
```

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
  ],
  "audit": {
    "logDirectory": ".sorta/audit",
    "rotationSizeBytes": 10485760,
    "rotationPeriod": "daily",
    "retentionDays": 30,
    "minRetentionDays": 7
  }
}
```

### Configuration Fields

| Field | Description |
|-------|-------------|
| `sourceDirectories` | Directories to scan for files |
| `prefixRules` | List of prefix-to-target mappings |
| `audit.logDirectory` | Directory for audit log files (default: `.sorta/audit`) |
| `audit.rotationSizeBytes` | Rotate log when it exceeds this size (default: 10MB) |
| `audit.rotationPeriod` | Time-based rotation: `daily`, `weekly`, or empty (default: `daily`) |
| `audit.retentionDays` | Delete logs older than this (0 = unlimited, default: 30) |
| `audit.minRetentionDays` | Never delete logs younger than this (default: 7) |

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

## Audit Trail

Sorta maintains a complete audit trail of all file operations in JSON Lines format. Every run is assigned a unique ID, and every file operation is logged with:

- Timestamp (ISO 8601)
- Source and destination paths
- File identity (SHA-256 hash, size, modification time)
- Operation status and reason codes

### Audit Log Location

By default, audit logs are stored in `.sorta/audit/` relative to the config file location. The active log is `sorta-audit.jsonl`, with rotated segments named `sorta-audit-YYYYMMDD-HHMMSS.jsonl`.

### Undo Safety

The undo system includes several safety features:

- **Identity verification**: Files are verified by content hash before undo
- **Collision detection**: Won't overwrite files that exist at the undo destination
- **Partial undo**: Continues with remaining files if individual operations fail
- **Idempotency**: Running undo twice produces the same result
- **Cross-machine support**: Use path mappings to undo on a different machine

### Event Types

| Event | Description |
|-------|-------------|
| `MOVE` | File moved to classified destination |
| `ROUTE_TO_REVIEW` | File moved to for-review directory |
| `SKIP` | File skipped (already processed, no match, etc.) |
| `DUPLICATE_DETECTED` | File renamed due to destination conflict |
| `PARSE_FAILURE` | Date parsing failed |
| `ERROR` | Operation error occurred |
| `UNDO_MOVE` | File restored during undo |
| `UNDO_SKIP` | File skipped during undo |

## Running Tests

```bash
go test ./... -v
```

All property-based tests run 100 iterations by default using the `gopter` library.

## License

MIT
