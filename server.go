package picoms

import (
	"bufio"
	"bytes"
	"context"
	"encoding/xml"
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
	mux.HandleFunc("/", s.DescribeDevice)
	mux.HandleFunc("/service", s.DescribeService)
	mux.HandleFunc("/control", s.Control)
	mux.HandleFunc("/event", s.Event)
	s.Handler = requestLog(mux)

	return &s, nil
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.Server.Shutdown(ctx)
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

func (s *Server) DescribeDevice(w http.ResponseWriter, r *http.Request) {
	b, err := xml.MarshalIndent(&deviceDescription{
		ConfigID: 0,
		SpecVersion: specVersion{
			Major: 1,
			Minor: 0,
		},
		Device: device{
			DeviceType:   "urn:schemas-upnp-org:device:MediaServer:1",
			FriendlyName: "picoms",
			Manufacturer: "ichiban",
			ModelName:    serverProductToken,
			UDN:          fmt.Sprintf("uuid:%s", s.UUID),
			ServiceList: serviceList{
				Services: []service{
					{
						ServiceType: "urn:schemas-upnp-org:service:ContentDirectory:1",
						ServiceID:   "urn:upnp-org:serviceId:ContentDirectory",
						SCPDURL:     "/service",
						ControlURL:  "/control",
						EventSubURL: "/event",
					},
				},
			},
		},
	}, "", "  ")
	if err != nil {
		panic(err)
	}
	w.Header().Set("Content-Type", `text/xml; charset="utf-8"`)
	if _, err := w.Write([]byte("<?xml version=\"1.0\"?>\n")); err != nil {
		panic(err)
	}
	if _, err := w.Write(b); err != nil {
		panic(err)
	}
}

type deviceDescription = struct {
	XMLName     xml.Name    `xml:"urn:schemas-upnp-org:device-1-0 root"`
	ConfigID    int         `xml:"configId,attr"`
	SpecVersion specVersion `xml:"specVersion"`
	Device      device      `xml:"device"`
}

type specVersion struct {
	Major int `xml:"major"`
	Minor int `xml:"minor"`
}

type device struct {
	DeviceType   string      `xml:"deviceType"`
	FriendlyName string      `xml:"friendlyName"`
	Manufacturer string      `xml:"manufacturer"`
	ModelName    string      `xml:"modelName"`
	UDN          string      `xml:"UDN"`
	ServiceList  serviceList `xml:"serviceList"`
}

type serviceList struct {
	Services []service `xml:"service"`
}

type service struct {
	ServiceType string `xml:"serviceType"`
	ServiceID   string `xml:"serviceId"`
	SCPDURL     string `xml:"SCPDURL"`
	ControlURL  string `xml:"controlURL"`
	EventSubURL string `xml:"eventSubURL"`
}

const argTypeObjectID = "A_ARG_TYPE_ObjectID"
const argTypeResult = "A_ARG_TYPE_Result"
const argTypeSearchCriteria = "A_ARG_TYPE_SearchCriteria"
const argTypeBrowseFlag = "A_ARG_TYPE_BrowseFlag"
const argTypeFilter = "A_ARG_TYPE_Filter"
const argTypeSortCriteria = "A_ARG_TYPE_SortCriteria"
const argTypeIndex = "A_ARG_TYPE_Index"
const argTypeCount = "A_ARG_TYPE_Count"
const argTypeUpdateID = "A_ARG_TYPE_UpdateID"
const argTypeTransferID = "A_ARG_TYPE_TransferID"
const argTypeTransferStatus = "A_ARG_TYPE_TransferStatus"
const argTypeTransferLength = "A_ARG_TYPE_TransferLength"
const argTypeTransferTotal = "A_ARG_TYPE_TransferTotal"
const argTypeTagValueList = "A_ARG_TYPE_TagValueList"
const argTypeURI = "A_ARG_TYPE_URI"

const dataTypeString = "string"
const dataTypeUI4 = "ui4"
const dataTypeURI = "uri"

func (s *Server) DescribeService(w http.ResponseWriter, r *http.Request) {
	b, err := xml.MarshalIndent(&serviceDescription{
		SpecVersion: specVersion{
			Major: 1,
			Minor: 0,
		},
		ActionList: actionList{
			Actions: []action{
				{
					Name: "GetSearchCapabilities",
					ArgumentList: argumentList{
						Arguments: []argument{
							{
								Name:                 "SearchCaps",
								Direction:            out,
								RelatedStateVariable: "SearchCapabilities",
							},
						},
					},
				},
				{
					Name: "GetSortCapabilities",
					ArgumentList: argumentList{
						Arguments: []argument{
							{
								Name:                 "SortCaps",
								Direction:            out,
								RelatedStateVariable: "SortCapabilities",
							},
						},
					},
				},
				{
					Name: "GetSystemUpdateID",
					ArgumentList: argumentList{
						Arguments: []argument{
							{
								Name:                 "Id",
								Direction:            out,
								RelatedStateVariable: "SystemUpdateID",
							},
						},
					},
				},
				{
					Name: "Browse",
					ArgumentList: argumentList{
						Arguments: []argument{
							{
								Name:                 "ObjectID",
								Direction:            in,
								RelatedStateVariable: argTypeObjectID,
							},
							{
								Name:                 "BrowseFlag",
								Direction:            in,
								RelatedStateVariable: argTypeBrowseFlag,
							},
							{
								Name:                 "Filter",
								Direction:            in,
								RelatedStateVariable: argTypeFilter,
							},
							{
								Name:                 "StartingIndex",
								Direction:            in,
								RelatedStateVariable: argTypeIndex,
							},
							{
								Name:                 "RequestedCount",
								Direction:            in,
								RelatedStateVariable: argTypeCount,
							},
							{
								Name:                 "SortCriteria",
								Direction:            in,
								RelatedStateVariable: argTypeSortCriteria,
							},
							{
								Name:                 "Result",
								Direction:            out,
								RelatedStateVariable: argTypeResult,
							},
							{
								Name:                 "NumberReturned",
								Direction:            out,
								RelatedStateVariable: argTypeCount,
							},
							{
								Name:                 "TotalMatches",
								Direction:            out,
								RelatedStateVariable: argTypeCount,
							},
							{
								Name:                 "UpdateID",
								Direction:            out,
								RelatedStateVariable: argTypeUpdateID,
							},
						},
					},
				},
				{
					Name: "Search",
					ArgumentList: argumentList{
						Arguments: []argument{
							{
								Name:                 "ContainerID",
								Direction:            in,
								RelatedStateVariable: argTypeObjectID,
							},
							{
								Name:                 "SearchCriteria",
								Direction:            in,
								RelatedStateVariable: argTypeSearchCriteria,
							},
							{
								Name:                 "Filter",
								Direction:            in,
								RelatedStateVariable: argTypeFilter,
							},
							{
								Name:                 "StartingIndex",
								Direction:            in,
								RelatedStateVariable: argTypeIndex,
							},
							{
								Name:                 "RequestedCount",
								Direction:            in,
								RelatedStateVariable: argTypeCount,
							},
							{
								Name:                 "SortCriteria",
								Direction:            in,
								RelatedStateVariable: argTypeSortCriteria,
							},
							{
								Name:                 "Result",
								Direction:            out,
								RelatedStateVariable: argTypeResult,
							},
							{
								Name:                 "NumberReturned",
								Direction:            out,
								RelatedStateVariable: argTypeCount,
							},
							{
								Name:                 "TotalMatches",
								Direction:            out,
								RelatedStateVariable: argTypeCount,
							},
							{
								Name:                 "UpdateID",
								Direction:            out,
								RelatedStateVariable: argTypeUpdateID,
							},
						},
					},
				},
				{
					Name: "CreateObject",
					ArgumentList: argumentList{
						Arguments: []argument{
							{
								Name:                 "ContainerID",
								Direction:            in,
								RelatedStateVariable: argTypeObjectID,
							},
							{
								Name:                 "Elements",
								Direction:            in,
								RelatedStateVariable: argTypeResult,
							},
							{
								Name:                 "ObjectID",
								Direction:            out,
								RelatedStateVariable: argTypeObjectID,
							},
							{
								Name:                 "Result",
								Direction:            out,
								RelatedStateVariable: argTypeResult,
							},
						},
					},
				},
				{
					Name: "DestroyObject",
					ArgumentList: argumentList{
						Arguments: []argument{
							{
								Name:                 "ObjectID",
								Direction:            in,
								RelatedStateVariable: argTypeObjectID,
							},
						},
					},
				},
				{
					Name: "UpdateObject",
					ArgumentList: argumentList{
						Arguments: []argument{
							{
								Name:                 "ObjectID",
								Direction:            in,
								RelatedStateVariable: argTypeObjectID,
							},
							{
								Name:                 "CurrentTagValue",
								Direction:            in,
								RelatedStateVariable: argTypeTagValueList,
							},
							{
								Name:                 "NewTagValue",
								Direction:            in,
								RelatedStateVariable: argTypeTagValueList,
							},
						},
					},
				},
				{
					Name: "ImportResource",
					ArgumentList: argumentList{
						Arguments: []argument{
							{
								Name:                 "SourceURI",
								Direction:            in,
								RelatedStateVariable: argTypeURI,
							},
							{
								Name:                 "DestinationURI",
								Direction:            in,
								RelatedStateVariable: argTypeURI,
							},
							{
								Name:                 "TransferID",
								Direction:            out,
								RelatedStateVariable: argTypeTransferID,
							},
						},
					},
				},
				{
					Name: "ExportResource",
					ArgumentList: argumentList{
						Arguments: []argument{
							{
								Name:                 "SourceURI",
								Direction:            in,
								RelatedStateVariable: argTypeURI,
							},
							{
								Name:                 "DestinationURI",
								Direction:            in,
								RelatedStateVariable: argTypeURI,
							},
							{
								Name:                 "TransferID",
								Direction:            out,
								RelatedStateVariable: argTypeTransferID,
							},
						},
					},
				},
				{
					Name: "StopTransferResource",
					ArgumentList: argumentList{
						Arguments: []argument{
							{
								Name:                 "TransferID",
								Direction:            in,
								RelatedStateVariable: argTypeTransferID,
							},
						},
					},
				},
				{
					Name: "GetTransferProgress",
					ArgumentList: argumentList{
						Arguments: []argument{
							{
								Name:                 "TransferID",
								Direction:            in,
								RelatedStateVariable: argTypeTransferID,
							},
							{
								Name:                 "TransferStatus",
								Direction:            out,
								RelatedStateVariable: argTypeTransferStatus,
							},
							{
								Name:                 "TransferLength",
								Direction:            out,
								RelatedStateVariable: argTypeTransferLength,
							},
							{
								Name:                 "TransferTotal",
								Direction:            out,
								RelatedStateVariable: argTypeTransferTotal,
							},
						},
					},
				},
				{
					Name: "DeleteResource",
					ArgumentList: argumentList{
						Arguments: []argument{
							{
								Name:                 "ResourceURI",
								Direction:            in,
								RelatedStateVariable: argTypeURI,
							},
						},
					},
				},
				{
					Name: "CreateReference",
					ArgumentList: argumentList{
						Arguments: []argument{
							{
								Name:                 "ContainerID",
								Direction:            in,
								RelatedStateVariable: argTypeObjectID,
							},
							{
								Name:                 "ObjectID",
								Direction:            in,
								RelatedStateVariable: argTypeObjectID,
							},
							{
								Name:                 "NewID",
								Direction:            out,
								RelatedStateVariable: argTypeObjectID,
							},
						},
					},
				},
			},
		},
		ServiceStateTable: stateVariableList{
			StateVariables: []stateVariable{
				{
					SendEvents: true,
					Name:       "TransferIDs",
					DataType:   dataTypeString,
				},
				{
					Name:     argTypeObjectID,
					DataType: dataTypeString,
				},
				{
					Name:     argTypeResult,
					DataType: dataTypeString,
				},
				{
					Name:     argTypeSearchCriteria,
					DataType: dataTypeString,
				},
				{
					Name:     argTypeBrowseFlag,
					DataType: dataTypeString,
					AllowedValueList: &allowedValueList{
						allowedValues: []string{
							"BrowseMetadata",
							"BrowseDirectChildren",
						},
					},
				},
				{
					Name:     argTypeFilter,
					DataType: dataTypeString,
				},
				{
					Name:     argTypeSortCriteria,
					DataType: dataTypeString,
				},
				{
					Name:     argTypeIndex,
					DataType: dataTypeUI4,
				},
				{
					Name:     argTypeCount,
					DataType: dataTypeUI4,
				},
				{
					Name:     argTypeUpdateID,
					DataType: dataTypeUI4,
				},
				{
					Name:     argTypeTransferID,
					DataType: dataTypeUI4,
				},
				{
					Name:     argTypeTransferStatus,
					DataType: dataTypeString,
					AllowedValueList: &allowedValueList{
						allowedValues: []string{
							"COMPLETED",
							"ERROR",
							"IN_PROGRESS",
							"STOPPED",
						},
					},
				},
				{
					Name:     argTypeTransferLength,
					DataType: dataTypeString,
				},
				{
					Name:     argTypeTransferTotal,
					DataType: dataTypeString,
				},
				{
					Name:     argTypeTagValueList,
					DataType: dataTypeString,
				},
				{
					Name:     argTypeURI,
					DataType: dataTypeURI,
				},
				{
					Name:     "SearchCapabilities",
					DataType: dataTypeString,
				},
				{
					Name:     "SortCapabilities",
					DataType: dataTypeString,
				},
				{
					SendEvents: true,
					Name:       "SystemUpdateID",
					DataType:   dataTypeUI4,
				},
				{
					SendEvents: true,
					Name:       "ContainerUpdateIDs",
					DataType:   dataTypeString,
				},
			},
		},
	}, "", "  ")
	if err != nil {
		panic(err)
	}

	w.Header().Set("Content-Type", `text/xml; charset="utf-8"`)
	if _, err := w.Write([]byte("<?xml version=\"1.0\"?>\n")); err != nil {
		panic(err)
	}
	if _, err := w.Write(b); err != nil {
		panic(err)
	}
}

type serviceDescription struct {
	XMLName           xml.Name          `xml:"urn:schemas-upnp-org:service-1-0 scpd"`
	SpecVersion       specVersion       `xml:"specVersion"`
	ActionList        actionList        `xml:"actionList"`
	ServiceStateTable stateVariableList `xml:"serviceStateTable"`
}

type actionList struct {
	Actions []action `xml:"action"`
}

type action struct {
	Name         string       `xml:"name"`
	ArgumentList argumentList `xml:"argumentList"`
}

type argumentList struct {
	Arguments []argument `xml:"argument"`
}

type argument struct {
	Name                 string    `xml:"name"`
	Direction            direction `xml:"direction"`
	RelatedStateVariable string    `xml:"relatedStateVariable"`
}

type direction int

func (d direction) String() string {
	switch d {
	case in:
		return "in"
	case out:
		return "out"
	default:
		return ""
	}
}

func (d direction) MarshalText() ([]byte, error) {
	s := d.String()
	if s == "" {
		return nil, errors.New("unknown direction")
	}
	return []byte(s), nil
}

const (
	in direction = iota
	out
)

type stateVariableList struct {
	StateVariables []stateVariable `xml:"stateVariable"`
}

type stateVariable struct {
	SendEvents       binary            `xml:"sendEvents,attr"`
	Name             string            `xml:"name"`
	DataType         string            `xml:"dataType"`
	AllowedValueList *allowedValueList `xml:"allowedValueList,omitempty"`
}

type binary bool

func (b binary) String() string {
	if b {
		return "yes"
	} else {
		return "no"
	}
}

func (b binary) MarshalText() (text []byte, err error) {
	return []byte(b.String()), nil
}

type allowedValueList struct {
	allowedValues []string `xml:"allowedValue"`
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

func (s *Server) Control(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func (s *Server) Event(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}
