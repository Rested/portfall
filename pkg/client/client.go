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
	"k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"portfall/pkg/favicon"
	"portfall/pkg/logger"
	"strings"
	"sync"
	"time"
)

// Client is the core struct of Portfall - references k8s client and config and tracks active websites and namespaces
type Client struct {
	s                *kubernetes.Clientset
	conf             *rest.Config
	rawConf          *api.Config
	configPath       string
	currentContext   string
	websites         []*Website
	activeNamespaces []string
	log              *logger.CustomLogger
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

// Website is the internal representation of a Website
type Website struct {
	isForwarded    bool
	portForwardReq portForwardPodRequest
	icon           favicon.Icon
	// public
	LocalPort     int32  `json:"localPort"`
	PodPort       int32  `json:"podPort"`
	Title         string `json:"title"`
	IconUrl       string `json:"iconUrl"`
	IconRemoteUrl string `json:"iconRemoteUrl"`
	Namespace     string `json:"namespace"`
	PodName       string `json:"podName"`
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
			c.log.Debugf("%v", err)
		}
	}()

	select {
	case <-readyCh:
		break
	case <-time.After(10 * time.Second):
		close(stopCh)
		//close(readyCh)
		return nil, fmt.Errorf("timed out of portforward for pod %s on port %d after 10 seconds", pod.Name, portForwardReq.PodPort)
	}
	website := Website{
		isForwarded:    true,
		LocalPort:      int32(localPort),
		PodPort:        containerPort,
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
		c.log.Warnf("Found no namespaces")
		c.log.Debugf("%v", err)
		return make([]string, 0)
	}
	for _, ns := range namespaces.Items {
		nsList = append(nsList, ns.Name)
	}
	c.log.Infof("Found the following namespaces %v", nsList)
	return nsList
}

// RemoveWebsitesInNamespace takes a namespace's name and stops port-forwarding all websites and removes them
// from Client.websites and finally removes the namespace from Client.activeNamespaces
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

func (c *Client) handleWebsiteAdding(p v1.Pod, tp int32, resourceName string, resourceType string, queue chan *Website) {
	ws, err := c.getWebsiteForPort(p, tp)
	if err != nil {
		c.log.Warnf("Failed to get icons for pod %s in %s %s on port %d", p.Name, resourceName, resourceType, tp)
		c.log.Errorf("%v", err)
		queue <- &Website{}
	} else {
		queue <- ws
	}
}

func (c *Client) handleServicesInPod(services *v1.ServiceList, pod v1.Pod, wg *sync.WaitGroup, queue chan *Website) (handledPorts []int32) {
	for _, svc := range services.Items {
		matchCount := 0
		for k, v := range svc.Spec.Selector {
			if pod.Labels[k] == v {
				matchCount++
			}
		}
		if matchCount == len(svc.Spec.Selector) {
		portIter:
			for _, port := range svc.Spec.Ports {
				for _, p := range handledPorts {
					if p == port.TargetPort.IntVal {
						// this port has already been handled by another service so we are safe to skip it
						c.log.Infof("skipped port %d for service %s as it has already been handled", p, svc.Name)
						
						continue portIter
					}
				}
				handledPorts = append(handledPorts, port.TargetPort.IntVal)
				wg.Add(1)
				go c.handleWebsiteAdding(pod, port.TargetPort.IntVal, svc.Name, "service", queue)
			}
		}
	}
	return handledPorts
}

