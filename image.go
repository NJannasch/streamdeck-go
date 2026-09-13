package streamdeck

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/jpeg"

	"golang.org/x/image/bmp"
	xdraw "golang.org/x/image/draw"
)

const imageHeaderSize = 8

func encodeKeyImage(model Model, src image.Image) ([]byte, error) {
	if !model.HasDisplay() {
		return nil, fmt.Errorf("streamdeck: %s has no key display", model.Name)
	}
	transformed := transformImage(resizeImage(src, model.KeyWidth, model.KeyHeight), model.flipX, model.flipY, model.rotation)

	var output bytes.Buffer
	var err error
	switch model.KeyFormat {
	case ImageFormatJPEG:
		err = jpeg.Encode(&output, transformed, &jpeg.Options{Quality: 95})
	case ImageFormatBMP:
		err = bmp.Encode(&output, transformed)
	default:
		err = fmt.Errorf("unsupported image format %q", model.KeyFormat)
	}
	if err != nil {
		return nil, fmt.Errorf("streamdeck: encode key image: %w", err)
	}
	return output.Bytes(), nil
}

func encodeDisplayImage(src image.Image, width, height int, flipX, flipY bool, rotation int) ([]byte, error) {
	transformed := transformImage(resizeImage(src, width, height), flipX, flipY, rotation)
	var output bytes.Buffer
	if err := jpeg.Encode(&output, transformed, &jpeg.Options{Quality: 95}); err != nil {
		return nil, fmt.Errorf("streamdeck: encode display image: %w", err)
	}
	return output.Bytes(), nil
}

func resizeImage(src image.Image, width, height int) *image.RGBA {
	resized := image.NewRGBA(image.Rect(0, 0, width, height))
	xdraw.CatmullRom.Scale(resized, resized.Bounds(), src, src.Bounds(), xdraw.Over, nil)
	return resized
}

func transformImage(src image.Image, flipX, flipY bool, rotation int) *image.RGBA {
	w, h := src.Bounds().Dx(), src.Bounds().Dy()
	dw, dh := w, h
	if rotation == 90 || rotation == 270 {
		dw, dh = h, w
	}
	dst := image.NewRGBA(image.Rect(0, 0, dw, dh))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			sx, sy := x, y
			if flipX {
				sx = w - 1 - sx
			}
			if flipY {
				sy = h - 1 - sy
			}
			var dx, dy int
			switch rotation {
			case 90:
				dx, dy = sy, w-1-sx
			case 180:
				dx, dy = w-1-sx, h-1-sy
			case 270:
				dx, dy = h-1-sy, sx
			default:
				dx, dy = sx, sy
			}
			dst.Set(dx, dy, src.At(x+src.Bounds().Min.X, y+src.Bounds().Min.Y))
		}
	}
	return dst
}

func imagePackets(key byte, encoded []byte) [][]byte {
	return commandImagePackets(0x07, key, encoded)
}

func commandImagePackets(command, index byte, encoded []byte) [][]byte {
	const payloadSize = outputReportSize - imageHeaderSize
	packets := make([][]byte, 0, (len(encoded)+payloadSize-1)/payloadSize)

	for page, offset := 0, 0; offset < len(encoded); page++ {
		length := min(payloadSize, len(encoded)-offset)
		packet := make([]byte, outputReportSize)
		packet[0] = 0x02
		packet[1] = command
		packet[2] = index
		if offset+length == len(encoded) {
			packet[3] = 1
		}
		binary.LittleEndian.PutUint16(packet[4:6], uint16(length))
		binary.LittleEndian.PutUint16(packet[6:8], uint16(page))
		copy(packet[imageHeaderSize:], encoded[offset:offset+length])
		packets = append(packets, packet)
		offset += length
	}
	return packets
}

