# Delta Lake Protocol Specification

This document outlines the protocol for reading from and writing to Delta Lake tables. Referenced from <https://github.com/delta-io/delta/blob/master/PROTOCOL.md>

## Table of Contents

- [Read Protocol](#read-protocol)
- [Write Protocol](#write-protocol)
- [Reader Requirements](#reader-requirements)
- [Writer Requirements](#writer-requirements)

## Read Protocol

1. **Check for Last Checkpoint**
   - Read the `_last_checkpoint` object in the table's log directory, if it exists, to obtain a recent checkpoint ID.

2. **Discover New Log Files**
   - Use a LIST operation starting from the last checkpoint ID (or 0 if none exists) to find newer `.json` and `.parquet` files in the log directory.
   - Note: Due to eventual consistency in cloud object stores, this operation may return non-contiguous objects (e.g., `000004.json` and `000006.json` but not `000005.json`).
   - The client can use the largest ID returned as a target table version and wait for missing objects to become visible.

3. **Reconstruct Table State**
   - Use the checkpoint (if present) and subsequent log records to rebuild the table state.
   - This includes identifying data objects with add records but no corresponding remove records, along with their statistics.
   - This process is designed for parallel execution (e.g., using Spark jobs to read checkpoint Parquet files).

4. **Identify Relevant Files**
   - Use statistics to determine which data object files are relevant for the current read query.

5. **Read Data Objects**
   - Query the object store for the identified data objects.
   - This can be done in parallel across a cluster.
   - Note: Due to eventual consistency, some worker nodes may need to retry if objects aren't immediately available.

## Write Protocol

1. **Identify Log Record ID**
   - Determine the next log record ID (r+1) using steps 1-2 of the read protocol.
   - The transaction will read data at table version r (if needed) and attempt to write log record r+1.

2. **Read Current Table State**
   - If required, read data at table version r using the read protocol.
   - This combines the previous checkpoint and subsequent log records.

3. **Write New Data Objects**
   - Write any new data objects to the appropriate data directories.
   - Generate unique object names using GUIDs.
   - This step can be parallelized.
   - These objects are now ready to be referenced in a new log record.

4. **Write Transaction Log**
   - Atomically write the transaction's log record to the `r+1.json` log object.
   - If another client has already written this object, the write will fail.
   - On failure, the transaction can be retried, potentially reusing objects written in step 3.

5. **Create Checkpoint (Optional)**
   - Optionally create a new `.parquet` checkpoint for log record r+1.
   - Typical implementations create a checkpoint every 10 records by default.
   - After successful checkpoint creation, update the `_last_checkpoint` file to reference checkpoint r+1.

## Reader Requirements

- **Writers MUST** produce a Version Checksum file for each commit.
- **Writers MUST** ensure all metrics in the Version Checksum accurately reflect table state after Action Reconciliation.
- **Writers MUST** write the Version Checksum file only after successfully writing the corresponding Delta log entry.
- **Writers MUST NOT** overwrite existing Version Checksum files.

## Writer Requirements

- **Readers MAY** use Version Checksums to validate table state integrity.
- **If performing validation**, readers SHOULD verify all required fields match computed values.
- **If validation fails**, readers SHOULD surface the discrepancy to users via error messaging.
- **Readers MUST** continue functioning if Version Checksum files are missing.