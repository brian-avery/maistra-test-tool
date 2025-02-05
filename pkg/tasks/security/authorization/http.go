// Copyright 2021 Red Hat, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package authorizaton

import (
	"fmt"
	"io/ioutil"
	"strings"
	"testing"
	"time"

	"github.com/maistra/maistra-test-tool/pkg/examples"
	"github.com/maistra/maistra-test-tool/pkg/util"
)

func cleanupAuthorHTTP() {
	util.Log.Info("Cleanup")
	util.KubeDeleteContents("bookinfo", DetailsGETPolicy)
	util.KubeDeleteContents("bookinfo", ReviewsGETPolicy)
	util.KubeDeleteContents("bookinfo", RatingsGETPolicy)
	util.KubeDeleteContents("bookinfo", ProductpageGETPolicy)
	util.KubeDeleteContents("bookinfo", DenyAllPolicy)
	time.Sleep(time.Duration(20) * time.Second)
	bookinfo := examples.Bookinfo{"bookinfo"}
	bookinfo.Uninstall()
	util.Shell(`kubectl patch -n istio-system smcp/basic --type merge -p '{"spec":{"security":{"dataPlane":{"mtls":false},"controlPlane":{"mtls":false}}}}'`)
	util.Shell(`oc -n istio-system wait --for condition=Ready smcp/basic --timeout 180s`)
	time.Sleep(time.Duration(20) * time.Second)
}

func TestAuthorHTTP(t *testing.T) {
	defer cleanupAuthorHTTP()
	defer util.RecoverPanic(t)

	util.Log.Info("Authorization for HTTP traffic")
	util.Log.Info("Enable Control Plane MTLS")
	util.Shell(`kubectl patch -n istio-system smcp/basic --type merge -p '{"spec":{"security":{"dataPlane":{"mtls":true},"controlPlane":{"mtls":true}}}}'`)
	util.Shell(`oc -n istio-system wait --for condition=Ready smcp/basic --timeout 180s`)

	bookinfo := examples.Bookinfo{"bookinfo"}
	bookinfo.Install(true)
	productpageURL := fmt.Sprintf("http://%s/productpage", gatewayHTTP)

	t.Run("Security_authorization_rbac_deny_all_http", func(t *testing.T) {
		defer util.RecoverPanic(t)

		util.Log.Info("Configure access control for workloads using HTTP traffic")
		util.KubeApplyContents("bookinfo", DenyAllPolicy)
		time.Sleep(time.Duration(10) * time.Second)

		resp, _, err := util.GetHTTPResponse(productpageURL, nil)
		util.Inspect(err, "Failed to get HTTP Response", "", t)
		body, err := ioutil.ReadAll(resp.Body)
		util.Inspect(err, "Failed to read response body", "", t)
		if strings.Contains(string(body), "RBAC: access denied") {
			util.Log.Infof("Got access denied as expected: %s", string(body))
		} else {
			t.Errorf("RBAC deny all failed. Got response: %s", string(body))
			util.Log.Errorf("RBAC deny all failed. Got response: %s", string(body))
		}
		util.CloseResponseBody(resp)
	})

	t.Run("Security_authorization_rbac_allow_GET_http", func(t *testing.T) {
		defer util.RecoverPanic(t)

		util.Log.Info("Allow access with GET method to the productpage workload")
		util.KubeApplyContents("bookinfo", ProductpageGETPolicy)
		time.Sleep(time.Duration(10) * time.Second)

		util.GetHTTPResponse(productpageURL, nil) // dummy request to refresh previous page
		resp, _, err := util.GetHTTPResponse(productpageURL, nil)
		util.Inspect(err, "Failed to get HTTP Response", "", t)
		body, err := ioutil.ReadAll(resp.Body)
		util.Inspect(err, "Failed to read response body", "", t)
		if strings.Contains(string(body), "Error fetching product details") && strings.Contains(string(body), "Error fetching product reviews") {
			util.Log.Infof("Got expected page with Error fetching product details and Error fetching product reviews")
		} else {
			t.Errorf("Productpage GET policy failed. Got response: %s", string(body))
			util.Log.Errorf("Productpage GET policy failed. Got response: %s", string(body))
		}
		util.CloseResponseBody(resp)

		util.Log.Info("Allow other bookinfo services GET method")
		util.KubeApplyContents("bookinfo", DetailsGETPolicy)
		util.KubeApplyContents("bookinfo", ReviewsGETPolicy)
		util.KubeApplyContents("bookinfo", RatingsGETPolicy)
		time.Sleep(time.Duration(50) * time.Second)

		util.GetHTTPResponse(productpageURL, nil) // dummy request to refresh previous page
		resp, _, err = util.GetHTTPResponse(productpageURL, nil)
		util.Inspect(err, "Failed to get HTTP Response", "", t)

		body, err = ioutil.ReadAll(resp.Body)
		util.Inspect(err, "Failed to read response body", "", t)
		if strings.Contains(string(body), "Error fetching product details") || strings.Contains(string(body), "Error fetching product reviews") || strings.Contains(string(body), "Ratings service currently unavailable") {
			t.Errorf("GET policy failed. Got response: %s", string(body))
			util.Log.Errorf("GET policy failed. Got response: %s", string(body))
		} else {
			util.Log.Infof("Got expected page.")
		}
		util.CloseResponseBody(resp)
	})
}
