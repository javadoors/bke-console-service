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

package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/emicklei/go-restful/v3"
	"golang.org/x/oauth2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
)

const oauth2RedirectURL = "https://example.com/oauth2/oauth/authorize"

// oAuth2Mock mocks oauth2.config for test
type oAuth2Mock struct{}

func (o *oAuth2Mock) AuthCodeURL(state string, opts ...oauth2.AuthCodeOption) string {
	return oauth2RedirectURL
}

// Exchange takes the code and returns a token.
func (o *oAuth2Mock) Exchange(ctx context.Context, code string, opts ...oauth2.AuthCodeOption) (*oauth2.Token, error) {
	if code == "error-auth-code" {
		return nil, errors.New("auth code error")
	}
	return &oauth2.Token{
		AccessToken: "token-for-123456789",
		Expiry:      time.Unix(1, 1),
	}, nil
}

type mockTokenSource struct {
	t *oauth2.Token
}

func (s *mockTokenSource) Token() (*oauth2.Token, error) {
	return s.t, nil
}

func (o *oAuth2Mock) TokenSource(ctx context.Context, t *oauth2.Token) oauth2.TokenSource {
	return &mockTokenSource{
		t: t,
	}
}

func newFakeClientSet() *fake.Clientset {
	return fake.NewSimpleClientset(
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "session-123456789",
				Namespace: "session-secret",
			},
			Data: map[string][]byte{
				"Expiry":      []byte("0"),
				"SessionID":   []byte("123456789"),
				"AccessToken": []byte("token-for-123456789"),
			},
			Type: corev1.SecretTypeOpaque,
		},
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "session-777777777",
				Namespace: "session-secret",
			},
			Data: map[string][]byte{
				"Expiry":      []byte(strconv.FormatInt(time.Now().Unix()+100, intBase10)),
				"SessionID":   []byte("777777777"),
				"AccessToken": []byte("token-for-777777777"),
			},
			Type: corev1.SecretTypeOpaque,
		},
	)
}

func TestNewHandler(t *testing.T) {
	err := os.Setenv(clientIDEnv, "client-id")
	if err != nil {
		return
	}
	err = os.Setenv(clientSecretEnv, "client-secret")
	if err != nil {
		return
	}
	defer os.Unsetenv(clientIDEnv)
	defer os.Unsetenv(clientSecretEnv)

	_, err = NewHandler(&rest.Config{})
	if err != nil {
		t.Errorf("NewHandler() should return nil error, got %v", err)
		return
	}

}

func newTestAuthHandler() Handler {
	return Handler{
		kubeConfig:  &rest.Config{},
		clientset:   newFakeClientSet(),
		oauthConfig: &oAuth2Mock{},
	}
}

