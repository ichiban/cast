package upnp

import (
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"

	"github.com/stretchr/testify/assert"
	"golang.org/x/net/nettest"
)

func TestNewDevice(t *testing.T) {
	i, err := nettest.RoutedInterface("ip4", net.FlagUp|net.FlagBroadcast|net.FlagMulticast)
	assert.NoError(t, err)

	t.Run("nil interface", func(t *testing.T) {
		_, err := NewDevice(nil, "foo", nil)
		assert.Error(t, err)
	})

	t.Run("empty services", func(t *testing.T) {
		d, err := NewDevice(i, "foo", nil)
		assert.NoError(t, err)
		assert.NotNil(t, d)
	})

	t.Run("multiple services", func(t *testing.T) {
		var si1 mockServiceImpl
		si1.On("SetBaseURL", mock.Anything).Return().Once()
		defer si1.AssertExpectations(t)

		var si2 mockServiceImpl
		si2.On("SetBaseURL", mock.Anything).Return().Once()
		defer si2.AssertExpectations(t)

		var si3 mockServiceImpl
		si3.On("SetBaseURL", mock.Anything).Return().Once()
		defer si3.AssertExpectations(t)

		d, err := NewDevice(i, "foo", []Service{
			{
				Impl: &si1,
			},
			{
				Impl: &si2,
			},
			{
				Impl: &si3,
			},
		})
		assert.NoError(t, err)
		assert.NotNil(t, d)
		assert.Len(t, d.Services, 3)
	})
}

func TestDevice_Describe(t *testing.T) {
	d := Device{
		Type: "foo",
		Services: []Service{
			{
				Type: "serviceType1",
				ID:   "serviceID1",
			},
			{
				Type: "serviceType2",
				ID:   "serviceID2",
			},
			{
				Type: "serviceType3",
				ID:   "serviceID3",
			},
		},
	}
	w := httptest.NewRecorder()
	d.Describe(w, nil)
	resp := w.Result()
	body, err := ioutil.ReadAll(resp.Body)
	assert.NoError(t, err)
	assert.Equal(t, `<?xml version="1.0"?>
<root xmlns="urn:schemas-upnp-org:device-1-0" configId="0">
  <specVersion>
    <major>1</major>
    <minor>0</minor>
  </specVersion>
  <device>
    <deviceType>foo</deviceType>
    <friendlyName>picoms</friendlyName>
    <manufacturer>ichiban</manufacturer>
    <modelName>picoms/0.0</modelName>
    <UDN>uuid:00000000-0000-0000-0000-000000000000</UDN>
    <serviceList>
      <service>
        <serviceType>serviceType1</serviceType>
        <serviceId>serviceID1</serviceId>
        <SCPDURL>/0/service</SCPDURL>
        <controlURL>/0/control</controlURL>
        <eventSubURL>/0/event</eventSubURL>
      </service>
      <service>
        <serviceType>serviceType2</serviceType>
        <serviceId>serviceID2</serviceId>
        <SCPDURL>/1/service</SCPDURL>
        <controlURL>/1/control</controlURL>
        <eventSubURL>/1/event</eventSubURL>
      </service>
      <service>
        <serviceType>serviceType3</serviceType>
        <serviceId>serviceID3</serviceId>
        <SCPDURL>/2/service</SCPDURL>
        <controlURL>/2/control</controlURL>
        <eventSubURL>/2/event</eventSubURL>
      </service>
    </serviceList>
  </device>
</root>`, string(body))
}

func TestDevice_Advertise(t *testing.T) {
	var c mockConn
	c.On("Write", mock.Anything).Return(1024, nil).Times(10)
	c.On("Close").Return(nil)
	defer c.AssertExpectations(t)

	netDial = func(network, address string) (conn net.Conn, err error) {
		return &c, nil
	}
	defer func() { netDial = net.Dial }()

	done := make(chan struct{}, 1)

	var wg sync.WaitGroup

	d := Device{
		SearchAddr: &net.UDPAddr{},
		Interval:   300 * time.Millisecond,
		Services: []Service{
			{
				Type: "foo",
			},
			{
				Type: "foo",
			},
			{
				Type: "bar",
			},
		},
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		assert.NoError(t, d.Advertise(done))
	}()

	time.Sleep(500 * time.Millisecond)
	done <- struct{}{}
	wg.Wait()
}

func TestDevice_ReplySearch(t *testing.T) {
	done := make(chan struct{}, 1)

	var wg sync.WaitGroup

	d := Device{}
	wg.Add(1)
	go func() {
		wg.Done()
		assert.NoError(t, d.ReplySearch(done))
	}()

	done <- struct{}{}
	wg.Wait()
}

type mockServiceImpl struct {
	mock.Mock
}

func (m *mockServiceImpl) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	_ = m.Called(w, r)
}

func (m *mockServiceImpl) SetBaseURL(u *url.URL) {
	_ = m.Called(u)
}

type mockConn struct {
	mock.Mock
}

func (m *mockConn) Read(b []byte) (n int, err error) {
	args := m.Called(b)
	return args.Int(0), args.Error(1)
}

func (m *mockConn) Write(b []byte) (n int, err error) {
	args := m.Called(b)
	return args.Int(0), args.Error(1)
}

func (m *mockConn) Close() error {
	args := m.Called()
	return args.Error(0)
}

func (m *mockConn) LocalAddr() net.Addr {
	args := m.Called()
	return args.Get(0).(net.Addr)
}

func (m *mockConn) RemoteAddr() net.Addr {
	args := m.Called()
	return args.Get(0).(net.Addr)
}

func (m *mockConn) SetDeadline(t time.Time) error {
	args := m.Called(t)
	return args.Error(0)
}

func (m *mockConn) SetReadDeadline(t time.Time) error {
	args := m.Called(t)
	return args.Error(0)
}

func (m *mockConn) SetWriteDeadline(t time.Time) error {
	args := m.Called(t)
	return args.Error(0)
}
