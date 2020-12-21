package dto

import (
	"fmt"
	"net/http"

	"go.avito.ru/DO/moira"
)

type SilentPattern moira.SilentPatternData

type SilentPatternList struct {
	List []*moira.SilentPatternData `json:"list"`
}

func (silentPatternList *SilentPatternList) Bind(r *http.Request) error {
	if len(silentPatternList.List) == 0 {
		return fmt.Errorf("SilentPatternList must not be empty")
	} else {
		return nil
	}
}

func (silentPatternList *SilentPatternList) Render(w http.ResponseWriter, r *http.Request) error {
	return nil
}

func (silentPattern *SilentPattern) Bind(r *http.Request) error {
	if len(silentPattern.Pattern) == 0 {
		return fmt.Errorf("SilentPattern must have pattern")
	}
	return nil
}

func (silentPattern *SilentPattern) Render(w http.ResponseWriter, r *http.Request) error {
	return nil
}
