package keepz

type Currency string

const (
	CurrencyGEL Currency = "GEL"
	CurrencyUSD Currency = "USD"
	CurrencyEUR Currency = "EUR"
)

type ReceiverType string

const (
	ReceiverTypeBranch ReceiverType = "BRANCH"
	ReceiverTypeUser   ReceiverType = "USER"
	ReceiverTypeIBAN   ReceiverType = "IBAN"
)
