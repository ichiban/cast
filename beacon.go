package cast

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
)

const (
	multicastAddress = "239.255.255.250:1900"
)

const (
	deviceType  = "urn:schemas-upnp-org:device:MediaServer:1"
	serviceType = "urn:schemas-upnp-org:service:ContentDirectory:1"
)

const (
	MethodNotify  = "NOTIFY"
	MethodMSearch = "M-SEARCH"
)
const (
	headerCacheControl        = "CACHE-CONTROL"
	headerLocation            = "LOCATION"
	headerNotificationType    = "NT"
	headerNotificationSubType = "NTS"
	headerServer              = "SERVER"
	headerUniqueServiceName   = "USN"
	headerBootID              = "BOOTID.UPNP.ORG"
	headerConfigID            = "CONFIGID.UPNP.ORG"
	headerSearchPort          = "SEARCHPORT.UPNP.ORG"
	headerDate                = "DATE"
	headerExt                 = "EXT"
	headerSearchTarget        = "ST"
)

const (
	productToken = "unknown/0.0 UPnP/1.0 cast/0.0"
)

const (
	all        = "ssdp:all"
	rootDevice = "upnp:rootdevice"
)

type Beacon struct {
	Interface  *net.Interface
	UUID       string
	BaseURL    *url.URL
	Interval   time.Duration
	SearchAddr *net.UDPAddr
}

func (b *Beacon) Run(ctx context.Context) {
	go b.advertise(ctx)
	b.search(ctx)
}

func (b *Beacon) advertise(ctx context.Context) {
	w, err := net.Dial("udp", multicastAddress)
	if err != nil {
		log.WithError(err).Error("net.ListenMulticastUDP() failed.")
		return
	}
	defer func() {
		if err := w.Close(); err != nil {
			log.WithError(err).Error("Failed to close.")
		}
	}()

	defer func() {
		if err := b.bye(w); err != nil {
			log.WithError(err).Error("Failed to bye.")
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(b.Interval):
			if err := b.alive(w); err != nil {
				log.WithError(err).Error("Failed to alive.")
			}
		}
	}
}

func (b *Beacon) search(ctx context.Context) {
	reqs := make(chan *http.Request)
	defer close(reqs)

	addr, err := net.ResolveUDPAddr("udp", multicastAddress)
	if err != nil {
		log.WithError(err).Error("net.ResolveUDPAddr() failed.")
		return
	}

	multi, err := net.ListenMulticastUDP("udp", b.Interface, addr)
	if err != nil {
		log.WithError(err).Error("net.ListenMulticastUDP() failed.")
		return
	}
	defer func() {
		if err := multi.Close(); err != nil {
			log.WithError(err).Error("Failed to close.")
		}
	}()
	go readReqs(multi, reqs)

	uni, err := net.ListenUDP("udp", b.SearchAddr)
	if err != nil {
		b.SearchAddr.Port = 0
		uni, err = net.ListenUDP("udp", b.SearchAddr)
		if err != nil {
			log.WithError(err).Error("net.ListenUDP() failed.")
			return
		}
		b.SearchAddr = uni.LocalAddr().(*net.UDPAddr)
	}
	defer func() {
		if err := uni.Close(); err != nil {
			log.WithError(err).Error("Failed to close.")
		}
	}()
	go readReqs(uni, reqs)

	for {
		select {
		case <-ctx.Done():
			return
		case r := <-reqs:
			b.respondSearch(r)
		}
	}
}

func (b *Beacon) respondSearch(r *http.Request) {
	conn, err := net.Dial("udp", r.RemoteAddr)
	if err != nil {
		log.WithError(err).Error("net.Dial() failed.")
		return
	}
	defer func() {
		if err := conn.Close(); err != nil {
			log.WithError(err).Error("Failed to close.")
		}
	}()

	st := r.Header.Get(headerSearchTarget)

	uuid := fmt.Sprintf("uuid:%s", b.UUID)

	type record struct {
		st  string
		usn []string
	}
	var records []record
	switch {
	case st == all:
		records = []record{
			{st: rootDevice, usn: []string{uuid, rootDevice}},
			{st: uuid, usn: []string{uuid}},
			{st: deviceType, usn: []string{uuid, deviceType}},
			{st: serviceType, usn: []string{uuid, serviceType}},
		}
	case st == rootDevice:
		records = []record{
			{st: rootDevice, usn: []string{uuid, rootDevice}},
		}
	case st == uuid:
		records = []record{
			{st: uuid, usn: []string{uuid}},
		}
	case st == deviceType:
		records = []record{
			{st: deviceType, usn: []string{uuid, deviceType}},
		}
	case strings.HasPrefix(st, "urn:schemas-upnp-org:service:"):
		records = []record{
			{st: serviceType, usn: []string{uuid, serviceType}},
		}
	default:
		return
	}

	log.WithFields(log.Fields{
		"st":   st,
		"addr": r.RemoteAddr,
	}).Debug("search")

	for _, rc := range records {
		if err := b.respond(r.RemoteAddr, rc.st, strings.Join(rc.usn, "::")); err != nil {
			log.WithError(err).Warn("Failed to respond.")
		}
	}
}

