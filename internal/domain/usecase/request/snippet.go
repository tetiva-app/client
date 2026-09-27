package request

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"strings"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/domain"
	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/har"
	"github.com/tetiva-app/client/internal/domain/usecase/auth"
	"github.com/tetiva-app/client/internal/domain/usecase/websocket"
)

const (
	preScriptSnippetWarning     = "Pre-request script is not applied to snippets"
	hiddenSecretsSnippetWarning = "Secret values are hidden; turn on Include secret values to insert them"
)

// BuildSnippetInput renders the editor state: nothing is sent, run, opened or acquired.
func (u *usecase) BuildSnippetInput(ctx context.Context, req *entities.Request, opt BuildSnippetOpt) (SnippetInput, error) {
	const funcName = "request.BuildSnippetInput"

	switch req.Protocol {
	case entities.ProtocolHTTP, entities.ProtocolGraphQL, entities.ProtocolGRPC, entities.ProtocolWebSocket:
	default:
		return SnippetInput{}, &domain.ValidationError{Fields: map[string]string{
			"protocol": fmt.Sprintf("unknown protocol %q", req.Protocol),
		}}
	}

	ra, err := u.resolveAuthFor(ctx, req, opt.WorkspaceID)
	if err != nil {
		return SnippetInput{}, fmt.Errorf("%s: %w", funcName, err)
	}

	vars, allVars := map[string]string{}, map[string]string{}
	if opt.ResolveVariables {
		if vars, allVars, err = u.snippetVariables(ctx, opt); err != nil {
			return SnippetInput{}, fmt.Errorf("%s: %w", funcName, err)
		}
	}

	// Protected before auth and URL parsing: url.Parse would turn {{baseUrl}} into %7B%7B…
	ph := har.NewPlaceholders(snippetTexts(req, ra, allVars)...)
	keep := func(name string) bool {
		_, ok := vars[name]
		return ok
	}
	cp := protectRequest(req, ph, keep)
	var authWarnings []string
	if req.Protocol != entities.ProtocolGRPC {
		if ra, authWarnings, err = prepareSnippetAuth(ra, vars, allVars, ph, keep); err != nil {
			return SnippetInput{}, fmt.Errorf("%s: %w", funcName, err)
		}
	}

	preScript, err := u.scriptResolver.ResolvePreScript(ctx, req)
	if err != nil {
		return SnippetInput{}, fmt.Errorf("%s: %w", funcName, err)
	}

	out := SnippetInput{Protocol: req.Protocol}
	if preScript != "" {
		out.Warnings = append(out.Warnings, preScriptSnippetWarning)
	}
	out.Warnings = append(out.Warnings, authWarnings...)

	s := snippetCtx{
		vars: vars,
		ra:   ra,
		opt:  opt,
		prep: prepareOpt{
			TokenFromCacheOnly: opt.ResolveVariables,
			TokenPlaceholder:   !opt.ResolveVariables,
			SkipScripts:        true,
			SkipFileRead:       true,
		},
	}

	var warnings []string
	switch req.Protocol {
	case entities.ProtocolHTTP, entities.ProtocolGraphQL:
		var in har.Input
		if req.Protocol == entities.ProtocolHTTP {
			in, warnings, err = u.snippetHTTP(ctx, cp, s)
		} else {
			in, warnings, err = u.snippetGraphQL(ctx, cp, s)
		}
		if err != nil {
			return SnippetInput{}, fmt.Errorf("%s: %w", funcName, err)
		}
		harReq, buildWarnings := har.Build(in)
		ph.RestoreRequest(&harReq)
		out.HAR = &harReq
		warnings = append(warnings, buildWarnings...)

	case entities.ProtocolGRPC:
		out.GRPC = &GRPCSnippet{
			Target:   ph.Restore(substituteVariables(cp.URL, vars)),
			Service:  req.GRPCService,
			Method:   req.GRPCMethod,
			Message:  ph.Restore(substituteVariables(cp.Body, vars)),
			Metadata: restoreHeaderMap(ph, substituteMetadata(cp.GRPCMetadata, vars)),
		}

	case entities.ProtocolWebSocket:
		var ws WSSnippet
		ws, warnings, err = u.snippetWebSocket(ctx, cp, s)
		if err != nil {
			return SnippetInput{}, fmt.Errorf("%s: %w", funcName, err)
		}
		ws.URL = ph.Restore(ws.URL)
		ws.Headers = restoreHeaderMap(ph, ws.Headers)
		for i, sub := range ws.Subprotocols {
			ws.Subprotocols[i] = ph.Restore(sub)
		}
		for i := range ws.Messages {
			ws.Messages[i].Name = ph.Restore(ws.Messages[i].Name)
			ws.Messages[i].Data = ph.Restore(ws.Messages[i].Data)
		}
		out.WS = &ws
	}

	for _, w := range warnings {
		out.Warnings = append(out.Warnings, ph.Restore(w))
	}
	if leavesSecret(out, vars, allVars) {
		out.Warnings = append(out.Warnings, hiddenSecretsSnippetWarning)
	}
	return out, nil
}

