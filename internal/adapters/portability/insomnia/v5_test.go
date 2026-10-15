package insomnia_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/environment"
)

func TestImportV5_Collection(t *testing.T) {
	s := newStore()
	res := importFile(t, s, fixture(t, "airlines.insomnia.yaml"), true)

	root, rootID := s.collection(t, "Insomnia Airlines")
	flights, _ := s.collection(t, "Flights")
	assert.Equal(t, rootID, res.CollectionID)
	assert.Equal(t, "Flights and bookings.", root.Description)
	assert.Equal(t, rootID, *flights.ParentID)
	assert.Equal(t, "Search and manage flights.", flights.Description)
	assert.Equal(t, entities.AuthTypeAPIKey, flights.AuthType)
	assert.JSONEq(t, `{"key":"X-Api-Key","value":"{{api_key}}","addTo":"query"}`, flights.AuthData)
	assert.Equal(t, "insomnia.environment.set('started', Date.now());", flights.PreScript)

	assert.Equal(t, []string{"Get flight", "Add flight", "Boarding updates", "Departures", "Route chat"}, s.requestNames())
	assert.Equal(t, 1, res.Folders)
	assert.Equal(t, 5, res.Requests)
}

func TestImportV5_Requests(t *testing.T) {
	s := newStore()
	importFile(t, s, fixture(t, "airlines.insomnia.yaml"), true)

	get := s.request(t, "Get flight")
	assert.Equal(t, entities.MethodGET, get.Method)
	assert.Equal(t, "{{base_url}}/flights/{{flightId}}?include=crew", get.URL)
	assert.Equal(t, "Returns one flight.", get.Description)
	assert.Equal(t, []entities.HeaderItem{{Key: "Authorization", Value: "Bearer {{auth_token}}", Enabled: true}}, get.Headers)
	assert.Equal(t, entities.AuthTypeInherit, get.AuthType)
	assert.Empty(t, get.PreScript)
	assert.Contains(t, get.PostScript, "insomnia.test('Status is 200'")

	add := s.request(t, "Add flight")
	assert.Equal(t, entities.BodyTypeJSON, add.BodyType)
	assert.Equal(t, "{\n  \"number\": \"IA{% prompt 'Flight number', '', '101' %}\",\n  \"origin\": \"{{origin}}\"\n}", add.Body)
	assert.Equal(t, entities.AuthTypeBasic, add.AuthType)
	assert.JSONEq(t, `{"username":"{{ops-user}}","password":""}`, add.AuthData)

	ws := s.request(t, "Boarding updates")
	assert.Equal(t, entities.ProtocolWebSocket, ws.Protocol)
	assert.Equal(t, "wss://{{ws_host}}/boarding", ws.URL)
	assert.Equal(t, []entities.HeaderItem{{Key: "X-Gate", Value: "B12", Enabled: true}}, ws.Headers)
	assert.JSONEq(t, `{"token":"{{auth_token}}"}`, ws.AuthData)
	assert.Empty(t, ws.Body)

	g := s.request(t, "Route chat")
	assert.Equal(t, entities.ProtocolGRPC, g.Protocol)
	assert.Equal(t, "localhost:50051", g.URL)
	assert.Equal(t, "routeguide.RouteGuide", g.GRPCService)
	assert.Equal(t, "RouteChat", g.GRPCMethod)
	assert.Equal(t, "{\n\t\"message\": \"{{greeting}}\"\n}", g.Body)
	assert.Equal(t, map[string][]string{"x-crew": {"{{crew_id}}"}}, g.GRPCMetadata)

	gql := s.request(t, "Departures")
	assert.Equal(t, entities.ProtocolGraphQL, gql.Protocol)
	assert.Equal(t, "query($airport: String) {\n\tdepartures(airport: $airport) { number }\n}", gql.GraphQLQuery)
	assert.Equal(t, `{ "airport": "{{origin}}" }`, gql.GraphQLVariables)
	assert.Equal(t, entities.AuthTypeNone, gql.AuthType)
}

func TestImportV5_Environments(t *testing.T) {
	s := newStore()
	res := importFile(t, s, fixture(t, "airlines.insomnia.yaml"), false)

	assert.Equal(t, []string{"Insomnia Airlines · Staging", "Insomnia Airlines · Empty Env"}, res.EnvironmentNames)
	base := []environment.AddVariable{
		{Key: "base_url", Value: "https://{{host}}/api"},
		{Key: "ops-user", Value: "dispatcher"},
		{Key: "auth_token", Value: ""},
		{Key: "default_flight", Value: "IA101"},
		{Key: "crew.captain", Value: "Rivera"},
	}
	flightID := environment.AddVariable{Key: "flightId", Value: "{{default_flight}}"}
	assert.Equal(t, append(append([]environment.AddVariable{}, base...),
		environment.AddVariable{Key: "host", Value: "staging.airlines.example"},
		environment.AddVariable{Key: "ws_host", Value: "ws.staging.airlines.example"},
		flightID,
	), s.vars["Insomnia Airlines · Staging"])
	assert.Equal(t, append(append([]environment.AddVariable{}, base...), flightID), s.vars["Insomnia Airlines · Empty Env"])
}

func TestImportV5_Warnings(t *testing.T) {
	s := newStore()
	res := importFile(t, s, fixture(t, "airlines.insomnia.yaml"), false)

	assert.Equal(t, []string{
		`folder "Flights": folder environment variables were not imported`,
		`request "Departures": auth type "hawk" is not supported and was imported as no auth`,
		`1 disabled query parameter was dropped: Tetiva keeps query parameters in the URL`,
		`gRPC requests were imported without their proto files: pick a .proto file or use server reflection`,
		`1 Insomnia template tag was left as-is (prompt): Tetiva does not run it, use a variable or a script instead`,
		`1 Socket.IO request was skipped: Tetiva does not support Socket.IO`,
		`1 unit test was not imported`,
	}, res.Warnings)
}

func TestImportV4YAML(t *testing.T) {
	s := newStore()
	res := importFile(t, s, fixture(t, "sleep.insomnia.yaml"), false)

	_, rootID := s.collection(t, "Insomnia")
	actions, _ := s.collection(t, "Actions")
	assert.Equal(t, rootID, *actions.ParentID)
	assert.Equal(t, "{{base_url}}/sleep", s.request(t, "Get Sleep").URL)
	assert.Equal(t, []string{"Insomnia · Insomnia Environment"}, res.EnvironmentNames)
	assert.Equal(t, []environment.AddVariable{
		{Key: "base_url", Value: "{{scheme}}://{{host}}{{base_path}}"},
		{Key: "base_path", Value: ""},
		{Key: "scheme", Value: "http"},
		{Key: "host", Value: "127.0.0.1:4010"},
	}, s.vars["Insomnia · Insomnia Environment"])
	assert.Empty(t, res.Warnings)
}
