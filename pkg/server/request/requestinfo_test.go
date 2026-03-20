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

package request

import (
	"net/http"
	"testing"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	k8srequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/client-go/kubernetes/fake"

	"console-service/pkg/constant"
)

var testRequests = []struct {
	reqAdr              string
	reqID               string
	reqMtd              string
	reqEptClt           string
	reqEptError         error
	reqEptResource      string
	reqEptVerb          string
	reqEptK8SReq        bool
	reqEptNamespace     string
	reqEptIsResourceReq bool
}{
	{
		reqAdr:              "/api/v1/nodes",
		reqID:               "list nodes",
		reqMtd:              http.MethodGet,
		reqEptVerb:          "GET",
		reqEptError:         nil,
		reqEptClt:           "",
		reqEptResource:      "",
		reqEptK8SReq:        true,
		reqEptIsResourceReq: false,
	},
	{
		reqAdr:              "/apis/rbac.authorization.k8s.io/v1/namespaces/namespace1/roles",
		reqID:               "list roles",
		reqEptError:         nil,
		reqMtd:              http.MethodGet,
		reqEptClt:           "",
		reqEptNamespace:     "",
		reqEptVerb:          "GET",
		reqEptResource:      "",
		reqEptK8SReq:        true,
		reqEptIsResourceReq: false,
	},
	{
		reqAdr:              "/foo/bar",
		reqID:               "random query",
		reqEptError:         nil,
		reqMtd:              http.MethodGet,
		reqEptResource:      "",
		reqEptVerb:          "GET",
		reqEptClt:           "",
		reqEptK8SReq:        false,
		reqEptIsResourceReq: false,
	},
}

func newTestRequestInfoResolver() RequestInfoResolver {
	var groupVersions = schema.GroupVersion{Group: "resources.fuyao.io", Version: "v1alpha1"}
	requestInfoResolver := &RequestInfoFactory{
		GlobalResources: []schema.GroupResource{
			groupVersions.WithResource(constant.ResourcesPluralCluster).GroupResource(),
		},
		RequestInfoFactory: &k8srequest.RequestInfoFactory{
			GrouplessAPIPrefixes: sets.NewString("api", "kapi"),
			APIPrefixes:          sets.NewString("api", "apis", "kapis", "kapi"),
		},
	}

	return requestInfoResolver
}

func TestRequestInfoFactoryNewRequestInfo(arg *testing.T) {
	requestInfoResolver := newTestRequestInfoResolver()

	for _, testReq := range testRequests {
		arg.Run(testReq.reqAdr, func(arg *testing.T) {
			genReq, genErr := http.NewRequest(testReq.reqMtd, testReq.reqAdr, nil)
			if genErr != nil {
				arg.Fatal(genErr)
			}
			clientMake := fake.NewSimpleClientset()
			genReqInfo, genErr := requestInfoResolver.NewRequestInfo(genReq, clientMake)

			if genErr == nil {
				if genReqInfo.IsResourceRequest != testReq.reqEptIsResourceReq {
					arg.Errorf("%s: expected is resource request %v, actual %+v", testReq.reqID,
						testReq.reqEptIsResourceReq, genReqInfo.IsResourceRequest)
				}
				if genReqInfo.IsK8sRequest != testReq.reqEptK8SReq {
					arg.Errorf("%s: expected kubernetes request %v, actual %+v", testReq.reqID,
						testReq.reqEptK8SReq, genReqInfo.IsK8sRequest)
				}
				if genReqInfo.ClusterName != testReq.reqEptClt {
					arg.Errorf("%s: expected cluster %v, actual %+v", testReq.reqID,
						testReq.reqEptClt, genReqInfo.ClusterName)
				}
				if testReq.reqEptVerb != genReqInfo.Verb {
					arg.Errorf("%s: expected verb %v, actual %+v", testReq.reqID,
						testReq.reqEptVerb, genReqInfo.RequestInfo.Verb)
				}
				if genReqInfo.Namespace != testReq.reqEptNamespace {
					arg.Errorf("%s: expected namespace %v, actual %+v", testReq.reqID,
						testReq.reqEptNamespace, genReqInfo.RequestInfo.Namespace)
				}
				if genReqInfo.Resource != testReq.reqEptResource {
					arg.Errorf("%s: expected resource %v, actual %+v", testReq.reqID,
						testReq.reqEptResource, genReqInfo.RequestInfo.Resource)
				}

			} else {
				if testReq.reqEptError != genErr {
					arg.Errorf("%s: expected error %v, actual %v", testReq.reqID,
						testReq.reqEptError, genErr)
				}
			}
		})

	}
}