// login tests
func TestHandlerLoginHandler(t *testing.T) {

	h := newTestAuthHandler()

	t.Run("TestWithValidSession", func(t *testing.T) {
		req := restful.NewRequest(
			httptest.NewRequest("GET", "http://example.com/rest/auth/login", nil),
		)
		req.Request.AddCookie(&http.Cookie{
			Name:  "sessionID_bke",
			Value: "777777777",
		})
		recorder := httptest.NewRecorder()
		resp := restful.NewResponse(recorder)

		h.loginHandler(req, resp)

		if recorder.Code != http.StatusFound {
			t.Errorf("loginHandler() code = %v, want %v", recorder.Code, http.StatusFound)
		}
		if val := recorder.Header().Get("Set-Cookie"); val == "" {
			t.Errorf("loginHandler() should not set cookie [state]: %s", val)
		}
		wantLocation := "https://example.com/oauth2/oauth/authorize"
		if val := recorder.Header().Get("Location"); val != wantLocation {
			t.Errorf("loginHandler() redirect to wrong location: %s, should be %s", val, wantLocation)
		}
	})

	t.Run("TestWithInvalidSession", func(t *testing.T) {
		req := restful.NewRequest(
			httptest.NewRequest("GET", "http://example.com/rest/auth/login", nil),
		)
		req.Request.AddCookie(&http.Cookie{
			Name:  "sessionID_bke",
			Value: "invalid-sessionID",
		})
		recorder := httptest.NewRecorder()
		resp := restful.NewResponse(recorder)

		h.loginHandler(req, resp)

		if recorder.Code != http.StatusFound {
			t.Errorf("loginHandler() code = %v, want %v", recorder.Code, http.StatusFound)
		}
		if val := recorder.Header().Get("Set-Cookie"); val == "" {
			t.Errorf("loginHandler() failed to set cookie")
		}
		wantLocation := oauth2RedirectURL
		if val := recorder.Header().Get("Location"); !strings.HasPrefix(val, wantLocation) {
			t.Errorf("loginHandler() redirect to wrong location: %s, should start with %s", val, wantLocation)
		}
	})

	t.Run("TestLogin", func(t *testing.T) {
		req := restful.NewRequest(
			httptest.NewRequest("GET", "http://example.com/rest/auth/login", nil),
		)
		recorder := httptest.NewRecorder()
		resp := restful.NewResponse(recorder)

		h.loginHandler(req, resp)

		if recorder.Code != http.StatusFound {
			t.Errorf("loginHandler() code = %v, want %v", recorder.Code, http.StatusFound)
		}
		if val := recorder.Header().Get("Set-Cookie"); !strings.HasPrefix(val, "state=") {
			t.Errorf("loginHandler() failed to set cookie")
		}
		wantLocation := oauth2RedirectURL
		if val := recorder.Header().Get("Location"); !strings.HasPrefix(val, wantLocation) {
			t.Errorf("loginHandler() redirect to wrong location: %s, should start with %s", val, wantLocation)
		}
	})
}