func (c *Client) handleContainerPortsInPod(pod v1.Pod, handledPorts []int32, wg *sync.WaitGroup, queue chan *Website) {
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

func (c *Client) forwardAndGetIconsForWebsitesInNamespace(namespace string) ([]*Website, error) {
	var nsWebsites []*Website
	internalNS := namespace
	if namespace == "All Namespaces" {
		internalNS = ""
	}
	pods, err := c.s.CoreV1().Pods(internalNS).List(metav1.ListOptions{})
	if err != nil {
		c.log.Warnf("Failed to get pods in ns %s", namespace)
		return nil, err
	}
	services, err := c.s.CoreV1().Services(internalNS).List(metav1.ListOptions{})
	if err != nil {
		c.log.Warnf("Failed to get services in ns %s", namespace)
		return nil, err
	}

	var handledReplicationControllers []string
	var wg sync.WaitGroup
	queue := make(chan *Website, 1)
podLoop:
	for _, pod := range pods.Items {
		// skip pods in already active namespaces
		if namespace == "All Namespaces" {
			for _, n := range c.activeNamespaces {
				if n != namespace && n == pod.Namespace {
					continue podLoop
				}
			}
		}
		// skip not running pods
		if pod.Status.Phase != "Running" {
			continue podLoop
		}
		// has been scheduled from deletion
		if pod.DeletionTimestamp != nil {
			continue podLoop
		}
		// handle replication controllers we only need one pod from each replica
		for _, owner := range pod.OwnerReferences {
			if owner.Kind == "StatefulSet" || owner.Kind == "ReplicaSet" || owner.Kind == "DaemonSet" {
				for _, rc := range handledReplicationControllers {
					if rc == owner.Name {
						continue podLoop
					}
				}
				handledReplicationControllers = append(handledReplicationControllers, owner.Name)
			}
		}
		// services
		handledPorts := c.handleServicesInPod(services, pod, &wg, queue)
		// container ports
		c.handleContainerPortsInPod(pod, handledPorts, &wg, queue)
	}
	go func() {
		for w := range queue {
			c.log.Infof("received website forwarded (%t) to port %d from chan!", w.isForwarded, w.LocalPort)
			if w.isForwarded {
				nsWebsites = append(nsWebsites, w)
			}
			wg.Done()
		}
	}()

	c.log.Infof("waiting for all potential websites to be processed")
	wg.Wait()
	c.log.Infof("%d websites processed", len(nsWebsites))
	return nsWebsites, nil
}

func (c *Client) addDerivedDetailsToWebsites() {
	for _, website := range c.websites {
		if website.Title == "" {
			website.Title = website.icon.PageTitle
			if website.Title == "" {
				website.Title = website.portForwardReq.Pod.Name
			}
			website.IconUrl = fmt.Sprintf("file://%s", website.icon.FilePath)
			website.IconRemoteUrl = website.icon.RemoteUrl
			website.PodName = website.portForwardReq.Pod.Name
			website.Namespace = website.portForwardReq.Pod.Namespace
		}
	}
}

// GetWebsitesInNamespace takes a namespace's name and ensures that all websites in that namespace are port-forwarded.
// If the namespaces in the Website are not port-forwarded then forwardAndGetIconsForWebsitesInNamespace is called.
// Finally a json response of a list of Websites for the namespace specified is returned.
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
	var err error
	if !skip {
		nsWebsites, err = c.forwardAndGetIconsForWebsitesInNamespace(namespace)
		if err != nil {
			return ""
		}
		c.log.Infof("Got %d websites forwarded in ns %s", len(nsWebsites), namespace)
		c.websites = append(c.websites, nsWebsites...)
		c.addDerivedDetailsToWebsites()
	} else {
		c.log.Infof("skipping get websites for namespace %s as already in active namespaces %v", namespace, c.activeNamespaces)
		for _, w := range c.websites {
			if w.portForwardReq.Pod.Namespace == namespace {
				nsWebsites = append(nsWebsites, w)
			}
		}
	}
	c.activeNamespaces = append(c.activeNamespaces, namespace)

	jBytes, _ := json.Marshal(nsWebsites)
	return string(jBytes)
}

// getDefaultClientSetAndConfig is an initialization method to get the default kubernetes config and initialize
// the Clientset
func getDefaultClientSetAndConfig() (*kubernetes.Clientset, *rest.Config, *api.Config, string, error) {
	var configPath string
	if home := homeDir(); home != "" {
		configPath = filepath.Join(home, ".kube", "config")
		rawConfig, err := clientcmd.LoadFromFile(configPath)
		if err != nil {
			return &kubernetes.Clientset{}, &rest.Config{}, &api.Config{}, "", err
		}
		for k := range rawConfig.Contexts {
			rawConfig.CurrentContext = k
		}

		clientConf := clientcmd.NewNonInteractiveClientConfig(*rawConfig, rawConfig.CurrentContext,
			&clientcmd.ConfigOverrides{}, clientcmd.NewDefaultClientConfigLoadingRules())
		restConf, err := clientConf.ClientConfig()
		if err != nil {
			return &kubernetes.Clientset{}, &rest.Config{}, &api.Config{}, configPath, err
		}
		clientSet, err := kubernetes.NewForConfig(restConf)
		if err != nil {
			return &kubernetes.Clientset{}, &rest.Config{}, &api.Config{}, configPath, err
		}
		return clientSet, restConf, rawConfig, configPath, nil
	}
	return &kubernetes.Clientset{}, &rest.Config{}, &api.Config{}, configPath, errors.New("default config not found")
}

