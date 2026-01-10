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

Files that don't match any prefix rule go to a "For Review" directory.

## Installation

```bash
go build -o sorta ./cmd/sorta
```

## Usage

```bash
./sorta <config-file>
```

## Configuration

Create a JSON configuration file:

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
  "forReviewDirectory": "/Users/me/Documents/For Review"
}
```

### Configuration Fields

| Field | Description |
|-------|-------------|
| `sourceDirectories` | Directories to scan for files |
| `prefixRules` | List of prefix-to-target mappings |
| `forReviewDirectory` | Where unclassified files go |

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

## Running Tests

```bash
go test ./... -v
```

All property-based tests run 100 iterations by default.

## License

MIT
