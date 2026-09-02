package keepz

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
)

type OrderStatus string

const (
	OrderStatusInitial              OrderStatus = "INITIAL"
	OrderStatusProcessing           OrderStatus = "PROCESSING"
	OrderStatusSuccess              OrderStatus = "SUCCESS"
	OrderStatusFailed               OrderStatus = "FAILED"
	OrderStatusCanceled             OrderStatus = "CANCELED"
	OrderStatusExpired              OrderStatus = "EXPIRED"
	OrderStatusRefundRequested      OrderStatus = "REFUND_REQUESTED"
	OrderStatusPartiallyRefunded    OrderStatus = "PARTIALLY_REFUNDED"
	OrderStatusRefundedByOperator   OrderStatus = "REFUNDED_BY_OPERATOR"
	OrderStatusRefundedByIntegrator OrderStatus = "REFUNDED_BY_INTEGRATOR"
	OrderStatusRefundedByKeepz      OrderStatus = "REFUNDED_BY_KEEPZ"
	OrderStatusRefundFailed         OrderStatus = "REFUNDED_FAILED"
)

type CreateOrderRequest struct {
	Amount               float64                  `json:"amount"`
	ReceiverID           string                   `json:"receiverId"`
	ReceiverType         ReceiverType             `json:"receiverType"`
	IntegratorID         string                   `json:"integratorId,omitempty"`
	IntegratorOrderID    string                   `json:"integratorOrderId"`
	Currency             Currency                 `json:"currency,omitempty"`
	AcquiringCurrency    Currency                 `json:"acquiringCurrency,omitempty"`
	DistributionCurrency Currency                 `json:"distributionCurrency,omitempty"`
	TipReceiverID        string                   `json:"tipReceiverId,omitempty"`
	TipReceiverType      ReceiverType             `json:"tipReceiverType,omitempty"`
	SuccessRedirectURI   string                   `json:"successRedirectUri,omitempty"`
	FailRedirectURI      string                   `json:"failRedirectUri,omitempty"`
	CallbackURI          string                   `json:"callbackUri,omitempty"`
	DirectLinkProvider   string                   `json:"directLinkProvider,omitempty"`
	OpenBankingProvider  string                   `json:"openBankingLinkProvider,omitempty"`
	CryptoProvider       string                   `json:"cryptoPaymentProvider,omitempty"`
	Language             string                   `json:"language,omitempty"`
	CommissionType       string                   `json:"commissionType,omitempty"`
	SaveCard             *bool                    `json:"saveCard,omitempty"`
	CardToken            string                   `json:"cardToken,omitempty"`
	SubscriptionPlan     *SubscriptionPlan        `json:"subscriptionPlan,omitempty"`
	SplitDetails         []SplitDetail            `json:"splitDetails,omitempty"`
	OrderProperties      map[string]OrderProperty `json:"orderProperties,omitempty"`
}

type SubscriptionPlan struct {
	CallbackURI string   `json:"callbackUri,omitempty"`
	Currency    Currency `json:"currency,omitempty"`
	Amount      float64  `json:"amount,omitempty"`
	Period      string   `json:"period,omitempty"`
	StartDate   string   `json:"startDate,omitempty"`
	EndDate     string   `json:"endDate,omitempty"`
}

type SplitDetail struct {
	ReceiverType       ReceiverType `json:"receiverType"`
	ReceiverIdentifier string       `json:"receiverIdentifier"`
	Amount             float64      `json:"amount"`
}

type OrderProperty struct {
	Value      string `json:"value"`
	IsEditable bool   `json:"isEditable,omitempty"`
}

type CreateOrderResponse struct {
	IntegratorOrderID string `json:"integratorOrderId"`
	URLForQR          string `json:"urlForQR"`
}

func (c *API) CreateOrder(ctx context.Context, req CreateOrderRequest) (*CreateOrderResponse, error) {
	var res CreateOrderResponse
	err := c.doEncryptedJSON(ctx, http.MethodPost, c.ecommerceBaseURL, "/api/integrator/order", req.withIntegratorID(c.integratorID), nil, &res)
	return &res, err
}

type OrderRequest struct {
	IntegratorID      string `json:"integratorId,omitempty"`
	IntegratorOrderID string `json:"integratorOrderId"`
}

type OrderStatusResponse struct {
	IntegratorOrderID string      `json:"integratorOrderId"`
	Status            OrderStatus `json:"status"`
	TransactionID     int64       `json:"transactionId,omitempty"`
	Report            string      `json:"report,omitempty"`
}

