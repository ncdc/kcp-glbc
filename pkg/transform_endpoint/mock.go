package transform_endpoint

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
)

func mockHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			log.Printf("Error reading body: %v", err)
			http.Error(w, "can't read body", http.StatusBadRequest)
			return
		}

		fmt.Fprintln(w, string(body))
	}
}
