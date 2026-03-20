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

package httputil

import (
	"crypto/tls"
	"crypto/x509"
	"net/http"
	"os"
	"sync"

	"console-service/pkg/constant"
	"console-service/pkg/zlog"
)

// ResponseJson Http Response
type ResponseJson struct {
	Code int32  `json:"code,omitempty"`
	Msg  string `json:"msg,omitempty"`
	Data any    `json:"data,omitempty"`
}

// GetResponseJson get restful response struct
func GetResponseJson(code int32, msg string, data any) *ResponseJson {
	return &ResponseJson{
		Code: code,
		Msg:  msg,
		Data: data,
	}
}

// GetDefaultSuccessResponseJson get default success response json
func GetDefaultSuccessResponseJson() *ResponseJson {
	return &ResponseJson{
		Code: constant.Success,
		Msg:  "success",
		Data: nil,
	}
}

// GetDefaultClientFailureResponseJson get default failure response json
func GetDefaultClientFailureResponseJson() *ResponseJson {
	return &ResponseJson{
		Code: constant.ClientError,
		Msg:  "bad request",
		Data: nil,
	}
}

// GetDefaultServerFailureResponseJson get default failure response json
func GetDefaultServerFailureResponseJson() *ResponseJson {
	return &ResponseJson{
		Code: constant.ServerError,
		Msg:  "remote server busy",
		Data: nil,
	}
}

// GetParamsEmptyErrorResponseJson get default resource empty response json
func GetParamsEmptyErrorResponseJson() *ResponseJson {
	return &ResponseJson{
		Code: constant.ClientError,
		Msg:  "parameters not found",
		Data: nil,
	}
}

var (
	clientInstance *http.Client
	clientOnce     sync.Once
)

// GetHttpConfig get http config
func GetHttpConfig(enableTLS bool) (*tls.Config, error) {
	if enableTLS {
		cert, err := tls.LoadX509KeyPair(constant.TLSCertPath, constant.TLSKeyPath)
		if err != nil {
			return nil, err
		}

		// Load CA cert
		caCert, err := os.ReadFile(constant.CAPath)
		if err != nil {
			return nil, err
		}
		caCertPool := x509.NewCertPool()
		caCertPool.AppendCertsFromPEM(caCert)

		// Setup HTTPS client
		return &tls.Config{
			Certificates: []tls.Certificate{cert},
			MinVersion:   tls.VersionTLS13,
			RootCAs:      caCertPool,
		}, nil
	} else {
		return &tls.Config{
			InsecureSkipVerify: true,
		}, nil
	}
}

// GetHttpTransport returns an HTTP transport
func GetHttpTransport(enableTLS bool) (*http.Transport, error) {
	tlsConfig, err := GetHttpConfig(enableTLS)
	if err != nil {
		return nil, err
	}
	return &http.Transport{
		TLSClientConfig: tlsConfig,
	}, nil
}

// IsHttpsEnabled is https enabled
func IsHttpsEnabled() (bool, error) {
	if _, err := os.Stat(constant.TLSCertPath); err != nil {
		if os.IsNotExist(err) {
			zlog.Warnf("tls cert not exist %v, use http", err)
			return false, nil
		} else {
			zlog.Errorf("tls cert exist, but failed accessing file, %v", err)
			return false, err
		}
	}
	return true, nil
}
