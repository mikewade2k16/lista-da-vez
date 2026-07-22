package omnichannel

import "testing"

func TestProviderKeyringPreservesMultipleProvidersAndLegacy(t *testing.T) {
	svc := &AIService{box: testBrainBox(t)}
	legacyCipher, err := svc.box.Encrypt("gemini-secret")
	if err != nil {
		t.Fatal(err)
	}
	legacy, err := svc.decodeProviderKeyring(legacyCipher, "gemini")
	if err != nil || legacy["gemini"] != "gemini-secret" {
		t.Fatalf("legacy=%#v err=%v", legacy, err)
	}
	legacy["openai"] = "openai-secret"
	ciphertext, _, err := svc.encryptProviderKeyring(legacy)
	if err != nil {
		t.Fatal(err)
	}
	got, err := svc.decodeProviderKeyring(ciphertext, "gemini")
	if err != nil || got["gemini"] != "gemini-secret" || got["openai"] != "openai-secret" {
		t.Fatalf("keyring=%#v err=%v", got, err)
	}
}

func TestProviderKeyringRejectsUnknownProvider(t *testing.T) {
	if normalizeAIProviderKeyID("unknown") != "" {
		t.Fatal("unknown provider must fail closed")
	}
}
