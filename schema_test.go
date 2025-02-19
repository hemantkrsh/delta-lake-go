package delta

import (
	"encoding/json"
	"testing"
)

func TestSchema(t *testing.T) {
	s := NewSchema()
	if s == nil {
		t.Errorf("Schema is nil")
	}
}

func TestSchema_AddField(t *testing.T) {
	s := NewSchema()
	if s == nil {
		t.Errorf("Schema is nil")
	}
	s.AddField(&FieldSchema{
		Name:      "id",
		FieldType: "int",
		Nullable:  true,
	})
}

func TestSchema_AddField_Struct(t *testing.T) {
	s := NewSchema()
	if s == nil {
		t.Errorf("Schema is nil")
	}
	s.AddField(&FieldSchema{
		Name: "person",
		FieldType: &StructSchema{
			Fields: []SchemaType{
				&FieldSchema{
					Name:      "id",
					FieldType: "int",
					Nullable:  true,
				},
				&FieldSchema{
					Name:      "name",
					FieldType: "string",
					Nullable:  true,
				},
				&FieldSchema{
					Name:      "birthdate",
					FieldType: "date",
					Nullable:  true,
				},
			},
		},
		Nullable: true,
	})
}

// copilot generated test case

func TestSchema_AddField_Array(t *testing.T) {
	s := NewSchema()
	if s == nil {
		t.Errorf("Schema is nil")
	}
	s.AddField(&FieldSchema{
		Name: "person",
		FieldType: &ArraySchema{
			SchemaType:  ArrayType,
			ElementType: IntType,
			Nullable:    true,
		},
		Nullable: true,
	})

	jsonSchema, err := json.Marshal(s)
	if err != nil {
		t.Errorf("Failed to marshal schema")
	}

	expected := `{"fields":[{"name":"person","type":{"type":"array","element_type":"int","nullable":true},"nullable":true}]}`
	if string(jsonSchema) != expected {
		t.Errorf("Expected %s, got %s", expected, string(jsonSchema))
	}
}
