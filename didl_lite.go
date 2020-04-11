package picoms

import "encoding/xml"

type DIDLLite struct {
	XMLName    xml.Name    `xml:"urn:schemas-upnp-org:metadata-1-0/DIDL-Lite/ DIDL-Lite"`
	XMLNSDC    string      `xml:"xmlns:dc,attr"`
	XMLNSUPnP  string      `xml:"xmlns:upnp,attr"`
	Containers []container `xml:"container"`
	Items      []item      `xml:"item"`
}

type container struct {
	ID         string `xml:"id,attr"`
	Restricted bool   `xml:"restricted,attr"`
	ParentID   string `xml:"parentID,attr"`
	Searchable bool   `xml:"searchable,attr"`
	ChildCount int    `xml:"childCount,attr"`

	Title       string `xml:"dc:title"`
	Class       string `xml:"upnp:class"`
	StorageUsed int64  `xml:"upnp:storageUsed"`
}

type item struct {
	ID         string  `xml:"id,attr"`
	RefID      *string `xml:"refID,attr"`
	ParentID   string  `xml:"parentID,attr"`
	Restricted bool    `xml:"restricted,attr"`

	Title string `xml:"dc:title"`
	Class string `xml:"upnp:class"`
	Res   res    `xml:"res"`
}

type res struct {
	ProtocolInfo string `xml:"protocolInfo,attr"`
	URL          string `xml:",innerxml"`
}
