package cmd

import (
	"net/http"
	"net/http/pprof"

	"go.avito.ru/DO/moira"
)

// StartProfiling starts http server with profiling data at given port
func StartProfiling(logger moira.Logger, config ProfilerConfig) {

	pprofMux := http.NewServeMux()
	pprofMux.HandleFunc("/debug/pprof/", pprof.Index)
	pprofMux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	pprofMux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	pprofMux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	pprofMux.HandleFunc("/debug/pprof/trace", pprof.Trace)

	go func() {
		err := http.ListenAndServe(config.Listen, pprofMux)
		if err != nil {
			logger.InfoF("Can't start pprof server: %v", err)
		}
	}()

}
