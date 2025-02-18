package delta

/**
NewSchema()
AddField()
SchemaField - name, type, nullable
SchemaType - string, int, float, bool, date, time, datetime, array/list, map, struct
ListSchemaType - {type, elementtype, nullableelement}
MapSchemaType - {type, key type, value type, nullablevalue}
struct SchemaType - {type, []schemafield}
*/
/**
datatype
schema = struct
complextype = array, map and struct
const for datatype
schemafield for individual fields. this will be used in the struct schema as well
*/

type (
	SchemaType interface{}
)

type SchemaStruct = Schema

type DataType string

type FieldSchema struct {
	Name      string
	FieldType SchemaType
	Nullable  bool
}

type StructSchema struct {
	schemaType SchemaType
	Fields     []SchemaType
}

type ArraySchema struct {
	schemaType  DataType
	ElementType SchemaType
	Nullable    bool
}

type MapSchema struct {
	schemaType    DataType
	KeyType       DataType
	ValueType     SchemaType
	ValueNullable bool
}

type Schema struct {
	Fields []SchemaType
}

func NewSchema() *Schema {
	return &Schema{}
}

func (s *Schema) AddField(field SchemaType) {
	s.Fields = append(s.Fields, field)
}

const (
	StringType   DataType = "string"
	IntType      DataType = "int"
	FloatType    DataType = "float"
	BoolType     DataType = "bool"
	DateType     DataType = "date"
	TimeType     DataType = "time"
	DateTimeType DataType = "datetime"
	ArrayType    DataType = "array"
	MapType      DataType = "map"
	StructType   DataType = "struct"
)
