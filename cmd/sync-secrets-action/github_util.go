package main

import (
	"encoding/base64"
	"fmt"

	"github.com/google/go-github/v67/github"
	"golang.org/x/crypto/nacl/box"

	crypto_rand "crypto/rand"
)

func encryptSecretWithPublicKey(publicKey *github.PublicKey, secretName, secretValue string) (*github.EncryptedSecret, error) {
	decodedPublicKey, err := base64.StdEncoding.DecodeString(publicKey.GetKey())
	if err != nil {
		return nil, fmt.Errorf("failed to decode public key: %v", err)
	}

	var boxKey [32]byte
	copy(boxKey[:], decodedPublicKey)
	secretBytes := []byte(secretValue)
	encryptedBytes, err := box.SealAnonymous([]byte{}, secretBytes, &boxKey, crypto_rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt secret: %v", err)
	}

	encryptedString := base64.StdEncoding.EncodeToString(encryptedBytes)

	keyID := publicKey.GetKeyID()
	encryptedSecret := &github.EncryptedSecret{
		Name:           secretName,
		KeyID:          keyID,
		EncryptedValue: encryptedString,
	}
	return encryptedSecret, nil
}

func encryptDependabotWithPublicKey(publicKey *github.PublicKey, secretName, secretValue string) (*github.DependabotEncryptedSecret, error) {
	decodedPublicKey, err := base64.StdEncoding.DecodeString(publicKey.GetKey())
	if err != nil {
		return nil, fmt.Errorf("failed to decode public key: %v", err)
	}

	var boxKey [32]byte
	copy(boxKey[:], decodedPublicKey)
	secretBytes := []byte(secretValue)
	encryptedBytes, err := box.SealAnonymous([]byte{}, secretBytes, &boxKey, crypto_rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt secret: %v", err)
	}

	encryptedString := base64.StdEncoding.EncodeToString(encryptedBytes)

	keyID := publicKey.GetKeyID()
	encryptedSecret := &github.DependabotEncryptedSecret{
		Name:           secretName,
		KeyID:          keyID,
		EncryptedValue: encryptedString,
	}
	return encryptedSecret, nil
}
