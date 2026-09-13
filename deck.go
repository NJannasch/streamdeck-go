package streamdeck

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"sync"
	"time"
)

const (
	featureReportSize = 32
	outputReportSize  = 1024
)

// KeyEvent reports a physical key transition. Key zero is the top-left key.
type KeyEvent struct {
	Key     int
	Pressed bool
}

// DialEvent reports a dial press transition or rotation. Ticks is signed and
// nonzero for rotations; Pressed is used for push/release events.
type DialEvent struct {
	Dial    int
	Pressed bool
	Ticks   int
	Turn    bool
}

// TouchType identifies a tap, long press, or drag on a touchscreen.
type TouchType uint8

const (
	TouchTap TouchType = iota + 1
	TouchLongPress
	TouchDrag
)

// TouchEvent reports touchscreen coordinates. EndX and EndY are set for drags.
type TouchEvent struct {
	Type       TouchType
	X, Y       int
	EndX, EndY int
}

// UnitInfo is the geometry and image-gallery capacity reported by main-protocol
// firmware. Some firmware versions omit the trailing fields.
type UnitInfo struct {
	Rows, Columns       int
	KeyWidth, KeyHeight int
	LCDWidth, LCDHeight int
	BitsPerPixel        int
	ColorScheme         int
	KeyGalleryImages    int
	LCDGalleryImages    int
	DemoFrames          int
}

// Deck is an exclusively opened Stream Deck.
type Deck struct {
	info DeviceInfo
	dev  hidDevice

	writeMu sync.Mutex
	closeMu sync.Mutex
	closed  bool
	closeCh chan struct{}
	doneCh  chan struct{}
	events  chan KeyEvent
	dials   chan DialEvent
	touches chan TouchEvent
	errors  chan error
}

func newDeck(info DeviceInfo, dev hidDevice) *Deck {
	d := &Deck{
		info:    info,
		dev:     dev,
		closeCh: make(chan struct{}),
		doneCh:  make(chan struct{}),
		events:  make(chan KeyEvent, 32),
		dials:   make(chan DialEvent, 32),
		touches: make(chan TouchEvent, 32),
		errors:  make(chan error, 1),
	}
	go d.readLoop()
	return d
}

// Info returns the identity and model of the opened device.
func (d *Deck) Info() DeviceInfo { return d.info }

// Events returns key press and release events until the deck is closed. Events
// may be dropped when its buffer is full; callers should drain it continuously.
func (d *Deck) Events() <-chan KeyEvent { return d.events }

// DialEvents returns dial push, release, and rotation events. Events may be
// dropped when its buffer is full so unused control types cannot block keys.
func (d *Deck) DialEvents() <-chan DialEvent { return d.dials }

// TouchEvents returns touchscreen tap, long-press, and drag events. Events may
// be dropped when its buffer is full so unused control types cannot block keys.
func (d *Deck) TouchEvents() <-chan TouchEvent { return d.touches }

// Errors reports a terminal background read error. The channel closes with Deck.
func (d *Deck) Errors() <-chan error { return d.errors }

// Done closes after the input reader stops. It does not consume Errors and is
// useful for connection supervisors.
func (d *Deck) Done() <-chan struct{} { return d.doneCh }

// Close releases the device. It is safe to call more than once.
func (d *Deck) Close() error {
	d.closeMu.Lock()
	if d.closed {
		d.closeMu.Unlock()
		return nil
	}
	d.closed = true
	close(d.closeCh)
	d.closeMu.Unlock()
	return d.dev.Close()
}

func (d *Deck) isClosed() bool {
	d.closeMu.Lock()
	defer d.closeMu.Unlock()
	return d.closed
}

// SetBrightness changes display brightness, clamped to the range 0..100.
func (d *Deck) SetBrightness(percent int) error {
	if d.info.Model.protocol == protocolPedal {
		return errors.New("streamdeck: pedal has no display brightness")
	}
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}
	model := d.info.Model
	if model.protocol == protocolGen1 || model.protocol == protocolMini {
		payload := make([]byte, model.featureSize-1)
		copy(payload, []byte{0x55, 0xaa, 0xd1, 0x01, byte(percent)})
		return d.setFeatureReport(0x05, payload)
	}
	payload := make([]byte, model.featureSize-1)
	payload[0], payload[1] = 0x08, byte(percent)
	return d.setFeatureReport(0x03, payload)
}

