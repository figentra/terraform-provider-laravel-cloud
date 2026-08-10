// Package api houses the HTTP client + typed request/response shapes for
// the Laravel Cloud REST API. Every provider resource routes writes through
// this package; the provider layer converts Terraform state <-> API DTOs.
//
// Wire format follows Cloud's JSON:API-ish envelope: `{"data": {...},
// "included": [...]}` for singletons, `{"data": [...], "links": {...},
// "meta": {...}}` for collections.
//
// Base URL default: https://cloud.laravel.com/api
// Auth: Bearer token via `Authorization: Bearer <token>`
// Content-Type: application/json on requests + responses
package api

import (
	"encoding/json"
	"fmt"
	"time"
)

// Application represents a Laravel Cloud application record — the top-level
// deploy unit under an organisation. One Cloud app maps to one workspace
// service (identity, commerce, api, ai, ...) or one product surface.
//
// Fields mirror the Cloud API response shape verbatim; provider schema
// declares the Terraform-facing names alongside their JSON tags.
type Application struct {
	ID string `json:"id"`
	// OrganizationID is populated by the Envelope's JSON:API flatten from
	// `data.relationships.organization.data.id` when the response body
	// carries the organization relationship. Provider code prefers this
	// over the `?include=organization` included-resource block for
	// simplicity — a relationship-only response is enough to get the FK.
	OrganizationID            string     `json:"organization_id,omitempty"`
	Name                      string     `json:"name"`
	Slug                      string     `json:"slug"`
	Region                    string     `json:"region"`
	SourceControlProviderType string     `json:"source_control_provider_type"`
	Repository                *string    `json:"repository"`
	// RootDirectory is the sub-directory inside the repo Cloud treats as
	// the build root. Common values: `apps/api`, `services/identity`,
	// `packages/dashboard`. Nullable — Cloud defaults to the repo root
	// when unset. Added in v0.4.0.
	RootDirectory *string    `json:"root_directory,omitempty"`
	ClusterID     *string    `json:"cluster_id"`
	SlackChannel  *string    `json:"slack_channel"`
	AvatarURL     *string    `json:"avatar_url"`
	CreatedAt     *time.Time `json:"created_at"`
	UpdatedAt     *time.Time `json:"updated_at"`

	// Included relationships when the caller passes
	// `?include=organization,environments,defaultEnvironment`. Nil when
	// the include wasn't requested.
	Organization       *Organization `json:"organization,omitempty"`
	Environments       []Environment `json:"environments,omitempty"`
	DefaultEnvironment *Environment  `json:"default_environment,omitempty"`
}

// UnmarshalJSON handles Cloud's polymorphic `repository` field. As of
// 2026-08-10 Cloud responses for POST /applications return `repository`
// as an object `{"full_name":"owner/repo","default_branch":"main"}` rather
// than a bare string. Older endpoints (GET /applications/:id historically,
// and third-party API forks) still return a string. The provider surfaces
// `repository` as a plain string (`owner/repo`), so this method normalises
// the object form to its `full_name`.
//
// Every other field passes through via a type alias to avoid infinite
// recursion into json.Unmarshal.
func (a *Application) UnmarshalJSON(data []byte) error {
	// Alias eliminates recursion; the field-for-field shape mirrors
	// Application except Repository holds the raw JSON bytes so we can
	// decode it as either a string or an object.
	type applicationAlias Application
	aux := struct {
		Repository json.RawMessage `json:"repository"`
		*applicationAlias
	}{
		applicationAlias: (*applicationAlias)(a),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	if len(aux.Repository) == 0 || string(aux.Repository) == "null" {
		a.Repository = nil
		return nil
	}
	// Try string first (legacy shape).
	var repoStr string
	if err := json.Unmarshal(aux.Repository, &repoStr); err == nil {
		a.Repository = &repoStr
		return nil
	}
	// Fall back to object shape (post-2026-08 wire format).
	var repoObj struct {
		FullName      string `json:"full_name"`
		DefaultBranch string `json:"default_branch"`
	}
	if err := json.Unmarshal(aux.Repository, &repoObj); err != nil {
		return fmt.Errorf("unmarshal application.repository: unsupported shape %q: %w",
			string(aux.Repository), err)
	}
	if repoObj.FullName == "" {
		a.Repository = nil
		return nil
	}
	a.Repository = &repoObj.FullName
	return nil
}

// CreateApplicationRequest is the POST /applications body.
type CreateApplicationRequest struct {
	OrganizationID            string  `json:"organization_id"`
	Name                      string  `json:"name"`
	Region                    string  `json:"region"`
	SourceControlProviderType string  `json:"source_control_provider_type"`
	Repository                *string `json:"repository,omitempty"`
	// RootDirectory added in v0.4.0. Nullable — omitted when unset so
	// Cloud defaults to the repo root.
	RootDirectory *string `json:"root_directory,omitempty"`
	ClusterID     *string `json:"cluster_id,omitempty"`
	SlackChannel  *string `json:"slack_channel,omitempty"`
}

// UpdateApplicationRequest is the PATCH /applications/:id body. Every field
// is a pointer so operators can partial-update without wiping unset fields.
type UpdateApplicationRequest struct {
	Name          *string `json:"name,omitempty"`
	Repository    *string `json:"repository,omitempty"`
	RootDirectory *string `json:"root_directory,omitempty"`
	SlackChannel  *string `json:"slack_channel,omitempty"`
}

// Organization is Cloud's tenant boundary — a group of applications sharing
// billing + team access + audit boundaries. Provider consumers reference by
// slug or ID.
type Organization struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
}

