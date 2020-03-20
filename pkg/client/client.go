package client

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/phayes/freeport"
	"github.com/wailsapp/wails"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"portfall/pkg/favicon"
	"strings"
	"sync"
	"time"
)

type Client struct {
	s                *kubernetes.Clientset
	conf             *rest.Config
	websites         []*Website
	activeNamespaces []string
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
	StopCh chan struct{}
	// ReadyCh communicates when the tunnel is ready to receive traffic
	ReadyCh chan struct{}
}

type Website struct {
	isForwarded    bool
	portForwardReq PortForwardAPodRequest
	localPort      int32
	podPort        int32
	icon           favicon.Icon
}

type PresentAbleWebsite struct {
	LocalPort     int32  `json:"localPort"`
	PodPort       int32  `json:"podPort"`
	Title         string `json:"title"`
	IconUrl       string `json:"iconUrl"`
	IconRemoteUrl string `json:"iconRemoteUrl"`
	Namespace     string `json:"namespace"`
}

// usage based on https://github.com/gianarb/kube-port-forward
func PortForwardAPod(req PortForwardAPodRequest) error {
	path := fmt.Sprintf("/api/v1/namespaces/%s/pods/%s/portforward",
		req.Pod.Namespace, req.Pod.Name)
	hostIP := strings.TrimLeft(req.RestConfig.Host, "htps:/")

	transport, upgrader, err := spdy.RoundTripperFor(req.RestConfig)
	if err != nil {
		return err
	}

	dialer := spdy.NewDialer(
		upgrader,
		&http.Client{Transport: transport},
		http.MethodPost,
		&url.URL{Scheme: "https", Path: path, Host: hostIP})

	fw, err := portforward.New(
		dialer,
		[]string{fmt.Sprintf("%d:%d", req.LocalPort, req.PodPort)},
		req.StopCh,
		req.ReadyCh,
		os.Stdout,
		os.Stderr)

	if err != nil {
		return err
	}
	return fw.ForwardPorts()
}

func (c *Client) getWebsiteForPort(pod v1.Pod, containerPort int32) (*Website, error) {
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
	go func() {
		err := PortForwardAPod(portForwardReq)
		if err != nil {
			log.Print(err)
		}
	}()

	select {
	case <-readyCh:
		break
	case <-time.After(4 * time.Second):
		close(stopCh)
		//close(readyCh)
		return nil, fmt.Errorf("timed out of portforward for pod %s on port %d after 4 seconds", pod.Name, portForwardReq.PodPort)
	}
	website := Website{
		isForwarded:    true, // todo: ui decision as stated below
		localPort:      int32(localPort),
		podPort:        containerPort,
		portForwardReq: portForwardReq,
	}

	// get the favicon
	bestIcon, err := favicon.GetBest(fmt.Sprintf("http://localhost:%d", localPort))
	if err != nil {
		close(stopCh)
		return nil, err
	}
	website.icon = *bestIcon
	return &website, nil
	// todo: close here or not - ui decision
	// close(stopCh)
}

func (c *Client) ListNamespaces() (nsList []string) {
	namespaces, err := c.s.CoreV1().Namespaces().List(metav1.ListOptions{})
	if err != nil {
		log.Printf("Found no namespaces %v", err)
	}
	for _, ns := range namespaces.Items {
		nsList = append(nsList, ns.Name)
	}
	return nsList
}

func (c *Client) RemoveWebsitesInNamespace(namespace string) {
	var newWebsites []*Website
	var newNamespaces []string
websiteLoop:
	for _, website := range c.websites {
		if website.portForwardReq.Pod.Namespace == namespace {
			close(website.portForwardReq.StopCh)
			continue
		}
		if namespace == "All Namespaces" {
			for _, ns := range c.activeNamespaces {
				if website.portForwardReq.Pod.Namespace == ns {
					// don't close pods in still active namespaces
					continue websiteLoop
				}
			}
			close(website.portForwardReq.StopCh)
			continue
		}
		newWebsites = append(newWebsites, website)
	}
	for _, ns := range c.activeNamespaces {
		if ns != namespace {
			newNamespaces = append(newNamespaces, ns)
		}
	}
	c.activeNamespaces = newNamespaces
	c.websites = newWebsites
}

