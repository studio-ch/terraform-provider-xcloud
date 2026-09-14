package provider

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func providerConfig(origin string) string {
	return fmt.Sprintf(`provider "xcloud" {
 api_url = %q
 api_token = "test-key"
 poll_interval_seconds = 1
 operation_timeout_seconds = 10
}
`, origin)
}
func TestImageCredentialAndNetworkSpecLifecycle(t *testing.T) {
	m, s := newMockAPI(t)
	config := func(label, mode, password string) string {
		return providerConfig(s.URL) + fmt.Sprintf(`
resource "xcloud_registry_credential" "test" {
 display_name = %q
 registry_url = "https://registry.example"
 username = "robot"
 password = %q
}
resource "xcloud_image" "test" {
 region_id = %q
 name = "custom"
 oci_reference = "registry.example/custom:v1"
 credential_id = xcloud_registry_credential.test.id
 labels = { env = %q }
}
resource "xcloud_network" "test" {
 region_id = %q
 name = "private"
 spec_json = jsonencode({mode = %q, custom = {mtu = 1400}})
}
data "xcloud_registry_credential" "test" { id = xcloud_registry_credential.test.id }
data "xcloud_image" "test" {
 region_id = xcloud_image.test.region_id
 name = xcloud_image.test.name
}
data "xcloud_network" "test" {
 region_id = xcloud_network.test.region_id
 name = xcloud_network.test.name
}
`, label, password, testRegion, label, testRegion, mode)
	}
	resource.UnitTest(t, resource.TestCase{ProtoV6ProviderFactories: factories(), Steps: []resource.TestStep{
		{Config: config("initial", "nat", "secret-one"), Check: resource.ComposeAggregateTestCheckFunc(resource.TestCheckResourceAttr("xcloud_image.test", "labels.env", "initial"), resource.TestCheckResourceAttr("xcloud_image.test", "all_labels.source", "studio-cp-register"), resource.TestCheckResourceAttr("data.xcloud_network.test", "mode", "nat"))},
		{ResourceName: "xcloud_image.test", ImportState: true, ImportStateVerify: true, ImportStateVerifyIgnore: []string{"credential_id"}},
		{ResourceName: "xcloud_registry_credential.test", ImportState: true, ImportStateVerify: true, ImportStateVerifyIgnore: []string{"password"}},
		{Config: config("updated", "bridge", "secret-two"), Check: resource.ComposeAggregateTestCheckFunc(resource.TestCheckResourceAttr("xcloud_image.test", "labels.env", "updated"), resource.TestCheckResourceAttr("data.xcloud_registry_credential.test", "display_name", "updated"), resource.TestCheckResourceAttr("data.xcloud_network.test", "mode", "bridge"))},
	}, CheckDestroy: func(*terraform.State) error {
		m.mu.Lock()
		defer m.mu.Unlock()
		if len(m.rows) != 0 {
			return fmt.Errorf("leftover resources: %v", m.rows)
		}
		return nil
	}})
}
func TestInstanceExtendedLifecycle(t *testing.T) {
	m, s := newMockAPI(t)
	config := func(power, mode, password, tags, lifetime string, recovery bool) string {
		return providerConfig(s.URL) + fmt.Sprintf(`
resource "xcloud_instance" "test" {
 region_id = %q
 name = "mac"
 image_ref = "macos"
 cpu_cores = 2
 memory_gib = 4
 disk_gib = 40
 power_state = %q
 shutdown_mode = %q
 admin_password = %q
 tags = %s
 boot_into_recovery = %t
 %s
}
`, testRegion, power, mode, password, tags, recovery, lifetime)
	}
	resource.UnitTest(t, resource.TestCase{ProtoV6ProviderFactories: factories(), Steps: []resource.TestStep{
		{Config: config("stopped", "graceful", "password-one", `{env="dev"}`, "lifetime_seconds = 3600", false), Check: resource.ComposeAggregateTestCheckFunc(resource.TestCheckResourceAttr("xcloud_instance.test", "power_state", "stopped"), resource.TestCheckResourceAttr("xcloud_instance.test", "tags.env", "dev"), resource.TestCheckResourceAttr("xcloud_instance.test", "lifetime_seconds", "3600"))},
		{Config: config("suspended", "graceful", "password-two", `{env="prod"}`, "lifetime_seconds = 7200", true), Check: resource.ComposeAggregateTestCheckFunc(resource.TestCheckResourceAttr("xcloud_instance.test", "status", "suspended"), resource.TestCheckResourceAttr("xcloud_instance.test", "boot_into_recovery", "true"), resource.TestCheckResourceAttr("xcloud_instance.test", "lifetime_seconds", "7200"))},
		{Config: config("running", "hard", "password-two", `{}`, "", false), Check: resource.ComposeAggregateTestCheckFunc(resource.TestCheckResourceAttr("xcloud_instance.test", "tags.%", "0"), resource.TestCheckNoResourceAttr("xcloud_instance.test", "lifetime_seconds"), resource.TestCheckResourceAttr("xcloud_instance.test", "boot_into_recovery", "false"))},
		{Config: config("stopped", "hard", "password-two", `{}`, "", false), Check: resource.TestCheckResourceAttr("xcloud_instance.test", "status", "stopped")},
	}, CheckDestroy: func(*terraform.State) error {
		m.mu.Lock()
		defer m.mu.Unlock()
		joined := strings.Join(m.calls, "\n")
		for _, a := range []string{"shutdown", "suspend", "start", "stop", "boot-mode", "password", "tags"} {
			if !strings.Contains(joined, "/"+a) {
				return fmt.Errorf("action %s never called", a)
			}
		}
		return nil
	}})
}
func TestInstanceReplacementPreservesElasticIP(t *testing.T) {
	m, s := newMockAPI(t)
	m.dynamicIDs = true
	config := func(image string) string {
		return providerConfig(s.URL) + fmt.Sprintf(`resource "xcloud_instance" "test" {
 region_id = %q
 name = "mac"
 image_ref = %q
 cpu_cores = 2
 memory_gib = 4
 disk_gib = 40
}
resource "xcloud_elastic_ip" "test" {
 region_id = %q
 instance_id = xcloud_instance.test.id
}
`, testRegion, image, testRegion)
	}
	var originalVM, originalIP string
	resource.UnitTest(t, resource.TestCase{ProtoV6ProviderFactories: factories(), Steps: []resource.TestStep{
		{Config: config("macos-v1"), Check: func(s *terraform.State) error {
			originalVM = s.RootModule().Resources["xcloud_instance.test"].Primary.ID
			originalIP = s.RootModule().Resources["xcloud_elastic_ip.test"].Primary.ID
			return nil
		}},
		{Config: config("macos-v2"), Check: func(s *terraform.State) error {
			vm := s.RootModule().Resources["xcloud_instance.test"].Primary.ID
			ip := s.RootModule().Resources["xcloud_elastic_ip.test"].Primary
			if vm == originalVM {
				return fmt.Errorf("VM was not replaced")
			}
			if ip.ID != originalIP || ip.Attributes["public_address"] != "203.0.113.42" || ip.Attributes["instance_id"] != vm {
				return fmt.Errorf("IP was lost or not rebound: %#v", ip)
			}
			return nil
		}},
	}})
}
func TestInventoryDataSources(t *testing.T) {
	_, s := newMockAPI(t)
	config := testConfig(s.URL, 2, 50, "inventory")
	checks := []resource.TestCheckFunc{}
	names := map[string]string{"instance": "test", "network": "private", "security_group": "ssh", "volume": "test", "elastic_ip": "test", "ssh_key": "test"}
	for _, d := range inventoryDefinitions() {
		config += fmt.Sprintf("\ndata %q \"all\" {\n depends_on = [xcloud_instance.test, xcloud_volume_attachment.test, xcloud_elastic_ip.test]\n", "xcloud_"+d.plural)
		if d.regional {
			config += fmt.Sprintf("region_id = %q\n", testRegion)
		}
		config += "}\n"
		n := "1"
		if d.kind == "registry_credential" {
			n = "0"
		}
		checks = append(checks, resource.TestCheckResourceAttr("data.xcloud_"+d.plural+".all", "items.#", n))
		if name, ok := names[d.kind]; ok {
			if d.regional {
				checks = append(checks, resource.TestCheckResourceAttr("data.xcloud_"+d.kind+".read", "region_id", testRegion))
			}
			config += fmt.Sprintf("data %q \"read\" {\n", "xcloud_"+d.kind)
			if d.selector == "id" {
				config += fmt.Sprintf("id = xcloud_%s.test.id\n", d.kind)
			} else {
				config += fmt.Sprintf("name = %q\nregion_id = %q\ndepends_on = [xcloud_%s.test]\n", name, testRegion, d.kind)
			}
			config += "}\n"
			checks = append(checks, resource.TestCheckResourceAttrSet("data.xcloud_"+d.kind+".read", "response_json"))
		}
	}
	resource.UnitTest(t, resource.TestCase{ProtoV6ProviderFactories: factories(), Steps: []resource.TestStep{{Config: config, Check: resource.ComposeAggregateTestCheckFunc(checks...)}}})
}

