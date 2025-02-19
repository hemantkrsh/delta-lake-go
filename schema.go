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
	Name      string     `json:"name"`
	FieldType SchemaType `json:"type"`
	Nullable  bool       `json:"nullable"`
}

type StructSchema struct {
	SchemaType SchemaType   `json:"type"`
	Fields     []SchemaType `json:"fields"`
}

type ArraySchema struct {
	SchemaType  DataType   `json:"type"`
	ElementType SchemaType `json:"element_type"`
	Nullable    bool       `json:"nullable"`
}

type MapSchema struct {
	SchemaType    DataType   `json:"type"`
	KeyType       DataType   `json:"key_type"`
	ValueType     SchemaType `json:"value_type"`
	ValueNullable bool       `json:"value_nullable"`
}

type Schema struct {
	Fields []SchemaType `json:"fields"`
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
