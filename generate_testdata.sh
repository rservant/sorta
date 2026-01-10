#!/bin/bash

if [ -z "$1" ]; then
    echo "Usage: $0 <target-directory>"
    echo "Example: $0 testdata/vendor"
    exit 1
fi

TARGET="$1"

mkdir -p "$TARGET/Contracts"
mkdir -p "$TARGET/Invoices"
mkdir -p "$TARGET/Receipts"
mkdir -p "$TARGET/Reports"
mkdir -p "$TARGET/Statements"
mkdir -p "$TARGET/Mixed"

# Contracts (8 files)
touch "$TARGET/Contracts/Contract 2023-01-15 Acme Corp.pdf"
touch "$TARGET/Contracts/Contract 2023-03-22 Beta Inc.pdf"
touch "$TARGET/Contracts/Contract 2023-06-10 Gamma LLC.pdf"
touch "$TARGET/Contracts/Contract 2023-09-05 Delta Co.pdf"
touch "$TARGET/Contracts/Contract 2024-01-20 Epsilon Ltd.pdf"
touch "$TARGET/Contracts/Contract 2024-04-12 Zeta Corp.pdf"
touch "$TARGET/Contracts/Contract 2024-07-30 Eta Inc.pdf"
touch "$TARGET/Contracts/Contract 2024-10-15 Theta LLC.pdf"

# Invoices (8 files)
touch "$TARGET/Invoices/Invoice 2023-02-10 Supplier A.pdf"
touch "$TARGET/Invoices/Invoice 2023-04-18 Supplier B.pdf"
touch "$TARGET/Invoices/Invoice 2023-07-25 Supplier C.pdf"
touch "$TARGET/Invoices/Invoice 2023-10-30 Supplier D.pdf"
touch "$TARGET/Invoices/Invoice 2024-02-14 Supplier E.pdf"
touch "$TARGET/Invoices/Invoice 2024-05-22 Supplier F.pdf"
touch "$TARGET/Invoices/Invoice 2024-08-08 Supplier G.pdf"
touch "$TARGET/Invoices/Invoice 2024-11-19 Supplier H.pdf"

# Receipts (8 files)
touch "$TARGET/Receipts/Receipt 2023-01-05 Amazon.pdf"
touch "$TARGET/Receipts/Receipt 2023-03-12 Walmart.pdf"
touch "$TARGET/Receipts/Receipt 2023-06-20 Target.pdf"
touch "$TARGET/Receipts/Receipt 2023-09-28 Costco.pdf"
touch "$TARGET/Receipts/Receipt 2024-01-08 BestBuy.pdf"
touch "$TARGET/Receipts/Receipt 2024-04-15 HomeDepot.pdf"
touch "$TARGET/Receipts/Receipt 2024-07-22 Lowes.pdf"
touch "$TARGET/Receipts/Receipt 2024-10-30 Staples.pdf"

# Reports (8 files)
touch "$TARGET/Reports/Report 2023-01-31 Q1 Summary.pdf"
touch "$TARGET/Reports/Report 2023-04-30 Q2 Summary.pdf"
touch "$TARGET/Reports/Report 2023-07-31 Q3 Summary.pdf"
touch "$TARGET/Reports/Report 2023-10-31 Q4 Summary.pdf"
touch "$TARGET/Reports/Report 2024-01-31 Annual Review.pdf"
touch "$TARGET/Reports/Report 2024-04-30 Budget Analysis.pdf"
touch "$TARGET/Reports/Report 2024-07-31 Performance.pdf"
touch "$TARGET/Reports/Report 2024-10-31 Forecast.pdf"

# Statements (8 files)
touch "$TARGET/Statements/Statement 2023-01-31 Bank Account.pdf"
touch "$TARGET/Statements/Statement 2023-02-28 Credit Card.pdf"
touch "$TARGET/Statements/Statement 2023-03-31 Investment.pdf"
touch "$TARGET/Statements/Statement 2023-04-30 Savings.pdf"
touch "$TARGET/Statements/Statement 2024-01-31 Bank Account.pdf"
touch "$TARGET/Statements/Statement 2024-02-29 Credit Card.pdf"
touch "$TARGET/Statements/Statement 2024-03-31 Investment.pdf"
touch "$TARGET/Statements/Statement 2024-04-30 Savings.pdf"

# Mixed (10 files - 3 different patterns: Order, Memo, Quote)
touch "$TARGET/Mixed/Order 2023-02-14 Office Supplies.pdf"
touch "$TARGET/Mixed/Order 2023-05-20 Electronics.pdf"
touch "$TARGET/Mixed/Order 2023-08-10 Furniture.pdf"
touch "$TARGET/Mixed/Order 2024-03-15 Software.pdf"
touch "$TARGET/Mixed/Memo 2023-03-01 Policy Update.pdf"
touch "$TARGET/Mixed/Memo 2023-07-15 Team Meeting.pdf"
touch "$TARGET/Mixed/Memo 2024-01-10 New Procedures.pdf"
touch "$TARGET/Mixed/Quote 2023-04-22 Project Alpha.pdf"
touch "$TARGET/Mixed/Quote 2023-09-18 Project Beta.pdf"
touch "$TARGET/Mixed/Quote 2024-06-05 Project Gamma.pdf"

echo "Created 50 test files across 6 directories in $TARGET"
