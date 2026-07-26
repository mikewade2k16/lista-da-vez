package customerdata

const SegmentFieldCatalogVersion = "customer_data.segment_fields.v1"

var segmentFields = map[string]SegmentFieldDefinition{
	"relationship.lifecycle_status": {
		FieldKey: "relationship.lifecycle_status", DataType: "string",
		Operators: []string{"eq", "neq", "in", "not_in"}, Classification: "internal", Local: true,
	},
	"relationship.display_name": {
		FieldKey: "relationship.display_name", DataType: "string",
		Operators:      []string{"eq", "neq", "contains", "prefix", "exists", "not_exists"},
		Classification: "personal", Local: true,
	},
	"relationship.tag": {
		FieldKey: "relationship.tag", DataType: "string",
		Operators: []string{"eq", "neq", "in", "not_in"}, Classification: "internal", Local: true,
	},
	"relationship.owner_user_id": {
		FieldKey: "relationship.owner_user_id", DataType: "string",
		Operators:      []string{"eq", "neq", "in", "not_in", "exists", "not_exists"},
		Classification: "internal", Local: true,
	},
	"relationship.last_seen_at": {
		FieldKey: "relationship.last_seen_at", DataType: "datetime",
		Operators:      []string{"before", "after", "between", "within_last", "exists", "not_exists"},
		Classification: "internal", Local: true,
	},
	"relationship.created_at": {
		FieldKey: "relationship.created_at", DataType: "datetime",
		Operators: []string{"before", "after", "between", "within_last"}, Classification: "internal", Local: true,
	},
	"relationship.archived": {
		FieldKey: "relationship.archived", DataType: "boolean",
		Operators: []string{"is_true", "is_false"}, Classification: "internal", Local: true,
	},
}

func CurrentSegmentFieldCatalog() SegmentFieldCatalog {
	order := []string{
		"relationship.lifecycle_status",
		"relationship.display_name",
		"relationship.tag",
		"relationship.owner_user_id",
		"relationship.last_seen_at",
		"relationship.created_at",
		"relationship.archived",
	}
	fields := make([]SegmentFieldDefinition, 0, len(order))
	for _, key := range order {
		fields = append(fields, segmentFields[key])
	}
	return SegmentFieldCatalog{Version: SegmentFieldCatalogVersion, Fields: fields}
}
