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
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"path"
	"regexp"
	"strconv"
	"strings"

	"github.com/golang-jwt/jwt/v4"
	"k8s.io/apiserver/pkg/endpoints/handlers/responsewriters"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"console-service/pkg/auth"
	"console-service/pkg/constant"
	"console-service/pkg/plugin"
	"console-service/pkg/server/request"
	httputil2 "console-service/pkg/utils/httputil"
	"console-service/pkg/utils/multiclusterutil"
	"console-service/pkg/utils/util"
	"console-service/pkg/zlog"
)

type componentProxy struct {
	kubeConfig         *rest.Config
	clientset          kubernetes.Interface
	oauthProxy         *httputil.ReverseProxy
	singleClusterProxy *httputil.ReverseProxy
	multiClusterProxy  *httputil.ReverseProxy
	consolePluginProxy *httputil.ReverseProxy

	alertHost          string
	monitoringHost     string
	webTerminalHost    string
	oauthHost          string
	pluginHost         string
	marketPlaceHost    string
	applicationHost    string
	userManagementHost string

	nextHandler    http.Handler
	consoleHandler http.Handler
}

// AlertSilenceRequest structure
type AlertSilenceRequest struct {
	StartsAt  string    `json:"startsAt,omitempty"`
	EndsAt    string    `json:"endsAt,omitempty"`
	CreatedBy string    `json:"createdBy,omitempty"`
	Comment   string    `json:"comment,omitempty"`
	Matchers  []Matcher `json:"matchers,omitempty"`
}

// Matcher structure
type Matcher struct {
	Name    string `json:"name,omitempty"`
	Value   string `json:"value,omitempty"`
	IsRegex bool   `json:"isRegex,omitempty"`
	IsEqual bool   `json:"isEqual,omitempty"`
}

// JWTAccessClaims structure
type JWTAccessClaims struct {
	jwt.StandardClaims
}

const (
	// api prefix
	alertApiPrefix                = "/rest/alert"
	monitoringApiPrefix           = "/rest/monitoring/"
	webTerminalApiPrefix          = "/rest/webterminal/"
	pluginApiPrefix               = "/rest/plugin-management/"
	applicationApiPrefix          = "/rest/application-management/"
	marketApiPrefix               = "/rest/marketplace/"
	userManagementApiPrefix       = "/rest/user/"
	webPasswordPrefix             = "/password"
	oauthServerPrefix             = "/auth"
	oauthPasswordIdentityProvider = "fuyaoPasswordProvider"
)

// ProxyComponentRequest proxies requests to openFuyao components other than oauth and APIServer,
// otherwise passes the request to the next handler
func ProxyComponentRequest(consoleHandler http.Handler, nextHandler http.Handler, config *rest.Config) http.Handler {
	proxy := &componentProxy{
		kubeConfig:     config,
		nextHandler:    nextHandler,
		consoleHandler: consoleHandler,
	}

	proxy.setSingleClusterProxy()
	multiClusterHost := proxy.setMultiClusterProxy()
	proxy.setConsolePluginProxy(multiClusterHost)

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		zlog.Errorf("error creating client set, err: %v", err)
		return nextHandler
	}
	proxy.clientset = clientset

	insecureSkipVerify := proxy.setComponentProxyHosts()

	// oauthProxy and staticProxy only proxy to the current cluster service and without kube-apiserver proxy
	oauthProxy := httputil.NewSingleHostReverseProxy(parseHost(proxy.oauthHost))
	transport, err := httputil2.GetHttpTransport(!insecureSkipVerify)
	if err != nil {
		zlog.Warn("Fail to add https transport for oauth proxy, use http")
	}
	oauthProxy.Transport = transport
	proxy.oauthProxy = oauthProxy

	return proxy
}