func keyImagePackets(model Model, key int, encoded []byte) [][]byte {
	if model.protocol == protocolGen2 {
		return imagePackets(byte(key), encoded)
	}
	reportSize, headerSize, payloadSize := 1024, 16, 1008
	physicalKey, firstPage := key, 0
	if model.protocol == protocolGen1 {
		reportSize, payloadSize, firstPage = 8191, (len(encoded)+1)/2, 1
		physicalKey = reverseKey(model, key)
	}
	packets := make([][]byte, 0, (len(encoded)+payloadSize-1)/payloadSize)
	for page, offset := 0, 0; offset < len(encoded); page++ {
		length := min(payloadSize, len(encoded)-offset)
		packet := make([]byte, reportSize)
		packet[0], packet[1], packet[2], packet[5] = 0x02, 0x01, byte(page+firstPage), byte(physicalKey+1)
		if offset+length == len(encoded) {
			packet[4] = 1
		}
		copy(packet[headerSize:], encoded[offset:offset+length])
		packets = append(packets, packet)
		offset += length
	}
	return packets
}

func reverseKey(model Model, key int) int {
	if !model.reverseColumns {
		return key
	}
	row, column := key/model.Columns, key%model.Columns
	return row*model.Columns + model.Columns - 1 - column
}

func lcdPackets(command byte, encoded []byte, x, y, width, height int) [][]byte {
	const headerSize = 16
	const payloadSize = outputReportSize - headerSize
	packets := make([][]byte, 0, (len(encoded)+payloadSize-1)/payloadSize)
	for page, offset := 0, 0; offset < len(encoded); page++ {
		length := min(payloadSize, len(encoded)-offset)
		packet := make([]byte, outputReportSize)
		packet[0], packet[1] = 0x02, command
		binary.LittleEndian.PutUint16(packet[2:4], uint16(x))
		binary.LittleEndian.PutUint16(packet[4:6], uint16(y))
		binary.LittleEndian.PutUint16(packet[6:8], uint16(width))
		binary.LittleEndian.PutUint16(packet[8:10], uint16(height))
		if offset+length == len(encoded) {
			packet[10] = 1
		}
		binary.LittleEndian.PutUint16(packet[11:13], uint16(page))
		binary.LittleEndian.PutUint16(packet[13:15], uint16(length))
		copy(packet[headerSize:], encoded[offset:offset+length])
		packets = append(packets, packet)
		offset += length
	}
	return packets
}

func screenPackets(encoded []byte) [][]byte {
	const payloadSize = outputReportSize - imageHeaderSize
	packets := make([][]byte, 0, (len(encoded)+payloadSize-1)/payloadSize)
	for page, offset := 0, 0; offset < len(encoded); page++ {
		length := min(payloadSize, len(encoded)-offset)
		packet := make([]byte, outputReportSize)
		packet[0], packet[1] = 0x02, 0x0b
		if offset+length == len(encoded) {
			packet[3] = 1
		}
		binary.LittleEndian.PutUint16(packet[4:6], uint16(length))
		binary.LittleEndian.PutUint16(packet[6:8], uint16(page))
		copy(packet[imageHeaderSize:], encoded[offset:offset+length])
		packets = append(packets, packet)
		offset += length
	}
	return packets
}

func backgroundPackets(index byte, encoded []byte) [][]byte {
	const payloadSize = outputReportSize - imageHeaderSize
	packets := make([][]byte, 0, (len(encoded)+payloadSize-1)/payloadSize)
	for page, offset := 0, 0; offset < len(encoded); page++ {
		length := min(payloadSize, len(encoded)-offset)
		packet := make([]byte, outputReportSize)
		packet[0], packet[1], packet[2] = 0x02, 0x0d, index
		if offset+length == len(encoded) {
			packet[3] = 1
		}
		binary.LittleEndian.PutUint16(packet[4:6], uint16(page))
		binary.LittleEndian.PutUint16(packet[6:8], uint16(length))
		copy(packet[imageHeaderSize:], encoded[offset:offset+length])
		packets = append(packets, packet)
		offset += length
	}
	return packets
}
