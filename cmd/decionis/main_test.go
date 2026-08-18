package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestDeriveAction(t *testing.T) {
	cases := map[string][]string{
		"terraform.apply": {"terraform", "apply"},
		"npm.publish":     {"/usr/local/bin/npm", "publish"},
		"kubectl.delete":  {"kubectl", "delete", "deploy/api"},
		"deploy.sh":       {"./deploy.sh"},
		"rm":              {"rm", "-rf", "/"},
	}
	for want, command := range cases {
		if got := deriveAction(command); got != want {
			t.Errorf("deriveAction(%v) = %q, want %q", command, got, want)
		}
	}
}

func TestParseContextEntries(t *testing.T) {
	payload, err := parseContextEntries([]string{"change_freeze=true", "environment=production", "amount=250000", "ratio=0.93"})
	if err != nil {
		t.Fatal(err)
	}
	if payload["change_freeze"] != true || payload["environment"] != "production" ||
		payload["amount"] != float64(250000) || payload["ratio"] != 0.93 {
		t.Fatalf("unexpected payload: %#v", payload)
	}
	if _, err := parseContextEntries([]string{"missing-equals"}); err == nil {
		t.Fatal("entries without = must be rejected")
	}
}

func TestMergeMcpConfigPreservesExistingServers(t *testing.T) {
	existing := []byte(`{"mcpServers":{"other":{"command":"npx","args":["-y","other-server"]}},"unrelated":true}`)
	merged, err := mergeMcpConfig(existing, "decionis/mcp", "/workspaces/app/DECIONIS_POLICY.md")
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if err := json.Unmarshal(merged, &document); err != nil {
		t.Fatalf("merged output is not JSON: %v", err)
	}
	servers := document["mcpServers"].(map[string]any)
	if _, ok := servers["other"]; !ok {
		t.Fatal("existing server entry must be preserved")
	}
	if document["unrelated"] != true {
		t.Fatal("unrelated top-level keys must be preserved")
	}
	decionis := servers["decionis"].(map[string]any)
	if decionis["command"] != "docker" {
		t.Fatalf("unexpected command: %v", decionis["command"])
	}
	if !strings.Contains(string(merged), "/workspaces/app/DECIONIS_POLICY.md:/work/DECIONIS_POLICY.md:ro") {
		t.Fatal("policy mount missing")
	}

	if _, err := mergeMcpConfig([]byte("{corrupt"), "img", "/p"); err == nil {
		t.Fatal("corrupt existing .mcp.json must be rejected")
	}
}
