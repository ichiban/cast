package cast

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"path"
	"path/filepath"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/ichiban/cast/upnp"
)

const (
	browseMetaData       = "BrowseMetaData"
	browseDirectChildren = "BrowseDirectChildren"
)

const (
	classStorageFolder = "object.container.storageFolder"
	classItem          = "object.item"
	classImageItem     = "object.item.imageItem"
	classAudioItem     = "object.item.audioItem"
	classVideoItem     = "object.item.videoItem"
)

const (
	argTypeObjectID       = "A_ARG_TYPE_ObjectID"
	argTypeResult         = "A_ARG_TYPE_Result"
	argTypeSearchCriteria = "A_ARG_TYPE_SearchCriteria"
	argTypeBrowseFlag     = "A_ARG_TYPE_BrowseFlag"
	argTypeFilter         = "A_ARG_TYPE_Filter"
	argTypeSortCriteria   = "A_ARG_TYPE_SortCriteria"
	argTypeIndex          = "A_ARG_TYPE_Index"
	argTypeCount          = "A_ARG_TYPE_Count"
	argTypeUpdateID       = "A_ARG_TYPE_UpdateID"
	argTypeTransferID     = "A_ARG_TYPE_TransferID"
	argTypeTransferStatus = "A_ARG_TYPE_TransferStatus"
	argTypeTransferLength = "A_ARG_TYPE_TransferLength"
	argTypeTransferTotal  = "A_ARG_TYPE_TransferTotal"
	argTypeTagValueList   = "A_ARG_TYPE_TagValueList"
	argTypeURI            = "A_ARG_TYPE_URI"
)

const (
	dataTypeString = "string"
	dataTypeUI4    = "ui4"
	dataTypeURI    = "uri"
)

