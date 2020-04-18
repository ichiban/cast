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

type Server struct {
	http.Server

	UUID       uuid.UUID
	Interface  *net.Interface
	Interval   time.Duration
	SearchAddr *net.UDPAddr
	Services   []Service
}

func NewServer(i *net.Interface, services []Service) (*Server, error) {
	a, err := localAddress(i)
	if err != nil {
		return nil, err
	}

	sa, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", a, defaultSearchPort))
	if err != nil {
		return nil, err
	}

	addr := fmt.Sprintf("%s:%d", a, defaultHTTPPort)

	s := Server{
		Server: http.Server{
			Addr: addr,
		},

		UUID:       uuid.NewV4(),
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

func (s *Server) Describe(w http.ResponseWriter, r *http.Request) {
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

	b, err := xml.MarshalIndent(deviceDescription{
		ConfigID: 0,
		SpecVersion: SpecVersion{
			Major: 1,
			Minor: 0,
		},
		Device: device{
			DeviceType:   deviceType,
			FriendlyName: "picoms",
			Manufacturer: "ichiban",
			ModelName:    serverProductToken,
			UDN:          fmt.Sprintf("uuid:%s", s.UUID),
			ServiceList: serviceList{
				Services: []service{
					{
						ServiceType: serviceType,
						ServiceID:   serviceID,
						SCPDURL:     "/0/service",
						ControlURL:  "/0/control",
						EventSubURL: "/0/event",
					},
				},
			},
		},
	}, "", "  ")
	if err != nil {
		log.WithFields(log.Fields{
			"err": err,
		}).Error("failed to marshal device description")
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", `text/xml; charset="utf-8"`)
	if _, err := w.Write([]byte(xmlDeclaration)); err != nil {
		log.WithFields(log.Fields{
			"err": err,
		}).Error("failed to write xml declaration")
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	if _, err := w.Write(b); err != nil {
		log.WithFields(log.Fields{
			"err": err,
		}).Error("failed to write device declaration")
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
}

func (s *Server) Advertise(done <-chan struct{}) error {
	fs := log.Fields{
		"uuid": s.UUID,
		"addr": multicastAddress,
	}
	log.WithFields(fs).Info("start advertising")
	defer log.WithFields(fs).Info("end advertising")

	c, err := net.Dial("udp", multicastAddress)
	if err != nil {
		return err
	}
	defer c.Close()

	defer func() {
		if err := s.notifyByeBye(c, "upnp:rootdevice", fmt.Sprintf("uuid:%s::upnp:rootdevice", s.UUID)); err != nil {
			log.Print(err)
			return
		}
		time.Sleep(time.Duration(rand.Intn(300)) * time.Millisecond)
		if err := s.notifyByeBye(c, fmt.Sprintf("uuid:%s", s.UUID), fmt.Sprintf("uuid:%s", s.UUID)); err != nil {
			log.Print(err)
			return
		}
		time.Sleep(time.Duration(rand.Intn(300)) * time.Millisecond)
		if err := s.notifyByeBye(c, "urn:schemas-upnp-org:device:MediaServer:1", fmt.Sprintf("uuid:%s::urn:schemas-upnp-org:device:MediaServer:1", s.UUID)); err != nil {
			log.Print(err)
			return
		}
		time.Sleep(time.Duration(rand.Intn(300)) * time.Millisecond)
		if err := s.notifyByeBye(c, "urn:schemas-upnp-org:service:ContentDirectory:1", fmt.Sprintf("uuid:%s::urn:schemas-upnp-org:service:ContentDirectory:1", s.UUID)); err != nil {
			log.Print(err)
			return
		}
	}()

	for {
		select {
		case <-done:
			return nil
		case <-time.After(s.Interval):
			if err := s.notifyAlive(c, "upnp:rootdevice", fmt.Sprintf("uuid:%s::upnp:rootdevice", s.UUID)); err != nil {
				return err
			}
			time.Sleep(time.Duration(rand.Intn(300)) * time.Millisecond)
			if err := s.notifyAlive(c, fmt.Sprintf("uuid:%s", s.UUID), fmt.Sprintf("uuid:%s", s.UUID)); err != nil {
				return err
			}
			time.Sleep(time.Duration(rand.Intn(300)) * time.Millisecond)
			if err := s.notifyAlive(c, "urn:schemas-upnp-org:device:MediaServer:1", fmt.Sprintf("uuid:%s::urn:schemas-upnp-org:device:MediaServer:1", s.UUID)); err != nil {
				return err
			}
			time.Sleep(time.Duration(rand.Intn(300)) * time.Millisecond)
			if err := s.notifyAlive(c, "urn:schemas-upnp-org:service:ContentDirectory:1", fmt.Sprintf("uuid:%s::urn:schemas-upnp-org:service:ContentDirectory:1", s.UUID)); err != nil {
				return err
			}
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

func (s *Server) notifyAlive(w io.Writer, nt, usn string) error {
	defer log.WithFields(log.Fields{"nt": nt, "usn": usn}).Debug("notify alive")

	r := http.Request{
		Method:     MethodNotify,
		URL:        &url.URL{Path: "*"},
		ProtoMajor: 1,
		ProtoMinor: 1,
		Header: http.Header{
			headerCacheControl:        []string{fmt.Sprintf("max-age = %d", s.Interval/time.Second)},
			headerLocation:            []string{s.url().String()},
			headerNotificationType:    []string{nt},
			headerNotificationSubType: []string{"ssdp:alive"},
			headerServer:              []string{s.productTokens()},
			headerUniqueServiceName:   []string{usn},
			headerBootID:              []string{"0"},
			headerConfigID:            []string{"0"},
			headerSearchPort:          []string{strconv.Itoa(s.SearchAddr.Port)},
		},
		Host: multicastAddress,
	}
	return r.Write(w)
}

func (s *Server) productTokens() string {
	return strings.Join([]string{osProductToken, upnpProductToken, serverProductToken}, " ")
}

func (s *Server) notifyByeBye(w io.Writer, nt, usn string) error {
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

func (s *Server) ReplySearch(done <-chan struct{}) error {
	fs := log.Fields{
		"addr": s.Addr,
	}
	log.WithFields(fs).Info("start replying")
	defer log.WithFields(fs).Info("end replying")

	addr, err := net.ResolveUDPAddr("udp", multicastAddress)
	if err != nil {
		return fmt.Errorf("net.ResolveUDPAddr() failed: %w", err)
	}

	reqs := make(chan *http.Request)
	defer close(reqs)

	multi, err := net.ListenMulticastUDP("udp", s.Interface, addr)
	if err != nil {
		return fmt.Errorf("net.ListenMulticastUDP() failed: %w", err)
	}
	defer multi.Close()

	go func() {
		b := make([]byte, 1024)

		for {
			_, addr, err := multi.ReadFromUDP(b)
			if err != nil {
				continue
			}
			r, err := http.ReadRequest(bufio.NewReader(bytes.NewReader(b)))
			if err != nil {
				continue
			}
			r.RemoteAddr = addr.String()
			reqs <- r
		}
	}()

	uni, err := net.ListenUDP("udp", s.SearchAddr)
	if err != nil {
		s.SearchAddr.Port = 0
		uni, err = net.ListenUDP("udp", s.SearchAddr)
		if err != nil {
			return fmt.Errorf("net.ListenUDP() failed: %w", err)
		}
		s.SearchAddr = uni.LocalAddr().(*net.UDPAddr)
	}
	defer uni.Close()

	go func() {
		b := make([]byte, 1024)

		for {
			_, addr, err := multi.ReadFromUDP(b)
			if err != nil {
				continue
			}
			r, err := http.ReadRequest(bufio.NewReader(bytes.NewReader(b)))
			if err != nil {
				continue
			}
			r.RemoteAddr = addr.String()
			reqs <- r
		}
	}()

	for {
		select {
		case <-done:
			return nil
		case r := <-reqs:
			if r.Method != MethodMSearch {
				continue
			}

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

			switch r.Header.Get(headerSearchTarget) {
			case "ssdp:all":
				if err := s.respond(r, "upnp:rootdevice", fmt.Sprintf("uuid:%s::upnp:rootdevice", s.UUID)); err != nil {
					return err
				}
				if err := s.respond(r, fmt.Sprintf("uuid:%s", s.UUID), fmt.Sprintf("uuid:%s", s.UUID)); err != nil {
					return err
				}
				if err := s.respond(r, "urn:schemas-upnp-org:device:MediaServer:1", fmt.Sprintf("uuid:%s::urn:schemas-upnp-org:device:MediaServer:1", s.UUID)); err != nil {
					return err
				}
				if err := s.respond(r, "urn:schemas-upnp-org:service:ContentDirectory:1", fmt.Sprintf("uuid:%s::urn:schemas-upnp-org:service:ContentDirectory:1", s.UUID)); err != nil {
					return err
				}
			case "upnp:rootdevice":
				if err := s.respond(r, "upnp:rootdevice", fmt.Sprintf("uuid:%s::upnp:rootdevice", s.UUID)); err != nil {
					return err
				}
			case fmt.Sprintf("uuid:%s", s.UUID):
				if err := s.respond(r, fmt.Sprintf("uuid:%s", s.UUID), fmt.Sprintf("uuid:%s", s.UUID)); err != nil {
					return err
				}
			case "urn:schemas-upnp-org:device:MediaServer:1":
				if err := s.respond(r, "urn:schemas-upnp-org:device:MediaServer:1", fmt.Sprintf("uuid:%s::urn:schemas-upnp-org:device:MediaServer:1", s.UUID)); err != nil {
					return err
				}
			case "urn:schemas-upnp-org:service:ContentDirectory:1":
				if err := s.respond(r, "urn:schemas-upnp-org:service:ContentDirectory:1", fmt.Sprintf("uuid:%s::urn:schemas-upnp-org:service:ContentDirectory:1", s.UUID)); err != nil {
					return err
				}
			}
		}
	}
}

func (s *Server) respond(r *http.Request, st, usn string) error {
	resp := http.Response{
		Status:     http.StatusText(http.StatusOK),
		StatusCode: http.StatusOK,
		ProtoMajor: 1,
		ProtoMinor: 1,
		Header: http.Header{
			headerCacheControl:      []string{fmt.Sprintf("max-age = %d", s.Interval/time.Second)},
			headerDate:              []string{time.Now().Format(time.RFC1123)},
			headerExt:               []string{""},
			headerLocation:          []string{s.url().String()},
			headerServer:            []string{s.productTokens()},
			headerSearchTarget:      []string{st},
			headerUniqueServiceName: []string{usn},
			headerBootID:            []string{"0"},
			headerConfigID:          []string{"0"},
			headerSearchPort:        []string{strconv.Itoa(s.SearchAddr.Port)},
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

	// It's okay to fail because it's UDP!
	if err := w.Flush(); err != nil {
		log.WithFields(log.Fields{
			"err": err,
		}).Debug("flush failed")
	}

	return nil
}

func (s *Server) url() *url.URL {
	return &url.URL{
		Scheme: "http",
		Host:   s.Addr,
		Path:   "/",
	}
}
