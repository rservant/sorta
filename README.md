# Sorta

A file organization utility that declutters directories by moving files into structured folders based on filename prefixes and embedded ISO dates.

## How It Works

Sorta scans inbound directories for files matching this naming pattern:

```
<prefix> <YYYY-MM-DD> <description>.<ext>
```

For example:
- `Invoice 2024-03-15 Acme Corp.pdf` → moves to `Invoices/2024 Invoice/`
- `receipt 2023-11-20 Amazon order.pdf` → moves to `Receipts/2023 Receipt/`

Files that don't match any prefix rule go to a `for-review` subdirectory within each inbound directory.

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

# Enable verbose output for detailed progress
./sorta -v run
./sorta --verbose run

# Preview what would happen without moving files
./sorta run --dry-run
./sorta -v run --dry-run
```

The `-v`/`--verbose` flag can be combined with any command to show detailed progress information during execution.

### Watch Mode

Monitor directories and automatically organize files as they arrive:

```bash
# Start watching configured inbound directories
./sorta watch

# Override debounce period (seconds to wait after file activity settles)
./sorta watch --debounce 5

# Watch with verbose output
./sorta -v watch
```

Watch mode:
- Monitors all configured inbound directories for new files
- Waits for files to finish writing (debounce + stability check)
- Automatically organizes files according to your rules
- Ignores temporary files (.tmp, .part, .download, etc.)
- Displays a summary when stopped (Ctrl+C)

### Check Status

See pending files across all inbound directories without making changes:

```bash
# Show pending files grouped by destination
./sorta status

# Show verbose status with individual file paths
./sorta -v status
```

The status command scans all configured inbound directories and shows:
- Files grouped by their destination (organized location or for-review)
- Per-directory counts
- Grand total of pending files

### Dry-Run Mode

Preview file organization without modifying the filesystem:

```bash
./sorta run --dry-run
```

Dry-run mode:
- Shows each file that would be moved with its destination path
- Shows files that would go to for-review directories
- Displays a summary count (moved, for-review, skipped)
- Does NOT create directories, move files, or write audit logs

Use `-v` for additional details like matched prefix rules.

### View Configuration

```bash
./sorta config
./sorta -c myconfig.json config
```

### Add Inbound Directory

```bash
./sorta add-inbound /path/to/directory
```

Creates the config file if it doesn't exist.

### Auto-Discover Prefix Rules

```bash
./sorta discover /path/to/organized/files

# Limit scan depth (0 = immediate directory only, 1 = one level deep, etc.)
./sorta discover --depth 2 /path/to/organized/files

# Interactively approve or reject each discovered rule
./sorta discover --interactive /path/to/organized/files

# Combine depth limiting with interactive mode
./sorta discover --depth 2 --interactive /path/to/organized/files
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

**Discovery Options:**
- `--depth N`: Limit how deep to scan (default: unlimited). Use `--depth 0` for immediate directory only, `--depth 1` for one level of subdirectories, etc.
- `--interactive`: Prompt for each discovered rule with options to accept, reject, accept all, reject all, or quit

**Discovery Behavior:**
- Prefixes are extracted only from filenames, not directory names
- Subdirectories starting with ISO dates (e.g., `2024-01-15 Backup/`) are skipped during scanning
- This prevents false positives from date-organized folder structures
- In non-interactive terminals, `--interactive` falls back to auto-add with a warning

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

# View aggregate statistics across all runs
./sorta audit stats

# Filter stats to a specific time period
./sorta audit stats --since 2024-01-01
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
  "inboundDirectories": [
    "/Users/me/Downloads",
    "/Users/me/Desktop"
  ],
  "prefixRules": [
    { "prefix": "Invoice", "outboundDirectory": "/Users/me/Documents/Invoices" },
    { "prefix": "Receipt", "outboundDirectory": "/Users/me/Documents/Receipts" },
    { "prefix": "Statement", "outboundDirectory": "/Users/me/Documents/Statements" }
  ],
  "watch": {
    "debounceSeconds": 2,
    "stableThresholdMs": 1000,
    "ignorePatterns": [".tmp", ".part", ".download"]
  },
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
| `inboundDirectories` | Directories to scan for files |
| `prefixRules` | List of prefix-to-outbound mappings |
| `watch.debounceSeconds` | Seconds to wait after file activity before processing (default: 2) |
| `watch.stableThresholdMs` | Milliseconds file size must be stable before processing (default: 1000) |
| `watch.ignorePatterns` | File patterns to ignore in watch mode (default: .tmp, .part, .download) |
| `audit.logDirectory` | Directory for audit log files (default: `.sorta/audit`) |
| `audit.rotationSizeBytes` | Rotate log when it exceeds this size (default: 10MB) |
| `audit.rotationPeriod` | Time-based rotation: `daily`, `weekly`, or empty (default: `daily`) |
| `audit.retentionDays` | Delete logs older than this (0 = unlimited, default: 30) |
| `audit.minRetentionDays` | Never delete logs younger than this (default: 7) |

Note: The `forReviewDirectory` field is no longer used. Unclassified files are placed in a `for-review` subdirectory within each inbound directory.

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

Files that don't match any prefix rule are moved to a `for-review` subdirectory within their inbound directory:

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
