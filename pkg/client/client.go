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

// Client is the core struct of Portfall - references k8s client and config and tracks active websites and namespaces
type Client struct {
	s                *kubernetes.Clientset
	conf             *rest.Config
	configPath       string
	websites         []*website
	activeNamespaces []string
}

// Handles ongoing port-forwards for websites
type portForwardPodRequest struct {
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

// todo: merge website and presentable website
// website is the internal representation of a website
type website struct {
	isForwarded    bool
	portForwardReq portForwardPodRequest
	localPort      int32
	podPort        int32
	icon           favicon.Icon
}

// PresentableWebsite is the client representation of a port-forwarded website
type PresentableWebsite struct {
	LocalPort     int32  `json:"localPort"`
	PodPort       int32  `json:"podPort"`
	Title         string `json:"title"`
	IconUrl       string `json:"iconUrl"`
	IconRemoteUrl string `json:"iconRemoteUrl"`
	Namespace     string `json:"namespace"`
}

// PortForwardAPdd takes a portForwardPodRequest and creates the port forward to the given pod
// usage based on https://github.com/gianarb/kube-port-forward
func portForwardAPod(req portForwardPodRequest) error {
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

func (c *Client) getWebsiteForPort(pod v1.Pod, containerPort int32) (*website, error) {
	localPort, err := freeport.GetFreePort()
	if err != nil {
		return nil, err
	}
	// stopCh control the port forwarding lifecycle. When it gets closed the
	// port forward will terminate
	stopCh := make(chan struct{}, 1)
	// readyCh communicate when the port forward is ready to get traffic
	readyCh := make(chan struct{})
	portForwardReq := portForwardPodRequest{
		RestConfig: c.conf,
		Pod:        pod,
		LocalPort:  int32(localPort),
		PodPort:    containerPort,
		StopCh:     stopCh,
		ReadyCh:    readyCh,
	}
	go func() {
		err := portForwardAPod(portForwardReq)
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
	website := website{
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
}

// ListNamespaces returns a list of the names of available namespaces in the current cluster
func (c *Client) ListNamespaces() (nsList []string) {
	namespaces, err := c.s.CoreV1().Namespaces().List(metav1.ListOptions{})
	if err != nil {
		log.Printf("Found no namespaces %v", err)
		return make([]string, 0)
	}
	for _, ns := range namespaces.Items {
		nsList = append(nsList, ns.Name)
	}
	return nsList
}

// RemoveWebsitesInNamespace takes a namespace's name and stops port-forwarding all websites and removes them
// from Client.websites and finally removes the namespace from Client.activeNamespaces
func (c *Client) RemoveWebsitesInNamespace(namespace string) {
	var newWebsites []*website
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

func (c *Client) handleWebsiteAdding(p v1.Pod, tp int32, resourceName string, resourceType string, queue chan *website) {
	ws, err := c.getWebsiteForPort(p, tp)
	if err != nil {
		log.Printf("Failed to get icons for pod %s in %s %s on port %d", p.Name, resourceName, resourceType, tp)
		log.Print(err)
		queue <- &website{}
	} else {
		queue <- ws
	}
}

func (c *Client) handleServicesInPod(services *v1.ServiceList, pod v1.Pod, wg *sync.WaitGroup, queue chan *website) (handledPorts []int32) {
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
				go c.handleWebsiteAdding(pod, port.TargetPort.IntVal, svc.Name, "service", queue)
			}
		}
	}
	return handledPorts
}

func (c *Client) handleContainerPortsInPod(pod v1.Pod, handledPorts []int32, wg *sync.WaitGroup, queue chan *website) {
	for _, container := range pod.Spec.Containers {
	cpLoop:
		for _, port := range container.Ports {
			for _, hp := range handledPorts {
				if port.ContainerPort == hp {
					continue cpLoop
				}
			}
			wg.Add(1)
			go c.handleWebsiteAdding(pod, port.ContainerPort, container.Name, "container", queue)
		}
	}
}

func (c *Client) forwardAndGetIconsForWebsitesInNamespace(namespace string) ([]*website, error) {
	var nsWebsites []*website
	internalNS := namespace
	if namespace == "All Namespaces" {
		internalNS = ""
	}
	pods, err := c.s.CoreV1().Pods(internalNS).List(metav1.ListOptions{})
	if err != nil {
		log.Printf("Failed to get pods in ns %s", namespace)
		return nil, err
	}
	services, err := c.s.CoreV1().Services(internalNS).List(metav1.ListOptions{})
	if err != nil {
		log.Printf("Failed to get services in ns %s", namespace)
		return nil, err
	}

	var wg sync.WaitGroup
	queue := make(chan *website, 1)
	for _, pod := range pods.Items {
		// services
		handledPorts := c.handleServicesInPod(services, pod, &wg, queue)
		// container ports
		c.handleContainerPortsInPod(pod, handledPorts, &wg, queue)
	}
	go func() {
		for w := range queue {
			log.Printf("received website forwarded (%t) to port %d from chan!", w.isForwarded, w.localPort)
			if w.isForwarded {
				nsWebsites = append(nsWebsites, w)
			}
			wg.Done()
		}
	}()

	log.Printf("waiting for all potential websites to be processed")
	wg.Wait()
	log.Printf("%d websites processed", len(nsWebsites))
	return nsWebsites, nil
}

// GetWebsitesInNamespace takes a namespace's name and ensures that all websites in that namespace are port-forwarded.
// If the namespaces in the website are not port-forwarded then forwardAndGetIconsForWebsitesInNamespace is called.
// Finally a json response of a list of PresentableWebsites for the namespace specified is returned.
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

	var nsWebsites []*website
	var err error
	if !skip {
		nsWebsites, err = c.forwardAndGetIconsForWebsitesInNamespace(namespace)
		if err != nil {
			log.Print(err)
			return ""
		}
		log.Printf("Got %d websites forwarded in ns %s", len(nsWebsites), namespace)

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
	pw := make([]*PresentableWebsite, 0, len(nsWebsites))
	log.Printf("will return %d PresentAbleWebsites", len(nsWebsites))
	for _, w := range nsWebsites {
		title := w.icon.PageTitle
		if title == "" {
			title = w.portForwardReq.Pod.Name
		}

		pw = append(pw, &PresentableWebsite{
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

// getDefaultClientSetAndConfig is an initialization method to get the default kubernetes config and initialize
// the Clientset
func getDefaultClientSetAndConfig() (*kubernetes.Clientset, *rest.Config, string, error) {
	var configPath string
	if home := homeDir(); home != "" {
		configPath = filepath.Join(home, ".kube", "config")
		config, err := clientcmd.BuildConfigFromFlags("", configPath)
		if err != nil {
			return &kubernetes.Clientset{}, &rest.Config{}, "", err
		}
		clientSet, err := kubernetes.NewForConfig(config)
		if err != nil {
			return &kubernetes.Clientset{}, &rest.Config{}, "", err
		}
		return clientSet, config, configPath, nil
	}
	return &kubernetes.Clientset{}, &rest.Config{}, "", errors.New("default config not found")
}

// GetCurrentConfigPath simply returns the configPath
func (c *Client) GetCurrentConfigPath() string {
	// todo: can client simply access configPath directly?
	return c.configPath
}

// SetConfigPath takes a configPath string and tries to configure the Client for that config. If it was successful
// the configPath is returned. Otherwise the old configPath is returned.
func (c *Client) SetConfigPath(configPath string) string {
	config, err := clientcmd.BuildConfigFromFlags("", configPath)
	if err != nil {
		log.Printf("error building config from path %s", configPath)
		return c.configPath
	}
	clientSet, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Printf("error building clientset from config at %s", configPath)
		return c.configPath
	}

	namespaces, err := clientSet.CoreV1().Namespaces().List(metav1.ListOptions{})
	if err != nil || len(namespaces.Items) == 0 {
		log.Printf("no namespaces in cluster with config path %s - could be a connection issue", configPath)
		return c.configPath
	}
	c.s = clientSet
	c.configPath = configPath
	c.conf = config
	return configPath
}

// todo: search for all config files and present them in ui - autodetect

func homeDir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	return os.Getenv("USERPROFILE") // windows
}

// WailsInit takes the wails runtime and does some initialization - sets up the default client if possible
func (c *Client) WailsInit(_ *wails.Runtime) error {
	s, conf, confPath, err := getDefaultClientSetAndConfig()
	if err != nil {
		log.Printf("failed to get default config")
		return nil
	}
	namespaces, err := s.CoreV1().Namespaces().List(metav1.ListOptions{})
	if err != nil || len(namespaces.Items) == 0 {
		log.Printf("no namespaces in cluster with config path %s - could be a connection issue", confPath)
		return nil
	}
	c.s = s
	c.conf = conf
	c.configPath = confPath
	return nil
}

// WailsShutdown is called on shutdown and cleans up all port-forwards still active
func (c *Client) WailsShutdown() {
	for _, w := range c.websites {
		log.Printf("closing port forward on port %d of pod %s", w.podPort, w.portForwardReq.Pod.Name)
		close(w.portForwardReq.StopCh)
	}
}
