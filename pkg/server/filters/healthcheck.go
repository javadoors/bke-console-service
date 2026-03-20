/*
 * Copyright (c) 2025Huawei Technologies Co., Ltd.
 * openFuyao is licensed under Mulan PSL v2.
 * You can use this software according to the terms and conditions of the Mulan PSL v2.
 * You may obtain a copy of Mulan PSL v2 at:
 *          http://license.coscl.org.cn/MulanPSL2
 * THIS SOFTWARE IS PROVIDED ON AN "AS IS" BASIS, WITHOUT WARRANTIES OF ANY KIND,
 * EITHER EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO NON-INFRINGEMENT,
 * MERCHANTABILITY OR FIT FOR A PARTICULAR PURPOSE.
 * See the Mulan PSL v2 for more details.
 */

package filters

import (
	"net/http"
	"strings"

	"console-service/pkg/zlog"
)

const (
	healthcheckPrefix = "/health"
)

type healthCheckHandler struct {
	nextHandler http.Handler
}

// HandleHealthCheck intercept health check request
func HandleHealthCheck(handler http.Handler) http.Handler {
	return &healthCheckHandler{
		nextHandler: handler,
	}
}

// ServeHTTP handles request to health check
func (m *healthCheckHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if isHealthCheckRequest(req.URL.Path) {
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte("success"))
		if err != nil {
			zlog.Errorf("Failed to write response: %v", err)
			http.Error(w, "health check failed", http.StatusInternalServerError)
			return
		}
		return
	}
	m.nextHandler.ServeHTTP(w, req)
}

func isHealthCheckRequest(pathname string) bool {
	return strings.HasPrefix(pathname, healthcheckPrefix)
}
