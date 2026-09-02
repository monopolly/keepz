package keepz

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

type AccessTokenRequest struct {
	ClientID     string `json:"client_id,omitempty"`
	ClientSecret string `json:"client_secret,omitempty"`
	GrantType    string `json:"grant_type,omitempty"`
}

type AccessTokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
	TokenType   string `json:"token_type"`
}

func (c *API) GetAccessToken(ctx context.Context) (*AccessTokenResponse, error) {
	req := AccessTokenRequest{
		ClientID:     c.distributorClientID,
		ClientSecret: c.distributorClientSecret,
		GrantType:    "client_credentials",
	}
	var wrapper struct {
		Value AccessTokenResponse `json:"value"`
	}
	if err := c.doPlainJSON(ctx, http.MethodPost, c.distributorBaseURL, "/api/auth", req, nil, &wrapper); err != nil {
		return nil, err
	}
	return &wrapper.Value, nil
}

type CreateTransactionRequest struct {
	Amount             float64      `json:"amount"`
	Currency           Currency     `json:"currency"`
	Description        string       `json:"description"`
	ToIBAN             string       `json:"toIban,omitempty"`
	BeneficiaryName    string       `json:"beneficiaryName,omitempty"`
	UniqueID           string       `json:"uniqueId,omitempty"`
	ReceiverID         string       `json:"receiverId,omitempty"`
	ReceiverType       ReceiverType `json:"receiverType,omitempty"`
	ReceiverIdentifier string       `json:"receiverIdentifier,omitempty"`
}

type TransactionResponse struct {
	TransactionID      int64    `json:"transactionId"`
	Status             string   `json:"status"`
	StatusDescription  string   `json:"statusDescription"`
	Amount             float64  `json:"amount,omitempty"`
	ToIBAN             string   `json:"toIban,omitempty"`
	Currency           Currency `json:"currency,omitempty"`
	PaymentDescription string   `json:"paymentDescription,omitempty"`
	UniqueID           string   `json:"uniqueId"`
	CreatedAt          string   `json:"createdAt"`
	CommissionAmount   float64  `json:"commissionAmount,omitempty"`
}

func (c *API) CreateTransaction(ctx context.Context, req CreateTransactionRequest) (*TransactionResponse, error) {
	var wrapper struct {
		Value *TransactionResponse `json:"value,omitempty"`
		TransactionResponse
	}
	if err := c.doDistributorJSON(ctx, http.MethodPost, "/api/distributor", req, nil, &wrapper); err != nil {
		return nil, err
	}
	if wrapper.Value != nil {
		return wrapper.Value, nil
	}
	return &wrapper.TransactionResponse, nil
}

func (c *API) GetTransactionDetails(ctx context.Context, transactionID int64) (*TransactionResponse, error) {
	var wrapper struct {
		Value TransactionResponse `json:"value"`
	}
	q := url.Values{"transaction_id": {strconv.FormatInt(transactionID, 10)}}
	err := c.doDistributorJSON(ctx, http.MethodGet, "/api/distributor/details", nil, q, &wrapper)
	return &wrapper.Value, err
}

type BalanceResponse struct {
	Amount float64 `json:"amount"`
}

func (c *API) GetBalance(ctx context.Context, currency Currency) (*BalanceResponse, error) {
	var wrapper struct {
		Value BalanceResponse `json:"value"`
	}
	q := url.Values{}
	if currency != "" {
		q.Set("currency", string(currency))
	}
	err := c.doDistributorJSON(ctx, http.MethodGet, "/api/distributor/balance/check", nil, q, &wrapper)
	return &wrapper.Value, err
}

func (c *API) doDistributorJSON(ctx context.Context, method, path string, payload any, query url.Values, out any) error {
	token, err := c.distributorToken(ctx)
	if err != nil {
		return err
	}
	return c.doPlainJSON(ctx, method, c.distributorBaseURL, path, payload, query, out, "Authorization", "Bearer "+token)
}

func (c *API) distributorToken(ctx context.Context) (string, error) {
	c.tokenMu.Lock()
	defer c.tokenMu.Unlock()
	if c.accessToken != "" && time.Now().Before(c.tokenExpiry.Add(-time.Minute)) {
		return c.accessToken, nil
	}
	token, err := c.GetAccessToken(ctx)
	if err != nil {
		return "", err
	}
	c.accessToken = token.AccessToken
	c.tokenExpiry = time.Now().Add(time.Duration(token.ExpiresIn) * time.Second)
	return c.accessToken, nil
}
