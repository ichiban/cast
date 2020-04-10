package picoms

import (
	"encoding/xml"
	"io/ioutil"
	"net/http"
	"net/url"
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

type ContentDirectory struct {
}

func (c *ContentDirectory) handle(r *request) (*response, error) {
	switch {
	case r.Body.GetSearchCapabilities != nil:
		return nil, nil
	case r.Body.GetSortCapabilities != nil:
		return nil, nil
	case r.Body.GetSystemUpdateID != nil:
		return nil, nil
	case r.Body.Browse != nil:
		d := DIDLLite{
			XMLNSDC:   "http://purl.org/dc/elements/1.1/",
			XMLNSUPnP: "urn:schemas-upnp-org:metadata-1-0/upnp/",
			Containers: []container{
				{
					ID:          64,
					ParentID:    0,
					Restricted:  1,
					Searchable:  1,
					ChildCount:  4,
					Title:       "Browse Folders",
					Class:       "object.container.storageFolder",
					StorageUsed: -1,
				},
				{
					ID:          1,
					ParentID:    0,
					Restricted:  1,
					Searchable:  1,
					ChildCount:  4,
					Title:       "Music",
					Class:       "object.container.storageFolder",
					StorageUsed: -1,
				},
				{
					ID:          3,
					ParentID:    0,
					Restricted:  1,
					Searchable:  1,
					ChildCount:  4,
					Title:       "Pictures",
					Class:       "object.container.storageFolder",
					StorageUsed: -1,
				},
				{
					ID:          2,
					ParentID:    0,
					Restricted:  1,
					Searchable:  1,
					ChildCount:  2,
					Title:       "Video",
					Class:       "object.container.storageFolder",
					StorageUsed: -1,
				},
			},
		}

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
					NumberReturned: 4,
					TotalMatches:   4,
					UpdateID:       0,
				},
			},
		}, nil
	case r.Body.Search != nil:
		return nil, nil
	case r.Body.CreateObject != nil:
		return nil, nil
	case r.Body.DestroyObject != nil:
		return nil, nil
	case r.Body.UpdateObject != nil:
		return nil, nil
	case r.Body.ImportResource != nil:
		return nil, nil
	case r.Body.ExportResource != nil:
		return nil, nil
	case r.Body.StopTransferResource != nil:
		return nil, nil
	case r.Body.GetTransferProgress != nil:
		return nil, nil
	case r.Body.DeleteResource != nil:
		return nil, nil
	case r.Body.CreateReference != nil:
		return nil, nil
	default:
		return nil, nil
	}
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

func (c *ContentDirectory) Event(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNotImplemented)
}

type DIDLLite struct {
	XMLName    xml.Name    `xml:"urn:schemas-upnp-org:metadata-1-0/DIDL-Lite/ DIDL-Lite"`
	XMLNSDC    string      `xml:"xmlns:dc,attr"`
	XMLNSUPnP  string      `xml:"xmlns:upnp,attr"`
	Containers []container `xml:"container"`
}

type container struct {
	ID         int `xml:"id,attr"`
	ParentID   int `xml:"parentID,attr"`
	Restricted int `xml:"restricted,attr"`
	Searchable int `xml:"searchable,attr"`
	ChildCount int `xml:"childCount,attr"`

	Title       string `xml:"dc:title"`
	Class       string `xml:"upnp:class"`
	StorageUsed int    `xml:"upnp:storageUsed"`
}