// Reset asks the device to reset its display state.
func (d *Deck) Reset() error {
	model := d.info.Model
	if model.protocol == protocolPedal {
		return errors.New("streamdeck: pedal does not support reset")
	}
	if model.protocol == protocolGen1 || model.protocol == protocolMini {
		payload := make([]byte, model.featureSize-1)
		payload[0] = 0x63
		return d.setFeatureReport(0x0b, payload)
	}
	payload := make([]byte, model.featureSize-1)
	payload[0] = 0x02
	return d.setFeatureReport(0x03, payload)
}

// FirmwareVersion returns the firmware version reported by the device.
func (d *Deck) FirmwareVersion() (string, error) {
	model := d.info.Model
	report, err := d.getFeatureReport(model.firmwareReport)
	if err != nil {
		return "", err
	}
	// The report ID is removed by the HID transport. The string starts at
	// byte six of the complete report and therefore byte five here.
	if len(report) <= model.firmwareOffset {
		return "", errors.New("streamdeck: short firmware report")
	}
	return reportString(report[model.firmwareOffset:]), nil
}

// SerialNumber reads the serial number from the device firmware.
func (d *Deck) SerialNumber() (string, error) {
	model := d.info.Model
	report, err := d.getFeatureReport(model.serialReport)
	if err != nil {
		return "", err
	}
	if len(report) <= model.serialOffset {
		return "", errors.New("streamdeck: short serial report")
	}
	return reportString(report[model.serialOffset:]), nil
}

// UnitInfo reads hardware geometry from main-protocol devices.
func (d *Deck) UnitInfo() (UnitInfo, error) {
	if d.info.Model.protocol != protocolGen2 {
		return UnitInfo{}, fmt.Errorf("streamdeck: %s does not report unit information", d.info.Model.Name)
	}
	report, err := d.getFeatureReport(0x08)
	if err != nil {
		return UnitInfo{}, err
	}
	if len(report) < 12 {
		return UnitInfo{}, errors.New("streamdeck: short unit information report")
	}
	info := UnitInfo{
		Rows: int(report[0]), Columns: int(report[1]),
		KeyWidth: int(binary.LittleEndian.Uint16(report[2:4])), KeyHeight: int(binary.LittleEndian.Uint16(report[4:6])),
		LCDWidth: int(binary.LittleEndian.Uint16(report[6:8])), LCDHeight: int(binary.LittleEndian.Uint16(report[8:10])),
		BitsPerPixel: int(report[10]), ColorScheme: int(report[11]),
	}
	if len(report) > 12 {
		info.KeyGalleryImages = int(report[12])
	}
	if len(report) > 13 {
		info.LCDGalleryImages = int(report[13])
	}
	if len(report) > 14 {
		info.DemoFrames = int(report[14])
	}
	return info, nil
}

// SetKeyImage scales an image to the key, applies the model transform, encodes
// it as the native JPEG or BMP format, and displays it. Key zero is top-left.
func (d *Deck) SetKeyImage(key int, img image.Image) error {
	if key < 0 || key >= d.info.Model.KeyCount() {
		return fmt.Errorf("streamdeck: key index %d outside 0..%d", key, d.info.Model.KeyCount()-1)
	}
	if img == nil {
		return errors.New("streamdeck: image is nil")
	}

	encoded, err := encodeKeyImage(d.info.Model, img)
	if err != nil {
		return err
	}
	return d.SetKeyImageData(key, encoded)
}

