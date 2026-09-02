package keepz

import (
	"crypto/rsa"
	"fmt"
	"net/http"
	"sync"
	"time"
)

const (
	DevEcommerceBaseURL  = "https://gateway.dev.keepz.me/ecommerce-service"
	ProdEcommerceBaseURL = "https://gateway.keepz.me/ecommerce-service"
	DevDistributorURL    = "https://distributor.dev.keepz.me"
	ProdDistributorURL   = "https://distributor.keepz.me"
)

type Options struct {
	Dev bool

	Identifier   string
	IntegratorID string

	KeepzPublicKeyPEM       []byte
	IntegratorPrivateKeyPEM []byte

	DistributorClientID     string
	DistributorClientSecret string

	HTTPClient         *http.Client
	EcommerceBaseURL   string
	DistributorBaseURL string
}

type API struct {
	identifier   string
	integratorID string

	keepzPublicKey       *rsa.PublicKey
	integratorPrivateKey *rsa.PrivateKey

	distributorClientID     string
	distributorClientSecret string

	httpClient         *http.Client
	ecommerceBaseURL   string
	distributorBaseURL string

	tokenMu     sync.Mutex
	accessToken string
	tokenExpiry time.Time
}

func New(v Options) (*API, error) {
	c := &API{
		identifier:              v.Identifier,
		integratorID:            v.IntegratorID,
		distributorClientID:     v.DistributorClientID,
		distributorClientSecret: v.DistributorClientSecret,
		httpClient:              v.HTTPClient,
		ecommerceBaseURL:        v.EcommerceBaseURL,
		distributorBaseURL:      v.DistributorBaseURL,
	}
	if c.httpClient == nil {
		c.httpClient = http.DefaultClient
	}
	if c.ecommerceBaseURL == "" {
		if v.Dev {
			c.ecommerceBaseURL = DevEcommerceBaseURL
		} else {
			c.ecommerceBaseURL = ProdEcommerceBaseURL
		}
	}
	if c.distributorBaseURL == "" {
		if v.Dev {
			c.distributorBaseURL = DevDistributorURL
		} else {
			c.distributorBaseURL = ProdDistributorURL
		}
	}

	var err error
	if len(v.KeepzPublicKeyPEM) > 0 {
		c.keepzPublicKey, err = parseRSAPublicKey(v.KeepzPublicKeyPEM)
		if err != nil {
			return nil, fmt.Errorf("parse keepz public key: %w", err)
		}
	}
	if len(v.IntegratorPrivateKeyPEM) > 0 {
		c.integratorPrivateKey, err = parseRSAPrivateKey(v.IntegratorPrivateKeyPEM)
		if err != nil {
			return nil, fmt.Errorf("parse integrator private key: %w", err)
		}
	}

	return c, nil
}
