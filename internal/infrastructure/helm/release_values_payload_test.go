package helm

import (
	"encoding/json"
	"testing"
)

func TestValuesPayload_UnmarshalJSON_Map(t *testing.T) {
	data := []byte(`{"replicaCount": 3, "image": {"repository": "nginx", "tag": "alpine"}}`)
	var payload ValuesPayload
	err := json.Unmarshal(data, &payload)
	if err != nil {
		t.Fatalf("failed to unmarshal JSON map: %v", err)
	}

	m := payload.ToMap()
	if m["replicaCount"] != float64(3) {
		t.Fatalf("expected replicaCount 3, got %v", m["replicaCount"])
	}
	img, ok := m["image"].(map[string]interface{})
	if !ok || img["repository"] != "nginx" || img["tag"] != "alpine" {
		t.Fatalf("unexpected image sub-map: %+v", m["image"])
	}
}

func TestValuesPayload_UnmarshalJSON_RawYAMLString(t *testing.T) {
	// JSON encoded string with YAML payload
	data := []byte(`"replicaCount: 5\nimage:\n  repository: redis\n  tag: \"7.0\""`)
	var payload ValuesPayload
	err := json.Unmarshal(data, &payload)
	if err != nil {
		t.Fatalf("failed to unmarshal raw YAML string: %v", err)
	}

	m := payload.ToMap()
	if m["replicaCount"] != 5 && m["replicaCount"] != float64(5) {
		t.Fatalf("expected replicaCount 5, got %v (%T)", m["replicaCount"], m["replicaCount"])
	}
	img, ok := m["image"].(map[string]interface{})
	if !ok || img["repository"] != "redis" || img["tag"] != "7.0" {
		t.Fatalf("unexpected image sub-map: %+v", m["image"])
	}
}

func TestValuesPayload_UnmarshalJSON_EmptyAndNull(t *testing.T) {
	testCases := [][]byte{
		[]byte(`null`),
		[]byte(`""`),
		[]byte(`{}`),
		[]byte(`"   "`),
	}

	for _, tc := range testCases {
		var payload ValuesPayload
		err := json.Unmarshal(tc, &payload)
		if err != nil {
			t.Fatalf("unexpected error unmarshaling %s: %v", string(tc), err)
		}
		if payload == nil {
			t.Fatalf("expected non-nil map for %s", string(tc))
		}
		if len(payload) != 0 {
			t.Fatalf("expected empty map for %s, got %v", string(tc), payload)
		}
	}
}

func TestInstallRequest_Values_BothFormats(t *testing.T) {
	// 1. Map format
	mapJSON := []byte(`{
		"releaseName": "my-nginx",
		"chart": "nginx",
		"values": {
			"service": {"port": 80}
		}
	}`)
	var reqMap InstallRequest
	if err := json.Unmarshal(mapJSON, &reqMap); err != nil {
		t.Fatalf("failed to unmarshal map InstallRequest: %v", err)
	}
	if reqMap.ReleaseName != "my-nginx" {
		t.Fatalf("expected releaseName my-nginx, got %s", reqMap.ReleaseName)
	}
	if reqMap.Values["service"] == nil {
		t.Fatalf("expected service in values, got %v", reqMap.Values)
	}

	// 2. YAML string format
	yamlJSON := []byte(`{
		"releaseName": "my-redis",
		"chart": "redis",
		"values": "service:\n  port: 6379\n"
	}`)
	var reqYAML InstallRequest
	if err := json.Unmarshal(yamlJSON, &reqYAML); err != nil {
		t.Fatalf("failed to unmarshal yaml string InstallRequest: %v", err)
	}
	if reqYAML.ReleaseName != "my-redis" {
		t.Fatalf("expected releaseName my-redis, got %s", reqYAML.ReleaseName)
	}
	svc, ok := reqYAML.Values["service"].(map[string]interface{})
	if !ok || (svc["port"] != 6379 && svc["port"] != float64(6379)) {
		t.Fatalf("expected service.port 6379, got %v", reqYAML.Values["service"])
	}
}
