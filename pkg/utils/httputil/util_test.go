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
	"reflect"
	"testing"

	"github.com/agiledragon/gomonkey/v2"

	"console-service/pkg/constant"
)

func TestGetAllResponseJson(t *testing.T) {
	tests := []struct {
		name    string
		gotFunc func() *ResponseJson
		want    *ResponseJson
	}{
		{
			name:    "GetResponseJson",
			gotFunc: func() *ResponseJson { return GetResponseJson(constant.Success, "ok", "test") },
			want:    &ResponseJson{Code: constant.Success, Msg: "ok", Data: "test"},
		},
		{
			name:    "GetDefaultSuccessResponseJson",
			gotFunc: GetDefaultSuccessResponseJson,
			want:    &ResponseJson{Code: constant.Success, Msg: "success", Data: nil},
		},
		{
			name:    "GetDefaultClientFailureResponseJson",
			gotFunc: GetDefaultClientFailureResponseJson,
			want:    &ResponseJson{Code: constant.ClientError, Msg: "bad request", Data: nil},
		},
		{
			name:    "GetDefaultServerFailureResponseJson",
			gotFunc: GetDefaultServerFailureResponseJson,
			want:    &ResponseJson{Code: constant.ServerError, Msg: "remote server busy", Data: nil},
		},
		{
			name:    "GetParamsEmptyErrorResponseJson",
			gotFunc: GetParamsEmptyErrorResponseJson,
			want:    &ResponseJson{Code: constant.ClientError, Msg: "parameters not found", Data: nil},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.gotFunc(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("%s() = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}

func TestGetHttpConfigTLSFailed(t *testing.T) {
	tests := []struct {
		name      string
		enableTLS bool
		want      *tls.Config
		wantErr   bool
	}{
		{
			name:      "TestDisableTLS",
			enableTLS: false,
			want:      &tls.Config{InsecureSkipVerify: true},
			wantErr:   false,
		},
		{
			name:      "TestLoadCertFailed",
			enableTLS: true,
			want:      nil,
			wantErr:   true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetHttpConfig(tt.enableTLS)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetHttpConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetHttpConfig() got = %v, want %v", got, tt.want)
			}
		})
	}

	patch1 := gomonkey.ApplyFunc(tls.LoadX509KeyPair, func(certFile, keyFile string) (tls.Certificate, error) {
		return tls.Certificate{}, nil
	})
	defer patch1.Reset()

	t.Run("TestLoadCAFailed", func(t *testing.T) {
		got, err := GetHttpConfig(true)
		if err == nil {
			t.Errorf("GetHttpConfig() error = %v, wantErr true", err)
		}
		if got != nil {
			t.Errorf("GetHttpConfig() got = %v, want nil", got)
		}
	})
}

func TestGetHttpConfigSuccess(t *testing.T) {
	patch1 := gomonkey.ApplyFunc(tls.LoadX509KeyPair, func(certFile, keyFile string) (tls.Certificate, error) {
		return tls.Certificate{}, nil
	})
	defer patch1.Reset()

	fakeCABytes := []byte("-----BEGIN CERTIFICATE-----\nxxxxxxxx\n-----END CERTIFICATE-----")
	fakeCAPool := x509.NewCertPool()
	fakeCAPool.AppendCertsFromPEM(fakeCABytes)

	patch2 := gomonkey.ApplyFunc(os.ReadFile, func(filename string) ([]byte, error) {
		return fakeCABytes, nil
	})
	defer patch2.Reset()

	got, err := GetHttpConfig(true)
	want := &tls.Config{
		Certificates: []tls.Certificate{{}},
		MinVersion:   tls.VersionTLS13,
		RootCAs:      fakeCAPool,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("TLS Config is incorrect: %v, want %v", got, want)
		return
	}
	if err != nil {
		t.Errorf("error = %v, wantErr false", err)
		return
	}
}

func TestGetHttpTransport(t *testing.T) {
	type args struct {
		enableTLS bool
	}
	tests := []struct {
		name    string
		args    args
		want    *http.Transport
		wantErr bool
	}{
		{
			name: "GetHTTPConfigFailed",
			args: args{
				enableTLS: true,
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "GetHTTPConfigSucceeded",
			args: args{
				enableTLS: false,
			},
			want: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetHttpTransport(tt.args.enableTLS)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetHttpTransport() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetHttpTransport() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsHttpsEnabled(t *testing.T) {
	patch := gomonkey.ApplyFunc(os.Stat, func() (os.FileInfo, error) {
		return nil, os.ErrNotExist
	})
	defer patch.Reset()
	t.Run("PathNotExist", func(t *testing.T) {
		got, err := IsHttpsEnabled()
		if err != nil {
			t.Errorf("IsHttpsEnabled error = %v, want nil", err)
		}
		if got {
			t.Error("IsHttpsEnabled got = true, want false")
		}
	})

	patch = gomonkey.ApplyFunc(os.Stat, func() (os.FileInfo, error) {
		return nil, os.ErrPermission
	})
	t.Run("PathNotExist", func(t *testing.T) {
		got, err := IsHttpsEnabled()
		if err == nil {
			t.Error("IsHttpsEnabled error = nil, want not nil")
		}
		if got {
			t.Error("IsHttpsEnabled got = true, want false")
		}
	})

	patch = gomonkey.ApplyFunc(os.Stat, func() (os.FileInfo, error) {
		return nil, nil
	})
	t.Run("PathNotExist", func(t *testing.T) {
		got, err := IsHttpsEnabled()
		if err != nil {
			t.Errorf("IsHttpsEnabled error = %v, want nil", err)
		}
		if !got {
			t.Errorf("IsHttpsEnabled got = %v, want true", got)
		}
	})
}
