package delta

type Schema struct{}

/**
NewSchema()
AddField()
SchemaField - name, type, nullable
SchemaType - string, int, float, bool, date, time, datetime, array/list, map, struct
ListSchemaType - {type, elementtype, nullableelement}
MapSchemaType - {type, key type, value type, nullablevalue}
struct SchemaType - {type, []schemafield}
*/

type (
	SchemaDataType    interface{}
	SchemDataTypeName string
)

type SchemaStruct = Schema
