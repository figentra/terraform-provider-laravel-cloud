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

import "time"

// Application represents a Laravel Cloud application record — the top-level
// deploy unit under an organisation. One Cloud app maps to one workspace
// service (identity, commerce, api, ai, ...) or one product surface.
//
// Fields mirror the Cloud API response shape verbatim; provider schema
// declares the Terraform-facing names alongside their JSON tags.
type Application struct {
	ID                        string     `json:"id"`
	Name                      string     `json:"name"`
	Slug                      string     `json:"slug"`
	Region                    string     `json:"region"`
	SourceControlProviderType string     `json:"source_control_provider_type"`
	Repository                *string    `json:"repository"`
	ClusterID                 *string    `json:"cluster_id"`
	SlackChannel              *string    `json:"slack_channel"`
	AvatarURL                 *string    `json:"avatar_url"`
	CreatedAt                 *time.Time `json:"created_at"`
	UpdatedAt                 *time.Time `json:"updated_at"`

	// Included relationships when the caller passes
	// `?include=organization,environments,defaultEnvironment`. Nil when
	// the include wasn't requested.
	Organization       *Organization       `json:"organization,omitempty"`
	Environments       []Environment       `json:"environments,omitempty"`
	DefaultEnvironment *Environment        `json:"default_environment,omitempty"`
}

// CreateApplicationRequest is the POST /applications body.
type CreateApplicationRequest struct {
	OrganizationID            string  `json:"organization_id"`
	Name                      string  `json:"name"`
	Region                    string  `json:"region"`
	SourceControlProviderType string  `json:"source_control_provider_type"`
	Repository                *string `json:"repository,omitempty"`
	ClusterID                 *string `json:"cluster_id,omitempty"`
	SlackChannel              *string `json:"slack_channel,omitempty"`
}

// UpdateApplicationRequest is the PATCH /applications/:id body. Every field
// is a pointer so operators can partial-update without wiping unset fields.
type UpdateApplicationRequest struct {
	Name         *string `json:"name,omitempty"`
	Repository   *string `json:"repository,omitempty"`
	SlackChannel *string `json:"slack_channel,omitempty"`
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
type Envelope[T any] struct {
	Data     T                `json:"data"`
	Included []map[string]any `json:"included,omitempty"`
	Meta     map[string]any   `json:"meta,omitempty"`
}

// ErrorResponse is the shape Cloud returns on 4xx/5xx. Provider resources
// surface the `Message` in the Terraform diagnostic; the raw body is added
// to the diagnostic detail for debugging.
type ErrorResponse struct {
	Message string              `json:"message"`
	Errors  map[string][]string `json:"errors,omitempty"`
}