func (c *API) GetOrderStatus(ctx context.Context, req OrderRequest) (*OrderStatusResponse, error) {
	var res OrderStatusResponse
	err := c.doEncryptedJSON(ctx, http.MethodGet, c.ecommerceBaseURL, "/api/integrator/order/status", req.withIntegratorID(c.integratorID), nil, &res)
	return &res, err
}

type CancelOrderResponse struct {
	IntegratorOrderID string      `json:"integratorOrderId"`
	Status            OrderStatus `json:"status"`
}

func (c *API) CancelOrder(ctx context.Context, req OrderRequest) (*CancelOrderResponse, error) {
	var res CancelOrderResponse
	err := c.doEncryptedJSON(ctx, http.MethodDelete, c.ecommerceBaseURL, "/api/integrator/order/cancel", req.withIntegratorID(c.integratorID), nil, &res)
	return &res, err
}

type RefundOrderRequest struct {
	IntegratorID      string  `json:"integratorId,omitempty"`
	IntegratorOrderID string  `json:"integratorOrderId"`
	Amount            float64 `json:"amount,omitempty"`
}

type RefundOrderResponse struct {
	IntegratorOrderID string      `json:"integratorOrderId"`
	Status            OrderStatus `json:"status"`
}

func (c *API) RefundOrder(ctx context.Context, req RefundOrderRequest) (*RefundOrderResponse, error) {
	if req.IntegratorID == "" {
		req.IntegratorID = c.integratorID
	}
	var res RefundOrderResponse
	err := c.doEncryptedJSON(ctx, http.MethodPost, c.ecommerceBaseURL, "/api/integrator/order/refund/v2", req, nil, &res)
	return &res, err
}

type GetSavedCardsRequest struct {
	IntegratorOrderID string `json:"integratorOrderId"`
}

type CardInfo struct {
	Token          string `json:"token"`
	Provider       string `json:"provider"`
	CardMask       string `json:"cardMask"`
	ExpirationDate string `json:"expirationDate"`
	CardBrand      string `json:"cardBrand"`
}

type SavedCardsResponse struct {
	Cards []CardInfo `json:"cards,omitempty"`
	CardInfo
}

func (c *API) GetSavedCards(ctx context.Context, req GetSavedCardsRequest) (*SavedCardsResponse, error) {
	var res SavedCardsResponse
	err := c.doEncryptedJSON(ctx, http.MethodGet, c.ecommerceBaseURL, "/api/v1/integrator/card/order-id", req, nil, &res)
	return &res, err
}

type PaymentCallback struct {
	Amount            float64      `json:"amount"`
	ReceiverID        string       `json:"receiverId"`
	ReceiverType      ReceiverType `json:"receiverType"`
	IntegratorID      string       `json:"integratorId"`
	IntegratorOrderID string       `json:"integratorOrderId"`
	Status            OrderStatus  `json:"status"`
	CardInfo          *CardInfo    `json:"cardInfo,omitempty"`
	InitialCurrency   Currency     `json:"initialCurrency,omitempty"`
	AcquiringCurrency Currency     `json:"acquiringCurrency,omitempty"`
	AcquiringAmount   float64      `json:"acquiringAmount,omitempty"`
	TransactionID     int64        `json:"transactionId,omitempty"`
}

func (c *API) ParsePaymentCallback(r io.Reader) (*PaymentCallback, error) {
	var cb PaymentCallback
	if err := c.DecodeEncryptedJSON(r, &cb); err != nil {
		return nil, err
	}
	return &cb, nil
}

type SubscriptionCallback struct {
	SubscriptionID string  `json:"subscriptionId"`
	HistoryID      int64   `json:"historyId"`
	Status         string  `json:"status"`
	Amount         float64 `json:"amount"`
}

func (c *API) ParseSubscriptionCallback(r io.Reader) (*SubscriptionCallback, error) {
	var cb SubscriptionCallback
	if err := c.DecodeEncryptedJSON(r, &cb); err != nil {
		return nil, err
	}
	return &cb, nil
}

func (c *API) DecodeEncryptedJSON(r io.Reader, out any) error {
	var msg encryptedMessage
	if err := json.NewDecoder(r).Decode(&msg); err != nil {
		return err
	}
	plain, err := c.decryptMessage(msg)
	if err != nil {
		return err
	}
	return json.Unmarshal(plain, out)
}

func (r CreateOrderRequest) withIntegratorID(id string) CreateOrderRequest {
	if r.IntegratorID == "" {
		r.IntegratorID = id
	}
	return r
}

func (r OrderRequest) withIntegratorID(id string) OrderRequest {
	if r.IntegratorID == "" {
		r.IntegratorID = id
	}
	return r
}
