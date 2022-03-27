package cast

import (
	"net/http"
	"net/url"
	"runtime/debug"

	"github.com/sirupsen/logrus"
)

type Description struct {
	BaseURL      *url.URL
	FriendlyName string
	UUID         string
}

func (d *Description) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	if _, err := w.Write([]byte(xmlDeclaration)); err != nil {
		logrus.WithError(err).Error("Failed to write XML declaration.")
		return
	}
	if err := Template.ExecuteTemplate(w, "device_description.gohtml", d); err != nil {
		logrus.WithError(err).Error("Failed to describe.")
	}
}

func (d *Description) Version() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}
	return info.Main.Version
}
