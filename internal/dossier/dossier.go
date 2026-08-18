// Package dossier is a faithful Go port of @decionis/verify: offline Ed25519
// verification of a Decision Dossier's v2.0 proof bundle against a public
// JWKS, plus the reproducibility posture assessment. Verification only —
// signing material never exists in this repository
// (rules/security.rules.md Rule 0.4).
//
// All payload values must come from canonicaljson.Decode (json.Number
// preserved) or verification would hash different bytes than were signed.
package dossier

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/decionis/docker/internal/canonicaljson"
)

// ProofArtifact mirrors @decionis/verify's ProofArtifact.
type ProofArtifact struct {
	ArtifactKind            string `json:"artifact_kind"`
	MediaType               string `json:"media_type"`
	DocumentPath            string `json:"document_path"`
	CanonicalDocumentSHA256 string `json:"canonical_document_sha256"`
	Signature               string `json:"signature"`
}

// KeyRotationPolicy mirrors @decionis/verify's KeyRotationPolicy.
type KeyRotationPolicy struct {
	Strategy                    string   `json:"strategy"`
	ActiveKeyID                 string   `json:"active_key_id"`
	PreviousKeyIDs              []string `json:"previous_key_ids"`
	VerificationGracePeriodDays int      `json:"verification_grace_period_days"`
	RotatedAt                   *string  `json:"rotated_at"`
	PublicJwksPath              string   `json:"public_jwks_path"`
	PublicJwksURL               string   `json:"public_jwks_url,omitempty"`
}

// ProofBundle mirrors @decionis/verify's DecisionDossierProofBundle.
type ProofBundle struct {
	BundleType     string            `json:"bundle_type"`
	Version        string            `json:"version"`
	IssuedAt       string            `json:"issued_at"`
	Algorithm      string            `json:"algorithm"`
	KeyID          string            `json:"key_id"`
	RotationPolicy KeyRotationPolicy `json:"rotation_policy"`
	Artifacts      []ProofArtifact   `json:"artifacts"`
}

// Jwks is a public JWKS document.
type Jwks struct {
	Keys []map[string]any `json:"keys"`
}

// VerifyCheck mirrors @decionis/verify's VerifyCheck.
type VerifyCheck struct {
	Key      string `json:"key"`
	Label    string `json:"label"`
	Verified bool   `json:"verified"`
	Severity string `json:"severity"` // pass | warn | fail
	Detail   string `json:"detail"`
}

// VerifyResult mirrors @decionis/verify's VerifyResult. Verified is true only
// when every check passed; any error path yields Verified=false (fail closed).
type VerifyResult struct {
	Verified         bool          `json:"verified"`
	Available        bool          `json:"available"`
	KeyID            *string       `json:"key_id"`
	ArtifactsChecked int           `json:"artifacts_checked"`
	Checks           []VerifyCheck `json:"checks"`
}

func asRecord(value any) map[string]any {
	record, ok := value.(map[string]any)
	if !ok {
		return map[string]any{}
	}
	return record
}

func asString(value any) (string, bool) {
	s, ok := value.(string)
	return s, ok
}

// resolveJSONPointer mirrors the upstream RFC 6901 resolution (objects only).
func resolveJSONPointer(root any, pointer string) (any, bool) {
	if !strings.HasPrefix(pointer, "/") {
		return nil, false
	}
	current := root
	for _, rawSegment := range strings.Split(pointer, "/")[1:] {
		segment := strings.ReplaceAll(strings.ReplaceAll(rawSegment, "~1", "/"), "~0", "~")
		record, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		current, ok = record[segment]
		if !ok {
			return nil, false
		}
	}
	return current, true
}