func TestFlavorSizingAndMetadataDrift(t *testing.T) {
	m, s := newMockAPI(t)
	m.largeFlavor = true
	config := func(slug string) string {
		c := testConfig(s.URL, 2, 50, "flavor")
		c = strings.Replace(c, `slug = "test"`, fmt.Sprintf("slug = %q", slug), 1)
		c = strings.Replace(c, "cpu_cores = 2", "cpu_cores = data.xcloud_flavor.test.cpu_cores\n flavor_slug = data.xcloud_flavor.test.slug\n tags = {env = \"desired\"}", 1)
		return c
	}
	resource.UnitTest(t, resource.TestCase{ProtoV6ProviderFactories: factories(), Steps: []resource.TestStep{
		{Config: config("test"), Check: resource.TestCheckResourceAttr("xcloud_instance.test", "flavor_slug", "test")},
		{PreConfig: func() {
			m.mu.Lock()
			defer m.mu.Unlock()
			m.rows[instancesPath+"/"+testInstance]["tags"] = []any{map[string]any{"key": "env", "value": "external"}}
		}, Config: config("large"), Check: resource.ComposeAggregateTestCheckFunc(resource.TestCheckResourceAttr("xcloud_instance.test", "flavor_slug", "large"), resource.TestCheckResourceAttr("xcloud_instance.test", "cpu_cores", "4"), resource.TestCheckResourceAttr("xcloud_instance.test", "memory_gib", "8"), resource.TestCheckResourceAttr("xcloud_instance.test", "tags.env", "desired"))},
	}})
}

