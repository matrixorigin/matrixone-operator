// Copyright 2024 Matrix Origin
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package util

import (
	"fmt"
	"github.com/go-errors/errors"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
	"log"
	"net/http"
	"os"
	"time"
)

type PortForwardHandler struct {
	stopCh  chan struct{}
	readyCh <-chan struct{}
	errCh   <-chan error
}

func (h *PortForwardHandler) Ready(timeout time.Duration) error {
	select {
	case <-h.readyCh:
		return nil
	case err := <-h.errCh:
		return err
	case <-time.After(timeout):
		return errors.New("wait port-forward ready timeout")
	}
}

func (h *PortForwardHandler) Stop() {
	close(h.stopCh)
}

func PortForward(config *rest.Config, ns, podName string, localPort, remotePort int) (*PortForwardHandler, error) {
	stopCh := make(chan struct{}, 1)
	readyCh := make(chan struct{})
	errCh := make(chan error)
	cli, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	url := cli.CoreV1().RESTClient().Post().
		Resource("pods").
		Namespace(ns).
		Name(podName).
		SubResource("portforward").URL()
	transport, upgrader, err := spdy.RoundTripperFor(config)
	if err != nil {
		return nil, errors.WrapPrefix(err, "Could not create round tripper", 0)
	}
	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, "POST", url)

	fw, err := portforward.New(dialer, []string{fmt.Sprintf("%d:%d", localPort, remotePort)}, stopCh, readyCh, os.Stdout, os.Stderr)
	if err != nil {
		return nil, err
	}
	go func() {
		err = fw.ForwardPorts()
		if err != nil {
			log.Printf("portforward encounters error: %v\n", err)
			errCh <- err
		}
	}()

	return &PortForwardHandler{stopCh: stopCh, readyCh: readyCh, errCh: errCh}, nil
}
