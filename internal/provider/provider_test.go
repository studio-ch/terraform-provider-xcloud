package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	fwresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/studio-ch/terraform-provider-xcloud/internal/client"
)

const testRegion = "11111111-1111-4111-8111-111111111111"
const testInstance = "22222222-2222-4222-8222-222222222222"
const testVolume = "33333333-3333-4333-8333-333333333333"
const testSSHKey = "44444444-4444-4444-8444-444444444444"
const testIP = "55555555-5555-4555-8555-555555555555"

type mockAPI struct {
	mu                 sync.Mutex
	contract           map[string]any
	t                  *testing.T
	rows               map[string]map[string]any
	calls              []string
	failInstance       bool
	holdInstance       bool
	instanceGeneration int
	dynamicIDs         bool
	largeFlavor        bool
}

func newMockAPI(t *testing.T) (*mockAPI, *httptest.Server) {
	t.Helper()
	m := &mockAPI{rows: map[string]map[string]any{}, contract: loadPublicContract(t), t: t}
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		recorder := httptest.NewRecorder()
		m.serve(recorder, r)
		if err := checkPublicResponse(m.contract, r, recorder); err != nil {
			t.Error(err)
		}
		for key, values := range recorder.Header() {
			w.Header()[key] = values
		}
		w.WriteHeader(recorder.Code)
		_, _ = w.Write(recorder.Body.Bytes())
	}))
	t.Cleanup(s.Close)
	return m, s
}
func (m *mockAPI) serve(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	defer m.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	if r.Header.Get("Authorization") != "Bearer test-key" {
		http.Error(w, `{"detail":"bad bearer token"}`, 401)
		return
	}
	p := r.URL.Path
	m.calls = append(m.calls, r.Method+" "+p)
	send := func(v any) { _ = json.NewEncoder(w).Encode(v) }
	if p == "/v1/regions" {
		send(map[string]any{"data": []any{map[string]any{"id": testRegion, "slug": "ZRH1", "name": "Zurich", "architecture": "arm64", "services": map[string]bool{"xcloud": true}}}})
		return
	}
	if p == "/v1/xcloud/flavors" {
		if m.largeFlavor {
			send(map[string]any{"data": []any{map[string]any{"id": "flavor-id", "slug": "test", "label": "Test", "cpuCores": 2, "memoryGib": 4, "diskGib": 40, "platform": "linux", "hardwareGeneration": nil}, map[string]any{"id": "large-id", "slug": "large", "label": "Large", "cpuCores": 4, "memoryGib": 8, "diskGib": 80, "platform": "linux", "hardwareGeneration": nil}}})
			return
		}
		send(map[string]any{"data": []any{map[string]any{"id": "flavor-id", "slug": "test", "label": "Test", "cpuCores": 2, "memoryGib": 4, "diskGib": 40, "platform": "linux", "hardwareGeneration": nil}}})
		return
	}
	if p == "/v1/xcloud/images" && r.Method == "GET" {
		rows := []any{}
		for key, row := range m.rows {
			if strings.HasPrefix(key, imagesPath+"/") {
				rows = append(rows, row)
			}
		}
		rows = append(rows, map[string]any{"name": "ubuntu", "regionId": testRegion, "source": "default", "ociReference": "registry.example/ubuntu:24.04", "labels": map[string]string{"os": "linux"}})
		send(map[string]any{"data": rows})
		return
	}

	var body map[string]any
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&body)
	}
	if err := checkPublicRequest(m.contract, r, body); err != nil {
		m.t.Error(err)
		http.Error(w, `{"detail":"public API contract mismatch"}`, 400)
		return
	}
	// Instance attachment subroutes.
	if strings.HasPrefix(p, instancesPath+"/"+testInstance+"/volumes") {
		v := m.rows[volumesPath+"/"+testVolume]
		if v == nil {
			http.Error(w, `{"detail":"volume missing"}`, 404)
			return
		}
		if r.Method == "POST" {
			v["state"] = "attached"
			v["attachedInstanceId"] = testInstance
			w.WriteHeader(201)
			send(v)
			return
		}
		if r.Method == "DELETE" {
			v["state"] = "detached"
			v["attachedInstanceId"] = nil
			w.WriteHeader(204)
			return
		}
	}
	// Lifecycle and metadata subroutes.
	base := instancesPath + "/" + testInstance
	if m.dynamicIDs && strings.HasPrefix(p, instancesPath+"/") {
		parts := strings.Split(p, "/")
		if len(parts) >= 5 {
			base = strings.Join(parts[:5], "/")
		}
	}
	if strings.HasPrefix(p, base+"/") {
		row := m.rows[base]
		if row == nil {
			http.Error(w, `{"detail":"instance missing"}`, 404)
			return
		}
		switch strings.TrimPrefix(p, base+"/") {
		case "stop", "shutdown":
			row["status"] = "stopped"
		case "suspend":
			row["status"] = "suspended"
		case "start":
			row["status"] = "running"
		case "resize":
			if row["status"] != "stopped" {
				http.Error(w, `{"detail":"must stop before resize"}`, 409)
				return
			}
			for k, v := range body {
				row[k] = v
			}
		case "boot-mode":
			row["bootIntoRecovery"] = body["recovery"]
		case "tags":
			row["tags"] = body["tags"]
		case "password":
			row["adminPasswordSet"] = true
		case "ssh-keys":
			row["sshKeyIds"] = body["sshKeyIds"]
		case "security-groups":
			row["securityGroups"] = body["securityGroups"]
		default:
			http.Error(w, `{"detail":"unknown action"}`, 404)
			return
		}
		row["pendingAction"] = nil
		if strings.HasSuffix(p, "/start") || strings.HasSuffix(p, "/stop") || strings.HasSuffix(p, "/shutdown") || strings.HasSuffix(p, "/suspend") || strings.HasSuffix(p, "/boot-mode") {
			w.WriteHeader(202)
			return
		}
		if strings.HasSuffix(p, "/resize") {
			w.WriteHeader(202)
		}
		send(row)
		return
	}
	if p == volumesPath+"/"+testVolume+"/resize" {
		row := m.rows[volumesPath+"/"+testVolume]
		row["sizeGib"] = body["sizeGib"]
		send(row)
		return
	}
	if p == elasticIPsPath+"/"+testIP+"/target" {
		row := m.rows[elasticIPsPath+"/"+testIP]
		if row["targetInstanceId"] != nil && body["instanceId"] != nil && row["targetInstanceId"] != body["instanceId"] {
			http.Error(w, `{"detail":"detach the current target before moving"}`, 409)
			return
		}
		row["targetInstanceId"] = body["instanceId"]
		row["status"] = "bound"
		if body["instanceId"] == nil {
			row["status"] = "ready"
		}
		send(row)
		return
	}
	collections := map[string]string{registryCredentialsPath: "66666666-6666-4666-8666-666666666666", imagesPath: "custom", instancesPath: testInstance, volumesPath: testVolume, sshKeysPath: testSSHKey, elasticIPsPath: testIP, networksPath: "private", securityGroupsPath: "ssh"}
	if id, ok := collections[p]; ok {
		if r.Method == "GET" {
			rows := []any{}
			for key, v := range m.rows {
				if strings.HasPrefix(key, p+"/") {
					rows = append(rows, v)
				}
			}
			send(map[string]any{"data": rows})
			return
		}
		if r.Method == "POST" {
			if body == nil {
				http.Error(w, `{"detail":"no body"}`, 400)
				return
			}
			if p == instancesPath && m.dynamicIDs {
				m.instanceGeneration++
				id = fmt.Sprintf("22222222-2222-4222-8222-%012d", m.instanceGeneration)
			}
			if p == imagesPath {
				id = fmt.Sprint(body["regionId"]) + "/" + fmt.Sprint(body["name"])
			}
			body["id"] = id
			switch p {
			case instancesPath:
				body["status"] = "pending"
				body["pendingAction"] = "create"
				body["networkAddress"] = nil
				if _, ok := body["flavorSlug"]; !ok {
					body["flavorSlug"] = nil
				}
				body["hardwareGeneration"] = nil
				body["adminUsername"] = "admin"
				body["expiresAt"] = nil
				body["createdAt"] = "2026-09-14T00:00:00Z"
				if n, ok := body["lifetimeSeconds"].(float64); ok {
					body["expiresAt"] = time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC).Add(time.Duration(n) * time.Second).Format(time.RFC3339)
				}
				body["bootIntoRecovery"] = false
				body["tags"] = []any{}
				delete(body, "adminPassword")
			case volumesPath:
				body["state"] = "detached"
				body["attachedInstanceId"] = nil
			case sshKeysPath:
				delete(body, "publicKey")
				body["fingerprintSha256"] = "SHA256:test"
			case elasticIPsPath:
				body["status"] = "ready"
				body["publicAddress"] = "203.0.113.42"
				body["targetInstanceId"] = body["instanceId"]
				if body["instanceId"] != nil {
					body["status"] = "bound"
				}
			case registryCredentialsPath:
				body["source"] = "user"
				delete(body, "password")
			case imagesPath:
				body["source"] = "tenant"
				body["labels"] = map[string]any{"source": "studio-cp-register"}
				delete(body, "auth")
				delete(body, "precache")
			case networksPath:
				body["source"] = "tenant"
				spec, _ := body["spec"].(map[string]any)
				if spec == nil {
					spec = map[string]any{}
				}
				for _, k := range []string{"mode", "cidr", "gateway", "dhcp"} {
					if v, ok := body[k]; ok {
						spec[k] = v
					} else {
						body[k] = spec[k]
					}
				}
				spec["serverDefault"] = "default"
				body["spec"] = spec
			}
			m.rows[p+"/"+id] = body
			w.WriteHeader(201)
			send(body)
			return
		}
	}
	row := m.rows[p]
	if row == nil {
		http.Error(w, `{"detail":"resource missing"}`, 404)
		return
	}
	switch r.Method {
	case "GET":
		if p == base && row["pendingAction"] == "create" && !m.holdInstance {
			row["status"] = "running"
			row["pendingAction"] = nil
			row["networkAddress"] = "10.0.0.10"
			if m.failInstance {
				row["status"] = "error"
				row["lastError"] = "capacity unavailable"
			}
		}
		send(row)
	case "PATCH":
		if strings.HasPrefix(p, imagesPath+"/") {
			labels := body["labels"].(map[string]any)
			labels["source"] = "studio-cp-register"
		}
		for k, v := range body {
			if k != "password" {
				row[k] = v
			}
		}
		if p == base {
			if n, ok := body["lifetimeSeconds"].(float64); ok {
				row["expiresAt"] = time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC).Add(time.Duration(n) * time.Second).Format(time.RFC3339)
			} else if _, ok := body["lifetimeSeconds"]; ok {
				row["expiresAt"] = nil
			}
		}
		send(row)
	case "DELETE":
		if strings.HasPrefix(p, volumesPath) && row["state"] == "attached" {
			http.Error(w, `{"detail":"detach first"}`, 409)
			return
		}
		if p == base {
			for key, ip := range m.rows {
				if strings.HasPrefix(key, elasticIPsPath+"/") && ip["targetInstanceId"] == row["id"] {
					if r.URL.Query().Get("releaseElasticIps") != "false" {
						delete(m.rows, key)
					} else {
						ip["targetInstanceId"] = nil
						ip["status"] = "ready"
					}
				}
			}
		}
		if strings.HasPrefix(p, imagesPath+"/") && r.URL.Query().Get("deleteFromRegistry") != "false" {
			m.t.Error("image must retain registry bytes by default")
		}
		delete(m.rows, p)
		if p == base {
			w.WriteHeader(202)
		} else {
			w.WriteHeader(204)
		}
	default:
		http.Error(w, `{"detail":"unsupported route"}`, 404)
	}
}
func factories() map[string]func() (tfprotov6.ProviderServer, error) {
	return map[string]func() (tfprotov6.ProviderServer, error){"xcloud": providerserver.NewProtocol6WithError(New("test")())}
}
func TestProviderSchema(t *testing.T) {
	server := providerserver.NewProtocol6(New("test")())()
	resp, err := server.GetProviderSchema(context.Background(), &tfprotov6.GetProviderSchemaRequest{})
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range resp.Diagnostics {
		if d.Severity == tfprotov6.DiagnosticSeverityError {
			t.Fatalf("%s: %s", d.Summary, d.Detail)
		}
	}
	if len(resp.ResourceSchemas) != 9 || len(resp.DataSourceSchemas) != 20 {
		t.Fatalf("unexpected schema counts: %d/%d", len(resp.ResourceSchemas), len(resp.DataSourceSchemas))
	}
}
func testConfig(apiURL string, cpu, size int, name string) string {
	return fmt.Sprintf(`
provider "xcloud" {
 api_url = %q
 api_token = "test-key"
 poll_interval_seconds = 1
 operation_timeout_seconds = 10
}
data "xcloud_region" "test" { slug = "zrh1" }
data "xcloud_flavor" "test" {
 region_id = data.xcloud_region.test.id
 slug = "test"
}
data "xcloud_image" "test" {
 region_id = data.xcloud_region.test.id
 name = "ubuntu"
}
resource "xcloud_network" "test" {
 region_id = data.xcloud_region.test.id
 name = "private"
 mode = "nat"
 cidr = "10.0.0.0/24"
 gateway = "10.0.0.1"
 dhcp = true
 labels = { env = "test" }
}
resource "xcloud_security_group" "test" {
 region_id = data.xcloud_region.test.id
 name = "ssh"
 rules = [{ direction = "ingress", protocol = "tcp", cidr = "192.0.2.0/24", port_from = 22, port_to = 22, description = %q }]
}
resource "xcloud_ssh_key" "test" {
 name = %q
 public_key = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAITestPublicKey"
}
resource "xcloud_instance" "test" {
 region_id = data.xcloud_region.test.id
 name = %q
 image_ref = data.xcloud_image.test.name
 platform = "linux"
 cpu_cores = %d
 memory_gib = data.xcloud_flavor.test.memory_gib
 disk_gib = data.xcloud_flavor.test.disk_gib
 network_ref = xcloud_network.test.name
 ssh_key_ids = [xcloud_ssh_key.test.id]
 security_groups = [xcloud_security_group.test.name]
}
resource "xcloud_volume" "test" {
 region_id = data.xcloud_region.test.id
 name = "data"
 size_gib = %d
}
resource "xcloud_volume_attachment" "test" {
 instance_id = xcloud_instance.test.id
 volume_id = xcloud_volume.test.id
}
resource "xcloud_elastic_ip" "test" {
 region_id = data.xcloud_region.test.id
 instance_id = xcloud_instance.test.id
}
`, apiURL, name, name, name, cpu, size)
}
func TestProviderLifecycle(t *testing.T) {
	m, s := newMockAPI(t)
	config := testConfig(s.URL, 2, 50, "initial")
	steps := []resource.TestStep{{Config: config, Check: resource.ComposeAggregateTestCheckFunc(resource.TestCheckResourceAttr("xcloud_instance.test", "network_address", "10.0.0.10"), resource.TestCheckResourceAttr("xcloud_elastic_ip.test", "public_address", "203.0.113.42"), resource.TestCheckResourceAttr("xcloud_instance.test", "status", "running"))}}
	for _, name := range []string{"instance", "network", "security_group", "volume", "volume_attachment", "elastic_ip", "ssh_key"} {
		step := resource.TestStep{ResourceName: "xcloud_" + name + ".test", ImportState: true, ImportStateVerify: true}
		if name == "volume" {
			step.ImportStateVerifyIgnore = []string{"state"} // Attachment changed after the volume was created.
		}
		if name == "ssh_key" {
			step.ImportStateVerifyIgnore = []string{"public_key"}
		}
		steps = append(steps, step)
	}
	steps = append(steps, resource.TestStep{Config: testConfig(s.URL, 4, 75, "updated"), Check: resource.ComposeAggregateTestCheckFunc(resource.TestCheckResourceAttr("xcloud_instance.test", "cpu_cores", "4"), resource.TestCheckResourceAttr("xcloud_instance.test", "power_state", "running"), resource.TestCheckResourceAttr("xcloud_volume.test", "size_gib", "75"))})
	resource.UnitTest(t, resource.TestCase{ProtoV6ProviderFactories: factories(), Steps: steps, CheckDestroy: func(*terraform.State) error {
		m.mu.Lock()
		defer m.mu.Unlock()
		if len(m.rows) != 0 {
			return fmt.Errorf("resources left behind: %v", m.rows)
		}
		joined := strings.Join(m.calls, "\n")
		for _, action := range []string{"/shutdown", "/resize", "/start"} {
			if !strings.Contains(joined, instancesPath+"/"+testInstance+action) {
				return fmt.Errorf("missing lifecycle operation %s; calls:\n%s", action, joined)
			}
		}
		return nil
	}})
}
func TestInvalidSecurityGroupPorts(t *testing.T) {
	_, s := newMockAPI(t)
	config := fmt.Sprintf(`provider "xcloud" {
 api_url = %q
api_token = "test-key"
}
resource "xcloud_security_group" "test" {
 region_id = %q
name = "ssh"
rules = [{direction="ingress", protocol="tcp", cidr="192.0.2.0/24", port_from=22}]
}`, s.URL, testRegion)
	resource.UnitTest(t, resource.TestCase{ProtoV6ProviderFactories: factories(), Steps: []resource.TestStep{{Config: config, PlanOnly: true, ExpectError: regexp.MustCompile("Set both port_from and port_to")}}})
}

