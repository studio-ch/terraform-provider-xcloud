# Feature-Prüfung: Terraform-Provider für Xcloud

Stand: 14. September 2026, nach Umsetzung der deklarativen Ergänzungen.

**Die bestätigten Lifecycle-, IP- und Replay-Fehler sind behoben. Der Provider umfasst jetzt neun Ressourcen und 20 Datenquellen.** Die im ursprünglichen Audit benannten deklarativen Erweiterungen sind implementiert und lokal mit echtem Terraform gegen eine Test-API geprüft. Das ist keine Zusage, jede öffentliche API-Operation abzubilden, und noch keine Stage-Abnahme.

## Behobene Fehler

- **B1 – leere Lifecycle-Antworten:** Start, Stop, Shutdown, Suspend und Boot-Modus akzeptieren leere HTTP-202-Antworten und pollen anschließend den Zustand. Resize akzeptiert weiterhin die API-Antwort mit JSON. Die Regressionstests liegen dauerhaft unter `internal/provider/lifecycle_regression_test.go`.
- **B2 – IP-Verlust beim VM-Ersatz:** VM-DELETE sendet `releaseElasticIps=false`. Ein Terraform-Test ersetzt eine VM mit wechselnder UUID und prüft dieselbe Elastic-IP-ID, dieselbe öffentliche Adresse und die Zuordnung zur neuen VM. Die Test-API bildet die gefährliche Freigabe bei fehlendem Parameter nach.

- **B3 – zu breite Replay-Annahme:** Nur `POST /v1/xcloud/instances` erhält einen Idempotency-Key und darf als Write nach Transportfehlern, 408 oder 5xx wiederholt werden. Netzwerk-/Security-Group-Routen und andere VM-Writes persistieren keine Replay-Antworten. GET-Retries und die bestehende Behandlung von Rate-Limits bleiben erhalten. Regressionstests prüfen insbesondere bereits angenommene Writes mit verlorener Antwort.
- **Weitere Review-Korrekturen:** Unbekannte Query-Namen werden im Vertragscheck abgelehnt; VM-Refresh meldet `status=error` als Warnung mit API-Fehlergrund und erhält den State. Belegte Elastic IPs werden API-seitig beim Attach als `409 Conflict` mit Detach-Hinweis zurückgegeben (erfordert Ausrollen der API-Korrektur).

Die [ursprüngliche Testquelle](audit/2026-09-14-regressions.go.txt) und [damalige Fehlerausgabe](audit/2026-09-14-regressions.txt) dokumentieren den Zustand **vor** diesen Korrekturen.

## Ergänzte Fähigkeiten

| Bereich | Umsetzung |
| --- | --- |
| VM-Metadaten | Tags einschließlich Drift-Abgleich; macOS-Passwort bei Erstellung und Rotation; Lifetime setzen, ändern und entfernen. |
| VM-Lifecycle | Graceful oder Hard Shutdown, Suspend/Resume, Recovery-/Normal-Boot. Resize stellt den konfigurierten Betriebszustand wieder her. |
| Images | OCI-Registrierung mit anonymer, gespeicherter oder Ad-hoc-Authentifizierung; Precache; editierbare Labels; Import; Kataloglöschung und optionale Registry-Löschung. |
| Registry-Zugangsdaten | CRUD, Passwortrotation und Import. Plattformverwaltete Credentials bleiben schreibgeschützt. |
| Netzwerke | Beliebiges spec_json; effective_spec_json zeigt API-Defaults. Nur konfigurierte JSON-Felder werden auf Drift geprüft. Änderungen ersetzen das Netz. |
| Infrastruktur lesen | Einzelne Instanzen, Netze, Security Groups, Volumes, Elastic IPs, SSH-Keys und Registry-Credentials zusätzlich zu Region/Flavor/Image. Listen für alle zehn Objektarten. |
| Katalog-Metadaten | Typisierte Kernfelder plus response_json für den vollständigen öffentlichen Datensatz, einschließlich verfügbarer Preis-/Kapazitätsinformationen. |

Die beiden zusätzlichen Ressourcen heißen `xcloud_image` und `xcloud_registry_credential`. Sämtliche Ressourcen unterstützen Import. Die [Referenz](index.md) enthält alle Schemas; das [Katalog-Beispiel](../examples/catalog/main.tf) zeigt Image-Authentifizierung und freie Netzwerk-Spezifikationen.