func (b *Beacon) alive(w io.Writer) error {
	uuid := fmt.Sprintf("uuid:%s", b.UUID)

	for _, r := range []struct {
		nt  string
		usn []string
	}{
		{nt: rootDevice, usn: []string{uuid, rootDevice}},
		{nt: uuid, usn: []string{uuid}},
		{nt: deviceType, usn: []string{uuid, deviceType}},
		{nt: serviceType, usn: []string{uuid, serviceType}},
	} {
		time.Sleep(time.Duration(rand.Intn(300)) * time.Millisecond)
		if err := b.notifyAlive(w, r.nt, strings.Join(r.usn, "::")); err != nil {
			return err
		}
	}
	return nil
}

func (b *Beacon) bye(w io.Writer) error {
	uuid := fmt.Sprintf("uuid:%s", b.UUID)

	for _, r := range []struct {
		nt  string
		usn []string
	}{
		{nt: rootDevice, usn: []string{uuid, rootDevice}},
		{nt: uuid, usn: []string{uuid}},
		{nt: deviceType, usn: []string{uuid, deviceType}},
		{nt: serviceType, usn: []string{uuid, serviceType}},
	} {
		time.Sleep(time.Duration(rand.Intn(300)) * time.Millisecond)
		if err := b.notifyByeBye(w, r.nt, strings.Join(r.usn, "::")); err != nil {
			return err
		}
	}
	return nil
}

func (b *Beacon) respond(addr, st, usn string) error {
	resp := http.Response{
		Status:     http.StatusText(http.StatusOK),
		StatusCode: http.StatusOK,
		ProtoMajor: 1,
		ProtoMinor: 1,
		Header: http.Header{
			headerCacheControl:      []string{fmt.Sprintf("max-age = %d", b.Interval/time.Second)},
			headerDate:              []string{time.Now().Format(time.RFC1123)},
			headerExt:               []string{""},
			headerLocation:          []string{b.BaseURL.String()},
			headerServer:            []string{productToken},
			headerSearchTarget:      []string{st},
			headerUniqueServiceName: []string{usn},
			headerBootID:            []string{"0"},
			headerConfigID:          []string{"0"},
			headerSearchPort:        []string{strconv.Itoa(b.SearchAddr.Port)},
		},
	}
	conn, err := net.Dial("udp", addr)
	if err != nil {
		return fmt.Errorf("net.Dial() failed: %w", err)
	}
	defer func() {
		_ = conn.Close()
	}()

	log.WithFields(log.Fields{
		"st":   st,
		"usn":  usn,
		"addr": addr,
	}).Debug("respond search")
	return resp.Write(conn)
}

func (b *Beacon) notifyAlive(w io.Writer, nt, usn string) error {
	r := http.Request{
		Method:     MethodNotify,
		URL:        &url.URL{Path: "*"},
		ProtoMajor: 1,
		ProtoMinor: 1,
		Header: http.Header{
			headerCacheControl:        []string{fmt.Sprintf("max-age = %d", b.Interval/time.Second)},
			headerLocation:            []string{b.BaseURL.String()},
			headerNotificationType:    []string{nt},
			headerNotificationSubType: []string{"ssdp:alive"},
			headerServer:              []string{productToken},
			headerUniqueServiceName:   []string{usn},
			headerBootID:              []string{"0"},
			headerConfigID:            []string{"0"},
			headerSearchPort:          []string{strconv.Itoa(b.SearchAddr.Port)},
		},
		Host: multicastAddress,
	}
	log.WithFields(log.Fields{
		"nt":  nt,
		"usn": usn,
	}).Debug("notify alive")
	return r.Write(w)
}

func (b *Beacon) notifyByeBye(w io.Writer, nt, usn string) error {
	r := http.Request{
		Method:     MethodNotify,
		URL:        &url.URL{Path: "*"},
		ProtoMajor: 1,
		ProtoMinor: 1,
		Header: http.Header{
			headerNotificationType:    []string{nt},
			headerNotificationSubType: []string{"ssdp:byebye"},
			headerUniqueServiceName:   []string{usn},
			headerBootID:              []string{"0"},
			headerConfigID:            []string{"0"},
		},
		Host: multicastAddress,
	}
	log.WithFields(log.Fields{
		"nt":  nt,
		"usn": usn,
	}).Debug("notify byebye")
	return r.Write(w)
}

func readReqs(con *net.UDPConn, reqs chan *http.Request) {
	b := make([]byte, 1024)

	for {
		_, addr, err := con.ReadFromUDP(b)
		if err != nil {
			continue
		}
		r, err := http.ReadRequest(bufio.NewReader(bytes.NewReader(b)))
		if err != nil {
			continue
		}
		if r.Method != MethodMSearch {
			continue
		}
		r.RemoteAddr = addr.String()
		reqs <- r
	}
}
