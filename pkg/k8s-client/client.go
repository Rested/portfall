package k8s_client

import (
	"context"
	"errors"
	"fmt"
	"github.com/phayes/freeport"
	"google.golang.org/appengine/log"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"portfall/pkg/favicon"
	"strings"
)

type Client struct {
	s    *kubernetes.Clientset
	conf *rest.Config
}

type PortForwardAPodRequest struct {
	RestConfig *rest.Config
	// Pod is the selected pod for this port forwarding
	Pod v1.Pod
	// LocalPort is the local port that will be selected to expose the PodPort
	LocalPort int32
	// PodPort is the target port for the pod
	PodPort int32
	// StopCh is the channel used to manage the port forward lifecycle
	StopCh <-chan struct{}
	// ReadyCh communicates when the tunnel is ready to receive traffic
	ReadyCh chan struct{}
}

type Website struct {
	isForwarded bool
	localPort   int32
	podPort     int32
	title       string
	bestIcon    favicon.Icon
}

func PortForwardAPod(req PortForwardAPodRequest) error {
	path := fmt.Sprintf("/api/v1/namespaces/%s/pods/%s/portforward",
		req.Pod.Namespace, req.Pod.Name)
	hostIP := strings.TrimLeft(req.RestConfig.Host, "htps:/")

	transport, upgrader, err := spdy.RoundTripperFor(req.RestConfig)
	if err != nil {
		return err
	}

	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, http.MethodPost, &url.URL{Scheme: "https", Path: path, Host: hostIP})
	fw, err := portforward.New(dialer, []string{fmt.Sprintf("%d:%d", req.LocalPort, req.PodPort)}, req.StopCh, req.ReadyCh, os.Stdout, os.Stderr)
	if err != nil {
		return err
	}
	return fw.ForwardPorts()
}

func (c Client) getWebsiteForPort(pod v1.Pod, containerPort int32) (*Website, error) {
	localPort, err := freeport.GetFreePort()
	if err != nil {
		return nil, err
	}
	// stopCh control the port forwarding lifecycle. When it gets closed the
	// port forward will terminate
	stopCh := make(chan struct{}, 1)
	// readyCh communicate when the port forward is ready to get traffic
	readyCh := make(chan struct{})
	portForwardReq := PortForwardAPodRequest{
		RestConfig: c.conf,
		Pod:        pod,
		LocalPort:  int32(localPort),
		PodPort:    containerPort,
		StopCh:     stopCh,
		ReadyCh:    readyCh,
	}
	err := PortForwardAPod(portForwardReq)
	if err != nil {
		return nil, err
	}
	select {
	case <-readyCh:
		break
	}
	website := Website{
		isForwarded: true, // todo: ui decision as stated below
		localPort:   int32(localPort),
		podPort:     containerPort,
	}

	// get the favicon

	bestIcon, err := favicon.GetBest(fmt.Sprintf("http://localhost:%d", localPort))
	if err != nil {
		return nil, err
	}
	website.bestIcon = *bestIcon
	return &website, nil
	// todo: close here or not - ui decision
	// close(stopCh)
}

func (c Client) readPodsInNamespace(namespace string) ([]*v1.Pod, error) {
	pods, err := c.s.CoreV1().Pods(namespace).List(metav1.ListOptions{})
	if err != nil {
		return []*v1.Pod{}, err
	}
	for _, pod := range pods.Items {
		for _, container := range pod.Spec.Containers {
			for _, port := range container.Ports {
				website, err := c.getWebsiteForPort(pod, port.ContainerPort)
				if err != nil {
					log.Infof(context.TODO(), "Failed to get iconst for container %s in pod %s on port %d", container.Name, pod.Name, port.ContainerPort)
				}
			}
		}
	}

}

func GetDefaultClientSetAndConfig() (*kubernetes.Clientset, *rest.Config, error) {
	var configPath string
	if home := homeDir(); home != "" {
		configPath = filepath.Join(home, ".kube", "config")
		config, err := clientcmd.BuildConfigFromFlags("", configPath)
		if err != nil {
			return &kubernetes.Clientset{}, &rest.Config{}, err
		}
		clientSet, err := kubernetes.NewForConfig(config)
		if err != nil {
			return &kubernetes.Clientset{}, &rest.Config{}, err
		}
		return clientSet, config, nil
	}
	return &kubernetes.Clientset{}, &rest.Config{}, errors.New("default config not found")
}

func homeDir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	return os.Getenv("USERPROFILE") // windows
}
