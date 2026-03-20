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
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"path"
	"strconv"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/endpoints/handlers/responsewriters"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"

	"console-service/pkg/constant"
	"console-service/pkg/plugin"
	"console-service/pkg/server/request"
	"console-service/pkg/utils/multiclusterutil"
	"console-service/pkg/zlog"
)

type consolePluginProxy struct {
	config             *rest.Config
	client             *dynamic.DynamicClient
	nextHandler        http.Handler
	singleClusterProxy *httputil.ReverseProxy
	multiClusterProxy  *httputil.ReverseProxy
}

const (
	proxyMinLength        = 2
	consolePluginUrl      = "/apis/console.openfuyao.com/v1beta1/consoleplugins"
	consolePluginResource = "consoleplugin"

	listClustersUrl = "/rest/multicluster/v1beta1/resources/clusters"

	isPluginMjs     = 0
	isPluginBackend = 1
	notPlugin       = 2
)

// ProxyConsolePlugin proxies requests for console plugin resources
func ProxyConsolePlugin(handler http.Handler, config *rest.Config) http.Handler {
	client, err := dynamic.NewForConfig(config)
	if err != nil {
		zlog.Error("Fail to start dynamic client")
		return handler
	}

	pluginProxy := &consolePluginProxy{
		config:      config,
		client:      client,
		nextHandler: handler,
	}
	// proxy single cluster requests
	singleClusterHost := fmt.Sprintf("%s://%s", constant.SingleClusterProxyScheme, constant.SingleClusterProxyHost)
	pluginProxy.singleClusterProxy = httputil.NewSingleHostReverseProxy(parseHost(singleClusterHost))
	pluginProxy.singleClusterProxy.Transport = &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}

	// proxy multi cluster requests
	multiClusterHost := fmt.Sprintf("%s://%s", constant.MultiClusterProxyScheme, constant.MultiClusterProxyHost)
	pluginProxy.multiClusterProxy = httputil.NewSingleHostReverseProxy(parseHost(multiClusterHost))
	pluginProxy.multiClusterProxy.Transport = &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}

	return pluginProxy
}

func (cp *consolePluginProxy) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	info, ok := request.RequestInfoFrom(req.Context())
	if !ok {
		responsewriters.InternalError(w, req, fmt.Errorf("no RequestInfo found in the context"))
		return
	}

	status := checkConsolePluginType(req.URL.Path)
	if status == notPlugin {
		cp.nextHandler.ServeHTTP(w, req)
		return
	}

	pathParts := splitPath(req.URL.Path)
	if len(pathParts) < proxyMinLength {
		zlog.Error("Plugin name not specified")
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	// in isPluginMjs and isPluginBackend pluginName both resides at the 1st place
	pluginName := pathParts[1]
	zlog.Infof("ConsolePlugin Proxy: %s", pluginName)
	// for multicluster plugin we only proxy to current cluster
	if pluginName == constant.MultiClusterPluginName {
		info.SetSingleCluster()
		zlog.Infof("single cluster proxy for %s", pluginName)
	}
	requestUrl := preparePluginRequestURLByCluster(info, consolePluginUrl, pluginName)
	zlog.Infof("plugin request uri: %s", requestUrl)
	consolePlugin, err := getConsolePlugin(req, requestUrl, pluginName)
	if err != nil {
		// here err != nil means the multi-cluster plugin is not installed, need to return dummy "host" cluster
		if pluginName == constant.MultiClusterPluginName && req.URL.Path == listClustersUrl {
			returnDummyHostCluster(w)
			return
		}
		zlog.Errorf("Fail to get %s", pluginName)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	if status == isPluginMjs {
		req.URL.Path = strings.TrimPrefix(req.URL.Path, "/proxy/"+pluginName)
	}

	req.URL.Path = path.Join(getConsolePluginProxyPathPrefix(consolePlugin), req.URL.Path)
	if multiclusterutil.IsMultiClusterRequest(info) {
		req.URL.Path = path.Join(info.ClusterProxyURL, req.URL.Path)
		cp.multiClusterProxy.ServeHTTP(w, req)
	} else {
		cp.singleClusterProxy.ServeHTTP(w, req)
	}
	return
}

func returnDummyHostCluster(w http.ResponseWriter) {
	dummyClusterList := multiclusterutil.ReturnDummyHostCluster()
	w.WriteHeader(http.StatusOK)
	err := json.NewEncoder(w).Encode(dummyClusterList)
	if err != nil {
		zlog.Errorf("cannot return dummy cluster obj, err: %v", err)
	}
	return
}

func addAuthorizationHeader(req *http.Request, extReq *http.Request) {
	// add authorization header
	authInfo := req.Header.Get("Authorization")
	if authInfo != "" {
		zlog.Info("Successfully retrieve token from request")
	}
	extReq.Header.Set("Authorization", authInfo)

	authInfo = req.Header.Get(constant.OpenFuyaoAuthHeader)
	if authInfo != "" {
		zlog.Info("Successfully retrieve openfuyao token from request")
	}
	extReq.Header.Set(constant.OpenFuyaoAuthHeader, authInfo)
}

func getConsolePlugin(oriReq *http.Request, url string, consolePluginName string) (*plugin.ConsolePlugin, error) {
	// Create a new HTTP request
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		zlog.Errorf("Error creating request: %v", err)
		return nil, nil
	}

	// Add the Authorization header
	addAuthorizationHeader(oriReq, req)

	resp, err := requestConsolePlugin(req)
	if err != nil {
		zlog.Errorf("Error sending request: %v", err)
		return nil, err
	}
	defer resp.Body.Close()

	// Read and print the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		zlog.Errorf("Error reading response: %v", err)
		return nil, err
	}

	zlog.Infof("raw consoleplugin json return: %s", string(body))
	// Unmarshal the response into an unstructured.Unstructured object
	var obj unstructured.Unstructured
	err = json.Unmarshal(body, &obj.Object)
	if err != nil {
		zlog.Errorf("Error unmarshaling response into Unstructured: %v", err)
		return nil, err
	}
	var consolePlugin plugin.ConsolePlugin
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &consolePlugin)
	if err != nil {
		zlog.Errorf("Error converting to %s: %s, err: %v", consolePluginResource, consolePluginName, err)
		return nil, err
	}

	return &consolePlugin, nil
}

