package registrator

import (
	"errors"
	"flag"
	"log"
	"os"
	"time"

	"github.com/contiv/netplugin/registrator/bridge"
	dockerapi "github.com/fsouza/go-dockerclient"
	"github.com/gliderlabs/pkg/usage"
)

// Version maintains the registrator's version
var Version string

var versionChecker = usage.NewChecker("registrator", Version)

var hostIP = flag.String("ip", "", "IP for ports mapped to the host")
var internal = flag.Bool("internal", false, "Use internal ports instead of published ones")
var refreshInterval = flag.Int("ttl-refresh", 0, "Frequency with which service TTLs are refreshed")
var refreshTTL = flag.Int("ttl", 0, "TTL for services (default is no expiry)")
var forceTags = flag.String("tags", "", "Append tags for all registered services")
var resyncInterval = flag.Int("resync", 0, "Frequency with which services are resynchronized")
var deregister = flag.String("deregister", "always", "Deregister exited services \"always\" or \"on-success\"")
var retryAttempts = flag.Int("retry-attempts", 0, "Max retry attempts to establish a connection with the backend. Use -1 for infinite retries")
var retryInterval = flag.Int("retry-interval", 2000, "Interval (in millisecond) between retry-attempts.")

func getopt(name, def string) string {
	if env := os.Getenv(name); env != "" {
		return env
	}
	return def
}

func assert(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

// InitRegistrator creates a new bridge to Service registrator
func InitRegistrator(bridgeType string) error {
	log.Printf("Initing registrator from Dockplugin")

	if *hostIP != "" {
		log.Println("Forcing host IP to", *hostIP)
	}

	if (*refreshTTL == 0 && *refreshInterval > 0) || (*refreshTTL > 0 && *refreshInterval == 0) {
		assert(errors.New("-ttl and -ttl-refresh must be specified together or not at all"))
	} else if *refreshTTL > 0 && *refreshTTL <= *refreshInterval {
		assert(errors.New("-ttl must be greater than -ttl-refresh"))
	}

	if *retryInterval <= 0 {
		assert(errors.New("-retry-interval must be greater than 0"))
	}

	docker, err := dockerapi.NewClient(getopt("DOCKER_HOST", "unix:///var/run/docker.sock"))
	assert(err)

	if *deregister != "always" && *deregister != "on-success" {
		assert(errors.New("-deregister must be \"always\" or \"on-success\""))
	}

	log.Println("Creating a new bridge: ", bridgeType)

	b, err := bridge.New(docker, bridgeType, bridge.Config{
		HostIP:          *hostIP,
		Internal:        *internal,
		ForceTags:       *forceTags,
		RefreshTTL:      *refreshTTL,
		RefreshInterval: *refreshInterval,
		DeregisterCheck: *deregister,
	})

	if err != nil {
		log.Fatal("Bridge not created: ", err)
	} else {
		b.Sync(false)
	}

	return nil
}

func main() {
	if len(os.Args) == 2 && os.Args[1] == "--version" {
		versionChecker.PrintVersion()
		os.Exit(0)
	}
	log.Printf("Starting registrator %s ...", Version)

	flag.Parse()

	if *hostIP != "" {
		log.Println("Forcing host IP to", *hostIP)
	}

	if (*refreshTTL == 0 && *refreshInterval > 0) || (*refreshTTL > 0 && *refreshInterval == 0) {
		assert(errors.New("-ttl and -ttl-refresh must be specified together or not at all"))
	} else if *refreshTTL > 0 && *refreshTTL <= *refreshInterval {
		assert(errors.New("-ttl must be greater than -ttl-refresh"))
	}

	if *retryInterval <= 0 {
		assert(errors.New("-retry-interval must be greater than 0"))
	}

	docker, err := dockerapi.NewClient(getopt("DOCKER_HOST", "unix:///var/run/docker.sock"))
	assert(err)

	if *deregister != "always" && *deregister != "on-success" {
		assert(errors.New("-deregister must be \"always\" or \"on-success\""))
	}

	log.Println("Creating a new bridge: ", flag.Arg(0))

	b, err := bridge.New(docker, flag.Arg(0), bridge.Config{
		HostIP:          *hostIP,
		Internal:        *internal,
		ForceTags:       *forceTags,
		RefreshTTL:      *refreshTTL,
		RefreshInterval: *refreshInterval,
		DeregisterCheck: *deregister,
	})

	assert(err)

	attempt := 0
	for *retryAttempts == -1 || attempt <= *retryAttempts {
		log.Printf("Connecting to backend (%v/%v)", attempt, *retryAttempts)

		err = b.Ping()
		if err == nil {
			break
		}

		if err != nil && attempt == *retryAttempts {
			assert(err)
		}

		time.Sleep(time.Duration(*retryInterval) * time.Millisecond)
		attempt++
	}

	// Start event listener before listing containers to avoid missing anything
	events := make(chan *dockerapi.APIEvents)
	assert(docker.AddEventListener(events))
	log.Println("Listening for Docker events ...")

	b.Sync(false)

	quit := make(chan struct{})

	// Start the TTL refresh timer
	if *refreshInterval > 0 {
		ticker := time.NewTicker(time.Duration(*refreshInterval) * time.Second)
		go func() {
			for {
				select {
				case <-ticker.C:
					b.Refresh()
				case <-quit:
					ticker.Stop()
					return
				}
			}
		}()
	}

	// Start the resync timer if enabled
	if *resyncInterval > 0 {
		resyncTicker := time.NewTicker(time.Duration(*resyncInterval) * time.Second)
		go func() {
			for {
				select {
				case <-resyncTicker.C:
					b.Sync(true)
				case <-quit:
					resyncTicker.Stop()
					return
				}
			}
		}()
	}

	// Process Docker events
	for msg := range events {
		switch msg.Status {
		case "start":
			go b.Add(msg.ID)
		case "die":
			go b.RemoveOnExit(msg.ID)
		case "stop", "kill":
			go b.Remove(msg.ID)
		}
	}

	close(quit)
	log.Fatal("Docker event loop closed") // todo: reconnect?
}