// Environment lives in `environments.go`. Included here as a documentation
// anchor: the `Application` struct's `Environments []Environment` field
// unmarshals via reflection against the type declared in that file.

// Envelope is Cloud's JSON:API-ish response wrapper. Every singleton read
// returns `{"data": <resource>, "included": [...]}` — the `Data` field
// unmarshals into a typed pointer via `json.Unmarshal`.
//
// UnmarshalJSON below transparently flattens JSON:API resource objects of
// the shape `{"id": ..., "type": ..., "attributes": {...}}` into the flat
// shape the downstream `T` structs expect. See §Wire format handling for
// the full contract.
type Envelope[T any] struct {
	Data     T                `json:"data"`
	Included []map[string]any `json:"included,omitempty"`
	Meta     map[string]any   `json:"meta,omitempty"`
}

// UnmarshalJSON handles the Cloud API's JSON:API-style envelope where the
// actual resource attributes live under `data.attributes.*` and the ID sits
// at `data.id`. Given a payload of the shape:
//
//	{"data": {"id": "cache-…", "type": "caches",
//	          "attributes": {"name": "…", "size": "…", …}}}
//
// this method hoists `data.attributes.*` fields up + copies `data.id`, then
// unmarshals the resulting flat object into `T`. When the response is
// already flat REST (`data` contains resource fields directly without an
// `attributes` sub-object), the payload passes through unchanged so this
// wrapper is safe to use across every endpoint.
//
// Supports both singleton (`data` is object) and list (`data` is array)
// responses.
//
// The JSON:API resource-type discriminator (`data.type = "caches"`, etc.)
// is deliberately dropped — several Cloud model structs use `type` for
// domain-specific meaning (`Cache.Type` = engine "laravel_valkey";
// `WebsocketCluster.Type` = "reverb"), which the API places under
// `data.attributes.type`. Copying `data.type` (the resource discriminator)
// would overwrite the domain value.
func (e *Envelope[T]) UnmarshalJSON(b []byte) error {
	// Extract the outer envelope as raw bytes so we can inspect the
	// `data` shape (object vs array vs null).
	var raw struct {
		Data     json.RawMessage  `json:"data"`
		Included []map[string]any `json:"included,omitempty"`
		Meta     map[string]any   `json:"meta,omitempty"`
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	e.Included = raw.Included
	e.Meta = raw.Meta

	// Empty or explicit null — leave e.Data zero-valued.
	if len(raw.Data) == 0 || string(raw.Data) == "null" {
		return nil
	}

	// Detect array vs object by peeking at the first non-whitespace byte.
	first := byte(0)
	for _, c := range raw.Data {
		if c == ' ' || c == '\t' || c == '\n' || c == '\r' {
			continue
		}
		first = c
		break
	}

	if first == '[' {
		// List response — flatten each JSON:API resource item.
		var items []json.RawMessage
		if err := json.Unmarshal(raw.Data, &items); err != nil {
			return err
		}
		flat := make([]json.RawMessage, len(items))
		for i, item := range items {
			f, err := flattenJSONAPIResource(item)
			if err != nil {
				return err
			}
			flat[i] = f
		}
		flatBytes, err := json.Marshal(flat)
		if err != nil {
			return err
		}
		return json.Unmarshal(flatBytes, &e.Data)
	}

	// Singleton — flatten if JSON:API, else pass through as flat REST.
	flatItem, err := flattenJSONAPIResource(raw.Data)
	if err != nil {
		return err
	}
	return json.Unmarshal(flatItem, &e.Data)
}

// flattenJSONAPIResource transforms a JSON:API resource object of the shape
//
//	{
//	  "id":            "…",
//	  "type":          "…",
//	  "attributes":    {…},
//	  "relationships": {"<name>": {"data": {"id": "…", "type": "…"}}, …}
//	}
//
// into a flat object combining `attributes.*` + `id` + one
// `<relationship-name>_id` key per relationship. When the input isn't
// JSON:API (no `attributes` sub-object), returns the input unchanged so
// flat-REST responses fall through untouched.
//
// The JSON:API `data.type` field (resource discriminator like `"caches"` /
// `"applications"`) is deliberately NOT copied — see the Envelope
// UnmarshalJSON rationale above.
//
// Relationship-to-attribute mapping example:
//
//	"relationships": {
//	  "organization": {"data": {"id": "org-abc", "type": "organizations"}},
//	  "cluster":      {"data": {"id": "cluster-xyz", "type": "database-clusters"}}
//	}
//
// becomes:
//
//	"organization_id": "org-abc",
//	"cluster_id":      "cluster-xyz"
//
// The provider's Cache/Application/DatabaseSchema/WebsocketApp structs
// declare `organization_id`, `cluster_id`, etc. as `json:"cluster_id"` —
// so writing the `<name>_id` key into the flat object populates them
// automatically without per-resource wiring.
func flattenJSONAPIResource(b []byte) (json.RawMessage, error) {
	var probe struct {
		ID            string          `json:"id"`
		Type          string          `json:"type"`
		Attributes    json.RawMessage `json:"attributes"`
		Relationships json.RawMessage `json:"relationships"`
	}
	if err := json.Unmarshal(b, &probe); err != nil {
		// Not a JSON object we recognise — pass through untouched.
		return b, nil //nolint:nilerr
	}
	if len(probe.Attributes) == 0 || string(probe.Attributes) == "null" {
		// Flat REST — pass through as-is.
		return b, nil
	}
	// Parse attributes into a map so we can inject `id` + relationships
	// + re-marshal.
	var attrs map[string]json.RawMessage
	if err := json.Unmarshal(probe.Attributes, &attrs); err != nil {
		return nil, err
	}
	if probe.ID != "" {
		idBytes, _ := json.Marshal(probe.ID)
		attrs["id"] = idBytes
	}
	// Hoist JSON:API relationships into `<name>_id` attribute keys.
	if len(probe.Relationships) > 0 && string(probe.Relationships) != "null" {
		var rels map[string]struct {
			Data struct {
				ID   string `json:"id"`
				Type string `json:"type"`
			} `json:"data"`
		}
		if err := json.Unmarshal(probe.Relationships, &rels); err == nil {
			for name, rel := range rels {
				if rel.Data.ID == "" {
					continue
				}
				// snake_case the relationship name + append `_id` so
				// `relationships.organization` → `organization_id`,
				// `relationships.defaultEnvironment` → `default_environment_id`.
				key := camelToSnake(name) + "_id"
				// Do NOT overwrite an attribute-supplied value — some
				// Cloud endpoints put the FK under `attributes.<key>_id`
				// AND redundantly under `relationships.<key>`. When both
				// are present, the attributes value wins.
				if _, exists := attrs[key]; !exists {
					idBytes, _ := json.Marshal(rel.Data.ID)
					attrs[key] = idBytes
				}
			}
		}
	}
	return json.Marshal(attrs)
}

// camelToSnake converts camelCase or PascalCase identifiers into snake_case.
// `defaultEnvironment` → `default_environment`. Idempotent on already-snake
// inputs (`cluster` → `cluster`; `source_control_provider` unchanged).
func camelToSnake(in string) string {
	if in == "" {
		return in
	}
	out := make([]byte, 0, len(in)+4)
	for i := 0; i < len(in); i++ {
		c := in[i]
		if c >= 'A' && c <= 'Z' {
			if i > 0 && in[i-1] != '_' {
				out = append(out, '_')
			}
			c += 'a' - 'A'
		}
		out = append(out, c)
	}
	return string(out)
}

// ErrorResponse is the shape Cloud returns on 4xx/5xx. Provider resources
// surface the `Message` in the Terraform diagnostic; the raw body is added
// to the diagnostic detail for debugging.
type ErrorResponse struct {
	Message string              `json:"message"`
	Errors  map[string][]string `json:"errors,omitempty"`
}
