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
	"strings"
	"testing"
	"time"

	"github.com/maistra/maistra-test-tool/pkg/examples"
	"github.com/maistra/maistra-test-tool/pkg/util"
)

func cleanupEgressGateways() {
	util.Log.Info("Cleanup")
	sleep := examples.Sleep{"bookinfo"}
	util.KubeDeleteContents("bookinfo", cnnextGatewayHTTPS)
	util.KubeDeleteContents("bookinfo", cnnextGateway)
	util.KubeDeleteContents("bookinfo", cnnextServiceEntryTLS)
	util.KubeDeleteContents("bookinfo", cnnextServiceEntry)
	sleep.Uninstall()
	time.Sleep(time.Duration(20) * time.Second)
}

func TestEgressGateways(t *testing.T) {
	defer cleanupEgressGateways()
	defer util.RecoverPanic(t)

	util.Log.Info("TestEgressGateways")
	sleep := examples.Sleep{"bookinfo"}
	sleep.Install()
	sleepPod, err := util.GetPodName("bookinfo", "app=sleep")
	util.Inspect(err, "Failed to get sleep pod name", "", t)

	t.Run("TrafficManagement_egress_gateway_for_http_traffic", func(t *testing.T) {
		defer util.RecoverPanic(t)

		util.Log.Info("Create a ServiceEntry to external edition.cnn.com")
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
		util.KubeApplyContents("bookinfo", cnnextGateway)
		time.Sleep(time.Duration(20) * time.Second)

		command = `curl -sSL -o /dev/null -D - http://edition.cnn.com/politics`
		msg, err = util.PodExec("bookinfo", sleepPod, "sleep", command, false)
		util.Inspect(err, "Failed to get response", "", t)
		if strings.Contains(msg, "301 Moved Permanently") {
			util.Log.Infof("Success. Get http://edition.cnn.com/politics response: %s", msg)
		} else {
			util.Log.Infof("Error response: %s", msg)
			t.Errorf("Error response: %s", msg)
		}

		util.KubeDeleteContents("bookinfo", cnnextGateway)
		util.KubeDeleteContents("bookinfo", cnnextServiceEntry)
		time.Sleep(time.Duration(20) * time.Second)
	})

	t.Run("TrafficManagement_egress_gateway_for_https_traffic", func(t *testing.T) {
		defer util.RecoverPanic(t)

		util.Log.Info("Create a TLS ServiceEntry to external edition.cnn.com")
		util.KubeApplyContents("bookinfo", cnnextServiceEntryTLS)
		time.Sleep(time.Duration(10) * time.Second)

		command := `curl -sSL -o /dev/null -D - https://edition.cnn.com/politics`
		msg, err := util.PodExec("bookinfo", sleepPod, "sleep", command, false)
		util.Inspect(err, "Failed to get response", "", t)
		if strings.Contains(msg, "301 Moved Permanently") || !strings.Contains(msg, "200") {
			util.Log.Infof("Error response: %s", msg)
			t.Errorf("Error response: %s", msg)
		} else {
			util.Log.Infof("Success. Get https://edition.cnn.com/politics response: %s", msg)
		}

		util.Log.Info("Create a https Gateway to external edition.cnn.com")
		util.KubeApplyContents("bookinfo", cnnextGatewayHTTPS)
		time.Sleep(time.Duration(20) * time.Second)

		command = `curl -sSL -o /dev/null -D - https://edition.cnn.com/politics`
		msg, err = util.PodExec("bookinfo", sleepPod, "sleep", command, false)
		util.Inspect(err, "Failed to get response", "", t)
		if strings.Contains(msg, "301 Moved Permanently") || !strings.Contains(msg, "200") {
			util.Log.Infof("Error response: %s", msg)
			t.Errorf("Error response: %s", msg)
		} else {
			util.Log.Infof("Success. Get https://edition.cnn.com/politics response: %s", msg)
		}
	})
}
