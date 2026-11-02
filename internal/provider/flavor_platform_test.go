package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/studio-ch/terraform-provider-xcloud/internal/client"
)

func TestFlavorValidationUsesGuestSystem(t *testing.T) {
	systems := []string{"macos", "linux"}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("platform") != "linux" || r.URL.Query().Get("regionId") != testRegion {
			t.Errorf("wrong catalog query: %s", r.URL.RawQuery)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []any{map[string]any{"slug": "shared", "cpuCores": 2, "memoryGib": 6, "diskGib": 100, "platform": "macos", "supportedPlatforms": systems}}})
	}))
	defer server.Close()
	c, err := client.New(server.URL, "test", "test")
	if err != nil {
		t.Fatal(err)
	}
	resource := instanceResource{baseResource: baseResource{client: c}}
	model := instanceModel{Flavor: types.StringValue("shared"), RegionID: types.StringValue(testRegion), CPU: types.Int64Value(2), Memory: types.Int64Value(6), Disk: types.Int64Value(100), Platform: types.StringValue("linux"), HardwareGeneration: types.StringNull()}
	if err := resource.validateFlavor(context.Background(), model); err != nil {
		t.Fatalf("dual-platform preset rejected: %v", err)
	}
	systems = []string{"macos"}
	if err := resource.validateFlavor(context.Background(), model); err == nil {
		t.Fatal("macOS-only preset accepted for Linux")
	}
}
