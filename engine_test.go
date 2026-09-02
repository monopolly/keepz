package keepz

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"io"
	"net/http"
	"testing"
)

func TestEncryptedRoundTrip(t *testing.T) {
	keepzKey, keepzPubPEM, _ := testKeyPair(t)
	integratorKey, _, integratorPrivPEM := testKeyPair(t)

	api, err := New(Options{
		Identifier:              "identifier",
		IntegratorID:            "integrator",
		KeepzPublicKeyPEM:       keepzPubPEM,
		IntegratorPrivateKeyPEM: integratorPrivPEM,
	})
	if err != nil {
		t.Fatal(err)
	}

	msg, err := api.encryptPayload(CreateOrderRequest{Amount: 12.34, IntegratorOrderID: "order-1"})
	if err != nil {
		t.Fatal(err)
	}

	serverSide := testCryptoAPI(keepzKey, &integratorKey.PublicKey)
	plain, err := serverSide.decryptMessage(msg)
	if err != nil {
		t.Fatal(err)
	}
	var req CreateOrderRequest
	if err := json.Unmarshal(plain, &req); err != nil {
		t.Fatal(err)
	}
	if req.Amount != 12.34 || req.IntegratorOrderID != "order-1" {
		t.Fatalf("unexpected request: %+v", req)
	}

	resMsg, err := serverSide.encryptPayload(CreateOrderResponse{IntegratorOrderID: "order-1", URLForQR: "https://pay"})
	if err != nil {
		t.Fatal(err)
	}
	plain, err = api.decryptMessage(resMsg)
	if err != nil {
		t.Fatal(err)
	}
	var res CreateOrderResponse
	if err := json.Unmarshal(plain, &res); err != nil {
		t.Fatal(err)
	}
	if res.URLForQR != "https://pay" {
		t.Fatalf("unexpected response: %+v", res)
	}
}

func TestCreateOrder(t *testing.T) {
	keepzKey, keepzPubPEM, _ := testKeyPair(t)
	integratorKey, _, integratorPrivPEM := testKeyPair(t)

	api, err := New(Options{
		Identifier:              "identifier",
		IntegratorID:            "integrator-id",
		KeepzPublicKeyPEM:       keepzPubPEM,
		IntegratorPrivateKeyPEM: integratorPrivPEM,
	})
	if err != nil {
		t.Fatal(err)
	}

	serverCrypto := testCryptoAPI(keepzKey, &integratorKey.PublicKey)

	api.httpClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/integrator/order" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		var msg encryptedMessage
		if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
			return nil, err
		}
		if msg.Identifier != "identifier" {
			t.Fatalf("unexpected identifier: %q", msg.Identifier)
		}
		plain, err := serverCrypto.decryptMessage(msg)
		if err != nil {
			return nil, err
		}
		var req CreateOrderRequest
		if err := json.Unmarshal(plain, &req); err != nil {
			return nil, err
		}
		if req.IntegratorID != "integrator-id" {
			t.Fatalf("integrator id was not defaulted: %+v", req)
		}
		res, err := serverCrypto.encryptPayload(CreateOrderResponse{IntegratorOrderID: req.IntegratorOrderID, URLForQR: "https://checkout"})
		if err != nil {
			return nil, err
		}
		var body bytes.Buffer
		if err := json.NewEncoder(&body).Encode(res); err != nil {
			return nil, err
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(&body),
		}, nil
	})}
	api.ecommerceBaseURL = "https://example.test"

	res, err := api.CreateOrder(t.Context(), CreateOrderRequest{
		Amount:            1,
		ReceiverID:        "receiver",
		ReceiverType:      ReceiverTypeBranch,
		IntegratorOrderID: "order",
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.URLForQR != "https://checkout" {
		t.Fatalf("unexpected response: %+v", res)
	}
}

func testKeyPair(t *testing.T) (*rsa.PrivateKey, []byte, []byte) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	pub, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	pubPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pub})
	priv := x509.MarshalPKCS1PrivateKey(key)
	privPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: priv})
	return key, pubPEM, privPEM
}

func testCryptoAPI(privateKey *rsa.PrivateKey, publicKey *rsa.PublicKey) *API {
	return &API{
		integratorPrivateKey: privateKey,
		keepzPublicKey:       publicKey,
		httpClient:           http.DefaultClient,
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}
