package api

import (
	"go.avito.ru/DO/moira"
	"go.avito.ru/DO/moira/netbox"
	"go.avito.ru/DO/moira/sentry"
)

// Config for api configuration variables
type Config struct {
	EnableCORS         bool
	GrafanaPrefixes    []string
	Listen             string
	LimitLogger        moira.RateLimit
	LimitMetrics       moira.RateLimit
	Netbox             *netbox.Config
	Sentry             sentry.Config
	SuperUsers         []string // those who can turn off __all__ notifications
	TargetRewriteRules []RewriteRule
}

// Rewriting rules for targets
type RewriteRule struct {
	From string `yaml:"from"`
	To   string `yaml:"to"`
}