func TestHandlerCheckSessionCookie(t *testing.T) {
	tests := []struct {
		name    string
		handler Handler
		cookie  *http.Cookie
		want    bool
	}{
		{
			"TestNonExistingSessionID",
			newTestAuthHandler(),
			&http.Cookie{
				Name:  "sessionID_bke",
				Value: "non-existing-sessionID",
			},
			false,
		},
		{
			"TestExistingSessionID",
			newTestAuthHandler(),
			&http.Cookie{
				Name:  "sessionID_bke",
				Value: "777777777",
			},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &tt.handler
			if got := h.checkSessionCookie(tt.cookie); got != tt.want {
				t.Errorf("checkSessionCookie() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCreateLoginState(t *testing.T) {
	const lens = 2
	tests := []struct {
		name    string
		length  int
		wantErr bool
	}{
		{
			"TestLoginStateLength",
			1,
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := createLoginState(tt.length)
			if (err != nil) != tt.wantErr {
				t.Errorf("createLoginState() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			// login state is random, only checking length, StringLength = 2*ByteLength
			if len(got) != lens*tt.length {
				t.Errorf("createLoginState() got = %v, want length %v", got, lens*tt.length)
			}
		})
	}
}

// callback tests
func TestHandlerCallbackHandler(t *testing.T) {
	handler := newTestAuthHandler()
	generateMockRequest := func(method string, target string) *restful.Request {
		return restful.NewRequest(
			httptest.NewRequest(method, target, nil),
		)
	}
	t.Run("TestWithError", func(t *testing.T) {
		req := generateMockRequest("GET",
			"http://example.com/rest/auth/callback?error=TEST_ERROR&error_description=This%20is%20a%20test%20error")
		recorder := httptest.NewRecorder()
		resp := restful.NewResponse(recorder)

		handler.callbackHandler(req, resp)

		if recorder.Code != http.StatusOK {
			t.Errorf("loginHandler() code = %v, want %v", recorder.Code, http.StatusFound)
		}
		got := removeEmptyCharHTML(recorder.Body.String())
		want := removeEmptyCharHTML(fmt.Sprintf(responseTemplate, "TEST_ERROR", "This is a test error"))
		if got != want {
			t.Errorf("callbackHandler() got = %v, want %v", got, want)
		}
	})

	t.Run("TestParseQueryFailure", func(t *testing.T) {
		req := generateMockRequest("GET",
			"http://example.com/rest/auth/callback?state=123456&other=values")
		recorder := httptest.NewRecorder()
		resp := restful.NewResponse(recorder)

		handler.callbackHandler(req, resp)

		if recorder.Code != http.StatusUnauthorized {
			t.Errorf("callbackHandler() code = %v, want %v", recorder.Code, http.StatusFound)
		}
	})

	t.Run("TestCheckLoginStateFailure", func(t *testing.T) {
		req := generateMockRequest("GET",
			"http://example.com/rest/auth/callback?state=123456&code=acdefg")
		recorder := httptest.NewRecorder()
		resp := restful.NewResponse(recorder)

		handler.callbackHandler(req, resp)

		if recorder.Code != http.StatusSeeOther {
			t.Errorf("loginHandler() code = %v, want %v", recorder.Code, http.StatusOK)
		}
	})

	t.Run("TestSuccessfulCallback", func(t *testing.T) {
		req := generateMockRequest("GET",
			"http://example.com/rest/auth/callback?state=123456&code=acdefg")
		req.Request.AddCookie(&http.Cookie{
			Name:  "state",
			Value: "123456",
		})
		recorder := httptest.NewRecorder()
		resp := restful.NewResponse(recorder)

		handler.callbackHandler(req, resp)

	})
}

func TestGenerateErrorRespText(t *testing.T) {
	tests := []struct {
		name    string
		req     *restful.Request
		want    string
		wantErr bool
	}{
		{
			"TestNoError",
			&restful.Request{
				Request: &http.Request{
					URL: &url.URL{
						RawQuery: "code=123&state=456",
					},
				},
			},
			"",
			false,
		},
		{
			"TestWithErrorCode",
			&restful.Request{
				Request: &http.Request{
					URL: &url.URL{
						RawQuery: "error=TEST_ERROR&other=value",
					},
				},
			},
			fmt.Sprintf(responseTemplate, "TEST_ERROR", ""),
			true,
		},
		{
			"TestWithErrorDesc",
			&restful.Request{
				Request: &http.Request{
					URL: &url.URL{
						RawQuery: "error_description=This is a test error&other=value",
					},
				},
			},
			fmt.Sprintf(responseTemplate, "", "This is a test error"),
			true,
		},
		{
			"TestWithErrorCodeAndDesc",
			&restful.Request{
				Request: &http.Request{
					URL: &url.URL{
						RawQuery: "error=TEST_ERROR&error_description=This is a test error&other=value",
					},
				},
			},
			fmt.Sprintf(responseTemplate, "TEST_ERROR", "This is a test error"),
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := generateErrorRespText(tt.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("generateErrorRespText() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			// remove \n & \t characters, which don't matter in html
			got = removeEmptyCharHTML(got)
			tt.want = removeEmptyCharHTML(tt.want)
			if got != tt.want {
				t.Errorf("generateErrorRespText() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseCallbackQuery(t *testing.T) {
	tests := []struct {
		name           string
		req            *restful.Request
		wantAuthCode   string
		wantLoginState string
		wantErr        bool
	}{
		{
			"TesNoAuthCode",
			restful.NewRequest(
				httptest.NewRequest("GET", "http://example.com/callback?state=123456", nil),
			),
			"",
			"",
			true,
		},
		{
			"TestNoAuthCode",
			restful.NewRequest(
				httptest.NewRequest(
					"GET",
					"http://example.com/callback?code=qwerty",
					nil,
				),
			),
			"",
			"",
			true,
		},
		{
			"TestCorrectQuery",
			restful.NewRequest(
				httptest.NewRequest(
					"GET",
					"http://example.com/callback?state=123456&code=qwerty",
					nil,
				),
			),
			"qwerty",
			"123456",
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			authCode, loginState, err := parseCallbackQuery(tt.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseCallbackQuery() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if authCode != tt.wantAuthCode {
				t.Errorf("parseCallbackQuery() authCode = %v, want %v", authCode, tt.wantAuthCode)
			}
			if loginState != tt.wantLoginState {
				t.Errorf("parseCallbackQuery() loginState = %v, want %v", loginState, tt.wantLoginState)
			}
		})
	}
}

func TestCheckLoginState(t *testing.T) {
	tests := []struct {
		name       string
		req        *restful.Request
		loginState string
		wantErr    bool
	}{
		{
			"TestNoCookie",
			&restful.Request{
				Request: &http.Request{},
			},
			"123456",
			true,
		},
		{
			"TestWrongCookie",
			&restful.Request{
				Request: &http.Request{
					Header: http.Header{
						"Cookie": []string{"state=654321"},
					},
				},
			},
			"123456",
			true,
		},
		{
			"TestRightCookie",
			&restful.Request{
				Request: &http.Request{
					Header: http.Header{
						"Cookie": []string{"state=123456"},
					},
				},
			},
			"123456",
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := checkLoginState(tt.req, tt.loginState); (err != nil) != tt.wantErr {
				t.Errorf("checkLoginState() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestHandlerExchangeCodeForToken(t *testing.T) {
	tests := []struct {
		name     string
		handler  Handler
		authCode string
		want     string
		want1    time.Time
		wantErr  bool
	}{
		{
			"TestExchangeFailure",
			newTestAuthHandler(),
			"error-auth-code",
			"",
			time.Time{},
			true,
		},
		{
			"TestExchangeSuccess",
			newTestAuthHandler(),
			"",
			"token-for-123456789",
			time.Unix(1, 1),
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &tt.handler
			got, err := h.exchangeCodeForToken(tt.authCode)
			if (err != nil) != tt.wantErr {
				t.Errorf("exchangeCodeForToken() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if (err != nil) && got == nil {
				return
			}
			if got.AccessToken != tt.want {
				t.Errorf("exchangeCodeForToken() got = %v, want %v", got.AccessToken, tt.want)
			}
			if !reflect.DeepEqual(got.AccessTokenExpiry, tt.want1) {
				t.Errorf("exchangeCodeForToken() got1 = %v, want %v", got.AccessTokenExpiry, tt.want1)
			}
		})
	}
}

// logout tests
func TestHandlerLogoutCore(t *testing.T) {
	handler := newTestAuthHandler()

	t.Run("TestLogoutWithoutSession", func(t *testing.T) {
		req := httptest.NewRequest(
			"POST",
			"http://example.com/rest/auth/logout",
			nil,
		)
		recorder := httptest.NewRecorder()
		resp := restful.NewResponse(recorder)

		handler.LogoutCore(req, resp)

		if recorder.Code != http.StatusNoContent {
			t.Errorf("logoutCore()() code = %v, want %v", recorder.Code, http.StatusNoContent)
		}
	})

	t.Run("TestLogoutWithSession", func(t *testing.T) {
		req := httptest.NewRequest(
			"POST",
			"http://example.com/rest/auth/logout",
			nil,
		)
		req.AddCookie(&http.Cookie{
			Name:  "sessionID_bke",
			Value: "123456789",
		})
		recorder := httptest.NewRecorder()
		resp := restful.NewResponse(recorder)

		handler.LogoutCore(req, resp)

		if recorder.Code != http.StatusOK {
			t.Errorf("logoutCore() code = %v, want %v", recorder.Code, http.StatusOK)
		}
		if val := recorder.Header().Get("Set-Cookie"); val == "" {
			t.Errorf("loginHandler() failed to set cookie")
		}
		wantLocation := ""
		if val := recorder.Header().Get("Location"); val != wantLocation {
			t.Errorf("loginHandler() redirect to wrong location: %s, should be %s", val, wantLocation)
		}
	})

}

// helpers
func removeEmptyCharHTML(str string) string {
	str = strings.ReplaceAll(str, "\n", "")
	str = strings.ReplaceAll(str, "\t", "")
	return str
}
