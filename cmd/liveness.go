package cmd

import (
	"net/http"

	"go.avito.ru/DO/moira"
)

func liveness(writer http.ResponseWriter, _ *http.Request) {
	_, _ = writer.Write([]byte("ok"))
}

// StartLiveness starts http server with liveness check at given port
func StartLiveness(logger moira.Logger, config LivenessConfig) {
	mux := http.NewServeMux()
	mux.HandleFunc("/liveness", liveness)

	go func() {
		err := http.ListenAndServe(config.Listen, mux)
		if err != nil {
			logger.InfoF("Can't start liveness server: %v", err)
		}
	}()

}
