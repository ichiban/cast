package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"time"

	uuid "github.com/satori/go.uuid"

	"github.com/ichiban/cast"

	log "github.com/sirupsen/logrus"
)

const (
	defaultHTTPPort   = 8200
	defaultSearchPort = 1900
	defaultInterval   = 3 * time.Second
)

const wants = net.FlagUp | net.FlagBroadcast | net.FlagMulticast

var defaultInterface string

func init() {
	is, err := net.Interfaces()
	if err != nil {
		return
	}

	for _, i := range is {
		if i.Flags&wants == wants {
			defaultInterface = i.Name
			return
		}
	}
}

func main() {
	var iface string
	var name string
	var port int
	var interval time.Duration
	var dir string
	var verbose bool

	flag.StringVar(&iface, "interface", defaultInterface, "network interface")
	flag.StringVar(&name, "name", "Cast", "friendly name that appears on your player device")
	flag.IntVar(&port, "port", defaultHTTPPort, "HTTP port")
	flag.DurationVar(&interval, "interval", defaultInterval, "advertise interval")
	flag.StringVar(&dir, "dir", ".", "path to the directory containing media files")
	flag.BoolVar(&verbose, "verbose", false, "shows more logs")
	flag.Parse()

	if verbose {
		log.SetLevel(log.DebugLevel)
	}

	log.WithFields(log.Fields{
		"interface": iface,
		"name":      name,
		"port":      port,
		"verbose":   verbose,
	}).Info("Start")

	i, err := net.InterfaceByName(iface)
	if err != nil {
		log.WithError(err).Fatal("Unknown interface.")
	}

	addr, err := localAddress(i)
	if err != nil {
		log.WithError(err).Fatal("Failed to get local address.")
	}

	l, err := net.Listen("tcp", fmt.Sprintf("%s:%d", addr, port))
	if err != nil {
		log.WithError(err).Fatal("Failed to listen.")
	}

	baseURL := &url.URL{Scheme: "http", Host: l.Addr().String()}

	uuid := uuid.NewV4()

	b := cast.Beacon{
		Interface: i,
		UUID:      uuid.String(),
		BaseURL:   baseURL,
		Interval:  interval,
	}
	b.SearchAddr, err = net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", addr, defaultSearchPort))
	if err != nil {
		log.WithError(err).Fatal("Failed to resolve search addr.")
	}

	go b.Run(context.Background())

	ml, err := cast.NewMediaLibrary(baseURL.ResolveReference(&url.URL{Path: "/media/"}), dir)
	if err != nil {
		log.WithError(err).Fatal("Failed to create a media library.")
	}

	desc := cast.Description{
		BaseURL:      baseURL,
		FriendlyName: name,
		UUID:         uuid.String(),
	}

	mux := http.NewServeMux()
	mux.Handle("/", &desc)
	mux.HandleFunc("/control", ml.Control)
	// mux.HandleFunc("/event", nil)
	mux.Handle("/public/", http.FileServer(http.FS(cast.Public)))
	mux.Handle("/media/", http.StripPrefix("/media/", http.FileServer(http.FS(os.DirFS(dir)))))

	log.WithField("url", baseURL).Info("Start HTTP server.")
	defer log.WithField("url", baseURL).Info("Stop HTTP server.")
	switch err := http.Serve(l, requestLog(privateOnly(mux))); err {
	case http.ErrServerClosed:
		break
	default:
		log.WithError(err).Fatal("Failed to listen and serve.")
	}
}

func localAddress(i *net.Interface) (string, error) {
	as, err := i.Addrs()
	if err != nil {
		return "", err
	}

	for _, a := range as {
		var ip net.IP
		switch a := a.(type) {
		case *net.IPNet:
			ip = a.IP
		case *net.IPAddr:
			ip = a.IP
		default:
			continue
		}

		if ip.IsLoopback() {
			continue
		}

		if ip.To4() == nil {
			continue
		}

		return ip.String(), nil
	}

	return "", errors.New("not found")
}

func requestLog(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.WithFields(log.Fields{
			"addr": r.RemoteAddr,
			"url":  r.RequestURI,
			"ua":   r.Header.Get("User-Agent"),
			"st":   r.Header.Get(""),
		}).Info(r.Method)
		h.ServeHTTP(w, r)
	})
}

func privateOnly(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			log.WithError(err).Warn("Failed to split host:port.")
			http.Error(w, "", http.StatusInternalServerError)
			return
		}
		ip := net.ParseIP(host)
		if !ip.IsPrivate() {
			log.WithField("addr", r.RemoteAddr).WithField("ip", ip).Warn("not private")
			http.Error(w, "", http.StatusUnauthorized)
			return
		}
		h.ServeHTTP(w, r)
	})
}
