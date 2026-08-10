package api

import (
	"context"
	"errors"
	"fmt"
)

// HstsSettings maps to the SDK's `HstsData` — a nested object inside the
// environment's response_headers_hsts field. Every field is a pointer so
// omit-when-unset works cleanly on the wire.
type HstsSettings struct {
	MaxAge            *int  `json:"max_age,omitempty"`
	IncludeSubdomains *bool `json:"include_subdomains,omitempty"`
	Preload           *bool `json:"preload,omitempty"`
}

// EnvironmentNetworkSettings is the read-side shape Cloud returns under
// `data.attributes.network_settings` on GET /environments/:id. The write
// side is FLAT — the fields on UpdateEnvironmentNetworkSettingsRequest
// map to top-level env-update attributes (cache_strategy,
// response_headers_frame, ...). This asymmetry mirrors the SDK's
// NetworkSettingsData/UpdateEnvironmentData split.
type EnvironmentNetworkSettings struct {
	CacheStrategy              string        `json:"cache_strategy"`
	ResponseHeadersFrame       string        `json:"response_headers_frame"`
	ResponseHeadersContentType string        `json:"response_headers_content_type"`
	ResponseHeadersRobotsTag   string        `json:"response_headers_robots_tag"`
	ResponseHeadersHsts        *HstsSettings `json:"response_headers_hsts,omitempty"`
	FirewallRateLimitLevel     *string       `json:"firewall_rate_limit_level,omitempty"`
	FirewallUnderAttackMode    bool          `json:"firewall_under_attack_mode"`
}

// UpdateEnvironmentNetworkSettingsRequest is a focused PATCH body for the
// network-settings fields on `PATCH /environments/:id`. Every field is a
// pointer — Cloud's env update endpoint supports partial writes.
type UpdateEnvironmentNetworkSettingsRequest struct {
	CacheStrategy              *string       `json:"cache_strategy,omitempty"`
	ResponseHeadersFrame       *string       `json:"response_headers_frame,omitempty"`
	ResponseHeadersContentType *string       `json:"response_headers_content_type,omitempty"`
	ResponseHeadersRobotsTag   *string       `json:"response_headers_robots_tag,omitempty"`
	ResponseHeadersHsts        *HstsSettings `json:"response_headers_hsts,omitempty"`
	FirewallRateLimitLevel     *string       `json:"firewall_rate_limit_level,omitempty"`
	FirewallUnderAttackMode    *bool         `json:"firewall_under_attack_mode,omitempty"`
}

// networkSettingsReadShape captures the NESTED shape Cloud returns on
// GET /environments/:id under `data.attributes.network_settings`. Parsed
// separately from the flat write shape because the two are wire-asymmetric.
type networkSettingsReadShape struct {
	Cache struct {
		Strategy string `json:"strategy"`
	} `json:"cache"`
	ResponseHeaders struct {
		Frame       string `json:"frame"`
		ContentType string `json:"content_type"`
		RobotsTag   string `json:"robots_tag"`
		Hsts        struct {
			MaxAge            *int `json:"max_age"`
			IncludeSubdomains bool `json:"include_subdomains"`
			Preload           bool `json:"preload"`
		} `json:"hsts"`
	} `json:"response_headers"`
	Firewall struct {
		RateLimit struct {
			Level *string `json:"level"`
		} `json:"rate_limit"`
		UnderAttackModeStartedAt *string `json:"under_attack_mode_started_at"`
	} `json:"firewall"`
}

// envNetworkSettingsWrapper is the envelope shape Cloud returns on
// GET /environments/:id — we extract only the network_settings block.
type envNetworkSettingsWrapper struct {
	NetworkSettings networkSettingsReadShape `json:"network_settings"`
}

// GetEnvironmentNetworkSettings reads the network settings from an env.
// This is a focused view on GET /environments/:id/network-settings —
// Cloud exposes the block as `data.attributes.network_settings`.
//
// Wire: GET /environments/:id
// Success: HTTP 200 with the enveloped record.
func (c *Client) GetEnvironmentNetworkSettings(ctx context.Context, environmentID string) (*EnvironmentNetworkSettings, error) {
	if environmentID == "" {
		return nil, errors.New("environment id is required")
	}
	path := fmt.Sprintf("/environments/%s", environmentID)

	var env Envelope[envNetworkSettingsWrapper]
	if err := c.do(ctx, "GET", path, nil, &env); err != nil {
		return nil, err
	}
	raw := env.Data.NetworkSettings

	// Flatten nested read shape → flat provider-facing shape.
	out := &EnvironmentNetworkSettings{
		CacheStrategy:              raw.Cache.Strategy,
		ResponseHeadersFrame:       raw.ResponseHeaders.Frame,
		ResponseHeadersContentType: raw.ResponseHeaders.ContentType,
		ResponseHeadersRobotsTag:   raw.ResponseHeaders.RobotsTag,
		FirewallRateLimitLevel:     raw.Firewall.RateLimit.Level,
		FirewallUnderAttackMode:    raw.Firewall.UnderAttackModeStartedAt != nil,
	}
	// HSTS block — only surface when meaningful (max_age set OR a boolean
	// flag toggled). All-zero HSTS from an env that never configured it
	// reads as `nil` on our side so terraform doesn't churn on empty.
	if raw.ResponseHeaders.Hsts.MaxAge != nil ||
		raw.ResponseHeaders.Hsts.IncludeSubdomains ||
		raw.ResponseHeaders.Hsts.Preload {
		out.ResponseHeadersHsts = &HstsSettings{
			MaxAge:            raw.ResponseHeaders.Hsts.MaxAge,
			IncludeSubdomains: boolPtr(raw.ResponseHeaders.Hsts.IncludeSubdomains),
			Preload:           boolPtr(raw.ResponseHeaders.Hsts.Preload),
		}
	}
	return out, nil
}

// UpdateEnvironmentNetworkSettings PATCHes the env's network fields.
// Fields with nil pointers are omitted from the wire body — Cloud
// preserves their existing values.
//
// Wire: PATCH /environments/:id
// Success: HTTP 200 with the full env record.
func (c *Client) UpdateEnvironmentNetworkSettings(ctx context.Context, environmentID string, req UpdateEnvironmentNetworkSettingsRequest) error {
	if environmentID == "" {
		return errors.New("environment id is required")
	}
	path := fmt.Sprintf("/environments/%s", environmentID)

	// We don't need the response body — the read path re-fetches for
	// consistency. `nil` for the response argument tells `do()` to
	// discard the body.
	if err := c.do(ctx, "PATCH", path, req, nil); err != nil {
		return fmt.Errorf("update environment network settings: %w", err)
	}
	return nil
}

// boolPtr is a small helper — plugin-framework and API-side both need
// pointer-to-bool for optional fields.
func boolPtr(b bool) *bool { return &b }
