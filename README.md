# Delta Lake Implementation in Go

[![Go Reference](https://pkg.go.dev/badge/github.com/yourusername/delta-lake-go.svg)](https://pkg.go.dev/github.com/yourusername/delta-lake-go)

An educational implementation of the Delta Lake protocol in Go, inspired by the paper [Delta Lake: High-Performance ACID Table Storage over Cloud Object Stores](https://www.vldb.org/pvldb/vol13/p3411-armbrust.pdf). This project demonstrates how to build an ACID table storage layer on top of object storage using optimistic concurrency control.

## Overview

This project is a simplified, educational implementation of the Delta Lake protocol, demonstrating the core concepts from the paper for building an ACID-compliant table storage layer over cloud object stores (like S3, Azure Blob Storage, or GCS). The implementation focuses on the key concepts of:

- **Optimistic Concurrency Control** using object storage's conditional writes as the concurrency primitive
- **ACID Table Storage** demonstrating the core principles of atomic transactions over object storage
- **Checkpointing** for efficient reads and table state management

## Key Features

- **Optimistic Concurrency Control**: Implements the paper's approach using conditional writes for safe concurrent modifications
- **ACID Table Semantics**: Demonstrates the core principles of atomic transactions over object storage
- **Efficient Reads**: Uses checkpointing as described in the paper to speed up query performance
- **Cloud-Native**: Implements the cloud-optimized approach from the paper for any S3-compatible object storage

### Implementation Notes

This is an educational implementation that focuses on the core concepts from the paper:

- Demonstrates the transaction protocol for ACID table storage
- Implements the optimistic concurrency control model using conditional writes
- Implements inserts, reads, and deletes with support for simple expressions
- Implements checkpointing for efficient reads

Does not support:

- Updates
- Merges

## Architecture

### Core Components

1. **Delta Client**: The main interface for interacting with Delta tables
2. **Transaction**: Handles optimistic concurrency control
3. **Object Storage**: Abstracted storage layer (S3, Azure Blob, etc.)
4. **Checkpoint**: Periodically creates checkpoints for efficient reads

### Concurrency Model

The implementation uses an optimistic concurrency control model as per the paper:

1. **Read Phase**: Clients read the latest table version(through transaction id) and prepare their changes
2. **Validation Phase**: Before committing, clients verify no conflicting writes occurred through the txn id (log)
3. **Write Phase**: Changes are committed using conditional writes to ensure atomicity
4. **Conflict Resolution**: Conflicts are detected using transaction numbers and conditional writes

## Concurrency Control

This implementation mimics object storage's conditional writes (like S3's `If-None-Match` or `If-Match` headers) to implement optimistic concurrency control:

1. Writers read the current version and attempt to commit the next version
2. The commit operation fails if the current version has changed since reading
3. On conflict, the operation should be retried with the latest version. Though not part of this implementation, this is the ideal behaviour.

## Installation

```bash
go get github.com/yourusername/delta-lake-go
```

## Usage

### Basic Operations

```go
package main

import (
    "log"
    "github.com/yourusername/delta-lake-go/delta"
)

func main() {
    // Initialize with your preferred object storage implementation
    storage := NewS3Storage(bucket, region)
    client := delta.NewDeltaClient(storage)

    // Create a new table
    err := client.CreateTable("users", []string{"id", "name", "email"})
    if err != nil {
        log.Fatalf("Failed to create table: %v", err)
    }

    // Write data
    err = client.Write("users", []interface{}{1, "John Doe", "john@example.com"})
    if err != nil {
        log.Fatalf("Failed to write data: %v", err)
    }

    // Read data
    iter, err := client.Read("users")
    if err != nil {
        log.Fatalf("Failed to read data: %v", err)
    }
    defer iter.Close()

    for iter.Next() {
        row := iter.Value()
        log.Printf("Row: %v", row)
    }
}
```

### Transaction Example

```go
// Start a new transaction
txn, err := client.BeginTransaction()
if err != nil {
    log.Fatal(err)
}

// Make changes within the transaction
err = txn.Write("users", []interface{}{2, "Jane Smith", "jane@example.com"})
if err != nil {
    txn.Rollback()
    log.Fatal(err)
}

// Commit the transaction
if err := txn.Commit(); err != nil {
    log.Fatalf("Commit failed: %v", err)
}
```

### Reading Data

```go
// Start a new read transaction
if err := client.nwTxn(); err != nil {
    log.Fatalf("Failed to start transaction: %v", err)
}

// Read all data from the table
iter, err := client.read("users")
if err != nil {
    log.Fatalf("Failed to read data: %v", err)
}

// Iterate through the rows
var rows [][]any
ok, row := iter.next()
for ok || row != nil {
    if row != nil {
        rows = append(rows, row)
        log.Printf("Row: %v", row)
    }
    ok, row = iter.next()
}
```

### Delete with Simple Expressions

```go
// Start a new read transaction
if err := client.nwTxn(); err != nil {
    log.Fatalf("Failed to start transaction: %v", err)
}
// This will remove rows where age is greater than 25
err = client.remove("users", "age > 25")
if err != nil {
    log.Fatalf("Failed to remove data: %v", err)
}

// Commit the transaction
if err := client.commit(); err != nil {
    log.Fatalf("Failed to commit transaction: %v", err)
}
```

## Testing

Run the test suite:

```bash
go test -v ./...
```

## Documentation

For detailed documentation on the protocol from the delta-io project, refer:

- [Protocol Specification](./Protocol.md)

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Acknowledgments

This implementation is based on the paper:

**Delta Lake: High-Performance ACID Table Storage over Cloud Object Stores**  
Michael Armbrust, Ali Ghodsi, Reynold Xin, and Matei Zaharia  
*Proceedings of the VLDB Endowment*, Volume 13, Issue 12 (2020)  
[DOI: 10.14778/3415478.3415505](https://doi.org/10.14778/3415478.3415505)

[Delta Lake](https://delta.io/) the open source project and its community for their contributions to the broader ecosystem.

Note: This is an educational implementation and omits some production features for clarity and educational purposes.
