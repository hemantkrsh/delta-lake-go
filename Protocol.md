# Protocol

## Reads

1. Read the _last_checkpoint object in the table’s log direc-
tory, if it exists, to obtain a recent checkpoint ID.
2. Use a LIST operation whose start key is the last checkpoint
ID if present, or 0 otherwise, to find any newer .json and
.parquet files in the table’s log directory. This provides a list
files that can be used to reconstruct the table’s state starting
from a recent checkpoint. (Note that, due to eventual consis-
tency of the cloud object store, this LIST operation may return
a non-contiguous set of objects, such has 000004.json and
000006.json but not 000005.json. Nonetheless, the client
can use the largest ID returned as a target table version to read
from, and wait for missing objects to become visible.)
3. Use the checkpoint (if present) and subsequent log records
identified in the previous step to reconstruct the state of the
table—namely, the set of data objects that have add records
but no corresponding remove records, and their associated
data statistics. Our format is designed so that this task can run
in parallel: for example, in our Spark connector, we read the
checkpoint Parquet file and log objects using Spark jobs.
4. Use the statistics to identify the set of data object files relevant
for the read query.
5. Query the object store to read the relevant data objects, pos-
sibly in parallel across a cluster. Note that due to eventual
consistency of the cloud object stores, some worker nodes
may not be able to query objects that the query planner found
in the log; these can simply retry after a short amount of time

## Writes

1. Identify a recent log record ID, say r, using steps 1–2 of the
read protocol (i.e., looking forward from the last checkpoint
ID). The transaction will then read the data at table version r
(if needed) and attempt to write log record r + 1.
2. Read data at table version r, if required, using the same steps
as the read protocol (i.e. combining the previous checkpoint
and any further log records, then reading the data objects
referenced in those).
3. Write any new data objects that the transaction aims to add to
the table into new files in the correct data directories, generat-
ing the object names using GUIDs. This step can happen in
parallel. At the end, these objects are ready to reference in a
new log record.
4. Attempt to write the transaction’s log record into the r + 1
.json log object, if no other client has written this object.
This step needs to be atomic, and we discuss how to achieve
that in various object stores shortly. If the step fails, the
transaction can be retried; depending on the query’s semantics,
the client can also reuse the new data objects it wrote in step
3 and simply try to add them to the table in a new log record.
5. Optionally, write a new .parquet checkpoint for log record
r + 1. (In practice, our implementations do this every 10
records by default.) Then, after this write is complete, update
the _last_checkpoint file to point to checkpoint r + 1.

## Reader Requirements

Writers SHOULD produce a Version Checksum file for each commit
Writers MUST ensure all metrics in the Version Checksum accurately reflect table state after Action Reconciliation
Writers MUST write the Version Checksum file only after successfully writing the corresponding Delta log entry
Writers MUST NOT overwrite existing Version Checksum files

## Writer Requirements

Readers MAY use Version Checksums to validate table state integrity
If performing validation, readers SHOULD verify all required fields match computed values
If validation fails, readers SHOULD surface the discrepancy to users via error messaging
Readers MUST continue functioning if Version Checksum files are missing