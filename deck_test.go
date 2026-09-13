package streamdeck

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"sync"
	"testing"
	"time"
)

type report struct {
	id   byte
	data []byte
}

type fakeDevice struct {
	mu       sync.Mutex
	features []report
	outputs  []report
	input    chan report
	closed   chan struct{}
}

func newFakeDevice() *fakeDevice {
	return &fakeDevice{input: make(chan report, 4), closed: make(chan struct{})}
}

func (f *fakeDevice) Path() string                          { return "/dev/fake" }
func (f *fakeDevice) ProductId() uint16                     { return StreamDeckMK2.ProductID }
func (f *fakeDevice) SerialNumber() string                  { return "FAKE123" }
func (f *fakeDevice) Open(bool) error                       { return nil }
func (f *fakeDevice) GetFeatureReport(byte) ([]byte, error) { return nil, nil }
func (f *fakeDevice) Close() error {
	select {
	case <-f.closed:
	default:
		close(f.closed)
	}
	return nil
}
func (f *fakeDevice) GetInputReport() (byte, []byte, error) {
	select {
	case value := <-f.input:
		return value.id, value.data, nil
	case <-f.closed:
		return 0, nil, errors.New("closed")
	}
}
func (f *fakeDevice) SetFeatureReport(id byte, data []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.features = append(f.features, report{id: id, data: append([]byte(nil), data...)})
	return nil
}
func (f *fakeDevice) SetOutputReport(id byte, data []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.outputs = append(f.outputs, report{id: id, data: append([]byte(nil), data...)})
	return nil
}

func TestBrightnessPacket(t *testing.T) {
	device := newFakeDevice()
	deck := newDeck(DeviceInfo{Model: StreamDeckMK2}, device)
	defer deck.Close()

	if err := deck.SetBrightness(140); err != nil {
		t.Fatal(err)
	}
	if len(device.features) != 1 {
		t.Fatalf("got %d feature reports, want 1", len(device.features))
	}
	got := device.features[0]
	if got.id != 0x03 || len(got.data) != 31 || got.data[0] != 0x08 || got.data[1] != 100 {
		t.Fatalf("unexpected brightness report: id=%02x data=%x", got.id, got.data[:2])
	}
}

func TestKeyEventsReportTransitions(t *testing.T) {
	previous := make([]bool, StreamDeckMK2.KeyCount())
	report := make([]byte, StreamDeckMK2.keyOffset+StreamDeckMK2.KeyCount())
	report[StreamDeckMK2.keyOffset+3] = 1
	report[StreamDeckMK2.keyOffset+14] = 1

	events := keyEvents(StreamDeckMK2, previous, report)
	if len(events) != 2 || events[0] != (KeyEvent{Key: 3, Pressed: true}) || events[1] != (KeyEvent{Key: 14, Pressed: true}) {
		t.Fatalf("unexpected events: %#v", events)
	}
}

func TestImagePacketsCanBeReassembled(t *testing.T) {
	encoded := make([]byte, 2500)
	for i := range encoded {
		encoded[i] = byte(i)
	}

	packets := imagePackets(7, encoded)
	if len(packets) != 3 {
		t.Fatalf("got %d packets, want 3", len(packets))
	}
	var rebuilt []byte
	for page, packet := range packets {
		if len(packet) != outputReportSize || packet[0] != 0x02 || packet[1] != 0x07 || packet[2] != 7 {
			t.Fatalf("bad packet %d header: %x", page, packet[:8])
		}
		if gotPage := binary.LittleEndian.Uint16(packet[6:8]); int(gotPage) != page {
			t.Fatalf("packet %d has page %d", page, gotPage)
		}
		length := int(binary.LittleEndian.Uint16(packet[4:6]))
		rebuilt = append(rebuilt, packet[8:8+length]...)
	}
	if packets[len(packets)-1][3] != 1 {
		t.Fatal("last packet is not marked final")
	}
	if !equalBytes(rebuilt, encoded) {
		t.Fatal("reassembled image differs from input")
	}
}

func TestEncodeKeyImage(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 10, 20))
	src.Set(0, 0, color.White)
	encoded, err := encodeKeyImage(StreamDeckMK2, src)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := jpeg.Decode(bytes.NewReader(encoded))
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Bounds().Dx() != 72 || decoded.Bounds().Dy() != 72 {
		t.Fatalf("encoded bounds are %v", decoded.Bounds())
	}
}

