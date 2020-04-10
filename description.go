package picoms

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
)

const xmlDeclaration = "<?xml version=\"1.0\"?>\n"

// device

func (s *Server) Describe() ([]byte, error) {
	b, err := xml.MarshalIndent(deviceDescription{
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
		return nil, err
	}

	var buf bytes.Buffer
	if _, err := buf.Write([]byte(xmlDeclaration)); err != nil {
		return nil, err
	}
	if _, err := buf.Write(b); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
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

// service

func (c *ContentDirectory) Describe() ([]byte, error) {
	b, err := xml.MarshalIndent(&serviceDescription{
		SpecVersion: specVersion{
			Major: 1,
			Minor: 0,
		},
		ActionList: actionList{Actions: []action{
			{Name: "GetSearchCapabilities", ArgumentList: argumentList{Arguments: []argument{
				{Name: "SearchCaps", Direction: out, RelatedStateVariable: "SearchCapabilities"},
			}}},
			{Name: "GetSortCapabilities", ArgumentList: argumentList{Arguments: []argument{
				{Name: "SortCaps", Direction: out, RelatedStateVariable: "SortCapabilities"},
			}}},
			{Name: "GetSystemUpdateID", ArgumentList: argumentList{Arguments: []argument{
				{Name: "Id", Direction: out, RelatedStateVariable: "SystemUpdateID"},
			}}},
			{Name: "Browse", ArgumentList: argumentList{Arguments: []argument{
				{Name: "ObjectID", Direction: in, RelatedStateVariable: argTypeObjectID},
				{Name: "BrowseFlag", Direction: in, RelatedStateVariable: argTypeBrowseFlag},
				{Name: "Filter", Direction: in, RelatedStateVariable: argTypeFilter},
				{Name: "StartingIndex", Direction: in, RelatedStateVariable: argTypeIndex},
				{Name: "RequestedCount", Direction: in, RelatedStateVariable: argTypeCount},
				{Name: "SortCriteria", Direction: in, RelatedStateVariable: argTypeSortCriteria},
				{Name: "Result", Direction: out, RelatedStateVariable: argTypeResult},
				{Name: "NumberReturned", Direction: out, RelatedStateVariable: argTypeCount},
				{Name: "TotalMatches", Direction: out, RelatedStateVariable: argTypeCount},
				{Name: "UpdateID", Direction: out, RelatedStateVariable: argTypeUpdateID},
			}}},
			{Name: "Search", ArgumentList: argumentList{Arguments: []argument{
				{Name: "ContainerID", Direction: in, RelatedStateVariable: argTypeObjectID},
				{Name: "SearchCriteria", Direction: in, RelatedStateVariable: argTypeSearchCriteria},
				{Name: "Filter", Direction: in, RelatedStateVariable: argTypeFilter},
				{Name: "StartingIndex", Direction: in, RelatedStateVariable: argTypeIndex},
				{Name: "RequestedCount", Direction: in, RelatedStateVariable: argTypeCount},
				{Name: "SortCriteria", Direction: in, RelatedStateVariable: argTypeSortCriteria},
				{Name: "Result", Direction: out, RelatedStateVariable: argTypeResult},
				{Name: "NumberReturned", Direction: out, RelatedStateVariable: argTypeCount},
				{Name: "TotalMatches", Direction: out, RelatedStateVariable: argTypeCount},
				{Name: "UpdateID", Direction: out, RelatedStateVariable: argTypeUpdateID},
			}}},
			{Name: "CreateObject", ArgumentList: argumentList{Arguments: []argument{
				{Name: "ContainerID", Direction: in, RelatedStateVariable: argTypeObjectID},
				{Name: "Elements", Direction: in, RelatedStateVariable: argTypeResult},
				{Name: "ObjectID", Direction: out, RelatedStateVariable: argTypeObjectID},
				{Name: "Result", Direction: out, RelatedStateVariable: argTypeResult},
			}}},
			{Name: "DestroyObject", ArgumentList: argumentList{Arguments: []argument{
				{Name: "ObjectID", Direction: in, RelatedStateVariable: argTypeObjectID},
			}}},
			{Name: "UpdateObject", ArgumentList: argumentList{Arguments: []argument{
				{Name: "ObjectID", Direction: in, RelatedStateVariable: argTypeObjectID},
				{Name: "CurrentTagValue", Direction: in, RelatedStateVariable: argTypeTagValueList},
				{Name: "NewTagValue", Direction: in, RelatedStateVariable: argTypeTagValueList},
			}}},
			{Name: "ImportResource", ArgumentList: argumentList{Arguments: []argument{
				{Name: "SourceURI", Direction: in, RelatedStateVariable: argTypeURI},
				{Name: "DestinationURI", Direction: in, RelatedStateVariable: argTypeURI},
				{Name: "TransferID", Direction: out, RelatedStateVariable: argTypeTransferID},
			}}},
			{Name: "ExportResource", ArgumentList: argumentList{Arguments: []argument{
				{Name: "SourceURI", Direction: in, RelatedStateVariable: argTypeURI},
				{Name: "DestinationURI", Direction: in, RelatedStateVariable: argTypeURI},
				{Name: "TransferID", Direction: out, RelatedStateVariable: argTypeTransferID},
			}}},
			{Name: "StopTransferResource", ArgumentList: argumentList{Arguments: []argument{
				{Name: "TransferID", Direction: in, RelatedStateVariable: argTypeTransferID},
			}}},
			{Name: "GetTransferProgress", ArgumentList: argumentList{Arguments: []argument{
				{Name: "TransferID", Direction: in, RelatedStateVariable: argTypeTransferID},
				{Name: "TransferStatus", Direction: out, RelatedStateVariable: argTypeTransferStatus},
				{Name: "TransferLength", Direction: out, RelatedStateVariable: argTypeTransferLength},
				{Name: "TransferTotal", Direction: out, RelatedStateVariable: argTypeTransferTotal},
			}}},
			{Name: "DeleteResource", ArgumentList: argumentList{Arguments: []argument{
				{Name: "ResourceURI", Direction: in, RelatedStateVariable: argTypeURI},
			}}},
			{Name: "CreateReference", ArgumentList: argumentList{Arguments: []argument{
				{Name: "ContainerID", Direction: in, RelatedStateVariable: argTypeObjectID},
				{Name: "ObjectID", Direction: in, RelatedStateVariable: argTypeObjectID},
				{Name: "NewID", Direction: out, RelatedStateVariable: argTypeObjectID},
			}}},
		}},
		ServiceStateTable: stateVariableList{StateVariables: []stateVariable{
			{Name: "TransferIDs", DataType: dataTypeString, SendEvents: true},
			{Name: argTypeObjectID, DataType: dataTypeString},
			{Name: argTypeResult, DataType: dataTypeString},
			{Name: argTypeSearchCriteria, DataType: dataTypeString},
			{Name: argTypeBrowseFlag, DataType: dataTypeString, AllowedValueList: &allowedValueList{allowedValues: []string{"BrowseMetadata", "BrowseDirectChildren"}}},
			{Name: argTypeFilter, DataType: dataTypeString},
			{Name: argTypeSortCriteria, DataType: dataTypeString},
			{Name: argTypeIndex, DataType: dataTypeUI4},
			{Name: argTypeCount, DataType: dataTypeUI4},
			{Name: argTypeUpdateID, DataType: dataTypeUI4},
			{Name: argTypeTransferID, DataType: dataTypeUI4},
			{Name: argTypeTransferStatus, DataType: dataTypeString, AllowedValueList: &allowedValueList{allowedValues: []string{"COMPLETED", "ERROR", "IN_PROGRESS", "STOPPED"}}},
			{Name: argTypeTransferLength, DataType: dataTypeString},
			{Name: argTypeTransferTotal, DataType: dataTypeString},
			{Name: argTypeTagValueList, DataType: dataTypeString},
			{Name: argTypeURI, DataType: dataTypeURI},
			{Name: "SearchCapabilities", DataType: dataTypeString},
			{Name: "SortCapabilities", DataType: dataTypeString},
			{Name: "SystemUpdateID", DataType: dataTypeUI4, SendEvents: true},
			{Name: "ContainerUpdateIDs", DataType: dataTypeString, SendEvents: true},
		}},
	}, "", "  ")
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	if _, err := buf.Write([]byte(xmlDeclaration)); err != nil {
		return nil, err
	}
	if _, err := buf.Write(b); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

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
