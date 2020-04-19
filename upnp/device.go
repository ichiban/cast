package upnp

import (
	"bufio"
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"strconv"
	"strings"
	"time"

	uuid "github.com/satori/go.uuid"
	log "github.com/sirupsen/logrus"
)

var (
	osProductToken     = "unknown/0.0"
	upnpProductToken   = "UPnP/1.0"
	serverProductToken = "picoms/0.0"
)

const (
	all        = "ssdp:all"
	rootDevice = "upnp:rootdevice"
)

func init() {
	out, err := exec.Command("uname", "-sr").Output()
	if err != nil {
		return
	}
	osProductToken = strings.ReplaceAll(string(out), " ", "/")
}

const (
	MethodNotify  = "NOTIFY"
	MethodMSearch = "M-SEARCH"
)

const (
	defaultHTTPPort   = 8200
	defaultSearchPort = 1900
	defaultInterval   = 3 * time.Second
)

type Device struct {
	http.Server

	UUID       uuid.UUID
	Type       string
	Interface  *net.Interface
	Interval   time.Duration
	SearchAddr *net.UDPAddr
	Services   []Service
}

func NewDevice(i *net.Interface, deviceType string, services []Service) (*Device, error) {
	a, err := localAddress(i)
	if err != nil {
		return nil, err
	}

	sa, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", a, defaultSearchPort))
	if err != nil {
		return nil, err
	}

	addr := fmt.Sprintf("%s:%d", a, defaultHTTPPort)

	s := Device{
		Server: http.Server{
			Addr: addr,
		},

		UUID:       uuid.NewV4(),
		Type:       deviceType,
		Interface:  i,
		Interval:   defaultInterval,
		SearchAddr: sa,
		Services:   services,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", s.Describe)
	for i, s := range s.Services {
		mux.HandleFunc(fmt.Sprintf("/%d/service", i), s.Describe)
		mux.HandleFunc(fmt.Sprintf("/%d/control", i), s.Control)
		mux.HandleFunc(fmt.Sprintf("/%d/event", i), s.Event)
		mux.Handle(fmt.Sprintf("/%d/", i), http.StripPrefix(fmt.Sprintf("/%d", i), s.Impl))
		s.Impl.SetBaseURL(&url.URL{
			Scheme: "http",
			Host:   addr,
			Path:   fmt.Sprintf("/%d/", i),
		})
	}
	s.Handler = mux

	return &s, nil
}

func (d *Device) Describe(w http.ResponseWriter, r *http.Request) {
	type service struct {
		ServiceType string `xml:"serviceType"`
		ServiceID   string `xml:"serviceId"`
		SCPDURL     string `xml:"SCPDURL"`
		ControlURL  string `xml:"controlURL"`
		EventSubURL string `xml:"eventSubURL"`
	}

	type serviceList struct {
		Services []service `xml:"service"`
	}

	type device struct {
		DeviceType   string      `xml:"deviceType"`
		FriendlyName string      `xml:"friendlyName"`
		Manufacturer string      `xml:"manufacturer"`
		ModelName    string      `xml:"modelName"`
		UDN          string      `xml:"UDN"`
		ServiceList  serviceList `xml:"serviceList"`
	}

	type deviceDescription = struct {
		XMLName     xml.Name    `xml:"urn:schemas-upnp-org:device-1-0 root"`
		ConfigID    int         `xml:"configId,attr"`
		SpecVersion SpecVersion `xml:"specVersion"`
		Device      device      `xml:"device"`
	}

	desc := deviceDescription{
		ConfigID: 0,
		SpecVersion: SpecVersion{
			Major: 1,
			Minor: 0,
		},
		Device: device{
			DeviceType:   d.Type,
			FriendlyName: "picoms",
			Manufacturer: "ichiban",
			ModelName:    serverProductToken,
			UDN:          fmt.Sprintf("uuid:%s", d.UUID),
		},
	}
	for i, s := range d.Services {
		desc.Device.ServiceList.Services = append(desc.Device.ServiceList.Services, service{
			ServiceType: s.Type,
			ServiceID:   s.ID,
			SCPDURL:     fmt.Sprintf("/%d/service", i),
			ControlURL:  fmt.Sprintf("/%d/control", i),
			EventSubURL: fmt.Sprintf("/%d/event", i),
		})
	}

	b, _ := xml.MarshalIndent(&desc, "", "  ")
	w.Header().Set("Content-Type", `text/xml; charset="utf-8"`)
	if _, err := w.Write([]byte(xmlDeclaration)); err != nil {
		log.WithFields(log.Fields{
			"err": err,
		}).Error("failed to write xml declaration")
		return
	}
	if _, err := w.Write(b); err != nil {
		log.WithFields(log.Fields{
			"err": err,
		}).Error("failed to write device declaration")
		return
	}
}

var netDial = net.Dial

func (d *Device) Advertise(done <-chan struct{}) error {
	fs := log.Fields{
		"uuid": d.UUID,
		"addr": multicastAddress,
	}
	log.WithFields(fs).Info("start advertising")
	defer log.WithFields(fs).Info("end advertising")

	c, err := netDial("udp", multicastAddress)
	if err != nil {
		return err
	}
	defer c.Close()

	defer d.bye(c)
	for {
		select {
		case <-done:
			return nil
		case <-time.After(d.Interval):
			if err := d.alive(c); err != nil {
				return err
			}
		}
	}
}

func (d *Device) alive(w io.Writer) error {
	uuid := fmt.Sprintf("uuid:%d", d.UUID)

	if err := d.notifyAlive(w, rootDevice, strings.Join([]string{uuid, rootDevice}, "::")); err != nil {
		return err
	}

	time.Sleep(time.Duration(rand.Intn(300)) * time.Millisecond)
	if err := d.notifyAlive(w, uuid, uuid); err != nil {
		return err
	}

	time.Sleep(time.Duration(rand.Intn(300)) * time.Millisecond)
	if err := d.notifyAlive(w, d.Type, strings.Join([]string{uuid, d.Type}, "::")); err != nil {
		return err
	}

	uniqServiceTypes := make(map[string]struct{}, len(d.Services))
	for _, s := range d.Services {
		uniqServiceTypes[s.Type] = struct{}{}
	}
	for t := range uniqServiceTypes {
		time.Sleep(time.Duration(rand.Intn(300)) * time.Millisecond)
		if err := d.notifyAlive(w, t, strings.Join([]string{uuid, t}, "::")); err != nil {
			return err
		}
	}

	return nil
}

func (d *Device) bye(w io.Writer) {
	uuid := fmt.Sprintf("uuid:%d", d.UUID)

	if err := d.notifyByeBye(w, rootDevice, strings.Join([]string{uuid, rootDevice}, "::")); err != nil {
		log.Print(err)
		return
	}

	time.Sleep(time.Duration(rand.Intn(300)) * time.Millisecond)
	if err := d.notifyByeBye(w, uuid, uuid); err != nil {
		log.Print(err)
		return
	}

	time.Sleep(time.Duration(rand.Intn(300)) * time.Millisecond)
	if err := d.notifyByeBye(w, d.Type, strings.Join([]string{uuid, d.Type}, "::")); err != nil {
		log.Print(err)
		return
	}

	uniqServiceTypes := make(map[string]struct{}, len(d.Services))
	for _, s := range d.Services {
		uniqServiceTypes[s.Type] = struct{}{}
	}
	for t := range uniqServiceTypes {
		time.Sleep(time.Duration(rand.Intn(300)) * time.Millisecond)
		if err := d.notifyByeBye(w, t, strings.Join([]string{uuid, t}, "::")); err != nil {
			log.Print(err)
			return
		}
	}
}

const (
	multicastAddress = "239.255.255.250:1900"
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

func (d *Device) notifyAlive(w io.Writer, nt, usn string) error {
	defer log.WithFields(log.Fields{"nt": nt, "usn": usn}).Debug("notify alive")

	r := http.Request{
		Method:     MethodNotify,
		URL:        &url.URL{Path: "*"},
		ProtoMajor: 1,
		ProtoMinor: 1,
		Header: http.Header{
			headerCacheControl:        []string{fmt.Sprintf("max-age = %d", d.Interval/time.Second)},
			headerLocation:            []string{d.url().String()},
			headerNotificationType:    []string{nt},
			headerNotificationSubType: []string{"ssdp:alive"},
			headerServer:              []string{d.productTokens()},
			headerUniqueServiceName:   []string{usn},
			headerBootID:              []string{"0"},
			headerConfigID:            []string{"0"},
			headerSearchPort:          []string{strconv.Itoa(d.SearchAddr.Port)},
		},
		Host: multicastAddress,
	}
	return r.Write(w)
}

func (d *Device) productTokens() string {
	return strings.Join([]string{osProductToken, upnpProductToken, serverProductToken}, " ")
}

func (d *Device) notifyByeBye(w io.Writer, nt, usn string) error {
	defer log.WithFields(log.Fields{"nt": nt, "usn": usn}).Debug("notify bye bye")

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
	return r.Write(w)
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

var netListenMulticastUDP = net.ListenMulticastUDP
var netListenUDP = net.ListenUDP

func (d *Device) ReplySearch(done <-chan struct{}) error {
	fs := log.Fields{
		"addr": d.Addr,
	}
	log.WithFields(fs).Info("start replying")
	defer log.WithFields(fs).Info("end replying")

	addr, err := net.ResolveUDPAddr("udp", multicastAddress)
	if err != nil {
		return fmt.Errorf("net.ResolveUDPAddr() failed: %w", err)
	}

	reqs := make(chan *http.Request)
	defer close(reqs)

	multi, err := netListenMulticastUDP("udp", d.Interface, addr)
	if err != nil {
		return fmt.Errorf("net.ListenMulticastUDP() failed: %w", err)
	}
	defer multi.Close()
	go readReqs(multi, reqs)

	uni, err := netListenUDP("udp", d.SearchAddr)
	if err != nil {
		d.SearchAddr.Port = 0
		uni, err = netListenUDP("udp", d.SearchAddr)
		if err != nil {
			return fmt.Errorf("net.ListenUDP() failed: %w", err)
		}
		d.SearchAddr = uni.LocalAddr().(*net.UDPAddr)
	}
	defer uni.Close()
	go readReqs(uni, reqs)

	for {
		select {
		case <-done:
			return nil
		case r := <-reqs:
			if err := d.respondSearch(r); err != nil {
				return err
			}
		}
	}
}

func (d *Device) respondSearch(r *http.Request) error {
	f := log.Fields{
		"addr":   r.RemoteAddr,
		"method": r.Method,
		"url":    r.URL,
	}
	for k, v := range r.Header {
		if len(v) == 0 {
			continue
		}
		f[k] = v[0]
	}
	log.WithFields(f).Debug("ssdp req")

	st := r.Header.Get(headerSearchTarget)
	var err error
	switch {
	case st == all:
		err = d.respondAll(r)
	case st == rootDevice:
		err = d.respondRootDevice(r)
	case st == fmt.Sprintf("uuid:%d", d.UUID):
		err = d.respondUUID(r)
	case st == d.Type:
		err = d.respondDevice(r)
	case strings.HasPrefix(st, "urn:schemas-upnp-org:service:"):
		for _, srv := range d.Services {
			if srv.Type == st {
				err = d.respondServices(r)
				break
			}
		}
	}
	return err
}

func (d *Device) respondAll(r *http.Request) error {
	uuid := fmt.Sprintf("uuid:%d", d.UUID)

	if err := d.respond(r, rootDevice, strings.Join([]string{uuid, rootDevice}, "::")); err != nil {
		return err
	}

	if err := d.respond(r, uuid, uuid); err != nil {
		return err
	}

	if err := d.respond(r, d.Type, strings.Join([]string{uuid, d.Type}, "::")); err != nil {
		return err
	}

	uniqServiceTypes := make(map[string]struct{}, len(d.Services))
	for _, s := range d.Services {
		uniqServiceTypes[s.Type] = struct{}{}
	}
	for t := range uniqServiceTypes {
		if err := d.respond(r, t, strings.Join([]string{uuid, t}, "::")); err != nil {
			return err
		}
	}

	return nil
}

func (d *Device) respondServices(r *http.Request) error {
	uuid := fmt.Sprintf("uuid:%d", d.UUID)

	uniqServiceTypes := make(map[string]struct{}, len(d.Services))
	for _, s := range d.Services {
		uniqServiceTypes[s.Type] = struct{}{}
	}
	for t := range uniqServiceTypes {
		if err := d.respond(r, t, strings.Join([]string{uuid, t}, "::")); err != nil {
			return err
		}
	}

	return nil
}

func (d *Device) respondDevice(r *http.Request) error {
	uuid := fmt.Sprintf("uuid:%d", d.UUID)

	if err := d.respond(r, d.Type, strings.Join([]string{uuid, d.Type}, "::")); err != nil {
		return err
	}
	return nil
}

func (d *Device) respondUUID(r *http.Request) error {
	uuid := fmt.Sprintf("uuid:%d", d.UUID)

	if err := d.respond(r, uuid, uuid); err != nil {
		return err
	}
	return nil
}

func (d *Device) respondRootDevice(r *http.Request) error {
	uuid := fmt.Sprintf("uuid:%d", d.UUID)

	if err := d.respond(r, rootDevice, strings.Join([]string{uuid, rootDevice}, "::")); err != nil {
		return err
	}
	return nil
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

func (d *Device) respond(r *http.Request, st, usn string) error {
	resp := http.Response{
		Status:     http.StatusText(http.StatusOK),
		StatusCode: http.StatusOK,
		ProtoMajor: 1,
		ProtoMinor: 1,
		Header: http.Header{
			headerCacheControl:      []string{fmt.Sprintf("max-age = %d", d.Interval/time.Second)},
			headerDate:              []string{time.Now().Format(time.RFC1123)},
			headerExt:               []string{""},
			headerLocation:          []string{d.url().String()},
			headerServer:            []string{d.productTokens()},
			headerSearchTarget:      []string{st},
			headerUniqueServiceName: []string{usn},
			headerBootID:            []string{"0"},
			headerConfigID:          []string{"0"},
			headerSearchPort:        []string{strconv.Itoa(d.SearchAddr.Port)},
		},
	}

	conn, err := net.Dial("udp", r.RemoteAddr)
	if err != nil {
		return fmt.Errorf("net.Dial() failed: %w", err)
	}
	defer conn.Close()

	f := log.Fields{
		"addr": r.RemoteAddr,
	}
	for k, v := range resp.Header {
		f[k] = v[0]
	}
	log.WithFields(f).Debug("ssdp resp")

	w := bufio.NewWriter(conn)
	if err := resp.Write(w); err != nil {
		return err
	}

	// It'd okay to fail because it'd UDP!
	if err := w.Flush(); err != nil {
		log.WithFields(log.Fields{
			"err": err,
		}).Debug("flush failed")
	}

	return nil
}

func (d *Device) url() *url.URL {
	return &url.URL{
		Scheme: "http",
		Host:   d.Addr,
		Path:   "/",
	}
}
