//go:build live

package keepz

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"
)

// liveAPI builds a client from every value available in creds_test.go.
//
// The credentials Keepz issued are PRODUCTION credentials: the same integrator
// id is rejected by gateway.dev.keepz.me with 6004 "Active Integrator not
// found", because identifiers are registered per environment. So these tests
// run against prod until Keepz issues dev credentials.
func liveAPI(t *testing.T) *API {
	t.Helper()
	api, err := New(Options{
		Identifier:              credIdentifier,
		IntegratorID:            credIntegratorID,
		KeepzPublicKeyPEM:       []byte(credKeepzPublicKey),
		IntegratorPrivateKeyPEM: []byte(credIntegratorPrivateKey),
		HTTPClient:              &http.Client{Timeout: 30 * time.Second},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if api.ecommerceBaseURL != ProdEcommerceBaseURL {
		t.Fatalf("ecommerce base url = %q, want %q", api.ecommerceBaseURL, ProdEcommerceBaseURL)
	}
	return api
}

func uuidv4(t *testing.T) string {
	t.Helper()
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		t.Fatal(err)
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	h := hex.EncodeToString(b[:])
	return fmt.Sprintf("%s-%s-%s-%s-%s", h[0:8], h[8:12], h[12:16], h[16:20], h[20:32])
}

// TestCredentials checks that the credentials in creds_test.go are complete and
// parseable. No network.
func TestCredentials(t *testing.T) {
	api := liveAPI(t)

	if credIdentifier == "" {
		t.Error("missing: identifier")
	}
	if credIntegratorID == "" {
		t.Error("missing: integratorId")
	}
	if credReceiverID == "" {
		t.Error("missing: receiverId")
	}
	if api.keepzPublicKey == nil {
		t.Error("missing: keepz public key (requests cannot be encrypted)")
	} else if got := api.keepzPublicKey.N.BitLen(); got != 4096 {
		t.Errorf("keepz public key is %d bit, want 4096", got)
	}
	if api.integratorPrivateKey == nil {
		t.Error("missing: integrator private key (responses cannot be decrypted)")
	} else if got := api.integratorPrivateKey.N.BitLen(); got != 4096 {
		t.Errorf("integrator private key is %d bit, want 4096", got)
	}

	// The two keys must be a *different* pair: we encrypt with Keepz's key and
	// decrypt with ours. If they matched, Keepz could not read us.
	if api.keepzPublicKey != nil && api.integratorPrivateKey != nil &&
		api.keepzPublicKey.N.Cmp(api.integratorPrivateKey.N) == 0 {
		t.Error("keepz public key is the public half of the integrator private key; " +
			"one of the two credentials is wrong")
	}
}

// TestLiveConnect verifies the whole encrypted request path without creating or
// changing anything: it looks up a random order id, which tells us how far the
// request got.
//
//	6004 "Active Integrator not found" -> identifier not registered here
//	                                      (rejected before decryption)
//	6010 "Failed to decrypt data!"     -> wrong request-encryption key
//	6005 "Integrator order not found"  -> identifier, encryption and payload are
//	                                      all correct; only the order is unknown
func TestLiveConnect(t *testing.T) {
	if testing.Short() {
		t.Skip("network test")
	}
	api := liveAPI(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, err := api.GetOrderStatus(ctx, OrderRequest{IntegratorOrderID: uuidv4(t)})

	var apiErr *Error
	switch {
	case err == nil:
		t.Fatal("random order id unexpectedly resolved")
	case !errors.As(err, &apiErr):
		t.Fatalf("transport error: %v", err)
	}

	t.Logf("statusCode=%d group=%d %s", apiErr.StatusCode, apiErr.ExceptionGroup, apiErr.Message)
	switch apiErr.StatusCode {
	case 6005:
		// Keepz decrypted our AES key with its private key, decrypted the
		// payload and read integratorId/integratorOrderId out of it.
		t.Log("ok: identifier accepted and request decrypted by Keepz")
	case 6004:
		t.Fatalf("integrator %s is not registered in this environment", credIntegratorID)
	case 6010:
		t.Fatal("credKeepzPublicKey is not the key Keepz decrypts our requests with")
	default:
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestLiveOrder creates a real order and cancels it. It runs against
// PRODUCTION, so it is opt-in: KEEPZ_LIVE_ORDER=1 go test -run TestLiveOrder
//
// This is the only test that can exercise response *decryption*, since Keepz
// returns errors as plain JSON and only success payloads are encrypted.
func TestLiveOrder(t *testing.T) {
	if testing.Short() {
		t.Skip("network test")
	}
	if os.Getenv("KEEPZ_LIVE_ORDER") != "1" {
		t.Skip("creates a real order in production; set KEEPZ_LIVE_ORDER=1 to run")
	}
	api := liveAPI(t)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	orderID := uuidv4(t)

	order, err := api.CreateOrder(ctx, CreateOrderRequest{
		Amount:            1,
		ReceiverID:        credReceiverID,
		ReceiverType:      credReceiverType,
		IntegratorOrderID: orderID,
		Currency:          CurrencyGEL,
		CommissionType:    credCommissionType,
		Language:          "EN",
	})
	if err != nil {
		logKeepzError(t, "CreateOrder", err)
		t.FailNow()
	}
	// Reaching here means the encrypted response was decrypted with our
	// private key, so the full round trip works.
	if order.URLForQR == "" {
		t.Fatalf("CreateOrder returned no urlForQR: %+v", order)
	}
	if order.IntegratorOrderID != orderID {
		t.Errorf("integratorOrderId = %q, want %q", order.IntegratorOrderID, orderID)
	}
	t.Logf("CreateOrder ok: order=%s urlForQR=%s", orderID, order.URLForQR)

	// Always try to cancel, even if the status check below fails.
	defer func() {
		res, err := api.CancelOrder(context.WithoutCancel(ctx), OrderRequest{IntegratorOrderID: orderID})
		if err != nil {
			logKeepzError(t, "CancelOrder", err)
			return
		}
		t.Logf("CancelOrder ok: status=%s", res.Status)
		if res.Status != OrderStatusCanceled {
			t.Errorf("status = %q, want %q", res.Status, OrderStatusCanceled)
		}
	}()

	status, err := api.GetOrderStatus(ctx, OrderRequest{IntegratorOrderID: orderID})
	if err != nil {
		logKeepzError(t, "GetOrderStatus", err)
		return
	}
	t.Logf("GetOrderStatus ok: status=%s transactionId=%d", status.Status, status.TransactionID)
	if status.Status != OrderStatusInitial {
		t.Errorf("status = %q, want %q", status.Status, OrderStatusInitial)
	}
}

func logKeepzError(t *testing.T, op string, err error) {
	t.Helper()
	var apiErr *Error
	if errors.As(err, &apiErr) {
		t.Errorf("%s: http=%d statusCode=%d group=%d message=%q body=%s",
			op, apiErr.HTTPStatusCode, apiErr.StatusCode, apiErr.ExceptionGroup,
			apiErr.Message, apiErr.Body)
		return
	}
	t.Errorf("%s: %v", op, err)
}