func TestImageAuthenticationVariants(t *testing.T) {
	for _, tc := range []struct{ name, auth string }{{"anonymous", ""}, {"adhoc", `username = "robot"
 password = "registry-secret"`}} {
		t.Run(tc.name, func(t *testing.T) {
			_, s := newMockAPI(t)
			config := providerConfig(s.URL) + fmt.Sprintf(`resource "xcloud_image" "test" {
 region_id = %q
 name = "custom"
 oci_reference = "registry.example/image:v1"
 precache = true
 %s
}`, testRegion, tc.auth)
			resource.UnitTest(t, resource.TestCase{ProtoV6ProviderFactories: factories(), Steps: []resource.TestStep{{Config: config, Check: resource.TestCheckResourceAttr("xcloud_image.test", "source", "tenant")}}})
		})
	}
}
func TestNetworkSpecConflictRejectedBeforeCreate(t *testing.T) {
	_, s := newMockAPI(t)
	config := providerConfig(s.URL) + fmt.Sprintf(`resource "xcloud_network" "test" {
 region_id = %q
 name = "private"
 mode = "nat"
 spec_json = jsonencode({mode = "bridge"})
}`, testRegion)
	resource.UnitTest(t, resource.TestCase{ProtoV6ProviderFactories: factories(), Steps: []resource.TestStep{{Config: config, PlanOnly: true, ExpectError: regexp.MustCompile("Conflicting network spec")}}})
}