var Description = upnp.ServiceDescription{
	SpecVersion: upnp.SpecVersion{
		Major: 1,
		Minor: 0,
	},
	ActionList: upnp.ActionList{Actions: []upnp.Action{
		{Name: "GetSearchCapabilities", ArgumentList: upnp.ArgumentList{Arguments: []upnp.Argument{
			{Name: "SearchCaps", Direction: upnp.Out, RelatedStateVariable: "SearchCapabilities"},
		}}},
		{Name: "GetSortCapabilities", ArgumentList: upnp.ArgumentList{Arguments: []upnp.Argument{
			{Name: "SortCaps", Direction: upnp.Out, RelatedStateVariable: "SortCapabilities"},
		}}},
		{Name: "GetSystemUpdateID", ArgumentList: upnp.ArgumentList{Arguments: []upnp.Argument{
			{Name: "Id", Direction: upnp.Out, RelatedStateVariable: "SystemUpdateID"},
		}}},
		{Name: "Browse", ArgumentList: upnp.ArgumentList{Arguments: []upnp.Argument{
			{Name: "ObjectID", Direction: upnp.In, RelatedStateVariable: argTypeObjectID},
			{Name: "BrowseFlag", Direction: upnp.In, RelatedStateVariable: argTypeBrowseFlag},
			{Name: "Filter", Direction: upnp.In, RelatedStateVariable: argTypeFilter},
			{Name: "StartingIndex", Direction: upnp.In, RelatedStateVariable: argTypeIndex},
			{Name: "RequestedCount", Direction: upnp.In, RelatedStateVariable: argTypeCount},
			{Name: "SortCriteria", Direction: upnp.In, RelatedStateVariable: argTypeSortCriteria},
			{Name: "Result", Direction: upnp.Out, RelatedStateVariable: argTypeResult},
			{Name: "NumberReturned", Direction: upnp.Out, RelatedStateVariable: argTypeCount},
			{Name: "TotalMatches", Direction: upnp.Out, RelatedStateVariable: argTypeCount},
			{Name: "UpdateID", Direction: upnp.Out, RelatedStateVariable: argTypeUpdateID},
		}}},
		{Name: "Search", ArgumentList: upnp.ArgumentList{Arguments: []upnp.Argument{
			{Name: "ContainerID", Direction: upnp.In, RelatedStateVariable: argTypeObjectID},
			{Name: "SearchCriteria", Direction: upnp.In, RelatedStateVariable: argTypeSearchCriteria},
			{Name: "Filter", Direction: upnp.In, RelatedStateVariable: argTypeFilter},
			{Name: "StartingIndex", Direction: upnp.In, RelatedStateVariable: argTypeIndex},
			{Name: "RequestedCount", Direction: upnp.In, RelatedStateVariable: argTypeCount},
			{Name: "SortCriteria", Direction: upnp.In, RelatedStateVariable: argTypeSortCriteria},
			{Name: "Result", Direction: upnp.Out, RelatedStateVariable: argTypeResult},
			{Name: "NumberReturned", Direction: upnp.Out, RelatedStateVariable: argTypeCount},
			{Name: "TotalMatches", Direction: upnp.Out, RelatedStateVariable: argTypeCount},
			{Name: "UpdateID", Direction: upnp.Out, RelatedStateVariable: argTypeUpdateID},
		}}},
		{Name: "CreateObject", ArgumentList: upnp.ArgumentList{Arguments: []upnp.Argument{
			{Name: "ContainerID", Direction: upnp.In, RelatedStateVariable: argTypeObjectID},
			{Name: "Elements", Direction: upnp.In, RelatedStateVariable: argTypeResult},
			{Name: "ObjectID", Direction: upnp.Out, RelatedStateVariable: argTypeObjectID},
			{Name: "Result", Direction: upnp.Out, RelatedStateVariable: argTypeResult},
		}}},
		{Name: "DestroyObject", ArgumentList: upnp.ArgumentList{Arguments: []upnp.Argument{
			{Name: "ObjectID", Direction: upnp.In, RelatedStateVariable: argTypeObjectID},
		}}},
		{Name: "UpdateObject", ArgumentList: upnp.ArgumentList{Arguments: []upnp.Argument{
			{Name: "ObjectID", Direction: upnp.In, RelatedStateVariable: argTypeObjectID},
			{Name: "CurrentTagValue", Direction: upnp.In, RelatedStateVariable: argTypeTagValueList},
			{Name: "NewTagValue", Direction: upnp.In, RelatedStateVariable: argTypeTagValueList},
		}}},
		{Name: "ImportResource", ArgumentList: upnp.ArgumentList{Arguments: []upnp.Argument{
			{Name: "SourceURI", Direction: upnp.In, RelatedStateVariable: argTypeURI},
			{Name: "DestinationURI", Direction: upnp.In, RelatedStateVariable: argTypeURI},
			{Name: "TransferID", Direction: upnp.Out, RelatedStateVariable: argTypeTransferID},
		}}},
		{Name: "ExportResource", ArgumentList: upnp.ArgumentList{Arguments: []upnp.Argument{
			{Name: "SourceURI", Direction: upnp.In, RelatedStateVariable: argTypeURI},
			{Name: "DestinationURI", Direction: upnp.In, RelatedStateVariable: argTypeURI},
			{Name: "TransferID", Direction: upnp.Out, RelatedStateVariable: argTypeTransferID},
		}}},
		{Name: "StopTransferResource", ArgumentList: upnp.ArgumentList{Arguments: []upnp.Argument{
			{Name: "TransferID", Direction: upnp.In, RelatedStateVariable: argTypeTransferID},
		}}},
		{Name: "GetTransferProgress", ArgumentList: upnp.ArgumentList{Arguments: []upnp.Argument{
			{Name: "TransferID", Direction: upnp.In, RelatedStateVariable: argTypeTransferID},
			{Name: "TransferStatus", Direction: upnp.Out, RelatedStateVariable: argTypeTransferStatus},
			{Name: "TransferLength", Direction: upnp.Out, RelatedStateVariable: argTypeTransferLength},
			{Name: "TransferTotal", Direction: upnp.Out, RelatedStateVariable: argTypeTransferTotal},
		}}},
		{Name: "DeleteResource", ArgumentList: upnp.ArgumentList{Arguments: []upnp.Argument{
			{Name: "ResourceURI", Direction: upnp.In, RelatedStateVariable: argTypeURI},
		}}},
		{Name: "CreateReference", ArgumentList: upnp.ArgumentList{Arguments: []upnp.Argument{
			{Name: "ContainerID", Direction: upnp.In, RelatedStateVariable: argTypeObjectID},
			{Name: "ObjectID", Direction: upnp.In, RelatedStateVariable: argTypeObjectID},
			{Name: "NewID", Direction: upnp.Out, RelatedStateVariable: argTypeObjectID},
		}}},
	}},
	ServiceStateTable: upnp.StateVariableList{StateVariables: []upnp.StateVariable{
		{Name: "TransferIDs", DataType: dataTypeString, SendEvents: true},
		{Name: argTypeObjectID, DataType: dataTypeString},
		{Name: argTypeResult, DataType: dataTypeString},
		{Name: argTypeSearchCriteria, DataType: dataTypeString},
		{Name: argTypeBrowseFlag, DataType: dataTypeString, AllowedValueList: &upnp.AllowedValueList{AllowedValues: []string{browseMetaData, browseDirectChildren}}},
		{Name: argTypeFilter, DataType: dataTypeString},
		{Name: argTypeSortCriteria, DataType: dataTypeString},
		{Name: argTypeIndex, DataType: dataTypeUI4},
		{Name: argTypeCount, DataType: dataTypeUI4},
		{Name: argTypeUpdateID, DataType: dataTypeUI4},
		{Name: argTypeTransferID, DataType: dataTypeUI4},
		{Name: argTypeTransferStatus, DataType: dataTypeString, AllowedValueList: &upnp.AllowedValueList{AllowedValues: []string{"COMPLETED", "ERROR", "IN_PROGRESS", "STOPPED"}}},
		{Name: argTypeTransferLength, DataType: dataTypeString},
		{Name: argTypeTransferTotal, DataType: dataTypeString},
		{Name: argTypeTagValueList, DataType: dataTypeString},
		{Name: argTypeURI, DataType: dataTypeURI},
		{Name: "SearchCapabilities", DataType: dataTypeString},
		{Name: "SortCapabilities", DataType: dataTypeString},
		{Name: "SystemUpdateID", DataType: dataTypeUI4, SendEvents: true},
		{Name: "ContainerUpdateIDs", DataType: dataTypeString, SendEvents: true},
	}},
}