func leavesSecret(in SnippetInput, shown, all map[string]string) bool {
	for _, text := range snippetStrings(in) {
		for _, m := range varPattern.FindAllStringSubmatch(text, -1) {
			_, known := all[m[1]]
			_, printed := shown[m[1]]
			if known && !printed {
				return true
			}
		}
	}
	return false
}

func snippetStrings(in SnippetInput) []string {
	var texts []string
	if r := in.HAR; r != nil {
		texts = append(texts, r.URL, r.BinaryFile)
		for _, nv := range slices.Concat(r.Headers, r.QueryString) {
			texts = append(texts, nv.Name, nv.Value)
		}
		if r.PostData != nil {
			texts = append(texts, r.PostData.Text)
			for _, p := range r.PostData.Params {
				texts = append(texts, p.Name, p.Value, p.FileName)
			}
		}
	}
	if g := in.GRPC; g != nil {
		texts = appendHeaderTexts(append(texts, g.Target, g.Message), g.Metadata)
	}
	if w := in.WS; w != nil {
		texts = appendHeaderTexts(append(append(texts, w.URL), w.Subprotocols...), w.Headers)
		for _, m := range w.Messages {
			texts = append(texts, m.Data)
		}
	}
	return texts
}

func appendHeaderTexts(texts []string, h map[string][]string) []string {
	for k, values := range h {
		texts = append(append(texts, k), values...)
	}
	return texts
}

func (u *usecase) snippetVariables(ctx context.Context, opt BuildSnippetOpt) (map[string]string, map[string]string, error) {
	active, err := u.envResolver.ActiveVariables(ctx, opt.WorkspaceID)
	if err != nil {
		return nil, nil, err
	}
	all := make(map[string]string, len(active))
	secret := map[string]bool{}
	for _, v := range active {
		all[v.Key] = v.Value
		secret[v.Key] = secret[v.Key] || v.IsSecret
	}
	if opt.IncludeSecrets {
		return all, all, nil
	}
	shown := make(map[string]string, len(all))
	for k, v := range all {
		if !secret[k] {
			shown[k] = v
		}
	}
	return shown, all, nil
}

type snippetCtx struct {
	vars map[string]string
	ra   ResolvedAuth
	opt  BuildSnippetOpt
	prep prepareOpt
}

func (u *usecase) snippetHTTP(ctx context.Context, req *entities.Request, s snippetCtx) (har.Input, []string, error) {
	prep, _, err := u.prepareHTTP(ctx, req, s.vars, s.ra, s.prep)
	if err != nil {
		return har.Input{}, nil, err
	}

	in := har.Input{
		Method:  string(prep.Method),
		URL:     prep.URL,
		Headers: prep.Headers,
		Body:    snippetBody(prep),
	}
	if prep.Auth != nil {
		in.AuthNote = string(prep.Auth.Type)
	}
	if s.opt.ResolveVariables {
		u.addJarCookies(ctx, s.opt.WorkspaceID, prep.URL, in.Headers)
	}
	return in, prep.Warnings, nil
}

