package main

import (
	"context"
	"flag"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/ichiban/picoms/upnp"

	"github.com/sirupsen/logrus"

	"github.com/ichiban/picoms"
)

func main() {
	var iface string
	var verbose bool

	flag.StringVar(&iface, "interface", "en0", "")
	flag.BoolVar(&verbose, "verbose", false, "")
	flag.Parse()

	if verbose {
		logrus.SetLevel(logrus.DebugLevel)
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	done := make(chan struct{}, 1)
	defer close(done)

	i, err := net.InterfaceByName(iface)
	if err != nil {
		panic(err)
	}

	s, err := upnp.NewServer(i, []upnp.Service{
		{
			Desc: &picoms.Description,
			Impl: &picoms.ContentDirectory{
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
