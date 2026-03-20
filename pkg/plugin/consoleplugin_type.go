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

package plugin

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// ConsolePluginSpec specifies the expected status of a console plugin resource
type ConsolePluginSpec struct {
	// PluginName is the unique name of the plugin. The name should only include alphabets, digits and '-'
	PluginName string `json:"pluginName"`

	// Order is the index of the plugin. The value should only be non-negative integers
	Order *int64 `json:"order,omitempty"`

	// DisplayName is the display name of the plugin on the UI entrypoint, should be between 1 and 128 characters.
	DisplayName string `json:"displayName"`

	// SubPages stands for the pages under the main console plugin. Only applicable for "Side" Entrypoint
	SubPages []ConsolePluginName `json:"subPages,omitempty"`

	// Entrypoint is the location where the entrypoint of the plugin will be rendered on the console webpage.
	// Current support values are [Nav, Side]
	Entrypoint ConsolePluginEntrypoint `json:"entrypoint"`

	Backend *ConsolePluginBackend `json:"backend"`

	Enabled bool `json:"enabled"`
}

// ConsolePluginName is the name of the plugin
type ConsolePluginName struct {
	// PageName is the unique name of the page. The name should only include alphabets, digits and '-'
	PageName string `json:"pageName"`

	// DisplayName is the display name of the plugin on the UI entrypoint, should be between 1 and 128 characters.
	DisplayName string `json:"displayName"`
}

// ConsolePluginEntrypoint is an enumeration of entrypoint location
type ConsolePluginEntrypoint string

const (
	// NavEntrypoint renders the entrypoint in the top navigation bar.
	NavEntrypoint ConsolePluginEntrypoint = "Nav"

	// SideEntrypoint renders the entrypoint in the side menu.
	SideEntrypoint ConsolePluginEntrypoint = "Side"
)

// ConsolePluginBackend defines the backend configuration for a console plugin
type ConsolePluginBackend struct {
	Type ConsolePluginBackendType `json:"type"`

	Service *ConsolePluginService `json:"service"`
}

// ConsolePluginBackendType defines the type of backend service for a console plugin
type ConsolePluginBackendType string

const (
	// ServiceBackendType indicates a backend service of type Service
	ServiceBackendType ConsolePluginBackendType = "Service"
)

// ConsolePluginService defines the Kubernetes Service configuration for a console plugin backend
type ConsolePluginService struct {
	Name string `json:"name"`

	Namespace string `json:"namespace"`

	Port int32 `json:"port"`

	BasePath string `json:"basePath"`
}

// ConsolePluginStatus contains the status information of a console plugin
type ConsolePluginStatus struct {
	Link string `json:"link"`
}

// ConsolePlugin defines a custom resource for managing console plugin configurations
type ConsolePlugin struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ConsolePluginSpec   `json:"spec,omitempty"`
	Status ConsolePluginStatus `json:"status,omitempty"`
}

// ConsolePluginList contains a list of ConsolePlugin
type ConsolePluginList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ConsolePlugin `json:"items"`
}

// ConsolePluginTrimmed contains only the essential info of a plugin for front-end
type ConsolePluginTrimmed struct {
	Release     string              `json:"release"`
	DisplayName string              `json:"displayName"`
	PluginName  string              `json:"pluginName"`
	Order       *string             `json:"order,omitempty"`
	SubPages    []ConsolePluginName `json:"subPages"`
	Entrypoint  string              `json:"entrypoint"`
	URL         string              `json:"url"`
	Enabled     bool                `json:"enabled"`
}