func (sp *componentProxy) setComponentProxyHosts() bool {
	csConfig, err := util.GetConsoleServiceConfig(sp.clientset)
	insecureSkipVerify := true
	if err != nil {
		zlog.Warnf("read console-service-config config map failed, reading default set")
	} else {
		sp.alertHost = csConfig.AlertHost
		sp.monitoringHost = csConfig.MonitoringHost
		sp.webTerminalHost = csConfig.WebTerminalHost
		sp.oauthHost = csConfig.OAuthServerHost
		sp.applicationHost = csConfig.ApplicationHost
		sp.pluginHost = csConfig.PluginHost
		sp.marketPlaceHost = csConfig.MarketPlaceHost
		sp.userManagementHost = csConfig.UserManagementHost
		insecureSkipVerify = csConfig.InsecureSkipVerify == "true"
	}
	return insecureSkipVerify
}

func (sp *componentProxy) setConsolePluginProxy(multiClusterHost string) {
	// proxy console-plugin requests, using the same host as multiClusterHost
	sp.consolePluginProxy = httputil.NewSingleHostReverseProxy(parseHost(multiClusterHost))
	sp.consolePluginProxy.Transport = &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
}

func (sp *componentProxy) setMultiClusterProxy() string {
	// proxy multi cluster requests
	multiClusterHost := fmt.Sprintf("%s://%s", constant.MultiClusterProxyScheme, constant.MultiClusterProxyHost)
	sp.multiClusterProxy = httputil.NewSingleHostReverseProxy(parseHost(multiClusterHost))
	sp.multiClusterProxy.Transport = &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	return multiClusterHost
}

func (sp *componentProxy) setSingleClusterProxy() {
	// proxy single cluster requests
	singleClusterHost := fmt.Sprintf("%s://%s", constant.SingleClusterProxyScheme, constant.SingleClusterProxyHost)
	sp.singleClusterProxy = httputil.NewSingleHostReverseProxy(parseHost(singleClusterHost))
	sp.singleClusterProxy.Transport = &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
}

func parseHost(host string) *url.URL {
	if len(host) == 0 {
		zlog.Errorf("Configured host is empty")
		return nil
	}
	target, err := url.Parse(host)
	if err != nil {
		zlog.Errorf("Failed to parse host %s", host)
	}
	return target
}

// ServeHTTP handles request to ProxyComponentRequest
func (sp componentProxy) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	// fetch requestinfo for multi-cluster info
	info, ok := request.RequestInfoFrom(req.Context())
	if !ok {
		responsewriters.InternalError(w, req, fmt.Errorf("no RequestInfo found in the context"))
		return
	}

	if isUserManagementRequest(req.URL.Path) {
		zlog.Infof("User management request %s", req.URL.Path)
		sp.proxyByCluster(w, req, info, sp.userManagementHost)
		return
	}

	if isApplicationRequest(req.URL.Path) {
		zlog.Infof("Application management request %s", req.URL.Path)
		sp.proxyByCluster(w, req, info, sp.applicationHost)
		return
	}

	if isPluginRequest(req.URL.Path) {
		zlog.Infof("Plugin management request %s", req.URL.Path)
		sp.proxyConsolePluginManagementByCluster(w, req, info, sp.pluginHost)
		return
	}

	if isMarketPlaceRequest(req.URL.Path) {
		zlog.Infof("Marketplace request %s", req.URL.Path)
		sp.proxyByCluster(w, req, info, sp.marketPlaceHost)
		return
	}

	if isAlertRequest(req.URL.Path) {
		zlog.Infof("Alert request %s", req.URL.Path)
		req.URL.Path = strings.TrimPrefix(req.URL.Path, alertApiPrefix)
		ok := sp.checkCreatedBySessionUser(req)
		if !ok {
			http.Error(w, "CreatedBy user is not valid", http.StatusBadRequest)
			return
		}
		sp.proxyByCluster(w, req, info, sp.alertHost)
		return
	}

	if isMonitoringRequest(req.URL.Path) {
		zlog.Infof("Monitoring request %s", req.URL.Path)
		sp.proxyByCluster(w, req, info, sp.monitoringHost)
		return
	}

	if isWebTerminalRequest(req.URL.Path) {
		zlog.Infof("WebTerminal request %s", req.URL.Path)
		sp.proxyByCluster(w, req, info, sp.webTerminalHost)
		return
	}

	if isOAuthRequest(req.URL.Path) {
		zlog.Infof("OAuth request %s", req.URL.Path)
		sp.modifyOAuthRequestResponse(w, req)
		sp.oauthProxy.ServeHTTP(w, req)
		return
	}

	sp.nextHandler.ServeHTTP(w, req)
}

