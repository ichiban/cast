package picoms

import (
	"encoding/xml"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"path"
	"path/filepath"

	log "github.com/sirupsen/logrus"
)

const (
	soapEnvelope = "http://schemas.xmlsoap.org/soap/envelope/"
	soapEncoding = "http://schemas.xmlsoap.org/soap/encoding"
)

const serviceContentDirectory1 = "urn:schemas-upnp-org:service:ContentDirectory:1"

const (
	browseMetaData       = "BrowseMetaData"
	browseDirectChildren = "BrowseDirectChildren"
)

const (
	classStorageFolder = "object.container.storageFolder"
	classItem          = "object.item"
)

type ContentDirectory struct {
	http.Server

	Path string
}

func (c *ContentDirectory) URL(p ...string) *url.URL {
	return &url.URL{
		Scheme: "http",
		Host:   c.Addr,
		Path:   path.Join(p...),
	}
}

func (c *ContentDirectory) Browse(objectID, browseFlag, filter string, startingIndex, requestedCount uint32, sortCriteria string) (*DIDLLite, uint32, uint32, uint32, error) {
	switch browseFlag {
	case browseMetaData:
		return nil, 0, 0, 0, errors.New("not implemented")
	case browseDirectChildren:
	default:
		return nil, 0, 0, 0, fmt.Errorf("unknown browse flag: %s", browseFlag)
	}

	dirname := objectID
	if dirname == "0" {
		dirname = c.Path
	}

	fis, err := ioutil.ReadDir(dirname)
	if err != nil {
		return nil, 0, 0, 0, err
	}

	d := DIDLLite{
		XMLNSDC:   "http://purl.org/dc/elements/1.1/",
		XMLNSUPnP: "urn:schemas-upnp-org:metadata-1-0/upnp/",
	}
	for _, fi := range fis {
		if fi.IsDir() {
			d.Containers = append(d.Containers, container{
				ID:          filepath.Join(dirname, fi.Name()),
				Restricted:  true,
				ParentID:    dirname,
				Searchable:  true,
				Title:       fi.Name(),
				Class:       classStorageFolder,
				StorageUsed: fi.Size(),
			})
		} else {
			path, err := filepath.Rel(c.Path, filepath.Join(dirname, fi.Name()))
			if err != nil {
				return nil, 0, 0, 0, err
			}
			d.Items = append(d.Items, item{
				ID:         filepath.Join(dirname, fi.Name()),
				ParentID:   dirname,
				Restricted: true,
				Title:      fi.Name(),
				Class:      "",
				Res: res{
					ProtocolInfo: "*:*:*:*",
					URL:          c.URL("media", path).String(),
				},
			})
		}
	}
	return &d, uint32(len(fis)), uint32(len(fis)), 0, nil
}

func (c *ContentDirectory) Control(w http.ResponseWriter, r *http.Request) {
	b, err := ioutil.ReadAll(r.Body)
	if err != nil {
		panic(err)
	}
	r.Body.Close()

	var req request
	if err := xml.Unmarshal(b, &req); err != nil {
		panic(err)
	}
	resp, err := c.handle(&req)
	if err != nil {
		panic(err)
	}
	b, err = xml.MarshalIndent(&resp, "", "  ")
	if err != nil {
		panic(err)
	}
	if _, err := w.Write([]byte(xmlDeclaration)); err != nil {
		panic(err)
	}
	if _, err := w.Write(b); err != nil {
		panic(err)
	}
}

func (c *ContentDirectory) handle(r *request) (*response, error) {
	switch {
	case r.Body.GetSearchCapabilities != nil:
		log.WithFields(log.Fields{}).Info("GetSearchCapabilities")
		return nil, nil
	case r.Body.GetSortCapabilities != nil:
		log.WithFields(log.Fields{}).Info("GetSortCapabilities")
		return &response{
			XMLNS:         soapEnvelope,
			EncodingStyle: soapEncoding,
			Body: responseBody{
				GetSortCapabilitiesResponse: &getSortCapabilitiesResponse{
					SortCaps: "",
				},
			},
		}, nil
	case r.Body.GetSystemUpdateID != nil:
		log.Info("GetSystemUpdateID")
		return nil, nil
	case r.Body.Browse != nil:
		log.WithFields(log.Fields{
			"ObjectID":       r.Body.Browse.ObjectID,
			"BrowseFlag":     r.Body.Browse.BrowseFlag,
			"Filter":         r.Body.Browse.Filter,
			"StartingIndex":  r.Body.Browse.StartingIndex,
			"RequestedCount": r.Body.Browse.RequestedCount,
			"SortCriteria":   r.Body.Browse.SortCriteria,
		}).Info("Browse")
		d, numberReturned, totalMatches, updateID, err := c.Browse(r.Body.Browse.ObjectID, r.Body.Browse.BrowseFlag, r.Body.Browse.Filter, r.Body.Browse.StartingIndex, r.Body.Browse.RequestedCount, r.Body.Browse.SortCriteria)

		b, err := xml.MarshalIndent(&d, "", "  ")
		if err != nil {
			return nil, err
		}

		return &response{
			XMLNS:         soapEnvelope,
			EncodingStyle: soapEncoding,
			Body: responseBody{
				BrowseResponse: &browseResponse{
					XMLNS:          serviceContentDirectory1,
					Result:         string(b),
					NumberReturned: numberReturned,
					TotalMatches:   totalMatches,
					UpdateID:       updateID,
				},
			},
		}, nil
	case r.Body.Search != nil:
		log.Info("Search")
		return nil, nil
	case r.Body.CreateObject != nil:
		log.Info("CreateObject")
		return nil, nil
	case r.Body.DestroyObject != nil:
		log.Info("DestroyObject")
		return nil, nil
	case r.Body.UpdateObject != nil:
		log.Info("UpdateObject")
		return nil, nil
	case r.Body.ImportResource != nil:
		log.Info("ImportResource")
		return nil, nil
	case r.Body.ExportResource != nil:
		log.Info("ExportResource")
		return nil, nil
	case r.Body.StopTransferResource != nil:
		log.Info("StopTransferResource")
		return nil, nil
	case r.Body.GetTransferProgress != nil:
		log.Info("GetTransferProgress")
		return nil, nil
	case r.Body.DeleteResource != nil:
		log.Info("DeleteResource")
		return nil, nil
	case r.Body.CreateReference != nil:
		log.Info("CreateReference")
		return nil, nil
	default:
		return nil, nil
	}
}