// SetKeyImageData writes a pre-encoded JPEG or BMP in the model's native
// format. This avoids re-encoding frames in animation loops.
func (d *Deck) SetKeyImageData(key int, encoded []byte) error {
	model := d.info.Model
	if key < 0 || key >= model.KeyCount() {
		return fmt.Errorf("streamdeck: key index %d outside 0..%d", key, model.KeyCount()-1)
	}
	if !model.HasDisplay() {
		return fmt.Errorf("streamdeck: %s has no key display", model.Name)
	}
	if len(encoded) == 0 {
		return errors.New("streamdeck: encoded image is empty")
	}
	if model.KeyFormat == ImageFormatJPEG && !(len(encoded) >= 2 && encoded[0] == 0xff && encoded[1] == 0xd8) {
		return errors.New("streamdeck: model requires JPEG data")
	}
	if model.KeyFormat == ImageFormatBMP && !(len(encoded) >= 2 && encoded[0] == 'B' && encoded[1] == 'M') {
		return errors.New("streamdeck: model requires BMP data")
	}
	packets := keyImagePackets(model, key, encoded)

	d.writeMu.Lock()
	defer d.writeMu.Unlock()
	if d.isClosed() {
		return ErrClosed
	}
	for _, packet := range packets {
		if err := d.dev.SetOutputReport(packet[0], packet[1:]); err != nil {
			return fmt.Errorf("streamdeck: write image for key %d: %w", key, err)
		}
	}
	return nil
}

// SetTouchscreenImage displays an image in a rectangular region of a Plus
// touchscreen. Width and height are inferred from the source image bounds.
func (d *Deck) SetTouchscreenImage(x, y int, img image.Image) error {
	if img == nil {
		return errors.New("streamdeck: image is nil")
	}
	m := d.info.Model
	if m.TouchscreenWidth == 0 {
		return fmt.Errorf("streamdeck: %s has no touchscreen", m.Name)
	}
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	if x < 0 || y < 0 || w < 1 || h < 1 || x+w > m.TouchscreenWidth || y+h > m.TouchscreenHeight {
		return fmt.Errorf("streamdeck: touchscreen rectangle (%d,%d %dx%d) outside %dx%d", x, y, w, h, m.TouchscreenWidth, m.TouchscreenHeight)
	}
	encoded, err := encodeDisplayImage(img, w, h, false, false, m.rotation)
	if err != nil {
		return err
	}
	ix, iy, iw, ih := x, y, w, h
	if m.rotation == 90 {
		ix, iy, iw, ih = y, x, h, w
	}
	return d.writePackets("touchscreen image", lcdPackets(0x0c, encoded, ix, iy, iw, ih))
}

// SetScreenImage sets the Neo status-bar display.
func (d *Deck) SetScreenImage(img image.Image) error {
	if img == nil {
		return errors.New("streamdeck: image is nil")
	}
	m := d.info.Model
	if m.ScreenWidth == 0 {
		return fmt.Errorf("streamdeck: %s has no separate screen", m.Name)
	}
	encoded, err := encodeDisplayImage(img, m.ScreenWidth, m.ScreenHeight, true, true, 0)
	if err != nil {
		return err
	}
	return d.writePackets("screen image", screenPackets(encoded))
}

// SetWindowImage sets the complete Neo status window or Plus touch window.
func (d *Deck) SetWindowImage(img image.Image) error {
	if img == nil {
		return errors.New("streamdeck: image is nil")
	}
	m := d.info.Model
	w, h := m.ScreenWidth, m.ScreenHeight
	if w == 0 {
		w, h = m.TouchscreenWidth, m.TouchscreenHeight
	}
	if w == 0 {
		return fmt.Errorf("streamdeck: %s has no window", m.Name)
	}
	encoded, err := encodeDisplayImage(img, w, h, m.flipX, m.flipY, m.rotation)
	if err != nil {
		return err
	}
	return d.writePackets("window image", commandImagePackets(0x0b, 0, encoded))
}

// SetLCDImage uploads one JPEG across the physical LCD beneath all keys.
func (d *Deck) SetLCDImage(img image.Image) error {
	if img == nil {
		return errors.New("streamdeck: image is nil")
	}
	m := d.info.Model
	if m.LCDWidth == 0 {
		return fmt.Errorf("streamdeck: %s does not expose full-LCD upload", m.Name)
	}
	encoded, err := encodeDisplayImage(img, m.LCDWidth, m.LCDHeight, m.flipX, m.flipY, m.rotation)
	if err != nil {
		return err
	}
	return d.writePackets("LCD image", commandImagePackets(0x08, 0, encoded))
}

