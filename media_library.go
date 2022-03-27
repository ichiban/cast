package cast

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io/fs"
	"io/ioutil"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gabriel-vasile/mimetype"
	log "github.com/sirupsen/logrus"
)

const xmlDeclaration = "<?xml version=\"1.0\"?>\n"

type MediaLibrary struct {
	Items MediaItems
}

func NewMediaLibrary(baseURL *url.URL, dir string) (*MediaLibrary, error) {
	var (
		items        []MediaItem
		id           int
		containerIDs = map[string]int{}
	)
	if err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		parentID := containerIDs[filepath.Dir(path)]

		if d.IsDir() {
			items = append(items, MediaItem{
				ID:       id,
				ParentID: parentID,
				Title:    d.Name(),
				Class:    MediaClassStorageFolder,
			})
			containerIDs[path] = id
		} else {
			var (
				class = MediaClassItem
				mime  = "*"
			)
			if m, err := mimetype.DetectFile(path); err == nil {
				mime = m.String()
				switch strings.Split(mime, "/")[0] {
				case "image":
					class = MediaClassImageItem
				case "audio":
					class = MediaClassAudioItem
				case "video":
					class = MediaClassVideoItem
				}
			}

			rel, err := filepath.Rel(dir, path)
			if err != nil {
				return err
			}

			items = append(items, MediaItem{
				ID:           id,
				ParentID:     parentID,
				Title:        d.Name(),
				Class:        class,
				ProtocolInfo: fmt.Sprintf("http-get:*:%s:*", mime),
				URL:          baseURL.ResolveReference(&url.URL{Path: rel}),
			})
		}

		id++
		return nil
	}); err != nil {
		return nil, err
	}
	return &MediaLibrary{Items: items}, nil
}

