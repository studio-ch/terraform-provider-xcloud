package provider

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	tfresource "github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/studio-ch/terraform-provider-xcloud/internal/client"
)

func TestSnapshotUpdateBody(t *testing.T) {
	old := snapshotModel{Name: types.StringValue("a"), Description: types.StringValue("d")}
	for _, tc := range []struct {
		name string
		plan snapshotModel
		want client.Object
	}{
		{"unchanged", old, nil},
		{"rename", snapshotModel{Name: types.StringValue("b"), Description: types.StringValue("d")}, client.Object{"name": "b"}},
		{"describe", snapshotModel{Name: types.StringValue("a"), Description: types.StringValue("e")}, client.Object{"description": "e"}},
		// Removing the argument must clear the description, not keep it.
		{"clear description", snapshotModel{Name: types.StringValue("a"), Description: types.StringNull()}, client.Object{"description": nil}},
		{"unknown name keeps it", snapshotModel{Name: types.StringUnknown(), Description: types.StringValue("d")}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.plan.updateBody(old)
			if fmt.Sprint(got) != fmt.Sprint(tc.want) {
				t.Fatalf("updateBody = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestSnapshotSettled(t *testing.T) {
	redact := func(s string) string { return s }
	if done, err := snapshotSettled(client.Object{"status": "creating"}, redact); done || err != nil {
		t.Fatal("creating must keep waiting")
	}
	if done, err := snapshotSettled(client.Object{"status": "ready"}, redact); !done || err != nil {
		t.Fatal("ready must succeed")
	}
	done, err := snapshotSettled(client.Object{"id": "s", "status": "failed", "lastError": "disk busy"}, redact)
	if !done || err == nil || !strings.Contains(err.Error(), "disk busy") {
		t.Fatalf("failed must end the wait with lastError, got %v", err)
	}
}

// A snapshot outlives its instance, and an instance outlives the snapshot
// it was created from. The API then reports a null id; refreshing that
// into state would plan a replacement of a resource that still exists.
func TestSnapshotSourceIDsSurviveDeletion(t *testing.T) {
	snap := snapshotModel{InstanceID: types.StringValue(testInstance)}
	snap.flatten(client.Object{"id": testSnapshot, "sourceInstanceId": nil})
	if snap.InstanceID.ValueString() != testInstance {
		t.Fatalf("instance_id = %v, want it kept", snap.InstanceID)
	}
	imported := snapshotModel{InstanceID: types.StringNull()}
	imported.flatten(client.Object{"id": testSnapshot, "sourceInstanceId": testInstance})
	if imported.InstanceID.ValueString() != testInstance {
		t.Fatalf("import must read instance_id, got %v", imported.InstanceID)
	}

	inst := instanceModel{SourceSnapshotID: types.StringValue(testSnapshot)}
	inst.flatten(client.Object{"id": testInstance, "sourceSnapshotId": nil, "imageRef": "ubuntu"})
	if inst.SourceSnapshotID.ValueString() != testSnapshot {
		t.Fatalf("source_snapshot_id = %v, want it kept", inst.SourceSnapshotID)
	}
}

func TestInstanceCreateBodySource(t *testing.T) {
	base := instanceModel{SSHKeys: types.SetNull(types.StringType), SecurityGroups: types.SetNull(types.StringType)}
	fromImage := base
	fromImage.ImageRef = types.StringValue("ubuntu")
	fromImage.SourceSnapshotID = types.StringNull()
	if b := fromImage.createBody(); b["imageRef"] != "ubuntu" || b["sourceSnapshotId"] != nil {
		t.Fatalf("image body = %v", b)
	}
	fromSnapshot := base
	fromSnapshot.ImageRef = types.StringUnknown()
	fromSnapshot.SourceSnapshotID = types.StringValue(testSnapshot)
	b := fromSnapshot.createBody()
	if _, ok := b["imageRef"]; ok || b["sourceSnapshotId"] != testSnapshot {
		t.Fatalf("snapshot body = %v; the API requires exactly one of imageRef and sourceSnapshotId", b)
	}
}

func TestInstanceSourceValidation(t *testing.T) {
	for _, tc := range []struct {
		name      string
		edit      func(*instanceModel)
		wantError bool
	}{
		{"image", func(m *instanceModel) { m.ImageRef = types.StringValue("ubuntu") }, false},
		{"snapshot", func(m *instanceModel) { m.SourceSnapshotID = types.StringValue(testSnapshot) }, false},
		{"neither", func(m *instanceModel) {}, true},
		{"both", func(m *instanceModel) {
			m.ImageRef = types.StringValue("ubuntu")
			m.SourceSnapshotID = types.StringValue(testSnapshot)
		}, true},
		{"unknown snapshot", func(m *instanceModel) { m.SourceSnapshotID = types.StringUnknown() }, false},
		{"snapshot with user_data", func(m *instanceModel) {
			m.SourceSnapshotID = types.StringValue(testSnapshot)
			m.UserData = types.StringValue("#cloud-config\nusers: []\n")
			m.UserDataFormat = types.StringValue("cloud-init")
		}, true},
		{"snapshot with admin_username", func(m *instanceModel) {
			m.SourceSnapshotID = types.StringValue(testSnapshot)
			m.AdminUsername = types.StringValue("admin")
		}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			r := newInstanceResource().(*instanceResource)
			var sr resource.SchemaResponse
			r.Schema(ctx, resource.SchemaRequest{}, &sr)
			m := instanceModel{Platform: types.StringValue("linux"), Tags: types.MapNull(types.StringType), SSHKeys: types.SetNull(types.StringType), SecurityGroups: types.SetNull(types.StringType)}
			tc.edit(&m)
			state := tfsdk.State{Schema: sr.Schema}
			if ds := state.Set(ctx, &m); ds.HasError() {
				t.Fatal(ds)
			}
			var resp resource.ValidateConfigResponse
			r.ValidateConfig(ctx, resource.ValidateConfigRequest{Config: tfsdk.Config{Schema: sr.Schema, Raw: state.Raw}}, &resp)
			if resp.Diagnostics.HasError() != tc.wantError {
				t.Fatalf("unexpected validation: %v", resp.Diagnostics)
			}
		})
	}
}

func snapshotConfig(origin, snapshotName string) string {
	return providerConfig(origin) + fmt.Sprintf(`
resource "xcloud_ssh_key" "test" {
 name = "snap"
 public_key = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAITestPublicKey"
}
resource "xcloud_instance" "base" {
 region_id = %[1]q
 name = "base"
 image_ref = "ubuntu"
 platform = "linux"
 cpu_cores = 2
 memory_gib = 4
 disk_gib = 40
 ssh_key_ids = [xcloud_ssh_key.test.id]
}
resource "xcloud_snapshot" "test" {
 instance_id = xcloud_instance.base.id
 name = %[2]q
}
resource "xcloud_instance" "copy" {
 region_id = %[1]q
 name = "copy"
 source_snapshot_id = xcloud_snapshot.test.id
 platform = xcloud_snapshot.test.platform
 cpu_cores = xcloud_snapshot.test.min_cpu_cores
 memory_gib = xcloud_snapshot.test.min_memory_gib
 disk_gib = xcloud_snapshot.test.disk_gib
}
`, testRegion, snapshotName)
}

// An instance is snapshotted, a second one is created from the snapshot,
// and renaming the snapshot afterwards changes neither of them.
func TestSnapshotLifecycleAndInstanceFromSnapshot(t *testing.T) {
	m, s := newMockAPI(t)
	m.dynamicIDs = true
	tfresource.UnitTest(t, tfresource.TestCase{ProtoV6ProviderFactories: factories(), Steps: []tfresource.TestStep{
		{Config: snapshotConfig(s.URL, "before-upgrade"), Check: tfresource.ComposeAggregateTestCheckFunc(
			tfresource.TestCheckResourceAttr("xcloud_snapshot.test", "status", "ready"),
			tfresource.TestCheckResourceAttr("xcloud_snapshot.test", "name", "before-upgrade"),
			tfresource.TestCheckResourceAttr("xcloud_snapshot.test", "source_instance_name", "base"),
			tfresource.TestCheckResourceAttr("xcloud_snapshot.test", "disk_gib", "40"),
			tfresource.TestCheckResourceAttr("xcloud_snapshot.test", "taken_while_running", "true"),
			tfresource.TestCheckResourceAttrPair("xcloud_snapshot.test", "instance_id", "xcloud_instance.base", "id"),
			tfresource.TestCheckResourceAttrPair("xcloud_instance.copy", "source_snapshot_id", "xcloud_snapshot.test", "id"),
			tfresource.TestCheckResourceAttr("xcloud_instance.copy", "image_ref", "ubuntu"),
			tfresource.TestCheckResourceAttr("xcloud_instance.copy", "status", "running"),
		)},
		{ResourceName: "xcloud_snapshot.test", ImportState: true, ImportStateVerify: true},
		{Config: snapshotConfig(s.URL, "baseline"), Check: tfresource.ComposeAggregateTestCheckFunc(
			tfresource.TestCheckResourceAttr("xcloud_snapshot.test", "name", "baseline"),
			tfresource.TestCheckResourceAttr("xcloud_snapshot.test", "id", testSnapshot),
			func(*terraform.State) error {
				m.mu.Lock()
				defer m.mu.Unlock()
				if n := strings.Count(strings.Join(m.calls, "\n"), "POST "+instancesPath+"\n"); n != 2 {
					return fmt.Errorf("renaming the snapshot recreated instances: %d creates", n)
				}
				return nil
			},
		)},
	}, CheckDestroy: func(*terraform.State) error {
		m.mu.Lock()
		defer m.mu.Unlock()
		if len(m.rows) != 0 {
			return fmt.Errorf("resources left behind: %v", m.rows)
		}
		if !strings.Contains(strings.Join(m.calls, "\n"), "DELETE "+snapshotsPath+"/"+testSnapshot) {
			return fmt.Errorf("snapshot was not deleted")
		}
		return nil
	}})
}

func TestSnapshotFailureReportsLastError(t *testing.T) {
	m, s := newMockAPI(t)
	m.failSnapshot = true
	config := providerConfig(s.URL) + fmt.Sprintf(`
resource "xcloud_instance" "base" {
 region_id = %q
 name = "base"
 image_ref = "ubuntu"
 platform = "macos"
 cpu_cores = 2
 memory_gib = 4
 disk_gib = 40
}
resource "xcloud_snapshot" "test" {
 instance_id = xcloud_instance.base.id
}
`, testRegion)
	tfresource.UnitTest(t, tfresource.TestCase{ProtoV6ProviderFactories: factories(), Steps: []tfresource.TestStep{
		{Config: config, ExpectError: regexp.MustCompile(`disk busy`)},
	}})
}