// StoreBackground uploads a reusable full-LCD background on Classic and XL
// family devices. The available index count depends on device firmware.
func (d *Deck) StoreBackground(index byte, img image.Image) error {
	if img == nil {
		return errors.New("streamdeck: image is nil")
	}
	m := d.info.Model
	if !supportsBackground(m) {
		return fmt.Errorf("streamdeck: %s does not expose background storage", m.Name)
	}
	encoded, err := encodeDisplayImage(img, m.LCDWidth, m.LCDHeight, m.flipX, m.flipY, m.rotation)
	if err != nil {
		return err
	}
	return d.writePackets("background", backgroundPackets(index, encoded))
}

// ShowBackground displays a background previously uploaded with StoreBackground.
func (d *Deck) ShowBackground(index byte) error {
	if !supportsBackground(d.info.Model) {
		return fmt.Errorf("streamdeck: %s does not expose background storage", d.info.Model.Name)
	}
	payload := make([]byte, d.info.Model.featureSize-1)
	payload[0], payload[1] = 0x13, index
	return d.setFeatureReport(0x03, payload)
}

func supportsBackground(model Model) bool {
	switch model.ProductID {
	case 0x006d, 0x0080, 0x00a5, 0x00b9, 0x006c, 0x008f, 0x00ba:
		return true
	default:
		return false
	}
}

// SetLCDColor fills the complete LCD with a color using a feature command.
func (d *Deck) SetLCDColor(value color.Color) error {
	if value == nil {
		return errors.New("streamdeck: color is nil")
	}
	if d.info.Model.protocol != protocolGen2 {
		return fmt.Errorf("streamdeck: %s does not support direct LCD color", d.info.Model.Name)
	}
	r, g, b, _ := value.RGBA()
	payload := make([]byte, d.info.Model.featureSize-1)
	payload[0], payload[1], payload[2], payload[3] = 0x05, byte(r>>8), byte(g>>8), byte(b>>8)
	return d.setFeatureReport(0x03, payload)
}

// SetSleepTimeout configures idle sleep. A zero duration disables sleep.
func (d *Deck) SetSleepTimeout(duration time.Duration) error {
	if d.info.Model.protocol != protocolGen2 {
		return fmt.Errorf("streamdeck: %s does not support sleep configuration", d.info.Model.Name)
	}
	if duration < 0 || duration/time.Second > time.Duration(^uint32(0)>>1) {
		return errors.New("streamdeck: invalid sleep timeout")
	}
	payload := make([]byte, d.info.Model.featureSize-1)
	payload[0] = 0x0d
	binary.LittleEndian.PutUint32(payload[1:5], uint32(duration/time.Second))
	return d.setFeatureReport(0x03, payload)
}

// SleepTimeout returns the configured idle sleep duration.
func (d *Deck) SleepTimeout() (time.Duration, error) {
	if d.info.Model.protocol != protocolGen2 {
		return 0, fmt.Errorf("streamdeck: %s does not support sleep configuration", d.info.Model.Name)
	}
	report, err := d.getFeatureReport(0x0a)
	if err != nil {
		return 0, err
	}
	if len(report) < 5 {
		return 0, errors.New("streamdeck: short sleep timeout report")
	}
	seconds := int32(binary.LittleEndian.Uint32(report[1:5]))
	if seconds < 0 {
		return 0, errors.New("streamdeck: invalid sleep timeout report")
	}
	return time.Duration(seconds) * time.Second, nil
}

// SetEncoderColor sets a Stream Deck Studio encoder knob LED.
func (d *Deck) SetEncoderColor(encoder int, value color.Color) error {
	if value == nil {
		return errors.New("streamdeck: color is nil")
	}
	if d.info.Model.ProductID != StreamDeckStudio.ProductID {
		return fmt.Errorf("streamdeck: %s has no encoder LEDs", d.info.Model.Name)
	}
	if encoder < 0 || encoder >= d.info.Model.DialCount {
		return fmt.Errorf("streamdeck: encoder index %d outside range", encoder)
	}
	r, g, b, _ := value.RGBA()
	packet := []byte{0x02, 0x10, byte(encoder), byte(r >> 8), byte(g >> 8), byte(b >> 8)}
	return d.writePackets("encoder color", [][]byte{packet})
}

