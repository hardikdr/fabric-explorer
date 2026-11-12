package main

import (
	"context"
	"encoding/json"
	"flag"
	"github.com/ironcore-dev/fabric-explorer/internal/correlate"
	"github.com/ironcore-dev/fabric-explorer/internal/httpapi"
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
	mode := flag.String("mode", "fixture", "fixture or live (explicit read-only Kubernetes access)")
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
	flag.Parse()
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
		log.Fatal("mode must be fixture or live")
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	store := source.NewStore(reader, cfg, *mode, contextName, *interval, *timeout)
	go store.Run(ctx)
	server := &http.Server{Addr: *addr, Handler: httpapi.Handler(store, *web), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second}
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
