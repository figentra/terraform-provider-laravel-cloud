package api

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestEnvelopeUnmarshalJSONAPI_Application verifies the Envelope
// UnmarshalJSON correctly flattens Cloud's JSON:API response into a flat
// Application struct with populated computed attrs. The exact payload
// mirrors what Cloud actually returns from POST/GET /applications.
func TestEnvelopeUnmarshalJSONAPI_Application(t *testing.T) {
	body := `{
	  "data": {
	    "id": "app-abc123",
	    "type": "applications",
	    "attributes": {
	      "name": "identity-service-dev",
	      "slug": "identity-service-dev-a1b2",
	      "region": "us-east-1",
	      "source_control_provider_type": "github",
	      "repository": "figentra-inc/backend",
	      "root_directory": "services/identity"
	    },
	    "relationships": {
	      "organization": {
	        "data": {"id": "org-xyz789", "type": "organizations"}
	      }
	    }
	  }
	}`
	var env Envelope[Application]
	if err := json.Unmarshal([]byte(body), &env); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if env.Data.ID != "app-abc123" {
		t.Errorf("ID: want=app-abc123 got=%q", env.Data.ID)
	}
	if env.Data.Name != "identity-service-dev" {
		t.Errorf("Name: want=identity-service-dev got=%q", env.Data.Name)
	}
	if env.Data.Region != "us-east-1" {
		t.Errorf("Region: want=us-east-1 got=%q", env.Data.Region)
	}
	if env.Data.SourceControlProviderType != "github" {
		t.Errorf("SourceControlProviderType: want=github got=%q", env.Data.SourceControlProviderType)
	}
}

// TestEnvelopeUnmarshalJSONAPI_DatabaseSchemaRelationship verifies that
// JSON:API relationships get hoisted to `<name>_id` attribute keys —
// specifically that `data.relationships.cluster.data.id` populates
// `DatabaseSchema.ClusterID`.
func TestEnvelopeUnmarshalJSONAPI_DatabaseSchemaRelationship(t *testing.T) {
	body := `{
	  "data": {
	    "id": "1320469",
	    "type": "databases",
	    "attributes": {
	      "name": "figentra_commerce_service_dev",
	      "status": "available"
	    },
	    "relationships": {
	      "cluster": {"data": {"id": "frosty-scene-09794276", "type": "database-clusters"}}
	    }
	  }
	}`
	var env Envelope[DatabaseSchema]
	if err := json.Unmarshal([]byte(body), &env); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if env.Data.ID != "1320469" {
		t.Errorf("ID: want=1320469 got=%q", env.Data.ID)
	}
	if env.Data.Name != "figentra_commerce_service_dev" {
		t.Errorf("Name: want=figentra_commerce_service_dev got=%q", env.Data.Name)
	}
	if env.Data.ClusterID != "frosty-scene-09794276" {
		t.Errorf("ClusterID: want=frosty-scene-09794276 got=%q (relationships hoist broken)", env.Data.ClusterID)
	}
}

// TestCamelToSnake verifies the helper handles the naming shapes Cloud
// actually uses.
func TestCamelToSnake(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"cluster", "cluster"},
		{"organization", "organization"},
		{"defaultEnvironment", "default_environment"},
		{"WebsocketCluster", "websocket_cluster"},
		{"already_snake", "already_snake"},
		{"", ""},
	} {
		if got := camelToSnake(tc.in); got != tc.want {
			t.Errorf("camelToSnake(%q): want=%q got=%q", tc.in, tc.want, got)
		}
	}
}

// TestEnvelopeUnmarshalJSONAPI_Cache verifies the Cache resource's `type`
// field reads the engine type from `attributes.type` (`"laravel_valkey"`),
// not the JSON:API discriminator `data.type` (`"caches"`).
func TestEnvelopeUnmarshalJSONAPI_Cache(t *testing.T) {
	body := `{
	  "data": {
	    "id": "cache-a2774c65",
	    "type": "caches",
	    "attributes": {
	      "name": "figentra-platform-service-dev",
	      "type": "laravel_valkey",
	      "region": "us-east-1",
	      "size": "valkey-pro.1gb",
	      "auto_upgrade_enabled": true,
	      "is_public": false
	    }
	  }
	}`
	var env Envelope[Cache]
	if err := json.Unmarshal([]byte(body), &env); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if env.Data.ID != "cache-a2774c65" {
		t.Errorf("ID: want=cache-a2774c65 got=%q", env.Data.ID)
	}
	if env.Data.Name != "figentra-platform-service-dev" {
		t.Errorf("Name: want=figentra-platform-service-dev got=%q", env.Data.Name)
	}
	if env.Data.Type == nil || *env.Data.Type != "laravel_valkey" {
		got := "<nil>"
		if env.Data.Type != nil {
			got = *env.Data.Type
		}
		t.Errorf("Type: want=laravel_valkey got=%q (should be attribute value, NOT the discriminator 'caches')", got)
	}
	if env.Data.IsPublic == nil || *env.Data.IsPublic != false {
		t.Errorf("IsPublic: want=false got=%v", env.Data.IsPublic)
	}
}

// TestEnvelopeUnmarshalJSONAPI_FlatRESTFallback verifies flat-REST payloads
// (no `attributes` sub-object) pass through unchanged.
func TestEnvelopeUnmarshalJSONAPI_FlatRESTFallback(t *testing.T) {
	body := `{"data": {"id": "org-1", "name": "Figentra", "slug": "figentra"}}`
	var env Envelope[Organization]
	if err := json.Unmarshal([]byte(body), &env); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if env.Data.ID != "org-1" || env.Data.Name != "Figentra" || env.Data.Slug != "figentra" {
		t.Errorf("flat REST fallback broken: %+v", env.Data)
	}
}

// TestEnvelopeUnmarshalJSONAPI_ListResponse verifies list responses
// (`data` is array) each item gets flattened.
func TestEnvelopeUnmarshalJSONAPI_ListResponse(t *testing.T) {
	body := `{
	  "data": [
	    {"id": "cache-1", "type": "caches", "attributes": {"name": "one", "size": "small"}},
	    {"id": "cache-2", "type": "caches", "attributes": {"name": "two", "size": "large"}}
	  ]
	}`
	var env Envelope[[]Cache]
	if err := json.Unmarshal([]byte(body), &env); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if len(env.Data) != 2 {
		t.Fatalf("list len: want=2 got=%d", len(env.Data))
	}
	if env.Data[0].ID != "cache-1" || env.Data[0].Name != "one" {
		t.Errorf("[0]: want id=cache-1 name=one got id=%q name=%q", env.Data[0].ID, env.Data[0].Name)
	}
	if env.Data[1].ID != "cache-2" || env.Data[1].Name != "two" {
		t.Errorf("[1]: want id=cache-2 name=two got id=%q name=%q", env.Data[1].ID, env.Data[1].Name)
	}
}

// diagHelper — surface any surprising unmarshal quirks by dumping the
// entire envelope shape on test failure.
func dumpEnvelope[T any](t *testing.T, env Envelope[T]) {
	b, _ := json.MarshalIndent(env, "", "  ")
	t.Logf("envelope: %s", string(b))
	_ = strings.HasPrefix // keep strings import silenced if unused
}