// SetEncoderRing sets all 24 segments around a Stream Deck Studio encoder.
// Missing colors are treated as black; extra colors are ignored.
func (d *Deck) SetEncoderRing(encoder int, colors []color.Color) error {
	if d.info.Model.ProductID != StreamDeckStudio.ProductID {
		return fmt.Errorf("streamdeck: %s has no encoder LEDs", d.info.Model.Name)
	}
	if encoder < 0 || encoder >= d.info.Model.DialCount {
		return fmt.Errorf("streamdeck: encoder index %d outside range", encoder)
	}
	packet := make([]byte, 3+24*3)
	packet[0], packet[1], packet[2] = 0x02, 0x0f, byte(encoder)
	for index := 0; index < len(colors) && index < 24; index++ {
		if colors[index] == nil {
			continue
		}
		r, g, b, _ := colors[index].RGBA()
		packet[3+index*3], packet[4+index*3], packet[5+index*3] = byte(r>>8), byte(g>>8), byte(b>>8)
	}
	return d.writePackets("encoder ring", [][]byte{packet})
}

func (d *Deck) writePackets(label string, packets [][]byte) error {
	d.writeMu.Lock()
	defer d.writeMu.Unlock()
	if d.isClosed() {
		return ErrClosed
	}
	for _, packet := range packets {
		if err := d.dev.SetOutputReport(packet[0], packet[1:]); err != nil {
			return fmt.Errorf("streamdeck: write %s: %w", label, err)
		}
	}
	return nil
}

// SetKeyColor fills one key with a solid color.
func (d *Deck) SetKeyColor(key int, value color.Color) error {
	if value == nil {
		return errors.New("streamdeck: color is nil")
	}
	if d.info.Model.protocol == protocolGen2 {
		if key < 0 || key >= d.info.Model.InputCount() {
			return fmt.Errorf("streamdeck: key index %d outside 0..%d", key, d.info.Model.InputCount()-1)
		}
		r, g, b, _ := value.RGBA()
		payload := make([]byte, d.info.Model.featureSize-1)
		payload[0], payload[1], payload[2], payload[3], payload[4] = 0x06, byte(key), byte(r>>8), byte(g>>8), byte(b>>8)
		return d.setFeatureReport(0x03, payload)
	}
	if !d.info.Model.HasDisplay() {
		return fmt.Errorf("streamdeck: %s has no key display", d.info.Model.Name)
	}
	bounds := image.Rect(0, 0, d.info.Model.KeyWidth, d.info.Model.KeyHeight)
	img := image.NewRGBA(bounds)
	draw.Draw(img, bounds, image.NewUniform(value), image.Point{}, draw.Src)
	return d.SetKeyImage(key, img)
}

// ClearKey fills one key with black.
func (d *Deck) ClearKey(key int) error {
	return d.SetKeyColor(key, color.Black)
}

// Clear fills every key with black.
func (d *Deck) Clear() error {
	for key := 0; key < d.info.Model.KeyCount(); key++ {
		if err := d.ClearKey(key); err != nil {
			return err
		}
	}
	return nil
}

// SetPanelImage displays one image across the complete key grid. The image is
// scaled to the logical key area; the physical gaps between keys remain visible.
func (d *Deck) SetPanelImage(src image.Image) error {
	if src == nil {
		return errors.New("streamdeck: image is nil")
	}
	model := d.info.Model
	width := model.Columns * model.KeyWidth
	height := model.Rows * model.KeyHeight
	panel := resizeImage(src, width, height)

	for row := 0; row < model.Rows; row++ {
		for column := 0; column < model.Columns; column++ {
			bounds := image.Rect(
				column*model.KeyWidth,
				row*model.KeyHeight,
				(column+1)*model.KeyWidth,
				(row+1)*model.KeyHeight,
			)
			key := row*model.Columns + column
			if err := d.SetKeyImage(key, panel.SubImage(bounds)); err != nil {
				return fmt.Errorf("streamdeck: set panel key %d: %w", key, err)
			}
		}
	}
	return nil
}

func (d *Deck) setFeatureReport(reportID byte, payload []byte) error {
	d.writeMu.Lock()
	defer d.writeMu.Unlock()
	if d.isClosed() {
		return ErrClosed
	}
	if err := d.dev.SetFeatureReport(reportID, payload); err != nil {
		return fmt.Errorf("streamdeck: set feature report 0x%02x: %w", reportID, err)
	}
	return nil
}

