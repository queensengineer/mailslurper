// Copyright 2013-2016 Adam Presley. All rights reserved
// Use of this source code is governed by the MIT license
// that can be found in the LICENSE file.

package main

import (
	"fmt"
	"net/http"
	"time"
)

func baseMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method == "OPTIONS" {
			fmt.Fprintf(writer, "")
			return
		}

		startTime := time.Now()

		writer.Header().Set("Access-Control-Allow-Origin", "*")
		writer.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
		writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CRSF-Token, Authorization")

		next.ServeHTTP(writer, request)
		logger.Infof("%s - %s (%s)", request.Method, request.URL.String(), time.Since(startTime))
	})
}
