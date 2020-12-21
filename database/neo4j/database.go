package neo4j

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"go.avito.ru/DO/moira"
	"go.avito.ru/DO/moira/cmd"
)

type DbConnector struct {
	apiURL   string
	username string
	password string
	dbName   string

	httpClient http.Client
	logger     moira.Logger
}

func NewDatabase(logger moira.Logger, config cmd.Neo4jConfig) (*DbConnector, error) {
	var password string
	if config.PasswordPath != "" {
		raw, err := ioutil.ReadFile(config.PasswordPath)
		if err != nil {
			return nil, err
		}
		password = strings.TrimSpace(string(raw))
	} else {
		password = config.Password
	}
	return &DbConnector{
		apiURL:   fmt.Sprintf("http://%s:%d", config.Host, config.Port),
		username: config.User,
		password: password,
		dbName:   config.DBName,

		httpClient: http.Client{Timeout: 3 * time.Second},
		logger:     logger,
	}, nil
}

func (db *DbConnector) Ping() bool {
	resp, err := db.httpClient.Get(db.apiURL)
	if err != nil {
		return false
	}
	return (resp.StatusCode == 200)
}