func (d *Deck) getFeatureReport(reportID byte) ([]byte, error) {
	if d.isClosed() {
		return nil, ErrClosed
	}
	report, err := d.dev.GetFeatureReport(reportID)
	if err != nil {
		return nil, fmt.Errorf("streamdeck: get feature report 0x%02x: %w", reportID, err)
	}
	return report, nil
}

func (d *Deck) readLoop() {
	defer close(d.doneCh)
	defer close(d.events)
	defer close(d.dials)
	defer close(d.touches)
	defer close(d.errors)
	states := make([]bool, d.info.Model.InputCount())
	dialStates := make([]bool, d.info.Model.DialCount)
	for {
		reportID, payload, err := d.dev.GetInputReport()
		if err != nil {
			if !d.isClosed() {
				select {
				case d.errors <- fmt.Errorf("streamdeck: read keys: %w", err):
				default:
				}
			}
			return
		}

		report := make([]byte, 1+len(payload))
		report[0] = reportID
		copy(report[1:], payload)
		for _, event := range keyEvents(d.info.Model, states, report) {
			states[event.Key] = event.Pressed
			select {
			case d.events <- event:
			case <-d.closeCh:
				return
			default:
			}
		}
		for _, event := range dialEvents(d.info.Model, dialStates, report) {
			if !event.Turn {
				dialStates[event.Dial] = event.Pressed
			}
			select {
			case d.dials <- event:
			case <-d.closeCh:
				return
			default:
			}
		}
		if event, ok := touchEvent(d.info.Model, report); ok {
			select {
			case d.touches <- event:
			case <-d.closeCh:
				return
			default:
			}
		}
	}
}

func keyEvents(model Model, previous []bool, report []byte) []KeyEvent {
	if len(previous) != model.InputCount() || len(report) < model.keyOffset+model.InputCount() {
		return nil
	}
	if model.DialCount > 0 && len(report) > 1 && report[1] != 0 {
		return nil
	}
	events := make([]KeyEvent, 0)
	for key, old := range previous {
		physicalKey := key
		if model.reverseColumns {
			physicalKey = reverseKey(model, key)
		}
		pressed := report[model.keyOffset+physicalKey] != 0
		if pressed != old {
			events = append(events, KeyEvent{Key: key, Pressed: pressed})
		}
	}
	return events
}

func dialEvents(model Model, previous []bool, report []byte) []DialEvent {
	if model.DialCount == 0 || len(report) < 5+model.DialCount || report[1] != 0x03 {
		return nil
	}
	subtype := report[4]
	result := make([]DialEvent, 0, model.DialCount)
	for dial := 0; dial < model.DialCount; dial++ {
		value := report[5+dial]
		if subtype == 1 && value != 0 {
			result = append(result, DialEvent{Dial: dial, Ticks: int(int8(value)), Turn: true})
		}
		if subtype == 0 {
			pressed := value != 0
			if dial < len(previous) && pressed != previous[dial] {
				result = append(result, DialEvent{Dial: dial, Pressed: pressed})
			}
		}
	}
	return result
}

func touchEvent(model Model, report []byte) (TouchEvent, bool) {
	if model.TouchscreenWidth == 0 || len(report) < 10 || report[1] != 0x02 {
		return TouchEvent{}, false
	}
	typeValue := TouchType(report[4])
	if typeValue < TouchTap || typeValue > TouchDrag {
		return TouchEvent{}, false
	}
	e := TouchEvent{Type: typeValue, X: int(binary.LittleEndian.Uint16(report[6:8])), Y: int(binary.LittleEndian.Uint16(report[8:10]))}
	if typeValue == TouchDrag && len(report) >= 14 {
		e.EndX, e.EndY = int(binary.LittleEndian.Uint16(report[10:12])), int(binary.LittleEndian.Uint16(report[12:14]))
	}
	return e, true
}

func reportString(data []byte) string {
	if index := bytes.IndexByte(data, 0); index >= 0 {
		data = data[:index]
	}
	return string(bytes.TrimSpace(data))
}

func resetImageStream(device hidDevice, model Model) error {
	if model.protocol == protocolPedal {
		return nil
	}
	size := outputReportSize
	if model.protocol == protocolGen1 {
		size = 8191
	}
	return device.SetOutputReport(0x02, make([]byte, size-1))
}