## Verifikation

- Echtes Terraform prüft Erstellung, Änderung, Import und Löschung gegen eine lokale HTTP-Test-API.
- Zusätzliche Szenarien: VM-Ersatz mit IP-Erhalt, Tags-Drift, Flavor-basiertes Provisionieren und Resize, Lifetime-Änderung und Entfernung, Passwortrotation, Suspend/Recovery, Image-Labels, Registry-Credentials und Netzwerk-Spec-Ersatz.
- Alle 20 Datenquellen sind registriert und werden in Terraform-Szenarien gelesen, einschließlich leerer Listen.
- Der öffentliche OpenAPI-Vertrag liegt im Provider unter `internal/provider/testdata/public-api.json`. Der Check läuft auch im isolierten Checkout und prüft ausgehende Requests einschließlich Query-Namen sowie deklarierte Erfolgsstatuscodes und JSON-/Leerantwort-Semantik.
- Der Vertragsprüfer ist **kein vollständiger OpenAPI-Validator**: Er prüft unter anderem nicht sämtliche Formate, Längen und alle Response-DTO-Felder. Lokale Tests ersetzen keine echte API-/Worker-Abnahme.
- API-Fehlerdiagnosen schwärzen API-Tokens und Passwortwerte aus dem jeweiligen Request, auch in verschachtelter Image-Authentifizierung.

Prüfbefehle:

```sh
GOWORK=off go test ./... -race -count=1
GOWORK=off go vet ./...
GOWORK=off make build
terraform fmt -check -recursive examples
```

## Verbleibende Grenzen

- Ein mehrdeutig fehlgeschlagenes Create kann trotz unterbundener Retries eine Ressource ohne State-Eintrag hinterlassen. Vor einem weiteren Apply muss deren Existenz geprüft und die Ressource gegebenenfalls importiert werden.
- Elastic IPs, SSH-Keys und Registry-Credentials werden mangels GET-by-ID per Listen-Scan gelesen: linear pro Ressource, potenziell quadratische Übertragung bei vielen einzeln verwalteten Objekten. Dies bleibt eine API-bedingte Skalierungsgrenze.

- Die signierte Version `0.1.0-beta.2` ist in der Terraform Registry veröffentlicht; direkte Installation, Signaturprüfung, Validierung und Schema-Laden wurden auf macOS ARM64 geprüft. Ein Test gegen eine reale Stage-/Produktionsumgebung steht weiterhin aus.
- Gast-Exec, Dateiübertragung, Wartungsaktionen, Image-Push-Jobs, Metriken, WebSocket-Konsole, Computer-/Workspace-/Screen-APIs sind nicht implementiert. Sie gehören zu operativen oder interaktiven API-Funktionen; dieser Ausbau betrifft deklarative Infrastruktur.
- Das API-DTO muss `bootIntoRecovery` liefern. Passwort- und SSH-Injektion erfolgen im Worker; erfolgreicher Apply bestätigt keine Gast-Anmeldung.
- Passwörter sind in Terraform als sensitive markiert, liegen aber im State. Entfernen eines Passwortarguments löscht kein bestehendes Passwort.
- Image- und Netzwerklisten können Regionsausfälle als leere Listen darstellen. Fehlende **verwaltete Ressourcen** erzeugen deshalb einen Refresh-Fehler und bleiben im State. Eine externe Löschung muss bestätigt werden, bevor der Eintrag manuell aus dem State entfernt wird.
- Netzwerk-Updates, Volume-Rename und Disk-Shrink sind nicht durch passende öffentliche Update-APIs gedeckt. Beim Ersatz eines Netzes mit unverändertem Namen müssen abhängige VMs gegebenenfalls mit `replace_triggered_by` ersetzt werden.
- `pendingElasticIp` und `attachToInstanceId` bei Create werden durch separate IP-/Attachment-Ressourcen funktional abgedeckt. Ein Terraform-Apply ist keine atomare Transaktion über diese Ressourcen.
- Registry-Bytes bleiben bei Image-Delete standardmäßig erhalten. Wer sie ausdrücklich löschen lässt, muss die Berechtigungen und gemeinsame Nutzung des OCI-Artefakts berücksichtigen.

