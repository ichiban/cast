package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"sync"

	log "github.com/sirupsen/logrus"
	flag "github.com/spf13/pflag"

	"github.com/ichiban/cast"
	"github.com/ichiban/cast/upnp"
)

var defaultInterface string

func init() {
	is, err := net.Interfaces()
	if err != nil {
		return
	}

	wants := net.FlagUp | net.FlagBroadcast | net.FlagMulticast
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
	var verbose bool

	flag.StringVarP(&iface, "interface", "i", defaultInterface, "network interface")
	flag.StringVarP(&name, "name", "n", "Cast", "friendly name that appears on your player device")
	flag.BoolVarP(&verbose, "verbose", "v", false, "shows more logs")
	flag.Parse()

	if verbose {
		log.SetLevel(log.DebugLevel)
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)

	done := make(chan struct{}, 1)
	defer close(done)

	i, err := net.InterfaceByName(iface)
	if err != nil {
		log.WithFields(log.Fields{
			"interface": iface,
			"err":       err,
		}).Error("unknown interface")
		return
	}

	path, close, err := path()
	if err != nil {
		log.WithFields(log.Fields{
			"err": err,
		}).Error("failed to get path")
		return
	}
	defer close()

	log.WithFields(log.Fields{
		"interface": iface,
		"name":      name,
		"verbose":   verbose,
		"path":      path,
	}).Info("cast")

	s, err := upnp.NewDevice(i, "urn:schemas-upnp-org:device:MediaServer:1", name, []upnp.Service{
		{
			ID:   "urn:upnp-org:serviceId:ContentDirectory",
			Type: "urn:schemas-upnp-org:service:ContentDirectory:1",
			Desc: &cast.Description,
			Impl: &cast.ContentDirectory{
				Path: path,
			},
		},
	})
	if err != nil {
		log.WithFields(log.Fields{
			"err": err,
		}).Error("failed to create device")
		return
	}

	var wg sync.WaitGroup
	defer wg.Wait()

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := s.Advertise(done); err != nil {
			log.WithFields(log.Fields{
				"err": err,
			}).Error("failed to advertise")
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := s.ReplySearch(done); err != nil {
			log.WithFields(log.Fields{
				"err": err,
			}).Error("failed to reply")
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = s.ListenAndServe()
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		<-sig
		if err := s.Shutdown(context.Background()); err != nil {
			log.WithFields(log.Fields{
				"err": err,
			}).Error("failed to shutdown")
		}
		done <- struct{}{}
		done <- struct{}{}
	}()
}

func path() (string, func(), error) {
	var path string
	var close func()
	switch len(flag.Args()) {
	case 0:
		dir, err := os.Getwd()
		if err != nil {
			return "", nil, err
		}
		path = dir
		close = func() {}
	case 1:
		path = flag.Args()[0]
		var err error
		path, err = filepath.Abs(path)
		if err != nil {
			return "", nil, err
		}
		f, err := os.Stat(path)
		if err != nil {
			return "", nil, err
		}
		if f.IsDir() {
			close = func() {}
			break
		}
		fallthrough
	default:
		temp, err := ioutil.TempDir("", "cast")
		if err != nil {
			return "", nil, err
		}
		close = func() { os.RemoveAll(temp) }

		count := map[string]int{}
		for _, p := range flag.Args() {
			p, err = filepath.Abs(p)
			if err != nil {
				return "", nil, err
			}
			if _, err := os.Stat(p); err != nil {
				return "", nil, err
			}
			base := filepath.Base(p)
			count[base]++
			if n := count[base]; n > 1 {
				base = fmt.Sprintf("%s (%d)", base, n)
			}
			if err := os.Symlink(p, filepath.Join(temp, base)); err != nil {
				return "", nil, err
			}
		}

		path = temp
	}
	return path, close, nil
}
