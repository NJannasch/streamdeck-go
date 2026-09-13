package streamdeck

import "github.com/buglloc/usbhid"

type hidDevice interface {
	Path() string
	ProductId() uint16
	SerialNumber() string
	Open(lock bool) error
	Close() error
	GetInputReport() (byte, []byte, error)
	GetFeatureReport(reportID byte) ([]byte, error)
	SetFeatureReport(reportID byte, data []byte) error
	SetOutputReport(reportID byte, data []byte) error
}

type usbHIDDevice struct {
	*usbhid.Device
}

func enumerate() ([]hidDevice, error) {
	devices, err := usbhid.Enumerate(usbhid.WithVidFilter(VendorID))
	if err != nil {
		return nil, err
	}

	result := make([]hidDevice, 0, len(devices))
	for _, device := range devices {
		result = append(result, &usbHIDDevice{Device: device})
	}
	return result, nil
}
