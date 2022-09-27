package util

import (
	"fmt"
	"github.com/pkg/errors"
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
		return nil, errors.Wrap(err, "Could not create round tripper")
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
