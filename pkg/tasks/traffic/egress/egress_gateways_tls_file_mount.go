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

package egress

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/maistra/maistra-test-tool/pkg/examples"
	"github.com/maistra/maistra-test-tool/pkg/util"
)

var (
	gatewayPatchAdd = `
[{
"op": "add",
"path": "/spec/template/spec/containers/0/volumeMounts/0",
"value": {
"mountPath": "/etc/istio/nginx-client-certs",
"name": "nginx-client-certs",
"readOnly": true
}
},
{
"op": "add",
"path": "/spec/template/spec/volumes/0",
"value": {
"name": "nginx-client-certs",
"secret": {
"secretName": "nginx-client-certs",
"optional": true
}
}
},
{
"op": "add",
"path": "/spec/template/spec/containers/0/volumeMounts/1",
"value": {
"mountPath": "/etc/istio/nginx-ca-certs",
"name": "nginx-ca-certs",
"readOnly": true
}
},
{
"op": "add",
"path": "/spec/template/spec/volumes/1",
"value": {
"name": "nginx-ca-certs",
"secret": {
"secretName": "nginx-ca-certs",
"optional": true
}
}
}]
`
)

func cleanupTLSOriginationFileMount() {
	util.Log.Info("Cleanup")
	sleep := examples.Sleep{"bookinfo"}
	nginx := examples.Nginx{"bookinfo"}
	util.KubeDeleteContents("istio-system", nginxMeshRule)
	util.KubeDeleteContents("bookinfo", nginxGatewayTLS)

	util.Shell(`kubectl -n %s rollout undo deploy istio-egressgateway`, "istio-system")
	time.Sleep(time.Duration(20) * time.Second)
	util.Shell(`oc wait --for condition=Ready -n %s smmr/default --timeout 180s`, "istio-system")
	util.Shell(`kubectl -n %s rollout history deploy istio-egressgateway`, "istio-system")

	util.Shell(`kubectl delete -n %s secret nginx-client-certs`, "istio-system")
	util.Shell(`kubectl delete -n %s secret nginx-ca-certs`, "istio-system")
	util.KubeDeleteContents("bookinfo", cnnextGatewayTLSFile)
	util.KubeDeleteContents("bookinfo", cnnextServiceEntry)
	nginx.Uninstall()
	sleep.Uninstall()
	time.Sleep(time.Duration(20) * time.Second)
}

func TestTLSOriginationFileMount(t *testing.T) {
	defer cleanupTLSOriginationFileMount()
	defer util.RecoverPanic(t)

	util.Log.Info("TestEgressGatewaysTLSOrigination File Mount")
	sleep := examples.Sleep{"bookinfo"}
	sleep.Install()
	sleepPod, err := util.GetPodName("bookinfo", "app=sleep")
	util.Inspect(err, "Failed to get sleep pod name", "", t)

	nginx := examples.Nginx{"bookinfo"}
	nginx.Install("../testdata/examples/x86/nginx/nginx_ssl.conf")

	t.Run("TrafficManagement_egress_gateway_perform_TLS_origination", func(t *testing.T) {
		defer util.RecoverPanic(t)

		util.Log.Info("Perform TLS origination with an egress gateway")
		util.KubeApplyContents("bookinfo", cnnextServiceEntry)
		time.Sleep(time.Duration(10) * time.Second)

		command := `curl -sSL -o /dev/null -D - http://edition.cnn.com/politics`
		msg, err := util.PodExec("bookinfo", sleepPod, "sleep", command, false)
		util.Inspect(err, "Failed to get response", "", t)
		if strings.Contains(msg, "301 Moved Permanently") {
			util.Log.Info("Success. Get http://edition.cnn.com/politics response")
		} else {
			util.Log.Infof("Error response: %s", msg)
			t.Errorf("Error response: %s", msg)
		}

		util.Log.Info("Create a Gateway to external edition.cnn.com")
		util.KubeApplyContents("bookinfo", cnnextGatewayTLSFile)
		time.Sleep(time.Duration(20) * time.Second)

		command = `curl -sSL -o /dev/null -D - http://edition.cnn.com/politics`
		msg, err = util.PodExec("bookinfo", sleepPod, "sleep", command, false)
		util.Inspect(err, "Failed to get response", "", t)
		if strings.Contains(msg, "301 Moved Permanently") || !strings.Contains(msg, "200") {
			util.Log.Infof("Error response: %s", msg)
			t.Errorf("Error response: %s", msg)
		} else {
			util.Log.Infof("Success. Get http://edition.cnn.com/politics response")
		}

		util.Log.Info("Cleanup the TLS origination example")
		util.KubeDeleteContents("bookinfo", cnnextGatewayTLSFile)
		util.KubeDeleteContents("bookinfo", cnnextServiceEntry)
		time.Sleep(time.Duration(20) * time.Second)
	})

	t.Run("TrafficManagement_egress_gateway_perform_MTLS_origination", func(t *testing.T) {
		defer util.RecoverPanic(t)

		util.Log.Info("Redeploy the egress gateway with the client certs")
		util.Shell(`kubectl create -n %s secret tls nginx-client-certs --key %s --cert %s`, "istio-system", nginxClientCertKey, nginxClientCert)
		util.Shell(`kubectl create -n %s secret generic nginx-ca-certs --from-file=%s`, "istio-system", nginxServerCACert)

		util.Log.Info("Patch egress gateway")
		util.Shell(`kubectl -n %s rollout history deploy istio-egressgateway`, "istio-system")
		util.Shell(`kubectl -n %s patch --type=json deploy istio-egressgateway -p='%s'`, "istio-system", strings.ReplaceAll(gatewayPatchAdd, "\n", ""))
		time.Sleep(time.Duration(20) * time.Second)
		util.Shell(`oc wait --for condition=Ready -n %s smmr/default --timeout 180s`, "istio-system")
		util.Log.Info("Verify the istio-egressgateway pod")
		util.Shell(`kubectl exec -n %s "$(kubectl -n %s get pods -l %s -o jsonpath='{.items[0].metadata.name}')" -- ls -al %s %s`,
			"istio-system", "istio-system",
			"istio=egressgateway",
			"/etc/istio/nginx-client-certs",
			"/etc/istio/nginx-ca-certs")
		util.Shell(`kubectl -n %s rollout history deploy istio-egressgateway`, "istio-system")

		util.Log.Info("Configure MTLS origination for egress traffic")
		util.KubeApplyContents("bookinfo", nginxGatewayTLS)
		time.Sleep(time.Duration(20) * time.Second)
		util.KubeApplyContents("istio-system", nginxMeshRule)
		time.Sleep(time.Duration(10) * time.Second)

		util.Log.Info("Verify NGINX server")
		cmd := fmt.Sprintf(`curl -sS http://my-nginx.bookinfo.svc.cluster.local`)
		msg, err := util.PodExec("bookinfo", sleepPod, "sleep", cmd, true)
		util.Inspect(err, "failed to get response", "", t)
		if !strings.Contains(msg, "Welcome to nginx") {
			t.Errorf("Expected Welcome to nginx; Got unexpected response: %s", msg)
			util.Log.Errorf("Expected Welcome to nginx; Got unexpected response: %s", msg)
		} else {
			util.Log.Infof("Success. Get expected response: %s", msg)
		}
	})
}