func (sp componentProxy) modifyOAuthRequestResponse(w http.ResponseWriter, req *http.Request) {
	req.URL.Path = path.Join("/oauth2"+oauthServerPrefix, req.URL.Path, oauthPasswordIdentityProvider)
	sp.oauthProxy.ModifyResponse = func(response *http.Response) error {
		authHandler, err := auth.NewHandler(sp.kubeConfig)
		if err != nil {
			zlog.Errorf("failed to initialize auth handler %v", err)
			return err
		}
		if response.StatusCode == http.StatusOK {
			zlog.Info("Password reset succeed, proxy to logout handler")
			authHandler.LogoutCore(req, w)
		}
		if response.StatusCode == http.StatusFound {
			zlog.Info("Password reset failed, maximum time reached")
			authHandler.LogoutCore(req, w)
		}
		return nil
	}
}

func (sp componentProxy) proxyByCluster(w http.ResponseWriter, req *http.Request,
	info *request.RequestInfo, host string) {
	req.URL.Path = path.Join(sp.convertToServiceProxyPath(host), req.URL.Path)
	if multiclusterutil.IsMultiClusterRequest(info) {
		req.URL.Path = path.Join(info.ClusterProxyURL, req.URL.Path)
		sp.multiClusterProxy.ServeHTTP(w, req)
	} else {
		sp.singleClusterProxy.ServeHTTP(w, req)
	}
}

func (sp componentProxy) proxyConsolePluginManagementByCluster(w http.ResponseWriter, req *http.Request,
	info *request.RequestInfo, host string) {
	// proxy consoleplugin management requests to specific cluster
	// 如果是单集群的，直接调用当前的authhandler
	req.URL.Path = path.Join(sp.convertToServiceProxyPath(host), req.URL.Path)
	if !multiclusterutil.IsMultiClusterRequest(info) {
		zlog.Infof("single cluster consoleplugin request: %s", req.URL.Path)
		sp.singleClusterProxy.ServeHTTP(w, req)
		return
	}
	// 如果是 multiclusterrequest, 正常proxy；
	req.URL.Path = path.Join(info.ClusterProxyURL, req.URL.Path)
	zlog.Infof("multi-cluster consoleplugin request: %s", req.URL.Path)

	sp.modifyConsolePluginProxyResponse(w, req, info)
	sp.consolePluginProxy.ServeHTTP(w, req)
}

func (sp componentProxy) modifyConsolePluginProxyResponse(w http.ResponseWriter, req *http.Request,
	info *request.RequestInfo) {
	sp.consolePluginProxy.ModifyResponse = func(response *http.Response) error {
		// only check request ending with /consoleplugins
		if !strings.HasSuffix(strings.TrimSuffix(req.URL.Path, "/"), "/consoleplugins") {
			zlog.Infof("request path is %s, not list consoleplugins passthrough", req.URL.Path)
			return nil
		}
		// if clustername is host and we are authorized, we don't need to append multicluster cp
		if multiclusterutil.IsMultiClusterRequest(info) && response.StatusCode != http.StatusForbidden &&
			info.ClusterName == "host" {
			zlog.Infof("request consoleplugins but the it's host cluster, passthrough, %s", req.URL.Path)
			return nil
		}
		// get multi-cluster plugin in host
		consolePluginTrimmed, err := sp.getHostPlugin(req, info, constant.MultiClusterPluginName)
		if consolePluginTrimmed == nil {
			return err
		}

		respJson, err := parseResponse(response)
		if err != nil {
			return err
		}

		err = mergeConsolePlugins(respJson, consolePluginTrimmed)
		if err != nil {
			return err
		}

		// 将修改后的 respJson 重新编码为 JSON
		newBody, err := json.Marshal(respJson)
		if err != nil {
			zlog.Errorf("failed to marshal updated response: %v", err)
			return err
		}

		// 替换 response.Body 为新的 JSON
		_, err = w.Write(newBody)
		if err != nil {
			zlog.Warnf("write response error: %v", err)
		}
		return nil
	}
}