func TestInstanceCreateFailureRetainsID(t *testing.T) {
	for _, failure := range []string{"worker error", "timeout"} {
		t.Run(failure, func(t *testing.T) {
			m, s := newMockAPI(t)
			m.failInstance = failure == "worker error"
			m.holdInstance = failure == "timeout"
			c, err := client.New(s.URL, "test-key", "test")
			if err != nil {
				t.Fatal(err)
			}
			c.PollInterval = time.Millisecond
			c.OperationTimeout = 200 * time.Millisecond
			r := newInstanceResource().(*instanceResource)
			r.client = c
			ctx := context.Background()
			var schemaResp fwresource.SchemaResponse
			r.Schema(ctx, fwresource.SchemaRequest{}, &schemaResp)
			model := instanceModel{ID: types.StringUnknown(), RegionID: types.StringValue(testRegion), Name: types.StringValue("test"), ImageRef: types.StringValue("ubuntu"), NetworkRef: types.StringValue("default"), Platform: types.StringValue("macos"), CPU: types.Int64Value(2), Memory: types.Int64Value(4), Disk: types.Int64Value(40), Flavor: types.StringUnknown(), HardwareGeneration: types.StringUnknown(), AdminUsername: types.StringUnknown(), DisplayWidth: types.Int64Value(1920), DisplayHeight: types.Int64Value(1080), Tags: types.MapValueMust(types.StringType, nil), SSHKeys: types.SetValueMust(types.StringType, nil), SecurityGroups: types.SetValueMust(types.StringType, nil), PowerState: types.StringValue("running"), Status: types.StringUnknown(), NetworkAddress: types.StringUnknown(), ExpiresAt: types.StringUnknown()}
			plan := tfsdk.Plan{Schema: schemaResp.Schema}
			if ds := plan.Set(ctx, &model); ds.HasError() {
				t.Fatal(ds)
			}
			resp := fwresource.CreateResponse{State: tfsdk.State{Schema: schemaResp.Schema}}
			r.Create(ctx, fwresource.CreateRequest{Plan: plan}, &resp)
			if !resp.Diagnostics.HasError() {
				t.Fatal("expected asynchronous failure")
			}
			var got instanceModel
			if ds := resp.State.Get(ctx, &got); ds.HasError() {
				t.Fatal(ds)
			}
			if got.ID.ValueString() != testInstance {
				t.Fatalf("lost created ID after %s: %s", failure, got.ID)
			}
		})
	}
}

