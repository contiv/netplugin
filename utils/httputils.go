/***
Copyright 2014 Cisco Systems Inc. All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package utils

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/gorilla/mux"
)

type httpAPIFunc func(w http.ResponseWriter, r *http.Request, vars map[string]string) (interface{}, error)

// MakeHTTPHandler is a simple Wrapper for http handlers
func MakeHTTPHandler(handlerFunc httpAPIFunc) http.HandlerFunc {
	// Create a closure and return an anonymous function
	return func(w http.ResponseWriter, r *http.Request) {
		// Call the handler
		resp, err := handlerFunc(w, r, mux.Vars(r))
		if err != nil {
			// Log error
			log.Errorf("Handler for %s %s returned error: %s", r.Method, r.URL, err)

			if resp == nil {
				// Send HTTP response
				http.Error(w, err.Error(), http.StatusInternalServerError)
			} else {
				// Send HTTP response as Json
				content, err := json.Marshal(resp)
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}

				w.WriteHeader(http.StatusInternalServerError)
				w.Write(content)
			}
		} else {
			// Send HTTP response as Json
			err = writeJSON(w, http.StatusOK, resp)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}
	}
}

// writeJSON: writes the value v to the http response stream as json with standard
// json encoding.
func writeJSON(w http.ResponseWriter, code int, v interface{}) error {
	// Set content type as json
	w.Header().Set("Content-Type", "application/json")

	// write the HTTP status code
	w.WriteHeader(code)

	// Write the Json output
	return json.NewEncoder(w).Encode(v)
}

// UnknownAction is a catchall handler for additional driver functions
func UnknownAction(w http.ResponseWriter, r *http.Request) {
	log.Infof("Unknown action at %q", r.URL.Path)
	content, _ := ioutil.ReadAll(r.Body)
	log.Infof("Body content: %s", string(content))
	http.NotFound(w, r)
}

// HTTPPost performs http POST operation
func HTTPPost(url string, req interface{}, resp interface{}) error {
	// Convert the req to json
	jsonStr, err := json.Marshal(req)
	if err != nil {
		log.Errorf("Error converting request data(%#v) to Json. Err: %v", req, err)
		return err
	}

	// Perform HTTP POST operation
	res, err := http.Post(url, "application/json", strings.NewReader(string(jsonStr)))
	if err != nil {
		log.Errorf("Error during http POST. Err: %v", err)
		return err
	}

	defer res.Body.Close()

	// Check the response code
	if res.StatusCode == http.StatusInternalServerError {
		eBody, err := ioutil.ReadAll(res.Body)
		if err != nil {
			return errors.New("HTTP StatusInternalServerError" + err.Error())
		}
		return errors.New(string(eBody))
	}

	if res.StatusCode != http.StatusOK {
		log.Errorf("HTTP error response. Status: %s, StatusCode: %d", res.Status, res.StatusCode)
		return fmt.Errorf("HTTP error response. Status: %s, StatusCode: %d", res.Status, res.StatusCode)
	}

	// Read the entire response
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		log.Errorf("Error during ioutil readall. Err: %v", err)
		return err
	}

	// Convert response json to struct
	err = json.Unmarshal(body, resp)
	if err != nil {
		log.Errorf("Error during json unmarshall. Err: %v", err)
		return err
	}

	log.Debugf("Results for (%s): %+v\n", url, resp)

	return nil
}

// HTTPDel performs http DELETE operation
func HTTPDel(url string) error {
	req, err := http.NewRequest("DELETE", url, nil)

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		panic(err)
	}
	defer res.Body.Close()

	// Check the response code
	if res.StatusCode == http.StatusInternalServerError {
		eBody, err := ioutil.ReadAll(res.Body)
		if err != nil {
			return errors.New("HTTP StatusInternalServerError " + err.Error())
		}
		return errors.New(string(eBody))
	}

	if res.StatusCode != http.StatusOK {
		log.Errorf("HTTP error response. Status: %s, StatusCode: %d", res.Status, res.StatusCode)
		return fmt.Errorf("HTTP error response. Status: %s, StatusCode: %d", res.Status, res.StatusCode)
	}

	log.Debugf("Results for (%s): %+v\n", url, res)
	return nil
}