func (u *usecase) snippetGraphQL(ctx context.Context, req *entities.Request, s snippetCtx) (har.Input, []string, error) {
	headers := substituteHeaders(entities.EnabledHeadersToMap(req.Headers), s.vars)
	endpoint := substituteVariables(req.URL, s.vars)
	headers, endpoint, warnings, err := u.snippetAuth(ctx, s, headers, endpoint)
	if err != nil {
		return har.Input{}, nil, err
	}

	body, bodyWarnings := GraphQLBodyLenient(
		substituteVariables(req.GraphQLQuery, s.vars),
		substituteVariables(req.GraphQLVariables, s.vars),
		req.GraphQLOperation,
	)
	warnings = append(warnings, bodyWarnings...)

	contentType := firstHeader(headers, "Content-Type")
	if contentType == "" {
		contentType = "application/json"
		headers["Content-Type"] = []string{contentType}
	}
	if s.opt.ResolveVariables {
		u.addJarCookies(ctx, s.opt.WorkspaceID, endpoint, headers)
	}

	return har.Input{
		Method:  http.MethodPost,
		URL:     endpoint,
		Headers: headers,
		Body:    har.Body{Kind: har.BodyText, MimeType: contentType, Text: body},
	}, warnings, nil
}

func (u *usecase) snippetWebSocket(ctx context.Context, req *entities.Request, s snippetCtx) (WSSnippet, []string, error) {
	headers := substituteHeaders(entities.EnabledHeadersToMap(req.Headers), s.vars)
	wsURL := substituteVariables(req.URL, s.vars)
	headers, wsURL, warnings, err := u.snippetAuth(ctx, s, headers, wsURL)
	if err != nil {
		return WSSnippet{}, nil, err
	}

	settings := websocket.ParseSettings(req.Body)
	messages := make([]WSSnippetMessage, 0, len(settings.Messages))
	for _, m := range settings.Messages {
		data := m.Data
		if m.Format != "binary" {
			data = substituteVariables(data, s.vars)
		}
		messages = append(messages, WSSnippetMessage{Name: m.Name, Format: m.Format, Data: data})
	}

	return WSSnippet{
		URL:          wsURL,
		Headers:      headers,
		Subprotocols: settings.Subprotocols,
		Messages:     messages,
	}, warnings, nil
}

func (u *usecase) snippetAuth(ctx context.Context, s snippetCtx, headers map[string][]string, rawURL string) (map[string][]string, string, []string, error) {
	if isRequesterAuth(s.ra.Type) {
		return headers, rawURL, []string{requesterAuthLabel(s.ra.Type) + " auth works over HTTP only and is left out of the snippet"}, nil
	}
	fields, err := auth.ParseFields(s.ra.Data)
	if err != nil {
		return nil, "", nil, err
	}
	headers, rawURL, _, warnings, err := u.applyResolvedAuth(ctx, s.ra, auth.Substitute(fields, s.vars), headers, rawURL, s.prep)
	if err != nil {
		return nil, "", nil, err
	}
	return headers, rawURL, warnings, nil
}

func (u *usecase) addJarCookies(ctx context.Context, workspaceID uuid.UUID, rawURL string, headers map[string][]string) {
	if u.cookieReader == nil {
		return
	}
	cookies := u.cookieReader.CookiesFor(ctx, workspaceID, rawURL)
	if len(cookies) == 0 {
		return
	}
	pairs := make([]string, 0, len(cookies))
	for _, c := range cookies {
		pairs = append(pairs, c.Name+"="+c.Value)
	}
	key := headerKey(headers, "Cookie")
	headers[key] = append(headers[key], strings.Join(pairs, "; "))
}