func TestNetworkAbsencePreservesState(t *testing.T) {
	_, s := newMockAPI(t)
	c, _ := client.New(s.URL, "test-key", "test")
	r := newNetworkResource().(*networkResource)
	r.client = c
	ctx := context.Background()
	var schemaResp fwresource.SchemaResponse
	r.Schema(ctx, fwresource.SchemaRequest{}, &schemaResp)
	model := networkModel{ID: types.StringValue(testRegion + "/private"), RegionID: types.StringValue(testRegion), Name: types.StringValue("private"), Labels: types.MapValueMust(types.StringType, nil)}
	state := tfsdk.State{Schema: schemaResp.Schema}
	if ds := state.Set(ctx, &model); ds.HasError() {
		t.Fatal(ds)
	}
	resp := fwresource.ReadResponse{State: state}
	r.Read(ctx, fwresource.ReadRequest{State: state}, &resp)
	if !resp.Diagnostics.HasError() {
		t.Fatal("ambiguous absence must stop refresh")
	}
	if resp.State.Raw.IsNull() {
		t.Fatal("an outage removed the network from state")
	}
}

func TestInstanceNotFoundRemovesState(t *testing.T) {
	_, s := newMockAPI(t)
	c, _ := client.New(s.URL, "test-key", "test")
	r := newInstanceResource().(*instanceResource)
	r.client = c
	ctx := context.Background()
	var schemaResp fwresource.SchemaResponse
	r.Schema(ctx, fwresource.SchemaRequest{}, &schemaResp)
	state := tfsdk.State{Schema: schemaResp.Schema}
	if ds := state.Set(ctx, &instanceModel{ID: types.StringValue(testInstance), Tags: types.MapNull(types.StringType), SSHKeys: types.SetNull(types.StringType), SecurityGroups: types.SetNull(types.StringType)}); ds.HasError() {
		t.Fatal(ds)
	}
	resp := fwresource.ReadResponse{State: state}
	r.Read(ctx, fwresource.ReadRequest{State: state}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatal(resp.Diagnostics)
	}
	if !resp.State.Raw.IsNull() {
		t.Fatal("deleted instance was retained")
	}
}

