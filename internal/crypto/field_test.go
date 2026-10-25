package crypto

import (
	"bytes"
	"testing"
)

func TestFieldCryptoRoundTrip(t *testing.T) {
	key, err := GenerateMasterKey()
	if err != nil {
		t.Fatal(err)
	}
	enc, err := EncryptField("sk-secret-value", key)
	if err != nil {
		t.Fatal(err)
	}
	if enc.EncryptionVersion != CurrentEncryptionVersion {
		t.Fatalf("version %d", enc.EncryptionVersion)
	}
	if bytes.Contains(enc.Ciphertext, []byte("sk-secret")) {
		t.Fatal("ciphertext leaks plaintext")
	}
	got, err := DecryptField(enc, key)
	if err != nil || got != "sk-secret-value" {
		t.Fatalf("round trip: got %q err=%v", got, err)
	}
}

func TestFieldCryptoTamper(t *testing.T) {
	key, _ := GenerateMasterKey()
	enc, err := EncryptField("sk-secret-value", key)
	if err != nil {
		t.Fatal(err)
	}
	enc.Ciphertext[0] ^= 0xff // GCM auth tag must reject a flipped byte
	if _, err := DecryptField(enc, key); err == nil {
		t.Fatal("tampered ciphertext decrypted")
	}
	other, _ := GenerateMasterKey()
	if _, err := DecryptField(enc, other); err == nil {
		t.Fatal("wrong master key decrypted")
	}
}

func TestMasterKeyLength(t *testing.T) {
	if _, err := EncryptField("x", []byte("short")); err == nil {
		t.Fatal("short key accepted")
	}
}
