package openapi

import (
	"bytes"
	"encoding/json"
	"strings"

	"github.com/getkin/kin-openapi/openapi2"
	"github.com/getkin/kin-openapi/openapi2conv"
	"github.com/getkin/kin-openapi/openapi3"
	"gopkg.in/yaml.v3"

	"github.com/tetiva-app/client/internal/adapters/portability"
	"github.com/tetiva-app/client/internal/domain"
)

type spec struct {
	doc      *openapi3.T
	src      *source
	warnings []portability.Warning
}

func detect(data []byte) bool {
	var p struct {
		OpenAPI string         `json:"openapi" yaml:"openapi"`
		Swagger string         `json:"swagger" yaml:"swagger"`
		Info    map[string]any `json:"info" yaml:"info"`
	}
	if json.Unmarshal(data, &p) != nil && yaml.Unmarshal(data, &p) != nil {
		return false
	}
	return p.Info != nil && (strings.HasPrefix(p.OpenAPI, "3.") || p.Swagger == "2.0")
}

func load(data []byte) (*spec, error) {
	src, err := parseSource(data)
	if err != nil {
		return nil, unreadable(err)
	}
	warnings := src.stubBrokenRefs()
	src.keepText()
	var buf bytes.Buffer
	if err := writeJSON(&buf, src.root, maxDocumentJSON); err != nil {
		return nil, unreadable(err)
	}

	loader := openapi3.NewLoader()
	var doc *openapi3.T
	if src.swagger2 {
		var v2 openapi2.T
		if err := json.Unmarshal(buf.Bytes(), &v2); err != nil {
			return nil, unreadable(err)
		}
		if doc, err = openapi2conv.ToV3(&v2); err != nil {
			return nil, unreadable(err)
		}
		err = loader.ResolveRefsIn(doc, nil)
	} else {
		doc, err = loader.LoadFromData(buf.Bytes())
	}
	if err != nil {
		return nil, unreadable(err)
	}
	return &spec{doc: doc, src: src, warnings: warnings}, nil
}

func unreadable(err error) error {
	return &domain.ValidationError{Fields: map[string]string{"content": "the OpenAPI document can't be read: " + err.Error()}}
}
