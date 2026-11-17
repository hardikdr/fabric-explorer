package main

import (
	"context"
	"encoding/json"
	"flag"
	"github.com/ironcore-dev/fabric-explorer/internal/correlate"
	"github.com/ironcore-dev/fabric-explorer/internal/httpapi"
	"github.com/ironcore-dev/fabric-explorer/internal/observation"
	"github.com/ironcore-dev/fabric-explorer/internal/source"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

func main() {
	mode := flag.String("mode", "fixture", "fixture, live, or ssh-lab (Mac-host development only)")
	fixture := flag.String("fixture", "testdata/kubernetes/clos.json", "fixture file")
	addr := flag.String("listen", "127.0.0.1:8080", "HTTP bind address")
	web := flag.String("web-dir", "web/dist", "static assets")
	interval := flag.Duration("poll-interval", 5*time.Second, "resource poll interval (1s–5m)")
	timeout := flag.Duration("poll-timeout", 4*time.Second, "per-source list deadline (1s–1m)")
	kubeconfig := flag.String("kubeconfig", "", "explicit kubeconfig path for local live mode")
	kubecontext := flag.String("context", "", "explicit context for local live mode")
	incluster := flag.Bool("in-cluster", false, "explicitly use service account credentials")
	namespaces := flag.String("cell-namespaces", "", "comma-separated Cell namespaces; empty means all")
	mapping := flag.String("mapping", "", "JSON Switch-name to Node-name mapping file")
	fabriclet := flag.String("fabriclet-container", "fabriclet", "exact recognized bootstrap container name; empty disables")
	sonicMode := flag.String("sonic-observations", "disabled", "disabled, ssh-lab (strict BatchMode), or replay")
	sonicCapture := flag.String("sonic-captures", "", "manual sanitized capture JSON for replay mode")
	sonicInterval := flag.Duration("sonic-poll-interval", 30*time.Second, "SONiC polling interval (10s–5m)")
	sonicCommandTimeout := flag.Duration("sonic-command-timeout", 12*time.Second, "per-command SSH deadline (1s–30s)")
	sonicSwitchTimeout := flag.Duration("sonic-switch-timeout", 90*time.Second, "per-switch deadline (5s–3m)")
	sonicPassword := flag.String("sonic-password-file", "", "runtime local password file for Go SSH auth; never included in API")
	sonicKnownHosts := flag.String("sonic-known-hosts", "", "known_hosts file; default Mac ~/.ssh/known_hosts")
	flag.Parse()
	if *sonicInterval < 10*time.Second || *sonicInterval > 5*time.Minute || *sonicCommandTimeout < time.Second || *sonicCommandTimeout > 30*time.Second || *sonicSwitchTimeout < 5*time.Second || *sonicSwitchTimeout > 3*time.Minute {
		log.Fatal("SONiC observation timeouts outside supported bounds")
	}
	if *mode == "ssh-lab" {
		flag.Visit(func(f *flag.Flag) {
			if f.Name == "kubeconfig" || f.Name == "context" || f.Name == "in-cluster" {
				log.Fatal("ssh-lab does not accept Kubernetes credential options")
			}
		})
		intervalSet, timeoutSet := false, false
		flag.Visit(func(f *flag.Flag) {
			intervalSet = intervalSet || f.Name == "poll-interval"
			timeoutSet = timeoutSet || f.Name == "poll-timeout"
		})
		if !intervalSet {
			*interval = 30 * time.Second
		}
		if !timeoutSet {
			*timeout = 45 * time.Second
		}
	}
	if *interval < time.Second || *interval > 5*time.Minute || *timeout < time.Second || *timeout > time.Minute {
		log.Fatal("poll intervals outside supported bounds")
	}
	cfg := correlate.Config{FabricletContainer: *fabriclet, Mapping: map[string]string{}}
	if *namespaces != "" {
		for _, ns := range strings.Split(*namespaces, ",") {
			if strings.TrimSpace(ns) == "" {
				log.Fatal("empty namespace")
			}
			cfg.CellNamespaces = append(cfg.CellNamespaces, strings.TrimSpace(ns))
			if len(cfg.CellNamespaces) > 64 {
				log.Fatal("at most 64 Cell namespaces are supported")
			}
		}
	}
	if *mapping != "" {
		b, err := os.ReadFile(*mapping)
		if err != nil {
			log.Fatal(err)
		}
		if err = json.Unmarshal(b, &cfg.Mapping); err != nil {
			log.Fatal("invalid mapping JSON")
		}
	}
	var reader source.Reader
	contextName := "fixture · 2 spine / 2 leaf"
	switch *mode {
	case "fixture":
		f, err := source.LoadFixture(*fixture)
		if err != nil {
			log.Fatal(err)
		}
		_ = f
		reader = source.FileFixture{Path: *fixture, Namespaces: cfg.CellNamespaces}
		contextName = "Fixture: " + *fixture
	case "ssh-lab":
		reader = &source.SSHReader{Namespaces: cfg.CellNamespaces}
		contextName = "Dell6 network-lab · SSH read-only Kubernetes inventory"
	case "live":
		var rc *rest.Config
		var err error
		if *incluster {
			if *kubecontext != "" || *kubeconfig != "" {
				log.Fatal("do not mix in-cluster and kubeconfig options")
			}
			rc, err = rest.InClusterConfig()
			contextName = "In-cluster ServiceAccount"
		} else {
			if *kubecontext == "" || *kubeconfig == "" {
				log.Fatal("live mode requires explicit --kubeconfig and --context, or --in-cluster")
			}
			loading := &clientcmd.ClientConfigLoadingRules{ExplicitPath: *kubeconfig}
			rc, err = clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loading, &clientcmd.ConfigOverrides{CurrentContext: *kubecontext}).ClientConfig()
			contextName = *kubecontext
		}
		if err != nil {
			log.Fatal("unable to configure Kubernetes reader; verify context and credentials")
		}
		rc.Timeout = *timeout
		rc.QPS = 5
		rc.Burst = 8
		client, err := dynamic.NewForConfig(rc)
		if err != nil {
			log.Fatal("unable to create Kubernetes reader")
		}
		reader = &source.KubeReader{Client: client, Namespaces: cfg.CellNamespaces, Limit: 500, MaxObjects: 10000}
	default:
		log.Fatal("mode must be fixture, live, or ssh-lab")
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	store := source.NewStore(reader, cfg, *mode, contextName, *interval, *timeout)
	go store.Run(ctx)
	var observerReader observation.Reader
	switch *sonicMode {
	case "disabled":
	case "ssh-lab":
		if *mode == "fixture" {
			log.Fatal("real SONiC polling cannot be associated with fixture Kubernetes inventory")
		}
		observerReader = observation.SSHReader{PasswordFile: *sonicPassword, KnownHostsFile: *sonicKnownHosts}
	case "replay":
		if *sonicCapture == "" {
			log.Fatal("replay requires --sonic-captures")
		}
		observerReader = observation.Replay{Path: *sonicCapture}
	default:
		log.Fatal("sonic-observations must be disabled, ssh-lab or replay")
	}
	observer := observation.NewStore(observerReader, store, *sonicMode, *sonicInterval, *sonicCommandTimeout, *sonicSwitchTimeout)
	go observer.Run(ctx)
	server := &http.Server{Addr: *addr, Handler: httpapi.Handler(store, *web, observer), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second}
	go func() {
		<-ctx.Done()
		shutdown, c := context.WithTimeout(context.Background(), 5*time.Second)
		defer c()
		_ = server.Shutdown(shutdown)
	}()
	log.Printf("Fabric Explorer %s · http://%s (read-only)", *mode, *addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