// GetCurrentConfigPath simply returns the configPath
func (c *Client) GetCurrentConfigPath() string {
	return c.configPath
}

// GetAvailableContexts gets a slice of available contexts for the current conf
func (c *Client) GetAvailableContexts() []string {
	contexts := make([]string, len(c.rawConf.Contexts))
	i := 0
	for k := range c.rawConf.Contexts {
		contexts[i] = k
		i++
	}
	return contexts
}

// GetCurrentContext returns the context active for the current conf
func (c *Client) GetCurrentContext() string {
	return c.currentContext
}

// SetConfigPath takes a configPath string and tries to configure the Client for that config. If it was successful
// the configPath is returned. Otherwise the old configPath is returned.
func (c *Client) SetConfigPath(configPath string, context string) []string {
	var rawConfig *api.Config
	var err error
	var useContext string
	if configPath != c.configPath {
		rawConfig, err = clientcmd.LoadFromFile(configPath)
		if err != nil {
			c.log.Debugf("%v", err)
			return []string{c.configPath, c.currentContext}
		}
		for k := range rawConfig.Contexts {
			useContext = k
			break
		}
	} else {
		rawConfig = c.rawConf
		useContext = context
		// nothing changed
		if useContext == c.currentContext {
			return []string{c.configPath, c.currentContext}
		}
	}

	clientConf := clientcmd.NewNonInteractiveClientConfig(*rawConfig, useContext, &clientcmd.ConfigOverrides{},
		clientcmd.NewDefaultClientConfigLoadingRules())
	restConf, err := clientConf.ClientConfig()

	if err != nil {
		c.log.Infof("error building restConf from path %s", configPath)
		c.log.Debugf("%v", err)
		return []string{c.configPath, c.currentContext}
	}
	clientSet, err := kubernetes.NewForConfig(restConf)
	if err != nil {
		c.log.Infof("error building clientset from restConf at %s", configPath)
		return []string{c.configPath, c.currentContext}
	}

	// close forwards in the old context
	c.closeAllPortForwards()
	c.rawConf = rawConfig
	c.currentContext = useContext
	c.s = clientSet
	c.configPath = configPath
	c.conf = restConf
	return []string{configPath, useContext}
}

func (c *Client) closeAllPortForwards() {
	for _, w := range c.websites {
		c.log.Infof("closing port forward on port %d of pod %s", w.PodPort, w.portForwardReq.Pod.Name)
		close(w.portForwardReq.StopCh)
	}
}

// todo: search for all config files and present them in ui - autodetect

func homeDir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	return os.Getenv("USERPROFILE") // windows
}

// WailsInit takes the wails runtime and does some initialization - sets up the default client if possible
func (c *Client) WailsInit(runtime *wails.Runtime) error {
	c.log = logger.NewCustomLogger("Client", runtime)
	s, conf, rawConf, confPath, err := getDefaultClientSetAndConfig()
	if err != nil {
		c.log.Warnf("failed to get default config: %v", err.Error())
		return nil
	}
	namespaces, err := s.CoreV1().Namespaces().List(metav1.ListOptions{})
	if err != nil || len(namespaces.Items) == 0 {
		c.rawConf = rawConf
		c.currentContext = c.rawConf.CurrentContext
		c.configPath = confPath
		c.s = s
		c.conf = conf
		c.log.Infof("no namespaces in cluster with config path %s - could be a connection issue", confPath)
		return nil
	}
	c.rawConf = rawConf
	c.currentContext = c.rawConf.CurrentContext
	c.s = s
	c.conf = conf
	c.configPath = confPath

	return nil
}

// WailsShutdown is called on shutdown and cleans up all port-forwards still active
func (c *Client) WailsShutdown() {
	c.closeAllPortForwards()
}
