package delta

// Actions
/**
Add file
Remove file
Update metadtata
Set Transaction** - not required, used only for streaming
Change protocol
Commit info
*/

/**
https://pl.seequality.net/delta-lake-101-part-2-transaction-log/
create table
{
  "commitInfo": {
    "timestamp": 1679759207135, --
    "operation": "CREATE TABLE", --
    "operationParameters": { --
      "isManaged": "true",
      "description": null,
      "partitionBy": "[]", --
      "properties": "{}" --
    },
    "isolationLevel": "Serializable",
    "isBlindAppend": true,
    "operationMetrics": {},
    "engineInfo": "Apache-Spark/3.3.1.5.2-84175989 Delta-Lake/2.2.0.1",
    "txnId": "4d8f952b-e3dd-4390-affc-ee2d8075a843" --
  }
}
protocol
{
  "protocol": {
    "minReaderVersion": 1,
    "minWriterVersion": 2
  }
}

metadata
{
  "metaData": {
    "id": "f913b755-8565-4508-8690-c0fb2aca13d6",
    "format": {
      "provider": "parquet",
      "options": {}
    },
    "schemaString": "{\"type\":\"struct\",\"fields\":[{\"name\":\"id\",\"type\":\"integer\",\"nullable\":true,\"metadata\":{}},{\"name\":\"name\",\"type\":\"string\",\"nullable\":true,\"metadata\":{}},{\"name\":\"birthdate\",\"type\":\"date\",\"nullable\":true,\"metadata\":{}}]}",
    "partitionColumns": [],
    "configuration": {},
    "createdTime": 1679759207032
  }
}
table metadata
  name
  description
  format
  schemastring
  partition columns
  created time
  configuration
*/

type CommitInfo struct {
	Timestamp           int64             `json:"timestamp"`
	Operation           string            `json:"operation"`
	OperationParameters map[string]string `json:"operationParameters"`
	TransactionId       string            `json:"txnId"`
}

type Protocol struct {
	MinReaderVersion int `json:"minReaderVersion"`
	MinWriterVersion int `json:"minWriterVersion"`
}

type Metadata struct {
	Name             string            `json:"name"`
	Description      string            `json:"description"`
	Format           string            `json:"format"`
	SchemaString     string            `json:"schemaString"`
	PartitionColumns []string          `json:"partitionColumns"`
	CreatedTime      int64             `json:"createdTime"`
	Configuration    map[string]string `json:"configuration"`
}

/**
change metadata action
operation = CREATE TABLE
first table operation
then txn entry
checkpoint

txn entry should have the operation(change metadta, add data) and then the metadata about the action.

next steps--
newmetadata
txn struct and commit and other methods
new transaction
delta write (parquet write) --deps on logstore(lets go with local file, and then s3)
txn commit
checkpoint --could be done later
*/
