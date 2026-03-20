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
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"console-service/pkg/auth"
	"console-service/pkg/server/config"
	"console-service/pkg/server/request"
	"console-service/pkg/utils/util"
)

var singleClusterCtx = request.WithRequestInfo(context.Background(), &request.RequestInfo{
	ClusterName:      "",
	ClusterProxyURL:  "",
	ClusterProxyHost: "",
})

var multiClusterCtx = request.WithRequestInfo(context.Background(), &request.RequestInfo{
	ClusterProxyURL:  "/clusters/test-cluster",
	ClusterProxyHost: "test-cluster",
})

var testHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusTeapot)
})

func createTestBackend(t *testing.T) *url.URL {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte(`{"code": 200, "msg": "OK", "data": []}`))
		if err != nil {
			t.Errorf("Failed to write response body: %v", err)
		}
	}))
	backendUrl, err := url.Parse(backend.URL)
	if err != nil {
		t.Fatalf("Failed to parse backend URL: %v", err)
	}
	t.Cleanup(func() {
		backend.Close()
	})
	return backendUrl
}

func createTestProxy(t *testing.T) *componentProxy {
	patch := gomonkey.ApplyFunc(util.GetConsoleServiceConfig, func(c kubernetes.Interface) (
		*config.ConsoleServiceConfig, error) {
		return &config.ConsoleServiceConfig{
			AlertHost:          "http://alert-service.alert-namespace.svc.cluster.local",
			MonitoringHost:     "http://monitoring-service.monitoring-namespace.svc.cluster.local",
			WebTerminalHost:    "http://webterminal-service.webterminal-namespace.svc.cluster.local",
			OAuthServerHost:    "https://oauth-service.oauth-namespace.svc.cluster.local",
			ApplicationHost:    "http://application-service.application-namespace.svc.cluster.local",
			PluginHost:         "http://plugin-service.plugin-namespace.svc.cluster.local",
			MarketPlaceHost:    "http://marketplace-service.marketplace-namespace.svc.cluster.local",
			UserManagementHost: "http://usermanagement-service.usermanagement-namespace.svc.cluster.local",
			InsecureSkipVerify: "true",
		}, nil
	})
	t.Cleanup(func() {
		patch.Reset()
	})

	proxy, ok := ProxyComponentRequest(testHandler, testHandler, &rest.Config{}).(*componentProxy)
	if !ok {
		t.Fatalf("Failed to create test component proxy")
		return nil
	}

	backendUrl := createTestBackend(t)
	proxy.consolePluginProxy = httputil.NewSingleHostReverseProxy(backendUrl)
	proxy.oauthProxy = httputil.NewSingleHostReverseProxy(backendUrl)
	proxy.singleClusterProxy = httputil.NewSingleHostReverseProxy(backendUrl)
	proxy.multiClusterProxy = httputil.NewSingleHostReverseProxy(backendUrl)

	return proxy
}

func TestServeProxyByCluster(t *testing.T) {
	proxy := createTestProxy(t)
	proxyByClusterTests := []struct {
		name   string
		target string
	}{
		{"TestServeAuthRequest", "/rest/user/test"},
		{"TestServeApplication", "/rest/application-management/test"},
		{"TestServePlugin", "/rest/plugin-management/test"},
		{"TestServeMarket", "/rest/marketplace/test"},
		{"TestServeMonitoring", "/rest/monitoring/test"},
		{"TestServeWebTerminal", "/rest/webterminal/test"},
	}
	for _, tt := range proxyByClusterTests {
		t.Run(tt.name, func(t *testing.T) {
			// Use single cluster proxy
			req := httptest.NewRequest("GET", tt.target, nil).WithContext(singleClusterCtx)
			recorder := httptest.NewRecorder()
			proxy.ServeHTTP(recorder, req)
			resp := recorder.Result()
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				t.Errorf("Expected status code %d, but got %d", http.StatusOK, resp.StatusCode)
			}
		})
	}
}

func patchRequestConsolePlugin(t *testing.T) {
	rawBody := `{"apiVersion":"console.openfuyao.com/v1beta1","kind":"ConsolePlugin","metadata":{"name":` +
		`"multicluster"},"spec":{"backend":{"service":{"name":"multi-cluster-website","namespace":"karmada-` +
		`system","port":8080},"type":"Service"},"displayName":"集群管理","enabled":true,"entrypoint":"/",` +
		`"pluginName":"multicluster","order":1}}`
	patches := gomonkey.ApplyFunc(requestConsolePlugin, func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBufferString(rawBody)),
		}, nil
	})
	t.Cleanup(func() {
		patches.Reset()
	})
}