## Matrix der 52 Xcloud-OpenAPI-Operationen

„Implementiert“ bezeichnet die Provider-Abbildung auf Quellcode-/lokaler Testbasis. „Indirekt“ bezeichnet eine über andere Leseoperationen vorhandene Fähigkeit. Registry-Credentials, SSH-Keys und Regionen stehen außerhalb dieser 52 Xcloud-Operationen und sind ebenfalls unterstützt.

| Methode | Pfad | Stand | Erläuterung |
| --- | --- | --- | --- |
| GET | `/v1/xcloud/instances` | Implementiert | List the current tenant Xcloud instances |
| POST | `/v1/xcloud/instances` | Implementiert | Passwort unterstützt; IP-Zuordnung über separate Elastic-IP-Ressource. |
| GET | `/v1/xcloud/instances/{id}` | Implementiert | Read one Xcloud instance |
| PATCH | `/v1/xcloud/instances/{id}` | Implementiert | Update Xcloud instance metadata |
| DELETE | `/v1/xcloud/instances/{id}` | Implementiert | releaseElasticIps=false; separat verwaltete IPs bleiben erhalten. |
| PATCH | `/v1/xcloud/instances/{id}/ssh-keys` | Implementiert | Replace the SSH key set for an Xcloud instance |
| PUT | `/v1/xcloud/instances/{id}/password` | Implementiert | Passwort im API-Zustand setzen; Gast-Injektion erfolgt asynchron. |
| PUT | `/v1/xcloud/instances/{id}/security-groups` | Implementiert | Replace the security-group set for an Xcloud instance |
| POST | `/v1/xcloud/instances/{id}/start` | Implementiert | Leere 202 akzeptieren und Status pollen. |
| POST | `/v1/xcloud/instances/{id}/stop` | Implementiert | Hard Stop über shutdown_mode=hard; leere 202 und Polling. |
| POST | `/v1/xcloud/instances/{id}/shutdown` | Implementiert | Graceful Shutdown, auch vor Resize. |
| POST | `/v1/xcloud/instances/{id}/suspend` | Implementiert | power_state=suspended; Resume per start. |
| POST | `/v1/xcloud/instances/{id}/boot-mode` | Implementiert | boot_into_recovery; wartet auf Flag und abgeschlossenen Auftrag. |
| POST | `/v1/xcloud/instances/{id}/resize` | Implementiert | Queue a resize action (CPU, memory, disk) |
| GET | `/v1/xcloud/instances/{id}/metrics` | Nicht implementiert | Metrik-Zeitreihen; keine deklarative Ressource. |
| GET | `/v1/xcloud/instances/{id}/tags` | Indirekt | Tags werden über das VM-DTO gelesen; kein separater Request nötig. |
| PUT | `/v1/xcloud/instances/{id}/tags` | Implementiert | Replace Xcloud instance tags |
| POST | `/v1/xcloud/instances/{id}/agent/actions` | Nicht implementiert | Operative Gast-/Job-Funktion; nicht als Terraform-Ressource implementiert. |
| GET | `/v1/xcloud/instances/{id}/agent/actions` | Nicht implementiert | Operative Gast-/Job-Funktion; nicht als Terraform-Ressource implementiert. |
| GET | `/v1/xcloud/instances/{id}/agent/actions/{actionId}` | Nicht implementiert | Operative Gast-/Job-Funktion; nicht als Terraform-Ressource implementiert. |
| POST | `/v1/xcloud/instances/{id}/push-image` | Nicht implementiert | Operative Gast-/Job-Funktion; nicht als Terraform-Ressource implementiert. |
| GET | `/v1/xcloud/instances/{id}/push-jobs` | Nicht implementiert | Operative Gast-/Job-Funktion; nicht als Terraform-Ressource implementiert. |
| GET | `/v1/xcloud/instances/{id}/volumes` | Indirekt | Zuordnungen sind über die Volume-Liste und attached_instance_id lesbar. |
| POST | `/v1/xcloud/instances/{id}/volumes` | Implementiert | Attach a data volume to an Xcloud instance (restarts the VM) |
| DELETE | `/v1/xcloud/instances/{id}/volumes/{volumeId}` | Implementiert | Detach a data volume from an Xcloud instance |
| GET | `/v1/xcloud/volumes` | Implementiert | List the current tenant Xcloud data volumes |
| POST | `/v1/xcloud/volumes` | Implementiert | Zuordnung über separate Volume-Attachment-Ressource. |
| GET | `/v1/xcloud/volumes/{id}` | Implementiert | Get one Xcloud volume |
| DELETE | `/v1/xcloud/volumes/{id}` | Implementiert | Delete an Xcloud volume (must be detached) |
| POST | `/v1/xcloud/volumes/{id}/resize` | Implementiert | Grow an Xcloud volume |
| GET | `/v1/xcloud/image-pushes/{id}` | Nicht implementiert | Operative Gast-/Job-Funktion; nicht als Terraform-Ressource implementiert. |
| POST | `/v1/xcloud/image-pushes/{id}/cancel` | Nicht implementiert | Operative Gast-/Job-Funktion; nicht als Terraform-Ressource implementiert. |
| GET | `/v1/xcloud/images` | Implementiert | Einzel-/Listen-Datenquellen und Refresh; dieselbe Ausfall-Ambiguität wie bei Netzen. |
| POST | `/v1/xcloud/images` | Implementiert | Register an OCI image in the tenant Xcloud catalog |
| PATCH | `/v1/xcloud/images/{regionId}/{name}` | Implementiert | Update a tenant Xcloud image's labels |
| DELETE | `/v1/xcloud/images/{regionId}/{name}` | Implementiert | Standardmäßig nur Katalogeintrag; Registry-Löschung ausdrücklich konfigurierbar. |
| GET | `/v1/xcloud/networks` | Implementiert | Einzel-/Listen-Datenquellen und Refresh; fehlende verwaltete Netze erzeugen wegen Ausfall-Ambiguität einen Fehler. |
| POST | `/v1/xcloud/networks` | Implementiert | Flache Felder und beliebiges spec_json; Änderungen ersetzen das Netz. |
| DELETE | `/v1/xcloud/networks/{name}` | Implementiert | Delete a tenant-owned Xcloud network |
| GET | `/v1/xcloud-security-groups` | Implementiert | List Xcloud security groups in the tenant namespace |
| POST | `/v1/xcloud-security-groups` | Implementiert | Create a security group in the tenant Xcloud namespace |
| GET | `/v1/xcloud-security-groups/{name}` | Implementiert | Read one Xcloud security group (with its rules) |
| PATCH | `/v1/xcloud-security-groups/{name}` | Implementiert | Replace a security group rule set (and/or labels) |
| DELETE | `/v1/xcloud-security-groups/{name}` | Implementiert | Delete a tenant-owned Xcloud security group |
| GET | `/v1/xcloud/flavors` | Implementiert | List enabled global Xcloud flavors |
| GET | `/v1/xcloud/elastic-ips` | Implementiert | List tenant Xcloud Elastic IPs |
| POST | `/v1/xcloud/elastic-ips` | Implementiert | Allocate an Elastic IP |
| PATCH | `/v1/xcloud/elastic-ips/{id}/target` | Implementiert | Attach or detach an Elastic IP |
| DELETE | `/v1/xcloud/elastic-ips/{id}` | Implementiert | Release an Elastic IP |
| POST | `/v1/xcloud/instances/{id}/agent/exec` | Nicht implementiert | Operative Gast-/Job-Funktion; nicht als Terraform-Ressource implementiert. |
| POST | `/v1/xcloud/instances/{id}/agent/files` | Nicht implementiert | Operative Gast-/Job-Funktion; nicht als Terraform-Ressource implementiert. |
| GET | `/v1/xcloud/instances/{id}/agent/files` | Nicht implementiert | Operative Gast-/Job-Funktion; nicht als Terraform-Ressource implementiert. |

SHA-256 des gebündelten, auf die relevanten öffentlichen Routen reduzierten OpenAPI-Vertrags: `b2ae05c062878c091b465d1d3b1cf198ddf8d7b4e0986e4578045e5bf3f37043`. Er stammt aus dem API-Quellcode-Export dieser Prüfung, nicht aus einer Abfrage einer laufenden Cloud.