func mergeConsolePlugins(respJson *httputil2.ResponseJson, consolePluginTrimmed *plugin.ConsolePluginTrimmed) error {
	// 将 Data 字段转换为 []plugin.ConsolePluginTrimmed
	if respJson.Data != nil {
		// 尝试将 Data 转换为 []plugin.ConsolePluginTrimmed
		var consolePluginList []plugin.ConsolePluginTrimmed
		consolePluginListBytes, err := json.Marshal(respJson.Data)
		if err != nil {
			zlog.Errorf("failed to marshal Data field: %v", err)
			return err
		}

		err = json.Unmarshal(consolePluginListBytes, &consolePluginList)
		if err != nil {
			zlog.Errorf("failed to unmarshal Data field into consolePluginList: %v", err)
			return err
		}

		consolePluginList = append(consolePluginList, *consolePluginTrimmed)
		respJson.Data = consolePluginList
	} else {
		respJson.Data = []plugin.ConsolePluginTrimmed{*consolePluginTrimmed}
	}
	return nil
}

func parseResponse(response *http.Response) (*httputil2.ResponseJson, error) {
	body, err := io.ReadAll(response.Body)
	if err != nil {
		zlog.Errorf("failed to read response body: %v", err)
		return nil, err
	}

	var respJson httputil2.ResponseJson

	zlog.Warnf("mult-cluster cosoleplugin response: %s", string(body))
	err = json.Unmarshal(body, &respJson)
	if err != nil {
		zlog.Errorf("failed to unmarshal response body: %v", err)
		return nil, err
	}
	return &respJson, nil
}

func (sp componentProxy) getHostPlugin(req *http.Request, info *request.RequestInfo,
	pluginName string) (*plugin.ConsolePluginTrimmed, error) {
	requestUrl := preparePluginRequestURLByCluster(info, consolePluginUrl, pluginName)
	cp, err := getConsolePlugin(req, requestUrl, pluginName)
	if err != nil {
		zlog.Info("the host cluster does not install multicluster plugin")
		return nil, nil
	}
	consolePluginTrimmed := trimConsolePlugin(cp)
	return &consolePluginTrimmed, nil
}

func trimConsolePlugin(cp *plugin.ConsolePlugin) plugin.ConsolePluginTrimmed {
	consolePluginTrimmed := plugin.ConsolePluginTrimmed{
		DisplayName: cp.Spec.DisplayName,
		PluginName:  cp.Spec.PluginName,
		Order:       formatOrder(cp.Spec.Order),
		SubPages:    cp.Spec.SubPages,
		Entrypoint:  string(cp.Spec.Entrypoint),
		URL:         cp.Status.Link,
		Enabled:     cp.Spec.Enabled,
	}
	if releaseName, ok := cp.ObjectMeta.Annotations["meta.helm.sh/release-name"]; ok {
		consolePluginTrimmed.Release = releaseName
	}
	return consolePluginTrimmed
}

func isAlertRequest(pathname string) bool {
	return strings.HasPrefix(pathname, alertApiPrefix+"/")
}

func isMonitoringRequest(pathname string) bool {
	return strings.HasPrefix(pathname, monitoringApiPrefix)
}

