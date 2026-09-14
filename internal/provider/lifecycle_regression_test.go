package provider

import (
	"context"
	"encoding/json"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/studio-ch/terraform-provider-xcloud/internal/client"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestAuditRealEmpty202Lifecycle(t *testing.T) {
	for _, action := range []string{"start", "stop"} {
		t.Run(action, func(t *testing.T) {
			target := "running"
			if action == "stop" {
				target = "stopped"
			}
			reads := 0
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				if req.Method == "POST" {
					w.WriteHeader(202)
					return
				}
				reads++
				_ = json.NewEncoder(w).Encode(map[string]any{"id": testInstance, "status": target, "pendingAction": nil})
			}))
			defer s.Close()
			c, _ := client.New(s.URL, "test-key", "audit")
			c.PollInterval = time.Millisecond
			r := newInstanceResource().(*instanceResource)
			r.client = c
			_, err := r.action(context.Background(), types.StringValue(testInstance), action, target, nil)
			if err != nil {
				t.Fatalf("valid empty 202 must lead to polling, got %v; GET polls=%d", err, reads)
			}
		})
	}
}
func TestAuditInstanceDeletionPreservesSeparateElasticIP(t *testing.T) {
	deleted := false
	release := ""
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method == "DELETE" {
			release = req.URL.Query().Get("releaseElasticIps")
			deleted = true
			w.WriteHeader(202)
			return
		}
		if deleted {
			w.WriteHeader(404)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"id": testInstance, "status": "running", "pendingAction": nil})
	}))
	defer s.Close()
	c, _ := client.New(s.URL, "test-key", "audit")
	c.PollInterval = time.Millisecond
	r := newInstanceResource().(*instanceResource)
	r.client = c
	ctx := context.Background()
	var schema resource.SchemaResponse
	r.Schema(ctx, resource.SchemaRequest{}, &schema)
	state := tfsdk.State{Schema: schema.Schema}
	if ds := state.Set(ctx, &instanceModel{ID: types.StringValue(testInstance), Tags: types.MapNull(types.StringType), SSHKeys: types.SetNull(types.StringType), SecurityGroups: types.SetNull(types.StringType)}); ds.HasError() {
		t.Fatal(ds)
	}
	resp := resource.DeleteResponse{State: state}
	r.Delete(ctx, resource.DeleteRequest{State: state}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatal(resp.Diagnostics)
	}
	if release != "false" {
		t.Fatalf("instance deletion must not release separately managed Elastic IPs: releaseElasticIps=%q (API defaults to true)", release)
	}
}
