# Xcloud provider

Manage infrastructure through the public Cloud Console API. See the [installation guide](../README.md), [basic example](../examples/basic/main.tf), [catalog example](../examples/catalog/main.tf) and [feature audit](feature-audit.md).

## Configuration

| Attribute | Type | Usage | Description |
| --- | --- | --- | --- |
| `api_token` | `string` | Optional; sensitive | API key with write:resources scope. Defaults to XCLOUD_API_TOKEN or CLOUDCONSOLE_API_TOKEN. |
| `api_url` | `string` | Optional | API origin. Defaults to XCLOUD_API_URL, CLOUDCONSOLE_API_URL, or https://api.cloud.flow.swiss. |
| `operation_timeout_seconds` | `number` | Optional | Total timeout for each resource operation, including polling. Default 1800 seconds. |
| `poll_interval_seconds` | `number` | Optional | Interval between asynchronous status requests. Default 5 seconds. |

## Resources

- [`xcloud_elastic_ip`](resources/elastic_ip.md)
- [`xcloud_image`](resources/image.md)
- [`xcloud_instance`](resources/instance.md)
- [`xcloud_network`](resources/network.md)
- [`xcloud_registry_credential`](resources/registry_credential.md)
- [`xcloud_security_group`](resources/security_group.md)
- [`xcloud_ssh_key`](resources/ssh_key.md)
- [`xcloud_volume`](resources/volume.md)
- [`xcloud_volume_attachment`](resources/volume_attachment.md)

## Data sources

- [`xcloud_elastic_ip`](data-sources/elastic_ip.md)
- [`xcloud_elastic_ips`](data-sources/elastic_ips.md)
- [`xcloud_flavor`](data-sources/flavor.md)
- [`xcloud_flavors`](data-sources/flavors.md)
- [`xcloud_image`](data-sources/image.md)
- [`xcloud_images`](data-sources/images.md)
- [`xcloud_instance`](data-sources/instance.md)
- [`xcloud_instances`](data-sources/instances.md)
- [`xcloud_network`](data-sources/network.md)
- [`xcloud_networks`](data-sources/networks.md)
- [`xcloud_region`](data-sources/region.md)
- [`xcloud_regions`](data-sources/regions.md)
- [`xcloud_registry_credential`](data-sources/registry_credential.md)
- [`xcloud_registry_credentials`](data-sources/registry_credentials.md)
- [`xcloud_security_group`](data-sources/security_group.md)
- [`xcloud_security_groups`](data-sources/security_groups.md)
- [`xcloud_ssh_key`](data-sources/ssh_key.md)
- [`xcloud_ssh_keys`](data-sources/ssh_keys.md)
- [`xcloud_volume`](data-sources/volume.md)
- [`xcloud_volumes`](data-sources/volumes.md)
