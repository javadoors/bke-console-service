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

package iputil

import (
	"net"
	"net/http"
	"strings"

	"console-service/pkg/zlog"
)

// RemoteIp Tries to extract the client IP from provided headers
func RemoteIp(req *http.Request) string {
	address := req.RemoteAddr
	if ip := req.Header.Get("x-client-ip"); ip != "" {
		// extract the XClientIp
		address = ip
	} else if ip := req.Header.Get("X-Real-IP"); ip != "" {
		// extract the XRealIp
		address = ip
	} else if ip = req.Header.Get("X-Forwarded-For"); ip != "" {
		// extract the XForwardedForIP
		xffIps := strings.Split(ip, ",")
		if len(xffIps) > 0 {
			address = strings.TrimSpace(xffIps[0])
		}
	} else {
		// extract another IP
		var err error
		address, _, err = net.SplitHostPort(address)
		if err != nil {
			zlog.Errorf("Unable to get remoteAddr: %v", err)
			return address
		}
	}

	// 不管请求是通过IPv4还是IPv6发送到本地环回接口，remoteAddr都将被设置为IPv4格式
	if address == "::1" {
		address = "127.0.0.1"
	}
	return address
}
