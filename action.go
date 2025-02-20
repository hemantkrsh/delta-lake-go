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

type commitInfo struct {
	timestamp           int64
	operation           string
	operationParameters map[string]string
	transactionId       string
}

type protocol struct {
	minReaderVersion int
	minWriterVersion int
}

type metadata struct {
	name             string
	description      string
	format           string
	schemaString     string
	partitionColumns []string
	createdTime      int64
	configuration    map[string]string
}
