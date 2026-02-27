package httpapi

type Meta struct {
	APIVersion    string `json:"api_version"`
	SchemaVersion int    `json:"schema_version"`
}

func NewMeta(schemaVersion int) Meta {
	if schemaVersion <= 0 {
		schemaVersion = 1
	}
	return Meta{
		APIVersion:    "v1",
		SchemaVersion: schemaVersion,
	}
}