func (m *MediaLibrary) Control(w http.ResponseWriter, r *http.Request) {
	b, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.WithError(err).Error("failed to read requestBody")
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	if err := r.Body.Close(); err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	var req requestEnvelope
	if err := xml.Unmarshal(b, &req); err != nil {
		log.WithError(err).Error("failed to unmarshal request")
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	p := req.Body.Action

	fs := make(log.Fields, len(p.Arguments)+1)
	fs["addr"] = r.RemoteAddr
	for _, arg := range p.Arguments {
		fs[arg.XMLName.Local] = arg.Value
	}
	log.WithFields(fs).Info(p.XMLName.Local)

	resp, err := m.control(p)
	if err != nil {
		log.WithError(err).Error("failed to control method")
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	b, err = xml.MarshalIndent(responseEnvelope{
		XMLNSS:        "http://schemas.xmlsoap.org/soap/envelope/",
		EncodingStyle: "http://schemas.xmlsoap.org/soap/encoding/",
		ResponseBody: responseBody{
			ActionResponse: resp,
		},
	}, "", "  ")
	if err != nil {
		log.WithError(err).Error("failed to marshal response")
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	if _, err := w.Write([]byte(xmlDeclaration)); err != nil {
		log.WithError(err).Error("failed to write xml declaration")
		return
	}
	if _, err := w.Write(b); err != nil {
		log.WithError(err).Error("failed to write requestBody")
		return
	}
}

var handlers = map[string]func(*MediaLibrary, *action) (*actionResponse, error){
	"GetSearchCapabilities":         (*MediaLibrary).getSearchCapabilities,
	"GetSortCapabilities":           (*MediaLibrary).getSortCapabilities,
	"GetSortExtensionCapabilities":  (*MediaLibrary).getSortExtensionCapabilities,
	"GetFeatureList":                (*MediaLibrary).getFeatureList,
	"GetSystemUpdateID":             (*MediaLibrary).getSystemUpdateID,
	"GetServiceResetToken":          (*MediaLibrary).getServiceResetToken,
	"Browse":                        (*MediaLibrary).browse,
	"Search":                        (*MediaLibrary).search,
	"CreateObject":                  (*MediaLibrary).createObject,
	"DestroyObject":                 (*MediaLibrary).destroyObject,
	"UpdateObject":                  (*MediaLibrary).updateObject,
	"MoveObject":                    (*MediaLibrary).moveObject,
	"ImportResource":                (*MediaLibrary).importResource,
	"ExportResource":                (*MediaLibrary).exportResource,
	"StopTransferResource":          (*MediaLibrary).stopTransferResource,
	"DeleteResource":                (*MediaLibrary).deleteResource,
	"GetTransferProgress":           (*MediaLibrary).getTransferProgress,
	"CreateReference":               (*MediaLibrary).createReference,
	"FreeFormQuery":                 (*MediaLibrary).freeFormQuery,
	"GetFreeFormQueryCapabilities":  (*MediaLibrary).getFreeFormQueryCapabilities,
	"RequestDeviceMode":             (*MediaLibrary).requestDeviceMode,
	"ExtendDeviceMode":              (*MediaLibrary).extendDeviceMode,
	"CancelDeviceMode":              (*MediaLibrary).cancelDeviceMode,
	"GetDeviceMode":                 (*MediaLibrary).getDeviceMode,
	"GetDeviceModeStatus":           (*MediaLibrary).getDeviceModeStatus,
	"GetPermissionsInfo":            (*MediaLibrary).getPermissionsInfo,
	"GetAllAvailableTransforms":     (*MediaLibrary).getAllAvailableTransforms,
	"GetAllowedTransforms":          (*MediaLibrary).getAllowedTransforms,
	"GetCurrentTransformStatusList": (*MediaLibrary).getCurrentTransformStatusList,
	"StartTransformTask":            (*MediaLibrary).startTransformTask,
	"GetTransforms":                 (*MediaLibrary).getTransforms,
	"GetTransformTaskResult":        (*MediaLibrary).getTransformTaskResult,
	"CancelTransformTask":           (*MediaLibrary).cancelTransformTask,
	"PauseTransformTask":            (*MediaLibrary).pauseTransformTask,
	"ResumeTransformTask":           (*MediaLibrary).resumeTransformTask,
	"RollbackTransformTask":         (*MediaLibrary).rollbackTransformTask,
	"EvaluateTransforms":            (*MediaLibrary).evaluateTransforms,
}

func (m *MediaLibrary) control(p *action) (*actionResponse, error) {
	action := p.XMLName.Local
	h, ok := handlers[action]
	if !ok {
		return nil, fmt.Errorf("unsupported action: %s", action)
	}
	return h(m, p)
}

type requestEnvelope struct {
	XMLName xml.Name `xml:"Envelope"`
	Body    requestBody
}

type requestBody struct {
	Action *action `xml:",any"`
}

type action struct {
	XMLName   xml.Name
	XMLNSU    string     `xml:"xmlns:u,attr"`
	Arguments []argument `xml:",any"`
}

func (a *action) response(args ...argument) *actionResponse {
	return &actionResponse{
		XMLName: xml.Name{
			Local: "u:" + a.XMLName.Local + "Response",
		},
		XMLNSU:    "urn:schemas-upnp-org:service:ContentDirectory:1",
		Arguments: args,
	}
}

type responseEnvelope struct {
	XMLName       xml.Name     `xml:"s:Envelope"`
	XMLNSS        string       `xml:"xmlns:s,attr"`
	EncodingStyle string       `xml:"s:encodingStyle,attr"`
	ResponseBody  responseBody `xml:"s:Body"`
}

type responseBody struct {
	ActionResponse *actionResponse `xml:",any"`
}

type actionResponse struct {
	XMLName   xml.Name
	XMLNSU    string     `xml:"xmlns:u,attr"`
	Arguments []argument `xml:",any"`
}

type argument struct {
	XMLName xml.Name
	Value   string `xml:",innerxml"`
}

type MediaItems []MediaItem

func (m MediaItems) String() string {
	var buf bytes.Buffer
	if _, err := buf.WriteString(xml.Header); err != nil {
		return ""
	}
	if err := Template.ExecuteTemplate(&buf, "didl_lite.gohtml", m); err != nil {
		return ""
	}

	var esc bytes.Buffer
	if err := xml.EscapeText(&esc, buf.Bytes()); err != nil {
		return ""
	}

	return esc.String()
}

type MediaItem struct {
	ID           int
	ParentID     int
	Restricted   int
	Title        string
	Class        MediaClass
	ProtocolInfo string
	URL          *url.URL
}

type MediaClass int

const (
	MediaClassStorageFolder MediaClass = iota
	MediaClassItem
	MediaClassImageItem
	MediaClassAudioItem
	MediaClassVideoItem
)

func (c MediaClass) String() string {
	return [...]string{
		MediaClassStorageFolder: "object.container.storageFolder",
		MediaClassItem:          "object.item",
		MediaClassImageItem:     "object.item.imageItem",
		MediaClassAudioItem:     "object.item.audioItem",
		MediaClassVideoItem:     "object.item.videoItem",
	}[c]
}

func (m *MediaLibrary) getSearchCapabilities(p *action) (*actionResponse, error) {
	return p.response(), nil
}

func (m *MediaLibrary) getSortCapabilities(p *action) (*actionResponse, error) {
	return p.response(argument{
		XMLName: xml.Name{Local: "SortCaps"},
		Value:   "name",
	}), nil
}

func (m *MediaLibrary) getSortExtensionCapabilities(p *action) (*actionResponse, error) {
	return p.response(), nil
}

func (m *MediaLibrary) getFeatureList(p *action) (*actionResponse, error) {
	return p.response(), nil
}

func (m *MediaLibrary) getSystemUpdateID(p *action) (*actionResponse, error) {
	return p.response(), nil
}

func (m *MediaLibrary) getServiceResetToken(p *action) (*actionResponse, error) {
	return p.response(), nil
}

func (m *MediaLibrary) browse(p *action) (*actionResponse, error) {
	var parentID int
	for _, arg := range p.Arguments {
		switch arg.XMLName.Local {
		case "ObjectID":
			id, err := strconv.Atoi(arg.Value)
			if err != nil {
				return nil, err
			}
			parentID = id
		}
	}

	res := make(MediaItems, 0, len(m.Items))
	for _, i := range m.Items {
		if i.ParentID == parentID {
			res = append(res, i)
		}
	}

	return p.response([]argument{
		{XMLName: xml.Name{Local: "Result"}, Value: res.String()},
		{XMLName: xml.Name{Local: "NumberReturned"}, Value: strconv.Itoa(len(res))},
		{XMLName: xml.Name{Local: "TotalMatches"}, Value: strconv.Itoa(len(res))},
		{XMLName: xml.Name{Local: "UpdateID"}, Value: "1"},
	}...), nil
}

func (m *MediaLibrary) search(p *action) (*actionResponse, error) {
	return p.response(), nil
}

func (m *MediaLibrary) createObject(p *action) (*actionResponse, error) {
	return p.response(), nil
}

func (m *MediaLibrary) destroyObject(p *action) (*actionResponse, error) {
	return p.response(), nil
}

func (m *MediaLibrary) updateObject(p *action) (*actionResponse, error) {
	return p.response(), nil
}

func (m *MediaLibrary) moveObject(p *action) (*actionResponse, error) {
	return p.response(), nil
}

func (m *MediaLibrary) importResource(p *action) (*actionResponse, error) {
	return p.response(), nil
}

func (m *MediaLibrary) exportResource(p *action) (*actionResponse, error) {
	return p.response(), nil
}

func (m *MediaLibrary) stopTransferResource(p *action) (*actionResponse, error) {
	return p.response(), nil
}

func (m *MediaLibrary) deleteResource(p *action) (*actionResponse, error) {
	return p.response(), nil
}

func (m *MediaLibrary) getTransferProgress(p *action) (*actionResponse, error) {
	return p.response(), nil
}

func (m *MediaLibrary) createReference(p *action) (*actionResponse, error) {
	return p.response(), nil
}

func (m *MediaLibrary) freeFormQuery(p *action) (*actionResponse, error) {
	return p.response(), nil
}

func (m *MediaLibrary) getFreeFormQueryCapabilities(p *action) (*actionResponse, error) {
	return p.response(), nil
}

func (m *MediaLibrary) requestDeviceMode(p *action) (*actionResponse, error) {
	return p.response(), nil
}

func (m *MediaLibrary) extendDeviceMode(p *action) (*actionResponse, error) {
	return p.response(), nil
}

func (m *MediaLibrary) cancelDeviceMode(p *action) (*actionResponse, error) {
	return p.response(), nil
}

func (m *MediaLibrary) getDeviceMode(p *action) (*actionResponse, error) {
	return p.response(), nil
}

func (m *MediaLibrary) getDeviceModeStatus(p *action) (*actionResponse, error) {
	return p.response(), nil
}

func (m *MediaLibrary) getPermissionsInfo(p *action) (*actionResponse, error) {
	return p.response(), nil
}

func (m *MediaLibrary) getAllAvailableTransforms(p *action) (*actionResponse, error) {
	return p.response(), nil
}

func (m *MediaLibrary) getAllowedTransforms(p *action) (*actionResponse, error) {
	return p.response(), nil
}

func (m *MediaLibrary) getCurrentTransformStatusList(p *action) (*actionResponse, error) {
	return p.response(), nil
}

func (m *MediaLibrary) startTransformTask(p *action) (*actionResponse, error) {
	return p.response(), nil
}

func (m *MediaLibrary) getTransforms(p *action) (*actionResponse, error) {
	return p.response(), nil
}

func (m *MediaLibrary) getTransformTaskResult(p *action) (*actionResponse, error) {
	return p.response(), nil
}

func (m *MediaLibrary) cancelTransformTask(p *action) (*actionResponse, error) {
	return p.response(), nil
}

func (m *MediaLibrary) pauseTransformTask(p *action) (*actionResponse, error) {
	return p.response(), nil
}

func (m *MediaLibrary) resumeTransformTask(p *action) (*actionResponse, error) {
	return p.response(), nil
}

func (m *MediaLibrary) rollbackTransformTask(p *action) (*actionResponse, error) {
	return p.response(), nil
}

func (m *MediaLibrary) evaluateTransforms(p *action) (*actionResponse, error) {
	return p.response(), nil
}