func (c *Client) GetWebsitesInNamespace(namespace string) string {
	skip := false
	if namespace != "All Namespaces" {
		for _, ns := range c.activeNamespaces {
			if ns == "All Namespaces" || ns == namespace {
				skip = true
				break
			}
		}
	}

	var nsWebsites []*Website
	if !skip {
		internalNS := namespace
		if namespace == "All Namespaces" {
			internalNS = ""
		}
		pods, err := c.s.CoreV1().Pods(internalNS).List(metav1.ListOptions{})
		if err != nil {
			log.Printf("Failed to get pods in ns %s", namespace)
			log.Print(err)
			return ""
		}
		services, err := c.s.CoreV1().Services(internalNS).List(metav1.ListOptions{})
		if err != nil {
			log.Printf("Failed to get services in ns %s", namespace)
			log.Print(err)
			return ""
		}

		var wg sync.WaitGroup
		for _, pod := range pods.Items {
			var handledPorts []int32
			// services
			for _, svc := range services.Items {
				matchCount := 0
				for k, v := range svc.Spec.Selector {
					if pod.Labels[k] == v {
						matchCount++
					}
				}
				if matchCount == len(svc.Spec.Selector) {
					for _, port := range svc.Spec.Ports {
						handledPorts = append(handledPorts, port.TargetPort.IntVal)
						wg.Add(1)
						go func(p v1.Pod, tp int32, s v1.Service) {
							defer wg.Done()
							website, err := c.getWebsiteForPort(p, tp)
							if err != nil {
								log.Printf("Failed to get icons for pod %s in svc %s on port %d", p.Name, s.Name, tp)
								log.Print(err)
							} else {
								nsWebsites = append(nsWebsites, website)
							}
						}(pod, port.TargetPort.IntVal, svc)
					}
				}
			}
			// container ports
			for _, container := range pod.Spec.Containers {
			cpLoop:
				for _, port := range container.Ports {
					for _, hp := range handledPorts {
						if port.ContainerPort == hp {
							continue cpLoop
						}
					}

					wg.Add(1)
					go func(p v1.Pod, cp int32, cont v1.Container) {
						defer wg.Done()
						website, err := c.getWebsiteForPort(p, cp)
						if err != nil {
							log.Printf("Failed to get icons for container %s in pod %s on port %d", cont.Name, p.Name, cp)
						} else {
							nsWebsites = append(nsWebsites, website)
						}
					}(pod, port.ContainerPort, container)
				}
			}
		}

		log.Printf("waiting for all potential websites to be processed")
		wg.Wait()
		if namespace == "All Namespaces" {
			c.websites = nsWebsites
		} else {
			c.websites = append(c.websites, nsWebsites...)

		}
	} else {
		log.Printf("skipping get websites for namespace %s as already in active namespaces %v", namespace, c.activeNamespaces)
		for _, w := range c.websites {
			if w.portForwardReq.Pod.Namespace == namespace {
				nsWebsites = append(nsWebsites, w)
			}
		}
	}
	c.activeNamespaces = append(c.activeNamespaces, namespace)
	pw := make([]*PresentAbleWebsite, 0, len(nsWebsites))
	for _, w := range nsWebsites {
		title := w.icon.PageTitle
		if title == "" {
			title = w.portForwardReq.Pod.Name
		}

		pw = append(pw, &PresentAbleWebsite{
			LocalPort:     w.localPort,
			PodPort:       w.podPort,
			Title:         title,
			IconUrl:       fmt.Sprintf("file://%s", w.icon.FilePath),
			IconRemoteUrl: w.icon.RemoteUrl,
			Namespace:     w.portForwardReq.Pod.Namespace,
		})
		log.Printf("Returning website %s on pod port %d to frontend", title, w.podPort)
	}
	jBytes, _ := json.Marshal(pw)
	return string(jBytes)
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

func (c *Client) WailsInit(runtime *wails.Runtime) error {
	s, conf, err := GetDefaultClientSetAndConfig()
	if err != nil {
		log.Printf("failed ot get default config")
	} else {
		c.s = s
		c.conf = conf
	}
	return nil
}
