package postman

// PostmanCollection represents a Postman Collection v2.1 JSON structure.
type PostmanCollection struct {
	Info  PostmanInfo    `json:"info"`
	Item  []PostmanItem  `json:"item"`
	Auth  *PostmanAuth   `json:"auth,omitempty"`
	Event []PostmanEvent `json:"event,omitempty"`
}

// PostmanInfo contains collection metadata.
type PostmanInfo struct {
	PostmanID   string `json:"_postman_id,omitempty"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Schema      string `json:"schema"`
	ExporterID  string `json:"_exporter_id,omitempty"`
}

// PostmanItem represents either a folder (has Item) or a request (has Request).
type PostmanItem struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Item        []PostmanItem   `json:"item,omitempty"`
	Request     *PostmanRequest `json:"request,omitempty"`
	Auth        *PostmanAuth    `json:"auth,omitempty"`
	Event       []PostmanEvent  `json:"event,omitempty"`
}

// IsFolder returns true if the item is a folder (has nested items).
func (i *PostmanItem) IsFolder() bool {
	return i.Item != nil
}

// PostmanRequest represents a Postman request.
type PostmanRequest struct {
	Method string       `json:"method"`
	Header []PostmanKV  `json:"header"`
	Body   *PostmanBody `json:"body,omitempty"`
	URL    PostmanURL   `json:"url"`
	Auth   *PostmanAuth `json:"auth,omitempty"`
}

// PostmanURL represents a Postman URL.
type PostmanURL struct {
	Raw   string      `json:"raw"`
	Host  []string    `json:"host,omitempty"`
	Path  []string    `json:"path,omitempty"`
	Query []PostmanKV `json:"query,omitempty"`
}

// PostmanKV is a key-value pair used for headers, query params, and form data.
type PostmanKV struct {
	Key      string `json:"key"`
	Value    string `json:"value"`
	Disabled bool   `json:"disabled,omitempty"`
	Type     string `json:"type,omitempty"`
}

// PostmanBody represents the request body.
type PostmanBody struct {
	Mode     string              `json:"mode"`
	Raw      string              `json:"raw,omitempty"`
	Options  *PostmanBodyOpt     `json:"options,omitempty"`
	FormData []PostmanKV         `json:"formdata,omitempty"`
	Graphql  *PostmanGraphQLBody `json:"graphql,omitempty"`
}

// PostmanGraphQLBody holds GraphQL query and variables for the graphql body mode.
type PostmanGraphQLBody struct {
	Query     string `json:"query"`
	Variables string `json:"variables,omitempty"`
}

// PostmanBodyOpt contains body options (language etc).
type PostmanBodyOpt struct {
	Raw *PostmanRawOpt `json:"raw,omitempty"`
}

// PostmanRawOpt specifies the raw body language.
type PostmanRawOpt struct {
	Language string `json:"language,omitempty"`
}

// PostmanAuth represents authentication configuration.
type PostmanAuth struct {
	Type   string      `json:"type"`
	Bearer []PostmanKV `json:"bearer,omitempty"`
	Basic  []PostmanKV `json:"basic,omitempty"`
	APIKey []PostmanKV `json:"apikey,omitempty"`
}

// PostmanEvent represents a pre-request or test script.
type PostmanEvent struct {
	Listen string        `json:"listen"`
	Script PostmanScript `json:"script"`
}

// PostmanScript holds script content.
type PostmanScript struct {
	Type string   `json:"type"`
	Exec []string `json:"exec"`
}

// PostmanEnvironment represents a Postman Environment JSON structure.
type PostmanEnvironment struct {
	ID     string            `json:"id"`
	Name   string            `json:"name"`
	Values []PostmanEnvValue `json:"values"`
	Scope  string            `json:"_postman_variable_scope,omitempty"`
}

// PostmanEnvValue is a single environment variable.
type PostmanEnvValue struct {
	Key     string `json:"key"`
	Value   string `json:"value"`
	Type    string `json:"type,omitempty"`
	Enabled bool   `json:"enabled"`
}

// SchemaV21 is the schema URL for Postman Collection v2.1.
const SchemaV21 = "https://schema.getpostman.com/json/collection/v2.1.0/collection.json"
