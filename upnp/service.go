package upnp

import (
	"encoding/xml"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"reflect"
	"strconv"

	log "github.com/sirupsen/logrus"
)

const (
	deviceType = "urn:schemas-upnp-org:device:MediaServer:1"
)

const xmlDeclaration = "<?xml version=\"1.0\"?>\n"

type Service struct {
	ID   string
	Type string
	Desc *ServiceDescription
	Impl ServiceImplementation
}

type ServiceImplementation interface {
	http.Handler
	SetBaseURL(*url.URL)
}

func (s *Service) Describe(w http.ResponseWriter, r *http.Request) {
	b, err := xml.MarshalIndent(s.Desc, "", "  ")
	if err != nil {
		log.WithFields(log.Fields{
			"err": err,
		}).Error("failed to marshal service description")
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
		}).Error("failed to write body")
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
}

type argument struct {
	XMLName xml.Name
	Value   string `xml:",innerxml"`
}

type action struct {
	XMLName   xml.Name
	XMLNSU    string     `xml:"xmlns:u,attr"`
	Arguments []argument `xml:",any"`
}

type body struct {
	Action action `xml:",any"`
}

type requestEnvelope struct {
	XMLName xml.Name `xml:"Envelope"`
	Body    body
}

type responseEnvelope struct {
	XMLName       xml.Name `xml:"s:Envelope"`
	XMLNSS        string   `xml:"xmlns:s,attr"`
	EncodingStyle string   `xml:"s:encodingStyle,attr"`
	ResponseBody  body     `xml:"s:Body"`
}

func (s *Service) Control(w http.ResponseWriter, r *http.Request) {
	b, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.WithFields(log.Fields{
			"err": err,
		}).Error("failed to read body")
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	r.Body.Close()

	var req requestEnvelope
	if err := xml.Unmarshal(b, &req); err != nil {
		log.WithFields(log.Fields{
			"err": err,
		}).Error("failed to unmarshal request")
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	resp, err := s.call(&req)
	if err != nil {
		log.WithFields(log.Fields{
			"err": err,
		}).Error("failed to call method")
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	b, err = xml.MarshalIndent(resp, "", "  ")
	if err != nil {
		log.WithFields(log.Fields{
			"err": err,
		}).Error("failed to marshal response")
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

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
		}).Error("failed to write body")
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
}

func (s *Service) call(req *requestEnvelope) (*responseEnvelope, error) {
	v := reflect.ValueOf(s.Impl)

	name := req.Body.Action.XMLName.Local
	a := s.Desc.ActionByName(name)

	var in []reflect.Value
	for _, arg := range req.Body.Action.Arguments {
		t := v.Elem().FieldByName(a.ArgumentByName(arg.XMLName.Local).RelatedStateVariable).Type()
		switch t.Kind() {
		case reflect.String:
			in = append(in, reflect.ValueOf(arg.Value))
		case reflect.Uint32:
			n, err := strconv.ParseUint(arg.Value, 10, 32)
			if err != nil {
				return nil, err
			}
			in = append(in, reflect.ValueOf(uint32(n)))
		default:
			return nil, fmt.Errorf("unknown argument type: %v", t)
		}
	}

	m := v.MethodByName(name)
	if m.Kind() == reflect.Invalid {
		return nil, fmt.Errorf("method not implemented: %s", name)
	}
	out := m.Call(in)

	last := out[len(out)-1]
	if last.Type() == reflect.TypeOf((*error)(nil)).Elem() {
		out = out[:len(out)-1]
		if err, ok := last.Interface().(error); ok {
			// TODO: return soap error
			return nil, fmt.Errorf("method %s failed: %v", name, err)
		}
	}

	resp := responseEnvelope{
		XMLNSS:        "http://schemas.xmlsoap.org/soap/envelope/",
		EncodingStyle: "http://schemas.xmlsoap.org/soap/encoding/",
		ResponseBody: body{
			Action: action{
				XMLName: xml.Name{
					Local: "u:" + name + "Response",
				},
				XMLNSU: s.Type,
			},
		},
	}

	for i, o := range out {
		arg := a.OutArgumentByIndex(i)
		resp.ResponseBody.Action.Arguments = append(resp.ResponseBody.Action.Arguments, argument{
			XMLName: xml.Name{
				Local: arg.Name,
			},
			Value: fmt.Sprint(o.Interface()),
		})
	}
	return &resp, nil
}

func (s *Service) Event(w http.ResponseWriter, r *http.Request) {
	log.WithFields(log.Fields{
		"method": r.Method,
		"url":    r.URL,
		"ua":     r.Header.Get("USER-AGENT"),
		"cb":     r.Header.Get("CALLBACK"),
		"to":     r.Header.Get("TIMEOUT"),
		"addr":   r.RemoteAddr,
	}).Info("Event")
	w.WriteHeader(http.StatusNotImplemented)
}

func (s *Service) Handle(w http.ResponseWriter, r *http.Request) {
	if h, ok := s.Impl.(http.Handler); ok {
		h.ServeHTTP(w, r)
		return
	}
	w.WriteHeader(http.StatusNotFound)
}

type SpecVersion struct {
	Major int `xml:"major"`
	Minor int `xml:"minor"`
}

type ServiceDescription struct {
	XMLName           xml.Name          `xml:"urn:schemas-upnp-org:Service-1-0 scpd"`
	SpecVersion       SpecVersion       `xml:"specVersion"`
	ActionList        ActionList        `xml:"actionList"`
	ServiceStateTable StateVariableList `xml:"serviceStateTable"`
}

func (s *ServiceDescription) ActionByName(name string) *Action {
	for _, a := range s.ActionList.Actions {
		if a.Name == name {
			return &a
		}
	}
	return nil
}

type ActionList struct {
	Actions []Action `xml:"action"`
}

type Action struct {
	Name         string       `xml:"name"`
	ArgumentList ArgumentList `xml:"argumentList"`
}

func (a *Action) ArgumentByName(name string) *Argument {
	for _, a := range a.ArgumentList.Arguments {
		if a.Name == name {
			return &a
		}
	}
	return nil
}

func (a *Action) OutArgumentByIndex(i int) *Argument {
	return &a.out()[i]
}

func (a *Action) out() []Argument {
	for i, arg := range a.ArgumentList.Arguments {
		if arg.Direction == Out {
			return a.ArgumentList.Arguments[i:]
		}
	}
	return nil
}

type ArgumentList struct {
	Arguments []Argument `xml:"argument"`
}

type Argument struct {
	Name                 string    `xml:"name"`
	Direction            direction `xml:"direction"`
	RelatedStateVariable string    `xml:"relatedStateVariable"`
}

type direction int

func (d direction) String() string {
	switch d {
	case In:
		return "in"
	case Out:
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
	In direction = iota
	Out
)

type StateVariableList struct {
	StateVariables []StateVariable `xml:"stateVariable"`
}

type StateVariable struct {
	SendEvents       binary            `xml:"sendEvents,attr"`
	Name             string            `xml:"name"`
	DataType         string            `xml:"dataType"`
	AllowedValueList *AllowedValueList `xml:"allowedValueList,omitempty"`
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

type AllowedValueList struct {
	AllowedValues []string `xml:"allowedValue"`
}
