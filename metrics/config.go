package metrics

import (
	"go.avito.ru/DO/moira"
)

type Config struct {
	Enabled bool
	Limits  moira.RateLimit
	Host    string
	Port    int
	Prefix  string
	IsTest  bool
}
