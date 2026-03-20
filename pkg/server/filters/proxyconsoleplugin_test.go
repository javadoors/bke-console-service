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
	"context"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"testing"

	"k8s.io/client-go/rest"
)

func createTestCPProxy(t *testing.T) *consolePluginProxy {
	proxy, ok := ProxyConsolePlugin(testHandler, &rest.Config{}).(*consolePluginProxy)
	if !ok {
		t.Fatalf("Failed to create test consoleplugin proxy")
		return nil
	}

	backendUrl := createTestBackend(t)
	proxy.singleClusterProxy = httputil.NewSingleHostReverseProxy(backendUrl)
	proxy.multiClusterProxy = httputil.NewSingleHostReverseProxy(backendUrl)

	return proxy
}

func TestCPProxyServeNonPluginReq(t *testing.T) {
	proxy := createTestCPProxy(t)

	tests := []struct {
		name           string
		context        context.Context
		expectedStatus int
	}{
		{
			"TestWithoutReqInfo",
			context.Background(),
			http.StatusInternalServerError,
		},
		{
			"TestNonPluginReq",
			singleClusterCtx,
			http.StatusTeapot,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/", nil).WithContext(tt.context)
			proxy.ServeHTTP(recorder, req)
			resp := recorder.Result()
			defer resp.Body.Close()
			if resp.StatusCode != tt.expectedStatus {
				t.Errorf("Expected status code %d, but got %d", tt.expectedStatus, resp.StatusCode)
			}
		})
	}
}

func TestCPProxyServePluginReq(t *testing.T) {
	proxy := createTestCPProxy(t)
	tests := []struct {
		name           string
		req            *http.Request
		expectedStatus int
		patch          bool
	}{
		{
			name:           "TestMjsReqInvalid",
			req:            httptest.NewRequest("GET", "/proxy", nil).WithContext(singleClusterCtx),
			expectedStatus: http.StatusBadRequest,
			patch:          false,
		},
		{
			name: "TestPluginReqFail",
			req: httptest.NewRequest("GET",
				"/rest/multicluster/v1beta1/resources/clusters", nil).WithContext(multiClusterCtx),
			expectedStatus: http.StatusOK,
			patch:          false,
		},
		{
			name:           "TestMjsReqMultiCluster",
			req:            httptest.NewRequest("GET", "/proxy/multicluster", nil).WithContext(multiClusterCtx),
			expectedStatus: http.StatusOK,
			patch:          true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			if tt.patch {
				patchRequestConsolePlugin(t)
			}
			proxy.ServeHTTP(recorder, tt.req)
			resp := recorder.Result()
			if resp.StatusCode != tt.expectedStatus {
				t.Errorf("Expected status code %d, but got %d", tt.expectedStatus, resp.StatusCode)
			}
			defer resp.Body.Close()
		})
	}
}
