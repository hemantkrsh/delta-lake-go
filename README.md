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

- Implements the optimistic concurrency control model using conditional writes
- Demonstrates the transaction protocol for ACID table storage
- Focuses on the core mechanics of the Delta protocol
- Omits some production features for clarity and educational purposes

## Architecture

### Core Components

1. **Delta Client**: The main interface for interacting with Delta tables
2. **Transaction Manager**: Handles optimistic concurrency control and conflict resolution
3. **Object Storage**: Abstracted storage layer (S3, Azure Blob, etc.)
4. **Log Store**: Manages the transaction log (Delta Log)
5. **Checkpoint Service**: Periodically creates checkpoints for efficient reads

### Concurrency Model

The implementation uses an optimistic concurrency control model with the following key concepts:

1. **Read Phase**: Clients read the latest table version and prepare their changes
2. **Validation Phase**: Before committing, clients verify no conflicting writes occurred
3. **Write Phase**: Changes are written using conditional writes to ensure atomicity
4. **Conflict Resolution**: Conflicts are detected using version numbers and conditional writes

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

## Testing

Run the test suite:

```bash
go test -v ./...
```

## Documentation

For detailed documentation, see:

- [Protocol Specification](./Protocol.md)
- [API Reference](https://pkg.go.dev/github.com/yourusername/delta-lake-go)

## Concurrency Control Details

This implementation uses object storage's conditional writes (like S3's `If-None-Match` or `If-Match` headers) to implement optimistic concurrency control:

1. Each table version is stored as a separate object
2. Writers read the current version and attempt to write the next version
3. The write operation fails if the current version has changed since reading
4. On conflict, the operation is retried with the latest version

## Performance Considerations

- **Checkpointing**: Regularly creates Parquet checkpoints to speed up reads
- **Compaction**: Merges small files to maintain read performance
- **Caching**: Caches frequently accessed metadata

## Contributing

Contributions are welcome! Please read our [Contributing Guide](CONTRIBUTING.md) for details.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Acknowledgments

This implementation is based on the paper:

**Delta Lake: High-Performance ACID Table Storage over Cloud Object Stores**  
Michael Armbrust, Ali Ghodsi, Reynold Xin, and Matei Zaharia  
*Proceedings of the VLDB Endowment*, Volume 13, Issue 12 (2020)  
[DOI: 10.14778/3415478.3415505](https://doi.org/10.14778/3415478.3415505)

We also acknowledge the [Delta Lake](https://delta.io/) open source project and its community for their contributions to the broader ecosystem.