var formats = []struct {
	ext   string
	class string
	mime  string
}{
	{ext: ".jpg", mime: "image/jpeg", class: classImageItem},
	{ext: ".png", mime: "image/png", class: classImageItem},
	{ext: ".gif", mime: "image/gif", class: classImageItem},
	{ext: ".mp3", mime: "audio/mpeg", class: classAudioItem},
	{ext: ".m4a", mime: "audio/mp4", class: classAudioItem},
	{ext: ".wma", mime: "audio/x-ms-wma", class: classAudioItem},
	{ext: ".wav", mime: "audio/x-wav", class: classAudioItem},
	{ext: ".pcm", mime: "audio/L16", class: classAudioItem},
	{ext: ".ogg", mime: "application/ogg", class: classAudioItem},
	{ext: ".avi", mime: "video/x-msvideo", class: classVideoItem},
	{ext: ".mpg", mime: "video/mpeg", class: classVideoItem},
	{ext: ".mp4", mime: "video/mp4", class: classVideoItem},
	{ext: ".wmv", mime: "video/x-ms-wmv", class: classVideoItem},
	{ext: ".flv", mime: "video/x-flv", class: classVideoItem},
	{ext: ".mov", mime: "video/quicktime", class: classVideoItem},
	{ext: ".3gp", mime: "video/3gpp", class: classVideoItem},
}

type ContentDirectory struct {
	BaseURL *url.URL
	Path    string

	TransferIDs               string
	A_ARG_TYPE_ObjectID       string
	A_ARG_TYPE_Result         string
	A_ARG_TYPE_SearchCriteria string
	A_ARG_TYPE_BrowseFlag     string
	A_ARG_TYPE_Filter         string
	A_ARG_TYPE_SortCriteria   string
	A_ARG_TYPE_Index          uint32
	A_ARG_TYPE_Count          uint32
	A_ARG_TYPE_UpdateID       uint32
	A_ARG_TYPE_TransferID     uint32
	A_ARG_TYPE_TransferStatus string
	A_ARG_TYPE_TransferLength string
	A_ARG_TYPE_TransferTotal  string
	A_ARG_TYPE_TagValueList   string
	A_ARG_TYPE_URI            url.URL
	SearchCapabilities        string
	SortCapabilities          string
	SystemUpdateID            uint32
	ContainerUpdateIDs        string
}

func (c *ContentDirectory) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	requestLog(http.StripPrefix("/media", http.FileServer(http.Dir(c.Path)))).ServeHTTP(w, r)
}

func (c *ContentDirectory) SetBaseURL(url *url.URL) {
	c.BaseURL = url
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
		name := fi.Name()
		if fi.IsDir() {
			d.Containers = append(d.Containers, container{
				ID:          filepath.Join(dirname, name),
				Restricted:  true,
				ParentID:    objectID,
				Searchable:  true,
				Title:       name,
				Class:       classStorageFolder,
				StorageUsed: fi.Size(),
			})
		} else {
			p, err := filepath.Rel(c.Path, filepath.Join(dirname, name))
			if err != nil {
				return nil, 0, 0, 0, err
			}

			ext := strings.ToLower(filepath.Ext(p))
			class := classItem
			mime := "*"
			for _, f := range formats {
				if f.ext == ext {
					class = f.class
					mime = f.mime
					break
				}
			}

			d.Items = append(d.Items, item{
				ID:         filepath.Join(dirname, name),
				ParentID:   objectID,
				Restricted: true,
				Title:      name,
				Class:      class,
				Res: res{
					ProtocolInfo: fmt.Sprintf("http-get:*:%s:*", mime),
					URL:          c.url("media", p).String(),
				},
			})
		}
	}
	return &d, uint32(len(fis)), uint32(len(fis)), 0, nil
}

func (c *ContentDirectory) url(p ...string) *url.URL {
	url := *c.BaseURL
	url.Path = path.Join(append([]string{url.Path}, p...)...)
	return &url
}

func requestLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t := time.Now()
		rw := responseWriter{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
		}
		next.ServeHTTP(&rw, r)
		log.WithFields(log.Fields{
			"addr":    r.RemoteAddr,
			"method":  r.Method,
			"elapsed": time.Since(t).Milliseconds(),
			"ua":      r.Header.Get("User-Agent"),
			"status":  rw.statusCode,
		}).Info(r.URL.String())
	})
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (r *responseWriter) WriteHeader(statusCode int) {
	r.statusCode = statusCode
	r.ResponseWriter.WriteHeader(statusCode)
}
