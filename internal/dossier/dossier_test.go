package dossier

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"

	"github.com/decionis/docker/internal/canonicaljson"
)

func loadFixture(t *testing.T) (payload map[string]any, jwks *Jwks) {
	t.Helper()
	rawDossier, err := os.ReadFile("testdata/dossier.json")
	if err != nil {
		t.Fatalf("read dossier fixture: %v", err)
	}
	decoded, err := canonicaljson.Decode(rawDossier)
	if err != nil {
		t.Fatalf("decode dossier fixture: %v", err)
	}
	payload, ok := decoded.(map[string]any)
	if !ok {
		t.Fatalf("dossier fixture is not an object")
	}
	rawJwks, err := os.ReadFile("testdata/jwks.json")
	if err != nil {
		t.Fatalf("read jwks fixture: %v", err)
	}
	jwks = &Jwks{}
	if err := json.Unmarshal(rawJwks, jwks); err != nil {
		t.Fatalf("decode jwks fixture: %v", err)
	}
	return payload, jwks
}

// TestVerifyFixtureEndToEnd proves the whole chain: canonical bytes match the
// node-produced expectation, the SHA-256 matches, and the real Ed25519
// signature verifies via the JWKS.
func TestVerifyFixtureEndToEnd(t *testing.T) {
	payload, jwks := loadFixture(t)

	rawExpected, err := os.ReadFile("testdata/expected.json")
	if err != nil {
		t.Fatalf("read expected fixture: %v", err)
	}
	var expected struct {
		CanonicalDocument string `json:"canonical_document"`
		DocumentSHA256    string `json:"document_sha256"`
	}
	if err := json.Unmarshal(rawExpected, &expected); err != nil {
		t.Fatalf("decode expected fixture: %v", err)
	}

	document, found := resolveJSONPointer(payload, "/decision")
	if !found {
		t.Fatal("fixture missing /decision document")
	}
	canonical, err := canonicaljson.Stringify(document)
	if err != nil {
		t.Fatalf("canonicalize: %v", err)
	}
	if canonical != expected.CanonicalDocument {
		t.Fatalf("canonical bytes diverge from node:\n node: %s\n go:   %s", expected.CanonicalDocument, canonical)
	}
	digest := sha256.Sum256([]byte(canonical))
	if hex.EncodeToString(digest[:]) != expected.DocumentSHA256 {
		t.Fatalf("sha256 diverges from node")
	}

	result := VerifyProofBundle(payload, jwks)
	if !result.Available {
		t.Fatal("proof bundle should be available")
	}
	if !result.Verified {
		t.Fatalf("fixture must verify; checks: %+v", result.Checks)
	}
	if result.ArtifactsChecked != 1 {
		t.Fatalf("expected 1 artifact checked, got %d", result.ArtifactsChecked)
	}
}

func TestVerifyFailsClosedOnTamperedDocument(t *testing.T) {
	payload, jwks := loadFixture(t)
	decision := payload["decision"].(map[string]any)
	decision["outcome"] = "APPROVE" // attacker flips the verdict

	result := VerifyProofBundle(payload, jwks)
	if result.Verified {
		t.Fatal("tampered document must not verify")
	}
	foundShaFail := false
	for _, check := range result.Checks {
		if check.Key == "portable_artifact:sha256" && !check.Verified && check.Severity == "fail" {
			foundShaFail = true
		}
	}
	if !foundShaFail {
		t.Fatalf("expected sha256 fail check, got %+v", result.Checks)
	}
}

func TestVerifyFailsClosedOnTamperedSignature(t *testing.T) {
	payload, jwks := loadFixture(t)
	integrity := payload["integrity"].(map[string]any)
	bundle := integrity["proof_bundle"].(map[string]any)
	artifact := bundle["artifacts"].([]any)[0].(map[string]any)
	artifact["signature"] = "AAAA" + artifact["signature"].(string)[4:]

	result := VerifyProofBundle(payload, jwks)
	if result.Verified {
		t.Fatal("tampered signature must not verify")
	}
}

