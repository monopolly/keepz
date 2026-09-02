# keepz

Go client for Keepz eCommerce payments and Direct Settlements.

## Keys and credentials

For eCommerce payments Keepz uses encrypted requests, responses, and callbacks.

You generate your own RSA key pair:

```bash
openssl genrsa -out integrator_private.pem 2048
openssl rsa -in integrator_private.pem -pubout -out integrator_public.pem
```

Keep the private key only on your side:

```go
IntegratorPrivateKeyPEM: []byte(os.Getenv("KEEPZ_PRIVATE_KEY_PEM"))
```

Send `integrator_public.pem` to Keepz when they configure your integration. Keepz uses this public key to encrypt responses and callbacks for your project.

Ask Keepz for these values:

- `KEEPZ_IDENTIFIER` - merchant/integrator identifier used in encrypted requests
- `KEEPZ_INTEGRATOR_ID` - integrator id used inside order payloads
- `KEEPZ_PUBLIC_KEY_PEM` - Keepz public key for the selected environment
- `KEEPZ_RECEIVER_ID` - receiver/branch/user id that will receive the payment
- receiver type for `KEEPZ_RECEIVER_ID`, for example `BRANCH`, `USER`, or `IBAN`
- dev and production access separately, if you need both environments

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

	keepz "github.com/monopolly/payments/keepz"
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
