package transform_endpoint

import (
	"encoding/json"
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
			http.Error(w, "unable to read body", http.StatusBadRequest)
			return
		}

		log.Logger.Info(string(body))

		payload := map[string]interface{}{}
		err = json.Unmarshal(body, &payload)
		if err != nil {
			http.Error(w, fmt.Sprintf("unable to deserialize request payload: %s", err), http.StatusBadRequest)
		}

		if resource, ok := payload["trafficResource"]; ok {
			out, err := json.Marshal(resource)
			if err != nil {
				http.Error(w, fmt.Sprintf("unable to serialize response: %s", err), http.StatusInternalServerError)
			}
			fmt.Fprintln(w, string(out))
		} else {
			http.Error(w, fmt.Sprintf("missing 'trafficResource from request': %s", err), http.StatusBadRequest)
		}
	}
}
