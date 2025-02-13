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

type Add struct {
	// Add file
}

/**
https://pl.seequality.net/delta-lake-101-part-2-transaction-log/
create table
{
  "commitInfo": {
    "timestamp": 1679759207135,
    "operation": "CREATE TABLE",
    "operationParameters": {
      "isManaged": "true",
      "description": null,
      "partitionBy": "[]",
      "properties": "{}"
    },
    "isolationLevel": "Serializable",
    "isBlindAppend": true,
    "operationMetrics": {},
    "engineInfo": "Apache-Spark/3.3.1.5.2-84175989 Delta-Lake/2.2.0.1",
    "txnId": "4d8f952b-e3dd-4390-affc-ee2d8075a843"
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
*/
