/*
 *
 *  * Copyright (c) 2025Huawei Technologies Co., Ltd.
 *  * openFuyao is licensed under Mulan PSL v2.
 *  * You can use this software according to the terms and conditions of the Mulan PSL v2.
 *  * You may obtain a copy of Mulan PSL v2 at:
 *  *          http://license.coscl.org.cn/MulanPSL2
 *  * THIS SOFTWARE IS PROVIDED ON AN "AS IS" BASIS, WITHOUT WARRANTIES OF ANY KIND,
 *  * EITHER EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO NON-INFRINGEMENT,
 *  * MERCHANTABILITY OR FIT FOR A PARTICULAR PURPOSE.
 *  * See the Mulan PSL v2 for more details.
 *
 */

/*
Package util include console-service level util function
*/
package util

import (
	"io/ioutil"

	"k8s.io/client-go/kubernetes"

	"console-service/pkg/constant"
	"console-service/pkg/server/config"
	"console-service/pkg/utils/k8sutil"
	"console-service/pkg/zlog"
)

// ClearByte clear byte slice by setting every index to zero
func ClearByte(value []byte) {
	for i := range value {
		value[i] = 0
	}
}

const (
	oauthServerHost    = "oauth-server-host"
	consoleServiceHost = "console-service-host"
	consoleWebsiteHost = "console-website-host"
	alertHost          = "alert-host"
	monitoringHost     = "monitoring-host"
	webTerminalHost    = "webterminal-host"
	marketplaceHost    = "marketplace-host"
	pluginHost         = "plugin-management-host"
	applicationHost    = "application-management-host"
	userManagementHost = "user-management-host"
	insecureSkipVerify = "insecure-skip-verify"
	serverName         = "server-name"
)

var configDir = "/etc/console-service/openfuyao-config"

// GetConsoleServiceConfig parse the configmap for console-service configuration from the cluster
func GetConsoleServiceConfig(c kubernetes.Interface) (*config.ConsoleServiceConfig, error) {
	configMap, err := k8sutil.GetConfigMap(c, constant.ConsoleServiceConfigmap,
		constant.ConsoleServiceDefaultNamespace)
	if err != nil {
		zlog.Warnf("failed to read config map from k8s cluster  %v", err)
		consoleServiceConfig, err := getConfigFromPod()
		if err != nil {
			zlog.Warnf("failed to read config map from container %v", err)
			return nil, err
		}
		return consoleServiceConfig, err
	}

	consoleServiceConfig := parseConfig(configMap.Data)
	return consoleServiceConfig, nil
}

func parseConfig(consoleServiceConfigMap map[string]string) *config.ConsoleServiceConfig {
	return &config.ConsoleServiceConfig{
		OAuthServerHost:    consoleServiceConfigMap[oauthServerHost],
		ConsoleServerHost:  consoleServiceConfigMap[consoleServiceHost],
		ConsoleWebsiteHost: consoleServiceConfigMap[consoleWebsiteHost],
		AlertHost:          consoleServiceConfigMap[alertHost],
		MonitoringHost:     consoleServiceConfigMap[monitoringHost],
		WebTerminalHost:    consoleServiceConfigMap[webTerminalHost],
		PluginHost:         consoleServiceConfigMap[pluginHost],
		ApplicationHost:    consoleServiceConfigMap[applicationHost],
		MarketPlaceHost:    consoleServiceConfigMap[marketplaceHost],
		UserManagementHost: consoleServiceConfigMap[userManagementHost],
		InsecureSkipVerify: consoleServiceConfigMap[insecureSkipVerify],
		ServerName:         consoleServiceConfigMap[serverName],
	}
}

func getConfigFromPod() (*config.ConsoleServiceConfig, error) {
	files, err := ioutil.ReadDir(configDir)
	if err != nil {
		return nil, err
	}
	consoleServiceConfigMap := make(map[string]string)
	for _, file := range files {
		if !file.IsDir() {
			filepath := configDir + "/" + file.Name()
			content, err := ioutil.ReadFile(filepath)
			if err != nil {
				zlog.Warnf("Error reading file %s: %v\n", filepath, err)
				continue
			}
			consoleServiceConfigMap[file.Name()] = string(content)
		}
	}
	return parseConfig(consoleServiceConfigMap), nil
}