func TestVerifyFailsClosedOnUnknownKey(t *testing.T) {
	payload, _ := loadFixture(t)
	wrongJwks := &Jwks{Keys: []map[string]any{{
		"kty": "OKP", "crv": "Ed25519", "kid": "some-other-key",
		"x": "JrQLj5P_89iXES9-vFgrIy29clF9CC_oPPsw3c5D0bs",
	}}}

	result := VerifyProofBundle(payload, wrongJwks)
	if result.Verified {
		t.Fatal("unknown key must not verify")
	}
	if !result.Available {
		t.Fatal("bundle is present; available must be true")
	}
	if result.Checks[0].Key != "public_key" || result.Checks[0].Severity != "fail" {
		t.Fatalf("expected public_key fail, got %+v", result.Checks[0])
	}
}

func TestVerifyReportsMissingBundleAsUnavailable(t *testing.T) {
	result := VerifyProofBundle(map[string]any{"decision": map[string]any{}}, nil)
	if result.Verified || result.Available {
		t.Fatal("missing bundle must be unverified and unavailable")
	}
	if result.Checks[0].Severity != "warn" {
		t.Fatalf("missing bundle is a warn, got %+v", result.Checks[0])
	}
}

func TestExtractDossierPayloadUnwrapsEnvelopes(t *testing.T) {
	payload, _ := loadFixture(t)
	for _, wrapper := range []string{"dossier", "dossier_payload", "data", "result"} {
		extracted := ExtractDossierPayload(map[string]any{wrapper: payload})
		if _, ok := asRecord(extracted["integrity"])["proof_bundle"]; !ok {
			t.Fatalf("wrapper %q not unwrapped", wrapper)
		}
	}
	direct := ExtractDossierPayload(payload)
	if _, ok := asRecord(direct["integrity"])["proof_bundle"]; !ok {
		t.Fatal("raw payload not accepted")
	}

	nested := ExtractDossierPayload(map[string]any{
		"service": "decionis-protocol",
		"dossier": map[string]any{
			"dossier_id":      "11111111-1111-4111-8111-111111111111",
			"dossier_payload": payload,
		},
	})
	if _, ok := asRecord(nested["integrity"])["proof_bundle"]; !ok {
		t.Fatal("nested dossier.dossier_payload envelope not unwrapped")
	}
}

func TestResolveJwksURL(t *testing.T) {
	payload, _ := loadFixture(t)
	got := ResolveJwksURL(payload, "https://api.decionis.com/v1/protocol/dossiers/abc")
	want := "https://api.decionis.com/.well-known/decionis-dossier-jwks.json"
	if got != want {
		t.Fatalf("relative resolution: got %q want %q", got, want)
	}

	integrity := payload["integrity"].(map[string]any)
	bundle := integrity["proof_bundle"].(map[string]any)
	rotation := bundle["rotation_policy"].(map[string]any)
	rotation["public_jwks_url"] = "https://keys.decionis.com/jwks.json"
	if got := ResolveJwksURL(payload, "https://api.decionis.com/x"); got != "https://keys.decionis.com/jwks.json" {
		t.Fatalf("absolute URL must win, got %q", got)
	}
}

func TestAssessReproducibilityFixtureIsReady(t *testing.T) {
	payload, _ := loadFixture(t)
	assessment := AssessReproducibility(payload)
	if assessment.Posture != "reproduction_ready" {
		t.Fatalf("fixture carries all inputs; got %s (%s)", assessment.Posture, assessment.Detail)
	}

	delete(payload, "inputs_snapshot")
	assessment = AssessReproducibility(payload)
	if assessment.Posture != "incomplete" {
		t.Fatalf("expected incomplete after dropping inputs, got %s", assessment.Posture)
	}

	empty := AssessReproducibility(map[string]any{})
	if empty.Posture != "signature_only" {
		t.Fatalf("expected signature_only for empty payload, got %s", empty.Posture)
	}
}