func TestElasticIPMoveDetachesFirst(t *testing.T) {
	m, s := newMockAPI(t)
	m.rows[elasticIPsPath+"/"+testIP] = map[string]any{"id": testIP, "regionId": testRegion, "targetInstanceId": testInstance, "publicAddress": "203.0.113.42", "status": "bound"}
	c, _ := client.New(s.URL, "test-key", "test")
	r := newElasticIPResource().(*elasticIPResource)
	r.client = c
	ctx := context.Background()
	var schemaResp fwresource.SchemaResponse
	r.Schema(ctx, fwresource.SchemaRequest{}, &schemaResp)
	old := elasticIPModel{ID: types.StringValue(testIP), RegionID: types.StringValue(testRegion), InstanceID: types.StringValue(testInstance), Address: types.StringValue("203.0.113.42"), Status: types.StringValue("bound")}
	state := tfsdk.State{Schema: schemaResp.Schema}
	if ds := state.Set(ctx, &old); ds.HasError() {
		t.Fatal(ds)
	}
	next := old
	next.InstanceID = types.StringValue("66666666-6666-4666-8666-666666666666")
	plan := tfsdk.Plan{Schema: schemaResp.Schema}
	if ds := plan.Set(ctx, &next); ds.HasError() {
		t.Fatal(ds)
	}
	resp := fwresource.UpdateResponse{State: state}
	r.Update(ctx, fwresource.UpdateRequest{Plan: plan, State: state}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatal(resp.Diagnostics)
	}
	var result elasticIPModel
	if ds := resp.State.Get(ctx, &result); ds.HasError() {
		t.Fatal(ds)
	}
	if !result.InstanceID.Equal(next.InstanceID) {
		t.Fatal("target was not updated")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.calls) != 2 {
		t.Fatalf("expected detach and attach requests, got %v", m.calls)
	}
}