func snippetBody(prep preparedHTTP) har.Body {
	mimeType := firstHeader(prep.Headers, "Content-Type")

	switch prep.BodyType {
	case entities.BodyTypeNone:
		return har.Body{}
	case entities.BodyTypeForm:
		var params []har.Param
		multipart := false
		for _, f := range prep.FormFields {
			if !f.Enabled || f.Key == "" {
				continue
			}
			if f.Type == "file" {
				multipart = true
				if f.Value != "" {
					params = append(params, har.Param{Name: f.Key, FileName: f.Value})
					continue
				}
			}
			params = append(params, har.Param{Name: f.Key, Value: f.Value})
		}
		switch {
		case len(params) == 0:
			return har.Body{}
		case multipart:
			return har.Body{Kind: har.BodyMultipart, MimeType: mimeType, Params: params}
		default:
			return har.Body{Kind: har.BodyURLEncoded, MimeType: mimeType, Params: params}
		}
	case entities.BodyTypeBinary:
		if prep.BinaryPath == "" {
			return har.Body{}
		}
		return har.Body{Kind: har.BodyBinary, MimeType: mimeType, BinaryFile: prep.BinaryPath}
	default:
		if prep.Body == "" {
			return har.Body{}
		}
		return har.Body{Kind: har.BodyText, MimeType: mimeType, Text: prep.Body}
	}
}

func protectRequest(req *entities.Request, ph *har.Placeholders, keep func(string) bool) *entities.Request {
	cp := *req
	cp.URL = ph.Protect(req.URL, keep)
	cp.Body = ph.Protect(req.Body, keep)
	cp.GraphQLQuery = ph.Protect(req.GraphQLQuery, keep)
	cp.GraphQLVariables = ph.Protect(req.GraphQLVariables, keep)
	cp.Headers = make([]entities.HeaderItem, len(req.Headers))
	for i, h := range req.Headers {
		h.Key = ph.Protect(h.Key, keep)
		h.Value = ph.Protect(h.Value, keep)
		cp.Headers[i] = h
	}
	if req.GRPCMetadata != nil {
		cp.GRPCMetadata = make(map[string][]string, len(req.GRPCMetadata))
		for k, values := range req.GRPCMetadata {
			protected := make([]string, len(values))
			for i, v := range values {
				protected[i] = ph.Protect(v, keep)
			}
			cp.GRPCMetadata[ph.Protect(k, keep)] = protected
		}
	}
	return &cp
}

// Variable values land next to the tokens, so they must not look like one either.
func snippetTexts(req *entities.Request, ra ResolvedAuth, vars map[string]string) []string {
	texts := []string{
		req.URL, req.Body, req.AuthData, ra.Data,
		req.GraphQLQuery, req.GraphQLVariables, req.GraphQLOperation,
		req.GRPCService, req.GRPCMethod,
	}
	for _, h := range req.Headers {
		texts = append(texts, h.Key, h.Value)
	}
	for k, values := range req.GRPCMetadata {
		texts = append(texts, k)
		texts = append(texts, values...)
	}
	for _, v := range vars {
		texts = append(texts, v)
	}
	return texts
}

func restoreHeaderMap(ph *har.Placeholders, h map[string][]string) map[string][]string {
	if h == nil {
		return nil
	}
	out := make(map[string][]string, len(h))
	for k, values := range h {
		restored := make([]string, len(values))
		for i, v := range values {
			restored[i] = ph.Restore(v)
		}
		out[ph.Restore(k)] = restored
	}
	return out
}

func headerKey(h map[string][]string, name string) string {
	for k := range h {
		if strings.EqualFold(k, name) {
			return k
		}
	}
	return name
}

func firstHeader(h map[string][]string, name string) string {
	if values := h[headerKey(h, name)]; len(values) > 0 {
		return values[0]
	}
	return ""
}

func requesterAuthLabel(t entities.AuthType) string {
	if t == entities.AuthTypeAWSSigV4 {
		return "AWS Signature V4"
	}
	return "Digest"
}
