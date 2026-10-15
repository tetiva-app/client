package hoppscotch

import "encoding/json"

type hoppCollection struct {
	Name             string           `json:"name"`
	Description      *string          `json:"description"`
	Folders          []hoppCollection `json:"folders"`
	Requests         []hoppRequest    `json:"requests"`
	Auth             *hoppAuth        `json:"auth"`
	Headers          []hoppKV         `json:"headers"`
	Variables        []hoppVariable   `json:"variables"`
	PreRequestScript string           `json:"preRequestScript"`
	TestScript       string           `json:"testScript"`
}

type hoppRequest struct {
	Name             string                  `json:"name"`
	Description      *string                 `json:"description"`
	Endpoint         *string                 `json:"endpoint"`
	Method           string                  `json:"method"`
	Params           []hoppKV                `json:"params"`
	Headers          []hoppKV                `json:"headers"`
	Body             json.RawMessage         `json:"body"`
	Auth             *hoppAuth               `json:"auth"`
	PreRequestScript string                  `json:"preRequestScript"`
	TestScript       string                  `json:"testScript"`
	RequestVariables []hoppKV                `json:"requestVariables"`
	Responses        map[string]hoppResponse `json:"responses"`

	URL       string  `json:"url"`
	Query     *string `json:"query"`
	Variables string  `json:"variables"`
}

type hoppKV struct {
	Key    string `json:"key"`
	Value  string `json:"value"`
	Active *bool  `json:"active"`
}

func (kv hoppKV) enabled() bool { return kv.Active == nil || *kv.Active }

type hoppVariable struct {
	Key          string `json:"key"`
	InitialValue string `json:"initialValue"`
	CurrentValue string `json:"currentValue"`
	Secret       bool   `json:"secret"`
}

type hoppBody struct {
	ContentType *string         `json:"contentType"`
	Body        json.RawMessage `json:"body"`
}

type hoppFormField struct {
	Key    string          `json:"key"`
	Value  json.RawMessage `json:"value"`
	Active *bool           `json:"active"`
	IsFile bool            `json:"isFile"`
}

type hoppResponse struct {
	Name    string   `json:"name"`
	Status  string   `json:"status"`
	Code    *int     `json:"code"`
	Headers []hoppKV `json:"headers"`
	Body    string   `json:"body"`
}

type hoppAuth struct {
	AuthType   string `json:"authType"`
	AuthActive *bool  `json:"authActive"`

	Username string `json:"username"`
	Password string `json:"password"`
	Token    string `json:"token"`

	Key   string `json:"key"`
	Value string `json:"value"`
	AddTo string `json:"addTo"`

	GrantTypeInfo *hoppGrant `json:"grantTypeInfo"`

	AccessKey    string `json:"accessKey"`
	SecretKey    string `json:"secretKey"`
	Region       string `json:"region"`
	ServiceName  string `json:"serviceName"`
	ServiceToken string `json:"serviceToken"`

	Secret                string `json:"secret"`
	PrivateKey            string `json:"privateKey"`
	Algorithm             string `json:"algorithm"`
	Payload               string `json:"payload"`
	JWTHeaders            string `json:"jwtHeaders"`
	HeaderPrefix          string `json:"headerPrefix"`
	ParamName             string `json:"paramName"`
	IsSecretBase64Encoded bool   `json:"isSecretBase64Encoded"`
}

type hoppGrant struct {
	GrantType            string `json:"grantType"`
	AuthEndpoint         string `json:"authEndpoint"`
	TokenEndpoint        string `json:"tokenEndpoint"`
	ClientID             string `json:"clientID"`
	ClientSecret         string `json:"clientSecret"`
	Scopes               string `json:"scopes"`
	Username             string `json:"username"`
	Password             string `json:"password"`
	ClientAuthentication string `json:"clientAuthentication"`
}