func TestInstanceErrorReadWarnsAndRetainsState(t *testing.T) {
	m, s := newMockAPI(t)
	m.rows[instancesPath+"/"+testInstance] = map[string]any{"id": testInstance, "status": "error", "pendingAction": nil, "lastError": "test-key capacity unavailable"}
	c, _ := client.New(s.URL, "test-key", "test")
	r := newInstanceResource().(*instanceResource)
	r.client = c
	ctx := context.Background()
	var schemaResp fwresource.SchemaResponse
	r.Schema(ctx, fwresource.SchemaRequest{}, &schemaResp)
	state := tfsdk.State{Schema: schemaResp.Schema}
	model := instanceModel{ID: types.StringValue(testInstance), PowerState: types.StringValue("running"), Tags: types.MapNull(types.StringType), SSHKeys: types.SetNull(types.StringType), SecurityGroups: types.SetNull(types.StringType)}
	if ds := state.Set(ctx, &model); ds.HasError() {
		t.Fatal(ds)
	}
	resp := fwresource.ReadResponse{State: state}
	r.Read(ctx, fwresource.ReadRequest{State: state}, &resp)
	if resp.Diagnostics.HasError() || resp.Diagnostics.WarningsCount() != 1 {
		t.Fatalf("expected a warning that permits repair/destroy: %v", resp.Diagnostics)
	}
	detail := resp.Diagnostics[0].Detail()
	if strings.Contains(detail, "test-key") || !strings.Contains(detail, "capacity unavailable") {
		t.Fatalf("incorrect error detail: %s", detail)
	}
	var actual instanceModel
	if ds := resp.State.Get(ctx, &actual); ds.HasError() {
		t.Fatal(ds)
	}
	if actual.ID.ValueString() != testInstance || actual.Status.ValueString() != "error" {
		t.Fatalf("error state was lost: %+v", actual)
	}
}