// ParseProofBundle validates and normalizes a persisted proof bundle,
// mirroring upstream parseProofBundle. Returns nil when the value is not a
// v2.0 Ed25519 proof bundle with at least one well-formed artifact.
func ParseProofBundle(value any) *ProofBundle {
	record := asRecord(value)
	bundleType, _ := asString(record["bundle_type"])
	version, _ := asString(record["version"])
	issuedAt, okIssued := asString(record["issued_at"])
	keyID, okKey := asString(record["key_id"])
	if bundleType != "decionis.decision_dossier.proof_bundle" || version != "2.0" || !okIssued || !okKey {
		return nil
	}

	rotation := asRecord(record["rotation_policy"])
	activeKeyID, okActive := asString(rotation["active_key_id"])
	publicJwksPath, okPath := asString(rotation["public_jwks_path"])
	if !okActive || activeKeyID == "" || !okPath || publicJwksPath == "" {
		return nil
	}

	var artifacts []ProofArtifact
	if list, ok := record["artifacts"].([]any); ok {
		for _, entry := range list {
			item := asRecord(entry)
			kind, _ := asString(item["artifact_kind"])
			mediaType, _ := asString(item["media_type"])
			documentPath, _ := asString(item["document_path"])
			sha, _ := asString(item["canonical_document_sha256"])
			signature, _ := asString(item["signature"])
			if kind == "" || mediaType == "" || documentPath == "" || sha == "" || signature == "" {
				continue
			}
			artifacts = append(artifacts, ProofArtifact{
				ArtifactKind:            kind,
				MediaType:               mediaType,
				DocumentPath:            documentPath,
				CanonicalDocumentSHA256: sha,
				Signature:               signature,
			})
		}
	}
	if len(artifacts) == 0 {
		return nil
	}

	var previousKeyIDs []string
	if list, ok := rotation["previous_key_ids"].([]any); ok {
		for _, entry := range list {
			if s, ok := asString(entry); ok {
				previousKeyIDs = append(previousKeyIDs, s)
			}
		}
	}

	gracePeriod := 0
	if raw, ok := rotation["verification_grace_period_days"]; ok {
		if parsed, err := strconv.Atoi(fmt.Sprint(raw)); err == nil {
			gracePeriod = parsed
		}
	}

	strategy, _ := asString(rotation["strategy"])
	if strategy == "" {
		strategy = "JWKS_OVERLAP"
	}
	var rotatedAt *string
	if s, ok := asString(rotation["rotated_at"]); ok {
		rotatedAt = &s
	}
	publicJwksURL, _ := asString(rotation["public_jwks_url"])

	return &ProofBundle{
		BundleType: "decionis.decision_dossier.proof_bundle",
		Version:    "2.0",
		IssuedAt:   issuedAt,
		Algorithm:  "Ed25519",
		KeyID:      keyID,
		RotationPolicy: KeyRotationPolicy{
			Strategy:                    strategy,
			ActiveKeyID:                 activeKeyID,
			PreviousKeyIDs:              previousKeyIDs,
			VerificationGracePeriodDays: gracePeriod,
			RotatedAt:                   rotatedAt,
			PublicJwksPath:              publicJwksPath,
			PublicJwksURL:               publicJwksURL,
		},
		Artifacts: artifacts,
	}
}

func findJwk(jwks *Jwks, keyIDs []string) map[string]any {
	if jwks == nil {
		return nil
	}
	wanted := map[string]bool{}
	for _, keyID := range keyIDs {
		if strings.TrimSpace(keyID) != "" {
			wanted[keyID] = true
		}
	}
	if len(wanted) == 0 {
		return nil
	}
	for _, key := range jwks.Keys {
		kty, _ := asString(key["kty"])
		crv, _ := asString(key["crv"])
		kid, _ := asString(key["kid"])
		if kty == "OKP" && crv == "Ed25519" && wanted[kid] {
			return key
		}
	}
	return nil
}

func publicKeyFromJwk(jwk map[string]any) (ed25519.PublicKey, error) {
	x, _ := asString(jwk["x"])
	raw, err := base64.RawURLEncoding.DecodeString(x)
	if err != nil {
		return nil, fmt.Errorf("invalid JWK x coordinate: %w", err)
	}
	if len(raw) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("invalid Ed25519 public key length %d", len(raw))
	}
	return ed25519.PublicKey(raw), nil
}

