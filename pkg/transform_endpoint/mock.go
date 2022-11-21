package transform_endpoint

import (
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/kuadrant/kcp-glbc/pkg/_internal/log"
)

func mockHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			log.Logger.Error(err, "error reading body")
			http.Error(w, "can't read body", http.StatusBadRequest)
			return
		}

		log.Logger.Info(string(body))
		fmt.Fprintln(w, string(body))
	}
}