func requestConsolePlugin(req *http.Request) (*http.Response, error) {
	// Create a custom HTTP client with TLS config to skip certificate verification
	client := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}}

	// Send the HTTP request
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func checkConsolePluginType(path string) int {
	if strings.HasPrefix(path, "/proxy") {
		return isPluginMjs
	} else if (strings.HasPrefix(path, "/rest") || strings.HasPrefix(path, "/ws")) &&
		!strings.HasPrefix(path, "/rest/console") {
		return isPluginBackend
	} else {
		return notPlugin
	}
}

func preparePluginRequestURLByCluster(info *request.RequestInfo, urlPath string, pluginName string) string {
	url := ""
	if multiclusterutil.IsMultiClusterRequest(info) && pluginName != constant.MultiClusterPluginName {
		urlPath = path.Join(info.ClusterProxyURL, urlPath)
		url = fmt.Sprintf("%s://%s%s/%s", info.ClusterProxyScheme, info.ClusterProxyHost, urlPath, pluginName)
	} else {
		url = fmt.Sprintf("%s://%s%s/%s", constant.SingleClusterProxyScheme, constant.SingleClusterProxyHost,
			urlPath, pluginName)
	}
	return url
}

func splitPath(path string) []string {
	path = strings.Trim(path, "/")
	if path == "" {
		return []string{}
	}
	return strings.Split(path, "/")
}

func getConsolePluginProxyPathPrefix(cp *plugin.ConsolePlugin) string {
	cpBackend := cp.Spec.Backend

	if cpBackend.Type == plugin.ServiceBackendType {
		service := cpBackend.Service
		name := service.Name
		namespace := service.Namespace
		port := service.Port
		basePath := service.BasePath
		scheme := "http"
		if service.Port == 443 || service.Name == "https" || service.Name == "proxy" {
			scheme = "https"
		}
		proxyPath := constant.ServiceProxyURL
		proxyPath = strings.Replace(proxyPath, "{namespace}", namespace, 1)
		if scheme == "https" {
			proxyPath = strings.Replace(proxyPath, "{service}", scheme+":"+name, 1)
		} else {
			proxyPath = strings.Replace(proxyPath, "{service}", name, 1)
		}
		proxyPath = strings.Replace(proxyPath, "{port}", strconv.Itoa(int(port)), 1)
		return path.Join(proxyPath, basePath)
	}

	return ""
}
