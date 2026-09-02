package keepz

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"strings"
)

type encryptedMessage struct {
	Identifier    string `json:"identifier,omitempty"`
	EncryptedData string `json:"encryptedData"`
	EncryptedKeys string `json:"encryptedKeys"`
	AES           bool   `json:"aes"`
}

func (c *API) encryptPayload(v any) (encryptedMessage, error) {
	if c.keepzPublicKey == nil {
		return encryptedMessage{}, errors.New("keepz public key is required for encrypted ecommerce requests")
	}
	plain, err := json.Marshal(v)
	if err != nil {
		return encryptedMessage{}, err
	}
	key := make([]byte, 32)
	iv := make([]byte, aes.BlockSize)
	if _, err := rand.Read(key); err != nil {
		return encryptedMessage{}, err
	}
	if _, err := rand.Read(iv); err != nil {
		return encryptedMessage{}, err
	}

	encryptedData, err := encryptAESCBC(plain, key, iv)
	if err != nil {
		return encryptedMessage{}, err
	}
	keyMaterial := base64.StdEncoding.EncodeToString(key) + "." + base64.StdEncoding.EncodeToString(iv)
	encryptedKeys, err := rsa.EncryptOAEP(sha256.New(), rand.Reader, c.keepzPublicKey, []byte(keyMaterial), nil)
	if err != nil {
		return encryptedMessage{}, err
	}
	return encryptedMessage{
		EncryptedData: base64.StdEncoding.EncodeToString(encryptedData),
		EncryptedKeys: base64.StdEncoding.EncodeToString(encryptedKeys),
		AES:           true,
	}, nil
}

func (c *API) decryptMessage(msg encryptedMessage) ([]byte, error) {
	if c.integratorPrivateKey == nil {
		return nil, errors.New("integrator private key is required for encrypted ecommerce responses")
	}
	encryptedKeys, err := base64.StdEncoding.DecodeString(msg.EncryptedKeys)
	if err != nil {
		return nil, fmt.Errorf("decode encrypted keys: %w", err)
	}
	keyMaterial, err := rsa.DecryptOAEP(sha256.New(), rand.Reader, c.integratorPrivateKey, encryptedKeys, nil)
	if err != nil {
		return nil, fmt.Errorf("decrypt encrypted keys: %w", err)
	}
	parts := strings.Split(string(keyMaterial), ".")
	if len(parts) != 2 {
		return nil, errors.New("invalid encrypted keys format")
	}
	key, err := base64.StdEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, fmt.Errorf("decode aes key: %w", err)
	}
	iv, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("decode aes iv: %w", err)
	}
	encryptedData, err := base64.StdEncoding.DecodeString(msg.EncryptedData)
	if err != nil {
		return nil, fmt.Errorf("decode encrypted data: %w", err)
	}
	return decryptAESCBC(encryptedData, key, iv)
}

func encryptAESCBC(plain, key, iv []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	plain = pkcs7Pad(plain, block.BlockSize())
	out := make([]byte, len(plain))
	cipher.NewCBCEncrypter(block, iv).CryptBlocks(out, plain)
	return out, nil
}

func decryptAESCBC(ciphertext, key, iv []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	if len(iv) != block.BlockSize() {
		return nil, errors.New("invalid aes iv size")
	}
	if len(ciphertext) == 0 || len(ciphertext)%block.BlockSize() != 0 {
		return nil, errors.New("invalid aes ciphertext size")
	}
	out := make([]byte, len(ciphertext))
	cipher.NewCBCDecrypter(block, iv).CryptBlocks(out, ciphertext)
	return pkcs7Unpad(out, block.BlockSize())
}

func pkcs7Pad(data []byte, blockSize int) []byte {
	padding := blockSize - len(data)%blockSize
	out := make([]byte, len(data)+padding)
	copy(out, data)
	for i := len(data); i < len(out); i++ {
		out[i] = byte(padding)
	}
	return out
}

func pkcs7Unpad(data []byte, blockSize int) ([]byte, error) {
	if len(data) == 0 || len(data)%blockSize != 0 {
		return nil, errors.New("invalid pkcs7 data size")
	}
	padding := int(data[len(data)-1])
	if padding == 0 || padding > blockSize || padding > len(data) {
		return nil, errors.New("invalid pkcs7 padding")
	}
	for _, v := range data[len(data)-padding:] {
		if int(v) != padding {
			return nil, errors.New("invalid pkcs7 padding")
		}
	}
	return data[:len(data)-padding], nil
}

func parseRSAPublicKey(keyData []byte) (*rsa.PublicKey, error) {
	der, err := keyDER(keyData)
	if err != nil {
		return nil, err
	}
	if pub, err := x509.ParsePKIXPublicKey(der); err == nil {
		rsaPub, ok := pub.(*rsa.PublicKey)
		if !ok {
			return nil, errors.New("key is not rsa public key")
		}
		return rsaPub, nil
	}
	pub, err := x509.ParsePKCS1PublicKey(der)
	if err != nil {
		return nil, err
	}
	return pub, nil
}

func parseRSAPrivateKey(keyData []byte) (*rsa.PrivateKey, error) {
	der, err := keyDER(keyData)
	if err != nil {
		return nil, err
	}
	if key, err := x509.ParsePKCS1PrivateKey(der); err == nil {
		return key, nil
	}
	parsed, err := x509.ParsePKCS8PrivateKey(der)
	if err != nil {
		return nil, err
	}
	key, ok := parsed.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("key is not rsa private key")
	}
	return key, nil
}

// keyDER accepts a key either as PEM or as the bare base64 DER body that Keepz
// hands out during integration (no -----BEGIN----- armor, possibly wrapped).
func keyDER(keyData []byte) ([]byte, error) {
	if block, _ := pem.Decode(keyData); block != nil {
		return block.Bytes, nil
	}
	cleaned := strings.Map(func(r rune) rune {
		switch r {
		case ' ', '\t', '\r', '\n':
			return -1
		}
		return r
	}, string(keyData))
	if cleaned == "" {
		return nil, errors.New("empty rsa key")
	}
	der, err := base64.StdEncoding.DecodeString(cleaned)
	if err != nil {
		return nil, fmt.Errorf("rsa key is neither pem nor base64 der: %w", err)
	}
	return der, nil
}