func TestServeProxyConsolePluginManagementByCluster(t *testing.T) {
	proxy := createTestProxy(t)
	patchRequestConsolePlugin(t)
	// Use  cluster proxy
	req := httptest.NewRequest("GET", "/rest/plugin-management/consoleplugins", nil).WithContext(multiClusterCtx)
	recorder := httptest.NewRecorder()
	proxy.ServeHTTP(recorder, req)
	resp := recorder.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status code %d, but got %d", http.StatusOK, resp.StatusCode)
	}
}

func generateToken(sub string) string {
	headerObj := map[string]interface{}{"alg": "none", "typ": "JWT"}
	headerBytes, err := json.Marshal(headerObj)
	if err != nil {
		return ""
	}
	header := base64.RawURLEncoding.EncodeToString(headerBytes)

	payloadObj := map[string]interface{}{"sub": sub}
	payloadBytes, err := json.Marshal(payloadObj)
	if err != nil {
		return ""
	}
	payload := base64.RawURLEncoding.EncodeToString(payloadBytes)

	return header + "." + payload + "."
}

func createTestAlertReq(createdBy, sessionID string) *http.Request {
	// Use single cluster proxy
	req := httptest.NewRequest("POST", "/rest/alert/api/v2/silences", nil).WithContext(singleClusterCtx)
	req.Body = io.NopCloser(bytes.NewBufferString(`{"createdBy":"` + createdBy + `"}`))
	if sessionID != "" {
		req.AddCookie(&http.Cookie{Name: "sessionID_bke", Value: sessionID})
	}
	return req
}

func TestServeAlertSkip(t *testing.T) {
	proxy := createTestProxy(t)
	tests := []struct {
		name       string
		req        *http.Request
		wantStatus int
	}{
		{
			"TestGetRequest",
			httptest.NewRequest("GET", "/rest/alert/test", nil).WithContext(singleClusterCtx),
			http.StatusOK,
		},
		{
			"TestSilenceRequest",
			httptest.NewRequest("POST", "/rest/alert/test", nil).WithContext(singleClusterCtx),
			http.StatusOK,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			proxy.ServeHTTP(recorder, tt.req)
			resp := recorder.Result()
			defer resp.Body.Close()
			if resp.StatusCode != tt.wantStatus {
				t.Errorf("Expected status code %d, but got %d", tt.wantStatus, resp.StatusCode)
			}
		})
	}
}

func TestServeAlertCheck(t *testing.T) {
	proxy := createTestProxy(t)
	tests := []struct {
		name  string
		req   *http.Request
		token string
		want  int
	}{
		{
			"TestNoCookie",
			createTestAlertReq("admin", ""),
			"xxxxxxx",
			http.StatusBadRequest,
		},
		{
			"TestInvalidToken",
			createTestAlertReq("admin", "test-session"),
			"xxxxxxx",
			http.StatusBadRequest,
		},
		{
			"TestPostByCreator",
			createTestAlertReq("admin", "test-session"),
			generateToken("admin"),
			http.StatusOK,
		},
		{
			"TestPostByOtherUser",
			createTestAlertReq("non-admin", "test-session"),
			generateToken("admin"),
			http.StatusBadRequest,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			patchToken := gomonkey.ApplyFunc(auth.GetTokenFromSessionID, func(clientset kubernetes.Interface,
				sessionID string) (string, string, error) {
				return test.token, "", nil
			})
			defer patchToken.Reset()
			proxy.ServeHTTP(rec, test.req)
			resp := rec.Result()
			defer resp.Body.Close()
			if test.want != resp.StatusCode {
				t.Errorf("Expected status code %d, but got %d", test.want, resp.StatusCode)
			}
		})
	}
}

func TestServeOauth(t *testing.T) {
	proxy := createTestProxy(t)
	tests := []struct {
		name       string
		respStatus int
		wantStatus int
	}{
		{
			"TestStatusOK",
			http.StatusOK,
			http.StatusNoContent,
		},
		{
			"TestStatusFound",
			http.StatusFound,
			http.StatusNoContent,
		},
		{
			"TestStatusInternalError",
			http.StatusInternalServerError,
			http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.respStatus)
			}))
			backendUrl, err := url.Parse(backend.URL)
			if err != nil {
				t.Fatalf("Failed to parse backend URL: %v", err)
			}
			proxy.oauthProxy = httputil.NewSingleHostReverseProxy(backendUrl)

			req := httptest.NewRequest("GET", "/password/test", nil).WithContext(singleClusterCtx)
			recorder := httptest.NewRecorder()

			proxy.ServeHTTP(recorder, req)
			backend.Close()

			resp := recorder.Result()
			defer resp.Body.Close()
			if resp.StatusCode != tt.wantStatus {
				t.Errorf("Expected status code %d, but got %d", tt.respStatus, resp.StatusCode)
			}
		})
	}
}
