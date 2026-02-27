package providers

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
)

func canonicalAnnouncementMessage(providerID string, assetID string, transport string, baseURL string, expiresAt int64, nonce string) string {
	return providerID + "|" + assetID + "|" + transport + "|" + baseURL + "|" + strconv.FormatInt(expiresAt, 10) + "|" + nonce
}

func verifyAnnouncementSignature(publicKeyHex string, signatureHex string, message string) error {
	pubKeyBytes, err := hex.DecodeString(strings.TrimSpace(publicKeyHex))
	if err != nil {
		return fmt.Errorf("decode public key: %w", err)
	}
	if len(pubKeyBytes) != 33 {
		return fmt.Errorf("public key must be 33 bytes compressed")
	}

	pubKey, err := btcec.ParsePubKey(pubKeyBytes)
	if err != nil {
		return fmt.Errorf("parse public key: %w", err)
	}

	sigBytes, err := hex.DecodeString(strings.TrimSpace(signatureHex))
	if err != nil {
		return fmt.Errorf("decode signature: %w", err)
	}
	sig, err := schnorr.ParseSignature(sigBytes)
	if err != nil {
		return fmt.Errorf("parse signature: %w", err)
	}

	hash := sha256.Sum256([]byte(message))
	if !sig.Verify(hash[:], pubKey) {
		return fmt.Errorf("invalid signature")
	}
	return nil
}
