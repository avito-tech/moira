package neo4j

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"regexp"
)

type Statement struct {
	Statement  string                 `json:"statement"`
	Parameters map[string]interface{} `json:"parameters"`
}

type Response struct {
	Results []struct {
		Data []ResponseItem `json:"data"`
	} `json:"results"`
	Errors json.RawMessage `json:"errors"`
}

type ResponseItem struct {
	Row []json.RawMessage `json:"row"`
}

func (db *DbConnector) PostQuery(query string, parameters map[string]interface{}) ([]ResponseItem, error) {
	return db.PostQueries([]string{query}, parameters)
}

func (db *DbConnector) PostQueries(queries []string, parameters map[string]interface{}) ([]ResponseItem, error) {
	payload := struct {
		Statements []Statement `json:"statements"`
	}{
		Statements: make([]Statement, len(queries)),
	}
	for i, query := range queries {
		payload.Statements[i] = Statement{
			Statement:  RemoveExtraWhitespace(query),
			Parameters: parameters,
		}
	}
	encodedPayload, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	endpointURL := db.apiURL + "/db/" + db.dbName + "/tx/commit"
	request, err := http.NewRequest(
		"POST",
		endpointURL,
		bytes.NewBuffer(encodedPayload),
	)
	if err != nil {
		return nil, err
	}
	request.SetBasicAuth(db.username, db.password)
	request.Header.Add("Content-Type", "application/json")
	response, err := db.httpClient.Do(request)
	if err != nil {
		return nil, err
	}

	defer response.Body.Close()
	rawResponse, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}

	decodedResponse := Response{}
	err = json.Unmarshal(rawResponse, &decodedResponse)
	if err != nil {
		return nil, err
	}

	if response.StatusCode >= 400 {
		err := fmt.Errorf("neo4j error: %s", string(decodedResponse.Errors))
		return nil, err
	}

	return decodedResponse.Results[0].Data, nil
}

var WhitespaceRegex *regexp.Regexp

func init() {
	WhitespaceRegex = regexp.MustCompile(`\s{2,}`)
}

func RemoveExtraWhitespace(str string) string {
	return WhitespaceRegex.ReplaceAllString(str, " ")
}

// ParsePath reads a raw path returned from Neo4j and returns a slice of IDs.
func (db *DbConnector) ParsePath(rawPath json.RawMessage, skipLast bool) ([]string, error) {
	type Item struct {
		ID string `json:"id"`
	}
	var parsed []Item
	if err := json.Unmarshal(rawPath, &parsed); err != nil {
		return nil, err
	}
	if skipLast && len(parsed) == 0 {
		return nil, fmt.Errorf("zero-length response")
	}

	result := make([]string, len(parsed))
	for i, item := range parsed {
		result[i] = item.ID
	}
	if skipLast {
		result = result[:len(result)-1]
	}
	return result, nil
}