func TestSetPanelImageWritesEveryKey(t *testing.T) {
	device := newFakeDevice()
	deck := newDeck(DeviceInfo{Model: StreamDeckMK2}, device)
	defer deck.Close()

	panel := image.NewRGBA(image.Rect(0, 0, 360, 216))
	if err := deck.SetPanelImage(panel); err != nil {
		t.Fatal(err)
	}

	finalPackets := make(map[int]bool)
	for _, output := range device.outputs {
		if output.id != 0x02 || len(output.data) < 3 || output.data[0] != 0x07 {
			t.Fatalf("unexpected output report: id=%02x data=%x", output.id, output.data[:min(3, len(output.data))])
		}
		if output.data[2] == 1 {
			finalPackets[int(output.data[1])] = true
		}
	}
	for key := 0; key < StreamDeckMK2.KeyCount(); key++ {
		if !finalPackets[key] {
			t.Errorf("key %d did not receive a final image packet", key)
		}
	}
}

func TestSupportedProductIDs(t *testing.T) {
	ids := []uint16{0x0060, 0x0063, 0x006c, 0x006d, 0x0080, 0x0084, 0x0086, 0x008f, 0x0090, 0x009a, 0x00a5, 0x00aa, 0x00b3, 0x00b8, 0x00b9, 0x00ba, 0x00c6}
	for _, id := range ids {
		if _, err := modelForProduct(id); err != nil {
			t.Errorf("product 0x%04x: %v", id, err)
		}
	}
}

func TestMiniImageUsesBMPProtocol(t *testing.T) {
	encoded, err := encodeKeyImage(StreamDeckMini, image.NewRGBA(image.Rect(0, 0, 80, 80)))
	if err != nil {
		t.Fatal(err)
	}
	if len(encoded) < 2 || string(encoded[:2]) != "BM" {
		t.Fatalf("image does not start with BMP signature: %x", encoded[:min(2, len(encoded))])
	}
	packets := keyImagePackets(StreamDeckMini, 2, encoded)
	if len(packets) == 0 || len(packets[0]) != 1024 {
		t.Fatalf("unexpected packets: %d", len(packets))
	}
	if got := packets[0][:6]; !bytes.Equal(got, []byte{0x02, 0x01, 0x00, 0x00, 0x00, 0x03}) {
		t.Fatalf("unexpected header: %x", got)
	}
}

func TestOriginalReversesKeyAndUsesTwoPackets(t *testing.T) {
	encoded := append([]byte("BM"), make([]byte, 15000)...)
	packets := keyImagePackets(StreamDeckOriginal, 0, encoded)
	if len(packets) != 2 {
		t.Fatalf("got %d packets", len(packets))
	}
	if len(packets[0]) != 8191 || packets[0][2] != 1 || packets[0][5] != 5 {
		t.Fatalf("unexpected first header: %x", packets[0][:8])
	}
	if packets[1][2] != 2 || packets[1][4] != 1 {
		t.Fatalf("unexpected final header: %x", packets[1][:8])
	}
}

func TestPlusControlParsing(t *testing.T) {
	push := make([]byte, 10)
	push[1], push[4], push[6] = 0x03, 0x00, 1
	got := dialEvents(StreamDeckPlus, make([]bool, 4), push)
	if len(got) != 1 || got[0] != (DialEvent{Dial: 1, Pressed: true}) {
		t.Fatalf("push: %#v", got)
	}
	turn := make([]byte, 10)
	turn[1], turn[4], turn[5] = 0x03, 0x01, 0xff
	got = dialEvents(StreamDeckPlus, make([]bool, 4), turn)
	if len(got) != 1 || got[0].Dial != 0 || got[0].Ticks != -1 || !got[0].Turn {
		t.Fatalf("turn: %#v", got)
	}
	touch := make([]byte, 14)
	touch[1], touch[4] = 0x02, byte(TouchDrag)
	binary.LittleEndian.PutUint16(touch[6:8], 123)
	binary.LittleEndian.PutUint16(touch[8:10], 45)
	binary.LittleEndian.PutUint16(touch[10:12], 400)
	binary.LittleEndian.PutUint16(touch[12:14], 90)
	event, ok := touchEvent(StreamDeckPlus, touch)
	if !ok || event != (TouchEvent{Type: TouchDrag, X: 123, Y: 45, EndX: 400, EndY: 90}) {
		t.Fatalf("touch: %#v %t", event, ok)
	}
}

