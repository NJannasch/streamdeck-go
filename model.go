package streamdeck

import "fmt"

// VendorID is Elgato's USB vendor ID.
const VendorID uint16 = 0x0fd9

// ImageFormat is the image encoding accepted by a device display.
type ImageFormat string

const (
	ImageFormatNone ImageFormat = ""
	ImageFormatJPEG ImageFormat = "JPEG"
	ImageFormatBMP  ImageFormat = "BMP"
)

type protocolKind uint8

const (
	protocolGen2 protocolKind = iota
	protocolGen1
	protocolMini
	protocolPedal
)

// Model describes the physical layout and capabilities of a Stream Deck.
type Model struct {
	Name      string
	ProductID uint16
	Rows      int
	Columns   int
	KeyWidth  int
	KeyHeight int
	KeyFormat ImageFormat

	DialCount         int
	TouchKeyCount     int
	TouchscreenWidth  int
	TouchscreenHeight int
	ScreenWidth       int
	ScreenHeight      int
	LCDWidth          int
	LCDHeight         int

	protocol       protocolKind
	keyOffset      int
	flipX          bool
	flipY          bool
	rotation       int
	reverseColumns bool
	serialReport   byte
	serialOffset   int
	firmwareReport byte
	firmwareOffset int
	featureSize    int
}

// KeyCount returns the number of physical keys. A Pedal therefore returns 3.
func (m Model) KeyCount() int { return m.Rows * m.Columns }

// InputCount includes Neo touch buttons in addition to its LCD keys.
func (m Model) InputCount() int { return m.KeyCount() + m.TouchKeyCount }

// HasDisplay reports whether the model has images behind its keys.
func (m Model) HasDisplay() bool { return m.KeyFormat != ImageFormatNone }

var (
	StreamDeckOriginal   = gen1Model("Stream Deck Original", 0x0060, 3, 5, 72, 72)
	StreamDeckOriginalV2 = gen2Model("Stream Deck Original (2019)", 0x006d, 3, 5, 72, 72)
	StreamDeckMK2        = gen2Model("Stream Deck MK.2", 0x0080, 3, 5, 72, 72)
	StreamDeckMK2Module  = gen2Model("Stream Deck MK.2 Module", 0x00b9, 3, 5, 72, 72)
	StreamDeckScissor    = gen2Model("Stream Deck Scissor Keys", 0x00a5, 3, 5, 72, 72)

	StreamDeckMini        = miniModel("Stream Deck Mini", 0x0063)
	StreamDeckMiniMK2     = miniModel("Stream Deck Mini MK.2", 0x0090)
	StreamDeckMiniModule  = miniModel("Stream Deck Mini Module", 0x00b8)
	StreamDeckMiniDiscord = miniModel("Stream Deck Mini Discord", 0x00b3)

	StreamDeckXL       = xlModel("Stream Deck XL", 0x006c)
	StreamDeckXLMK2    = xlModel("Stream Deck XL (2022)", 0x008f)
	StreamDeckXLModule = xlModel("Stream Deck XL Module", 0x00ba)

	StreamDeckNeo    = Model{Name: "Stream Deck Neo", ProductID: 0x009a, Rows: 2, Columns: 4, KeyWidth: 96, KeyHeight: 96, KeyFormat: ImageFormatJPEG, TouchKeyCount: 2, ScreenWidth: 248, ScreenHeight: 58, LCDWidth: 480, LCDHeight: 320, protocol: protocolGen2, keyOffset: 4, flipX: true, flipY: true, serialReport: 0x06, serialOffset: 1, firmwareReport: 0x05, firmwareOffset: 5, featureSize: 32}
	StreamDeckPedal  = Model{Name: "Stream Deck Pedal", ProductID: 0x0086, Rows: 1, Columns: 3, protocol: protocolPedal, keyOffset: 4, serialReport: 0x06, serialOffset: 1, firmwareReport: 0x05, firmwareOffset: 5, featureSize: 32}
	StreamDeckPlus   = plusModel("Stream Deck +", 0x0084, 2, 4, 120, 120, 4, 800, 100, 0)
	StreamDeckPlusXL = plusModel("Stream Deck + XL", 0x00c6, 4, 9, 112, 112, 6, 1200, 100, 90)
	StreamDeckStudio = Model{Name: "Stream Deck Studio", ProductID: 0x00aa, Rows: 2, Columns: 16, KeyWidth: 80, KeyHeight: 120, KeyFormat: ImageFormatJPEG, DialCount: 2, protocol: protocolGen2, keyOffset: 4, serialReport: 0x06, serialOffset: 4, firmwareReport: 0x05, firmwareOffset: 4, featureSize: 32}
)