// VerifyProofBundle independently verifies a dossier payload's Ed25519 proof
// bundle against a public JWKS — the Go parity of upstream
// verifyDossierProofBundle. Every failure resolves to Verified=false.
func VerifyProofBundle(payload map[string]any, jwks *Jwks) VerifyResult {
	bundle := ParseProofBundle(asRecord(payload["integrity"])["proof_bundle"])
	if bundle == nil {
		return VerifyResult{
			Verified:  false,
			Available: false,
			Checks: []VerifyCheck{{
				Key:      "proof_bundle",
				Label:    "Proof Bundle",
				Verified: false,
				Severity: "warn",
				Detail:   "No persisted Ed25519 proof bundle was available for verification.",
			}},
		}
	}

	keyIDs := append([]string{bundle.KeyID, bundle.RotationPolicy.ActiveKeyID}, bundle.RotationPolicy.PreviousKeyIDs...)
	jwk := findJwk(jwks, keyIDs)
	if jwk == nil {
		return VerifyResult{
			Verified:  false,
			Available: true,
			KeyID:     &bundle.KeyID,
			Checks: []VerifyCheck{{
				Key:      "public_key",
				Label:    "Public Key",
				Verified: false,
				Severity: "fail",
				Detail:   fmt.Sprintf("No Ed25519 public key matched proof bundle key %s.", bundle.KeyID),
			}},
		}
	}

	publicKey, err := publicKeyFromJwk(jwk)
	if err != nil {
		return VerifyResult{
			Verified:  false,
			Available: true,
			KeyID:     &bundle.KeyID,
			Checks: []VerifyCheck{{
				Key:      "public_key",
				Label:    "Public Key",
				Verified: false,
				Severity: "fail",
				Detail:   err.Error(),
			}},
		}
	}

	kid, _ := asString(jwk["kid"])
	if kid == "" {
		kid = bundle.KeyID
	}
	checks := []VerifyCheck{{
		Key:      "public_key",
		Label:    "Public Key",
		Verified: true,
		Severity: "pass",
		Detail:   fmt.Sprintf("Loaded Ed25519 public key %s.", kid),
	}}
	artifactsChecked := 0

	for _, artifact := range bundle.Artifacts {
		document, found := resolveJSONPointer(payload, artifact.DocumentPath)
		if !found {
			checks = append(checks, VerifyCheck{
				Key:      artifact.ArtifactKind + ":document",
				Label:    artifact.ArtifactKind + " Document",
				Verified: false,
				Severity: "fail",
				Detail:   fmt.Sprintf("Signed document path %s was not present in the dossier payload.", artifact.DocumentPath),
			})
			continue
		}

		canonicalDocument, err := canonicaljson.Stringify(document)
		if err != nil {
			checks = append(checks, VerifyCheck{
				Key:      artifact.ArtifactKind + ":document",
				Label:    artifact.ArtifactKind + " Document",
				Verified: false,
				Severity: "fail",
				Detail:   fmt.Sprintf("Signed document could not be canonicalized: %v.", err),
			})
			continue
		}

		digest := sha256.Sum256([]byte(canonicalDocument))
		actualSha := hex.EncodeToString(digest[:])
		hashVerified := strings.EqualFold(actualSha, artifact.CanonicalDocumentSHA256)
		hashDetail := fmt.Sprintf("Canonical digest %s.", actualSha)
		if !hashVerified {
			hashDetail = fmt.Sprintf("Expected %s, computed %s.", artifact.CanonicalDocumentSHA256, actualSha)
		}
		checks = append(checks, VerifyCheck{
			Key:      artifact.ArtifactKind + ":sha256",
			Label:    artifact.ArtifactKind + " SHA-256",
			Verified: hashVerified,
			Severity: severityFor(hashVerified),
			Detail:   hashDetail,
		})

		signatureVerified := false
		if signature, err := base64.RawURLEncoding.DecodeString(artifact.Signature); err == nil {
			signatureVerified = ed25519.Verify(publicKey, []byte(canonicalDocument), signature)
		}
		signatureDetail := "Ed25519 signature verified against the public JWKS."
		if !signatureVerified {
			signatureDetail = "Ed25519 signature did not verify against the public JWKS."
		}
		checks = append(checks, VerifyCheck{
			Key:      artifact.ArtifactKind + ":signature",
			Label:    artifact.ArtifactKind + " Signature",
			Verified: signatureVerified,
			Severity: severityFor(signatureVerified),
			Detail:   signatureDetail,
		})
		artifactsChecked++
	}

	verified := true
	for _, check := range checks {
		if !check.Verified {
			verified = false
			break
		}
	}
	return VerifyResult{
		Verified:         verified,
		Available:        true,
		KeyID:            &bundle.KeyID,
		ArtifactsChecked: artifactsChecked,
		Checks:           checks,
	}
}

func severityFor(ok bool) string {
	if ok {
		return "pass"
	}
	return "fail"
}

// ExtractDossierPayload pulls the signed payload out of whatever envelope a
// public endpoint returned, mirroring upstream extractDossierPayload.
func ExtractDossierPayload(body any) map[string]any {
	record := asRecord(body)
	if _, ok := asRecord(record["integrity"])["proof_bundle"]; ok {
		return record
	}
	for _, key := range []string{"dossier", "dossier_payload", "data", "result"} {
		candidate := asRecord(record[key])
		if _, ok := asRecord(candidate["integrity"])["proof_bundle"]; ok {
			return candidate
		}
	}
	return record
}

// ResolveJwksURL works out where to fetch the public JWKS for a dossier,
// mirroring upstream resolveJwksUrl: prefer the absolute URL the public API
// embeds; otherwise resolve the relative path against the dossier's URL.
func ResolveJwksURL(payload map[string]any, dossierURL string) string {
	bundle := ParseProofBundle(asRecord(payload["integrity"])["proof_bundle"])
	if bundle == nil {
		return ""
	}
	absolute := bundle.RotationPolicy.PublicJwksURL
	if strings.HasPrefix(strings.ToLower(absolute), "http://") ||
		strings.HasPrefix(strings.ToLower(absolute), "https://") {
		return absolute
	}
	base, err := url.Parse(dossierURL)
	if err != nil {
		return ""
	}
	relative, err := url.Parse(bundle.RotationPolicy.PublicJwksPath)
	if err != nil {
		return ""
	}
	return base.ResolveReference(relative).String()
}