func TestNeoScreenAndColorPackets(t *testing.T) {
	device := newFakeDevice()
	deck := newDeck(DeviceInfo{Model: StreamDeckNeo}, device)
	defer deck.Close()
	if err := deck.SetKeyColor(9, color.RGBA{R: 1, G: 2, B: 3, A: 255}); err != nil {
		t.Fatal(err)
	}
	if got := device.features[0]; got.id != 0x03 || !bytes.Equal(got.data[:5], []byte{0x06, 9, 1, 2, 3}) {
		t.Fatalf("color packet: id=%x data=%x", got.id, got.data[:5])
	}
	if err := deck.SetScreenImage(image.NewRGBA(image.Rect(0, 0, 248, 58))); err != nil {
		t.Fatal(err)
	}
	if len(device.outputs) == 0 || device.outputs[0].id != 0x02 || device.outputs[0].data[0] != 0x0b {
		t.Fatalf("missing screen command: %#v", device.outputs)
	}
}

func TestPrepareGIFProducesNativeFrames(t *testing.T) {
	device := newFakeDevice()
	deck := newDeck(DeviceInfo{Model: StreamDeckMK2}, device)
	defer deck.Close()
	value := &gif.GIF{Image: []*image.Paletted{image.NewPaletted(image.Rect(0, 0, 2, 2), color.Palette{color.Black, color.White})}, Delay: []int{1}, LoopCount: -1, Config: image.Config{Width: 2, Height: 2}}
	animation, err := deck.PrepareGIF(value)
	if err != nil {
		t.Fatal(err)
	}
	if animation.Loops != 1 || len(animation.Frames) != 1 || animation.Delays[0] != 10_000_000 {
		t.Fatalf("animation: %#v", animation)
	}
	if err := deck.PlayAnimation(context.Background(), 0, animation); err != nil {
		t.Fatal(err)
	}
}

func TestExpandedDisplayCommands(t *testing.T) {
	device := newFakeDevice()
	deck := newDeck(DeviceInfo{Model: StreamDeckMK2}, device)
	defer deck.Close()
	if err := deck.SetLCDColor(color.RGBA{R: 4, G: 5, B: 6, A: 255}); err != nil {
		t.Fatal(err)
	}
	if got := device.features[0]; got.id != 0x03 || !bytes.Equal(got.data[:4], []byte{0x05, 4, 5, 6}) {
		t.Fatalf("LCD color: id=%x data=%x", got.id, got.data[:4])
	}
	if err := deck.SetSleepTimeout(90 * time.Second); err != nil {
		t.Fatal(err)
	}
	if got := device.features[1]; got.id != 0x03 || got.data[0] != 0x0d || binary.LittleEndian.Uint32(got.data[1:5]) != 90 {
		t.Fatalf("sleep: id=%x data=%x", got.id, got.data[:5])
	}
	if err := deck.SetLCDImage(image.NewRGBA(image.Rect(0, 0, 10, 10))); err != nil {
		t.Fatal(err)
	}
	if len(device.outputs) == 0 || device.outputs[0].id != 0x02 || device.outputs[0].data[0] != 0x08 {
		t.Fatalf("LCD output: %#v", device.outputs)
	}
	device.outputs = nil
	if err := deck.StoreBackground(3, image.NewRGBA(image.Rect(0, 0, 10, 10))); err != nil {
		t.Fatal(err)
	}
	if len(device.outputs) == 0 || device.outputs[0].data[0] != 0x0d || device.outputs[0].data[1] != 3 {
		t.Fatalf("background output: %#v", device.outputs)
	}
	if err := deck.ShowBackground(3); err != nil {
		t.Fatal(err)
	}
	if got := device.features[len(device.features)-1]; got.data[0] != 0x13 || got.data[1] != 3 {
		t.Fatalf("show background: %x", got.data[:2])
	}
}

func TestCloseSignalsDone(t *testing.T) {
	device := newFakeDevice()
	deck := newDeck(DeviceInfo{Model: StreamDeckMK2}, device)
	if err := deck.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-deck.Done():
	case <-time.After(time.Second):
		t.Fatal("Done did not close")
	}
}

func equalBytes(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