func (c *ContentDirectory) Event(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNotImplemented)
}

type request struct {
	XMLName xml.Name `xml:"Envelope"`
	Body    requestBody
}

type response struct {
	XMLName       xml.Name     `xml:"s:Envelope"`
	XMLNS         string       `xml:"xmlns:s,attr"`
	EncodingStyle string       `xml:"s:encodingStyle,attr"`
	Body          responseBody `xml:"s:Body"`
}

type requestBody struct {
	GetSearchCapabilities *getSearchCapabilities
	GetSortCapabilities   *getSortCapabilities
	GetSystemUpdateID     *getSystemUpdateID
	Browse                *browse
	Search                *search
	CreateObject          *createObject
	DestroyObject         *destroyObject
	UpdateObject          *updateObject
	ImportResource        *importResource
	ExportResource        *exportResource
	StopTransferResource  *stopTransferResource
	GetTransferProgress   *getTransferProgress
	DeleteResource        *deleteResource
	CreateReference       *createReference
}

type responseBody struct {
	GetSearchCapabilitiesResponse *getSearchCapabilitiesResponse `xml:"u:GetSearchCapabilitiesResponse,omitempty"`
	GetSortCapabilitiesResponse   *getSortCapabilitiesResponse   `xml:"u:GetSortCapabilitiesResponse,omitempty"`
	GetSystemUpdateIDResponse     *getSystemUpdateIDResponse     `xml:"u:GetSystemUpdateIDResponse,omitempty"`
	BrowseResponse                *browseResponse                `xml:"u:BrowseResponse,omitempty"`
	SearchResponse                *searchResponse                `xml:"u:SearchResponse,omitempty"`
	CreateObjectResponse          *createObjectResponse          `xml:"u:CreateObjectResponse,omitempty"`
	DestroyObjectResponse         *destroyObjectResponse         `xml:"u:DestroyObjectResponse,omitempty"`
	UpdateObjectResponse          *updateObjectResponse          `xml:"u:UpdateObjectResponse,omitempty"`
	ImportResourceResponse        *importResourceResponse        `xml:"u:ImportResourceResponse,omitempty"`
	ExportResourceResponse        *exportResourceResponse        `xml:"u:ExportResourceResponse,omitempty"`
	StopTransferResourceResponse  *stopTransferResourceResponse  `xml:"u:StopTransferResourceResponse,omitempty"`
	GetTransferProgressResponse   *getTransferProgressResponse   `xml:"u:GetTransferProgressResponse,omitempty"`
	DeleteResourceResponse        *deleteResourceResponse        `xml:"u:DeleteResourceResponse,omitempty"`
	CreateReferenceResponse       *createReferenceResponse       `xml:"u:CreateReferenceResponse,omitempty"`
}

type getSearchCapabilities struct {
}

type getSearchCapabilitiesResponse struct {
	SearchCaps string
}

type getSortCapabilities struct {
}

type getSortCapabilitiesResponse struct {
	SortCaps string
}

type getSystemUpdateID struct {
}

type getSystemUpdateIDResponse struct {
	Id uint32
}

type browse struct {
	ObjectID       string
	BrowseFlag     string
	Filter         string
	StartingIndex  uint32
	RequestedCount uint32
	SortCriteria   string
}

type browseResponse struct {
	XMLNS          string `xml:"xmlns:u,attr"`
	Result         string
	NumberReturned uint32
	TotalMatches   uint32
	UpdateID       uint32
}

type search struct {
	ContainerID    string
	SearchCriteria string
	Filter         string
	StartingIndex  uint32
	RequestedCount uint32
	SortCriteria   string
}

type searchResponse struct {
	Result         string
	NumberReturned uint32
	TotalMatches   uint32
	UpdateID       uint32
}

type createObject struct {
	ContainerID string
	Elements    string
}

type createObjectResponse struct {
	ObjectID string
	Result   string
}

type destroyObject struct {
	ObjectID string
}

type destroyObjectResponse struct {
}

type updateObject struct {
	ObjectID        string
	CurrentTagValue string
	NewTagValue     string
}

type updateObjectResponse struct {
}

type importResource struct {
	SourceURI      url.URL
	DestinationURI url.URL
}

type importResourceResponse struct {
	TransferID string
}

type exportResource struct {
	SourceURI      url.URL
	DestinationURI url.URL
}

type exportResourceResponse struct {
	TransferID string
}

type stopTransferResource struct {
	TransferID int
}

type stopTransferResourceResponse struct {
}

type getTransferProgress struct {
	TransferID int
}

type getTransferProgressResponse struct {
	TransferStatus string
	TransferLength string
	TransferTotal  string
}

type deleteResource struct {
	ResourceURI url.URL
}

type deleteResourceResponse struct {
}

type createReference struct {
	ContainerID string
	ObjectID    string
}

type createReferenceResponse struct {
	NewID string
}
