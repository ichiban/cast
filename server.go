package picoms

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"path"
	"strconv"
	"strings"
	"time"

	uuid "github.com/satori/go.uuid"
	log "github.com/sirupsen/logrus"
)

var (
	osProductToken     = "unknown/0.0"
	upnpProductToken   = "UPnP/2.0"
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

type Server struct {
	http.Server
	ContentDirectory

	UUID       uuid.UUID
	Interface  *net.Interface
	Interval   time.Duration
	SearchAddr *net.UDPAddr
}

func NewServer(i *net.Interface, httpPort, searchPort int, interval time.Duration) (*Server, error) {
	a, err := localAddress(i)
	if err != nil {
		return nil, err
	}

	sa, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", a, searchPort))
	if err != nil {
		return nil, err
	}

	s := Server{
		Server: http.Server{
			Addr: fmt.Sprintf("%s:%d", a, httpPort),
		},
		UUID:       uuid.NewV4(),
		Interface:  i,
		Interval:   interval,
		SearchAddr: sa,
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/", Describe(&s))
	mux.HandleFunc("/service", Describe(&s.ContentDirectory))
	mux.HandleFunc("/control", s.Control)
	mux.HandleFunc("/event", s.Event)
	s.Handler = requestLog(mux)

	return &s, nil
}

func (s *Server) URL(p ...string) *url.URL {
	return &url.URL{
		Scheme: "http",
		Host:   s.Addr,
		Path:   path.Join(p...),
	}
}

func (s *Server) Advertise(done <-chan struct{}) error {
	log.Printf("start advertising")
	defer log.Printf("end advertising")

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
		case <-time.Tick(s.Interval):
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
			headerLocation:            []string{s.URL("/").String()},
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
	log.Printf("start replying")
	defer log.Printf("end replying")

	addr, err := net.ResolveUDPAddr("udp", "239.255.255.250:1900")
	if err != nil {
		return err
	}

	reqs := make(chan *http.Request)
	defer close(reqs)

	multi, err := net.ListenMulticastUDP("udp", s.Interface, addr)
	if err != nil {
		return err
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
			return err
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
			headerLocation:          []string{s.URL("/").String()},
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
		return err
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
	return w.Flush()
}

type describer interface {
	Describe() ([]byte, error)
}

func Describe(d describer) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		b, err := d.Describe()
		if err != nil {
			panic(err)
		}
		w.Header().Set("Content-Type", `text/xml; charset="utf-8"`)
		w.Write(b)
	}
}

func requestLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t := time.Now()
		next.ServeHTTP(w, r)
		log.WithFields(log.Fields{
			"addr":    r.RemoteAddr,
			"method":  r.Method,
			"url":     r.URL,
			"elapsed": time.Since(t).Milliseconds(),
			"ua":      r.Header.Get("User-Agent"),
		}).Info("access")
	})
}
