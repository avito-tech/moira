package dto

import (
	"net/http"

	"go.avito.ru/DO/moira"
)

type Maintenance moira.Maintenance

func (Maintenance) Render(_ http.ResponseWriter, _ *http.Request) error {
	return nil
}
