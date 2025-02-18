package delta

import (
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
