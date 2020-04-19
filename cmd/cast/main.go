package main

import (
	"context"
	"net"
	"os"
	"os/signal"
	"sync"

	"github.com/sirupsen/logrus"
	flag "github.com/spf13/pflag"

	"github.com/ichiban/cast"
	"github.com/ichiban/cast/upnp"
)

func main() {
	var iface string
	var name string
	var verbose bool

	flag.StringVarP(&iface, "interface", "i", "en0", "network interface")
	flag.StringVarP(&name, "name", "n", "Cast", "friendly name that appears on your player device")
	flag.BoolVarP(&verbose, "verbose", "v", false, "shows more logs")
	flag.Parse()

	if verbose {
		logrus.SetLevel(logrus.DebugLevel)
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)

	done := make(chan struct{}, 1)
	defer close(done)

	i, err := net.InterfaceByName(iface)
	if err != nil {
		panic(err)
	}

	s, err := upnp.NewDevice(i, "urn:schemas-upnp-org:device:MediaServer:1", name, []upnp.Service{
		{
			ID:   "urn:upnp-org:serviceId:ContentDirectory",
			Type: "urn:schemas-upnp-org:service:ContentDirectory:1",
			Desc: &cast.Description,
			Impl: &cast.ContentDirectory{
				Path: flag.Args()[0],
			},
		},
	})
	if err != nil {
		panic(err)
	}

	var wg sync.WaitGroup
	defer wg.Wait()

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := s.Advertise(done); err != nil {
			panic(err)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := s.ReplySearch(done); err != nil {
			panic(err)
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
			panic(err)
		}
		done <- struct{}{}
		done <- struct{}{}
	}()
}
