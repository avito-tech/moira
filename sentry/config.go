package sentry

// Config for sentry configuration
type Config struct {
	Dsn        string
	Enabled    bool
	IsFilePath bool
}