func isWebTerminalRequest(pathname string) bool {
	return strings.HasPrefix(pathname, webTerminalApiPrefix)
}

func isOAuthRequest(pathname string) bool {
	return strings.HasPrefix(pathname, webPasswordPrefix)
}

func isMarketPlaceRequest(pathname string) bool {
	return strings.HasPrefix(pathname, marketApiPrefix)
}

func isApplicationRequest(pathname string) bool {
	return strings.HasPrefix(pathname, applicationApiPrefix)
}

func isUserManagementRequest(pathname string) bool {
	return strings.HasPrefix(pathname, userManagementApiPrefix)
}

func isPluginRequest(pathname string) bool {
	return strings.HasPrefix(pathname, pluginApiPrefix)
}

func (sp componentProxy) checkCreatedBySessionUser(req *http.Request) bool {
	if req.Method != "POST" {
		return true
	}
	if req.URL.Path != "/api/v2/silences" {
		return true
	}

	// get user info from silence request
	var alert AlertSilenceRequest
	bodyBytes, err := io.ReadAll(req.Body)
	if err != nil {
		return false
	}
	req.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	err = json.NewDecoder(bytes.NewReader(bodyBytes)).Decode(&alert)

	if err != nil {
		zlog.Error("error to read body bytes from Alert Request: %v", err)
		return false
	}
	createdBy := alert.CreatedBy

	// get user info from session
	sessionID, err := req.Cookie(cookieNameSessionID)
	if err != nil {
		zlog.Error("Fail to get cookieNameSessionID from cookie: %v", err)
		return false
	}
	token, _, err := auth.GetTokenFromSessionID(sp.clientset, sessionID.Value)
	if err != nil && err.Error() != constant.OnlyAccessTokenExpiredErrorStr {
		zlog.Errorf("Fail to get token from cookie: %v", err)
		return false
	}

	// parse JWT
	var claims = JWTAccessClaims{
		StandardClaims: jwt.StandardClaims{},
	}
	_, _, err = jwt.NewParser().ParseUnverified(token, &claims)
	if err != nil {
		zlog.Errorf("Fail to parse tokenJWT: %v", err)
		return false
	}

	// check user info from token
	if claims.Subject != createdBy {
		zlog.Error("Unauthorized user changes.")
		return false
	}

	return true
}

const (
	protocolIndex  = 1
	serviceIndex   = 2
	namespaceIndex = 3
	portIndex      = 4
)

func (sp componentProxy) convertToServiceProxyPath(host string) string {
	// host: http(s)://my-service.my-namespace.svc.cluster.local:80
	re := regexp.MustCompile(`^(https?):\/\/([a-zA-Z0-9-]+)\.([a-zA-Z0-9-]+)(?:\.svc\.cluster\.local)?(?::(\d+))?$`)
	matches := re.FindStringSubmatch(host)
	if len(matches) <= 0 {
		zlog.Error("Invalid k8s service format.")
		return ""
	}

	// fetch each component
	protocol := matches[protocolIndex]
	svc := matches[serviceIndex]
	ns := matches[namespaceIndex]
	port := matches[portIndex]

	proxyPath := constant.ServiceProxyURL
	proxyPath = strings.Replace(proxyPath, "{namespace}", ns, 1)
	if protocol == "https" {
		proxyPath = strings.Replace(proxyPath, "{service}", protocol+":"+svc, 1)
		if port == "" {
			port = "443"
		}
	} else {
		proxyPath = strings.Replace(proxyPath, "{service}", svc, 1)
		if port == "" {
			port = "80"
		}
	}
	proxyPath = strings.Replace(proxyPath, "{port}", port, 1)

	return proxyPath
}

func formatOrder(order *int64) *string {
	if order != nil {
		formattedOrder := strconv.FormatInt(*order, constant.BaseTen)
		return &formattedOrder
	}
	return nil
}