func gen2Model(name string, pid uint16, rows, columns, width, height int) Model {
	return Model{Name: name, ProductID: pid, Rows: rows, Columns: columns, KeyWidth: width, KeyHeight: height, KeyFormat: ImageFormatJPEG, LCDWidth: 480, LCDHeight: 272, protocol: protocolGen2, keyOffset: 4, flipX: true, flipY: true, serialReport: 0x06, serialOffset: 1, firmwareReport: 0x05, firmwareOffset: 5, featureSize: 32}
}

func gen1Model(name string, pid uint16, rows, columns, width, height int) Model {
	return Model{Name: name, ProductID: pid, Rows: rows, Columns: columns, KeyWidth: width, KeyHeight: height, KeyFormat: ImageFormatBMP, protocol: protocolGen1, keyOffset: 1, flipX: true, flipY: true, reverseColumns: true, serialReport: 0x03, serialOffset: 4, firmwareReport: 0x04, firmwareOffset: 4, featureSize: 17}
}

func miniModel(name string, pid uint16) Model {
	return Model{Name: name, ProductID: pid, Rows: 2, Columns: 3, KeyWidth: 80, KeyHeight: 80, KeyFormat: ImageFormatBMP, protocol: protocolMini, keyOffset: 1, flipY: true, rotation: 90, serialReport: 0x03, serialOffset: 4, firmwareReport: 0x04, firmwareOffset: 4, featureSize: 17}
}

func xlModel(name string, pid uint16) Model {
	m := gen2Model(name, pid, 4, 8, 96, 96)
	m.LCDWidth, m.LCDHeight = 1024, 600
	return m
}

func plusModel(name string, pid uint16, rows, columns, width, height, dials, touchWidth, touchHeight, rotation int) Model {
	m := gen2Model(name, pid, rows, columns, width, height)
	m.DialCount, m.TouchscreenWidth, m.TouchscreenHeight = dials, touchWidth, touchHeight
	m.flipX, m.flipY, m.rotation = false, false, rotation
	if pid == 0x00c6 {
		m.LCDWidth, m.LCDHeight = 1280, 800
		m.serialOffset, m.firmwareOffset = 1, 5
	} else {
		m.LCDWidth, m.LCDHeight = 800, 480
		m.serialOffset, m.firmwareOffset = 4, 4
	}
	return m
}

var models = func() map[uint16]Model {
	all := []Model{StreamDeckOriginal, StreamDeckOriginalV2, StreamDeckMK2, StreamDeckMK2Module, StreamDeckScissor, StreamDeckMini, StreamDeckMiniMK2, StreamDeckMiniModule, StreamDeckMiniDiscord, StreamDeckXL, StreamDeckXLMK2, StreamDeckXLModule, StreamDeckNeo, StreamDeckPedal, StreamDeckPlus, StreamDeckPlusXL, StreamDeckStudio}
	result := make(map[uint16]Model, len(all))
	for _, model := range all {
		result[model.ProductID] = model
	}
	return result
}()

func modelForProduct(productID uint16) (Model, error) {
	model, ok := models[productID]
	if !ok {
		return Model{}, fmt.Errorf("streamdeck: unsupported product ID 0x%04x", productID)
	}
	return model, nil
}
