# keepz

Go client for Keepz eCommerce payments and Direct Settlements.

```bash
go get github.com/monopolly/keepz
```

## Keys and credentials

For eCommerce payments Keepz uses encrypted requests, responses, and callbacks.

You generate your own RSA key pair:

```bash
openssl genrsa -out integrator_private.pem 4096
openssl rsa -in integrator_private.pem -pubout -out integrator_public.pem
```

Keep the private key only on your side:

```go
IntegratorPrivateKeyPEM: []byte(os.Getenv("KEEPZ_PRIVATE_KEY_PEM"))
```

Send `integrator_public.pem` to Keepz when they configure your integration. Keepz uses this public key to encrypt responses and callbacks for your project.

Keys may be passed either as PEM or as the bare base64 DER body that Keepz
hands out during onboarding (no `-----BEGIN-----` armor); both are accepted.

Ask Keepz for these values:

- `KEEPZ_IDENTIFIER` - integrator identifier sent unencrypted with every
  eCommerce request. It is a UUID and is resolved *before* decryption, so a
  wrong value always yields `6004 Active Integrator not found`. In practice it
  is the same UUID as `KEEPZ_INTEGRATOR_ID`.
- `KEEPZ_INTEGRATOR_ID` - integrator id used inside order payloads
- `KEEPZ_PUBLIC_KEY_PEM` - the key requests are encrypted with. Keepz may label
  this "Integrator Public Key" on the onboarding sheet, but it is *not* the
  public half of your private key - Keepz holds the matching private half.
- `KEEPZ_RECEIVER_ID` - receiver/branch/user id that will receive the payment
- receiver type for `KEEPZ_RECEIVER_ID`, for example `BRANCH`, `USER`, or `IBAN`
- dev and production access separately, if you need both environments.
  Credentials are per-environment: a production integrator id is rejected by
  `gateway.dev.keepz.me` with `6004`.

For Direct Settlements also ask Keepz for:

- `KEEPZ_DISTRIBUTOR_CLIENT_ID`
- `KEEPZ_DISTRIBUTOR_CLIENT_SECRET`

Recommended storage:

- Store private keys and client secrets in env variables or a secret manager.
- Do not commit `integrator_private.pem`, `.env`, or production credentials.
- Use separate keys and credentials for dev and production.

```go
package main

import (
	"context"
	"os"

	"github.com/monopolly/keepz"
)

func main() {
	api, err := keepz.New(keepz.Options{
		Dev:                     true,
		Identifier:              os.Getenv("KEEPZ_IDENTIFIER"),
		IntegratorID:            os.Getenv("KEEPZ_INTEGRATOR_ID"),
		KeepzPublicKeyPEM:       []byte(os.Getenv("KEEPZ_PUBLIC_KEY_PEM")),
		IntegratorPrivateKeyPEM: []byte(os.Getenv("KEEPZ_PRIVATE_KEY_PEM")),
	})
	if err != nil {
		panic(err)
	}

	order, err := api.CreateOrder(context.Background(), keepz.CreateOrderRequest{
		Amount:            100,
		ReceiverID:        os.Getenv("KEEPZ_RECEIVER_ID"),
		ReceiverType:      keepz.ReceiverTypeBranch,
		IntegratorOrderID: "3fa85f64-5717-4562-b3fc-2c963f66afa6",
		Currency:          keepz.CurrencyGEL,
		CallbackURI:       "https://example.com/keepz/callback",
	})
	if err != nil {
		panic(err)
	}

	println(order.URLForQR)
}
```

Payment callback handler:

```go
func keepzCallback(w http.ResponseWriter, r *http.Request) {
	callback, err := api.ParsePaymentCallback(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Use callback.IntegratorOrderID and callback.Status to update your order.
	w.WriteHeader(http.StatusOK)
}
```

Direct Settlements:

```go
api, err := keepz.New(keepz.Options{
	Dev:                     true,
	DistributorClientID:     os.Getenv("KEEPZ_DISTRIBUTOR_CLIENT_ID"),
	DistributorClientSecret: os.Getenv("KEEPZ_DISTRIBUTOR_CLIENT_SECRET"),
})
if err != nil {
	panic(err)
}

tx, err := api.CreateTransaction(context.Background(), keepz.CreateTransactionRequest{
	Amount:          100,
	Currency:        keepz.CurrencyGEL,
	Description:     "Payout",
	ToIBAN:          "GE34TB0000000000000000",
	BeneficiaryName: "Giorgi Giorgadze",
	UniqueID:        "123e4567-e89b-12d3-a456-426614174000",
})
```

## Coverage

eCommerce (`gateway[.dev].keepz.me/ecommerce-service`), all encrypted:

| Method | Endpoint | Go |
| --- | --- | --- |
| POST | `/api/integrator/order` | `CreateOrder` |
| GET | `/api/integrator/order/status` | `GetOrderStatus` |
| DELETE | `/api/integrator/order/cancel` | `CancelOrder` |
| POST | `/api/integrator/order/refund/v2` | `RefundOrder` |
| GET | `/api/v1/integrator/card/order-id` | `GetSavedCards` |
| POST | `/api/v1/integrator/subscription/history` | `GetSubscriptionHistory` |
| POST | `/api/v1/integrator/subscription/revoke` | `RevokeSubscription` |
| - | payment callback | `ParsePaymentCallback` |
| - | subscription callback | `ParseSubscriptionCallback` |

Direct Settlements (`distributor[.dev].keepz.me`), OAuth2 bearer:

| Method | Endpoint | Go |
| --- | --- | --- |
| POST | `/api/auth` | `GetAccessToken` (cached automatically) |
| POST | `/api/distributor` | `CreateTransaction` |
| GET | `/api/distributor/details` | `GetTransactionDetails` |
| GET | `/api/distributor/balance/check` | `GetBalance` |

For anything not wrapped, `EcommerceRequest` (encrypted) and
`DistributorRequest` (bearer) take a raw path and payload.

## Tests

```bash
go test ./...                      # offline unit tests
```

Live tests hit the real Keepz API and are behind the `live` build tag:

```bash
cp creds_test.go.example creds_test.go   # gitignored, fill in your credentials
go test -tags live -run 'TestCredentials|TestLiveConnect' ./...
KEEPZ_LIVE_ORDER=1 go test -tags live -run TestLiveOrder ./...   # creates and cancels a real order
```
