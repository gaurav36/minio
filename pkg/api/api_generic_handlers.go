/*
 * Minimalist Object Storage, (C) 2015 Minio, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package api

import (
	"net/http"
	"strings"

	"github.com/minio-io/minio/pkg/api/config"
)

type vHandler struct {
	conf    config.Config
	handler http.Handler
}

type rHandler struct {
	handler http.Handler
}

// strip AccessKey from authorization header
func stripAccessKey(r *http.Request) string {
	fields := strings.Fields(r.Header.Get("Authorization"))
	if len(fields) < 2 {
		return ""
	}
	splits := strings.Split(fields[1], ":")
	if len(splits) < 2 {
		return ""
	}
	return splits[0]
}

// Validate handler is wrapper handler used for API request validation with authorization header.
// Current authorization layer supports S3's standard HMAC based signature request.
func validateHandler(conf config.Config, h http.Handler) http.Handler {
	return vHandler{conf, h}
}

// Validate handler ServeHTTP() wrapper
func (h vHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	accessKey := stripAccessKey(r)
	acceptsContentType := getContentType(r)
	if accessKey != "" {
		if err := h.conf.ReadConfig(); err != nil {
			error := getErrorCode(InternalError)
			errorResponse := getErrorResponse(error, "")
			setCommonHeaders(w, getContentTypeString(acceptsContentType))
			w.WriteHeader(error.HTTPStatusCode)
			w.Write(encodeErrorResponse(errorResponse, acceptsContentType))
		} else {
			user, ok := h.conf.Users[accessKey]
			if ok == false {
				error := getErrorCode(AccessDenied)
				errorResponse := getErrorResponse(error, "")
				setCommonHeaders(w, getContentTypeString(acceptsContentType))
				w.WriteHeader(error.HTTPStatusCode)
				w.Write(encodeErrorResponse(errorResponse, acceptsContentType))
			} else {
				ok, _ = ValidateRequest(user, r)
				if ok {
					h.handler.ServeHTTP(w, r)
				} else {
					error := getErrorCode(AccessDenied)
					errorResponse := getErrorResponse(error, "")
					setCommonHeaders(w, getContentTypeString(acceptsContentType))
					w.WriteHeader(error.HTTPStatusCode)
					w.Write(encodeErrorResponse(errorResponse, acceptsContentType))
				}
			}
		}
	} else {
		// Control reaches when no access key is found, ideally we would
		// like to throw back `403`. But for now with our tests lacking
		// this functionality it is better for us to be serving anonymous
		// requests as well.
		// We should remove this after adding tests to support signature request
		h.handler.ServeHTTP(w, r)
		// ## Uncommented below links of code after disabling anonymous requests
		// error := errorCodeError(AccessDenied)
		// errorResponse := getErrorResponse(error, "")
		// w.WriteHeader(error.HTTPStatusCode)
		// w.Write(writeErrorResponse(w, errorResponse, acceptsContentType))
	}
}

// Ignore resources handler is wrapper handler used for API request resource validation
// Since we do not support all the S3 queries, it is necessary for us to throw back a
// valid error message indicating such a feature to have been not implemented.
func ignoreResourcesHandler(h http.Handler) http.Handler {
	return rHandler{h}
}

// Resource handler ServeHTTP() wrapper
func (h rHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	acceptsContentType := getContentType(r)
	if ignoreUnImplementedObjectResources(r) || ignoreUnImplementedBucketResources(r) {
		error := getErrorCode(NotImplemented)
		errorResponse := getErrorResponse(error, "")
		setCommonHeaders(w, getContentTypeString(acceptsContentType))
		w.WriteHeader(error.HTTPStatusCode)
		w.Write(encodeErrorResponse(errorResponse, acceptsContentType))
	} else {
		h.handler.ServeHTTP(w, r)
	}
}

//// helpers

// Checks requests for unimplemented Bucket resources
func ignoreUnImplementedBucketResources(req *http.Request) bool {
	q := req.URL.Query()
	for name := range q {
		if unimplementedBucketResourceNames[name] {
			return true
		}
	}
	return false
}

// Checks requests for unimplemented Object resources
func ignoreUnImplementedObjectResources(req *http.Request) bool {
	q := req.URL.Query()
	for name := range q {
		if unimplementedObjectResourceNames[name] {
			return true
		}
	}
	return false
}

func writeErrorResponse(w http.ResponseWriter, req *http.Request, errorType int, acceptsContentType contentType, resource string) {
	error := getErrorCode(errorType)
	errorResponse := getErrorResponse(error, resource)
	// set headers
	setCommonHeaders(w, getContentTypeString(acceptsContentType))
	w.WriteHeader(error.HTTPStatusCode)
	// write body
	encodedErrorResponse := encodeErrorResponse(errorResponse, acceptsContentType)
	w.Write(encodedErrorResponse)
}
