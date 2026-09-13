package streamdeck

import (
	"context"
	"errors"
	"image"
	"image/draw"
	"image/gif"
	"time"
)

// Animation contains device-native encoded frames that can be replayed
// without repeatedly resizing and encoding them.
type Animation struct {
	Frames [][]byte
	Delays []time.Duration
	Loops  int // zero loops forever; positive values are total play counts
}

// PrepareGIF composites GIF disposal frames and encodes them for this deck.
func (d *Deck) PrepareGIF(value *gif.GIF) (*Animation, error) {
	if value == nil || len(value.Image) == 0 {
		return nil, errors.New("streamdeck: GIF has no frames")
	}
	width, height := value.Config.Width, value.Config.Height
	if width <= 0 || height <= 0 {
		width, height = value.Image[0].Bounds().Max.X, value.Image[0].Bounds().Max.Y
	}
	canvas := image.NewRGBA(image.Rect(0, 0, width, height))
	frames := make([][]byte, 0, len(value.Image))
	delays := make([]time.Duration, 0, len(value.Image))
	for index, frame := range value.Image {
		before := cloneRGBA(canvas)
		draw.Draw(canvas, frame.Bounds(), frame, frame.Bounds().Min, draw.Over)
		encoded, err := encodeKeyImage(d.info.Model, canvas)
		if err != nil {
			return nil, err
		}
		frames = append(frames, encoded)
		delay := 100 * time.Millisecond
		if index < len(value.Delay) && value.Delay[index] > 0 {
			delay = time.Duration(value.Delay[index]) * 10 * time.Millisecond
		}
		delays = append(delays, delay)
		disposal := byte(gif.DisposalNone)
		if index < len(value.Disposal) {
			disposal = value.Disposal[index]
		}
		switch disposal {
		case gif.DisposalBackground:
			draw.Draw(canvas, frame.Bounds(), image.Transparent, image.Point{}, draw.Src)
		case gif.DisposalPrevious:
			canvas = before
		}
	}
	loops := value.LoopCount + 1
	if value.LoopCount == 0 {
		loops = 0
	}
	if value.LoopCount < 0 {
		loops = 1
	}
	return &Animation{Frames: frames, Delays: delays, Loops: loops}, nil
}

// PlayGIF prepares and plays a GIF until it finishes or ctx is canceled.
func (d *Deck) PlayGIF(ctx context.Context, key int, value *gif.GIF) error {
	animation, err := d.PrepareGIF(value)
	if err != nil {
		return err
	}
	return d.PlayAnimation(ctx, key, animation)
}

// PlayAnimation displays pre-encoded animation frames.
func (d *Deck) PlayAnimation(ctx context.Context, key int, animation *Animation) error {
	if animation == nil || len(animation.Frames) == 0 {
		return errors.New("streamdeck: animation has no frames")
	}
	deadline := time.Now()
	for loop := 0; animation.Loops == 0 || loop < animation.Loops; loop++ {
		for index, frame := range animation.Frames {
			if err := d.SetKeyImageData(key, frame); err != nil {
				return err
			}
			delay := 100 * time.Millisecond
			if index < len(animation.Delays) && animation.Delays[index] > 0 {
				delay = animation.Delays[index]
			}
			deadline = deadline.Add(delay)
			wait := time.Until(deadline)
			if wait <= 0 {
				select {
				case <-ctx.Done():
					return ctx.Err()
				default:
					continue
				}
			}
			timer := time.NewTimer(wait)
			select {
			case <-ctx.Done():
				if !timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}
				return ctx.Err()
			case <-timer.C:
			}
		}
	}
	return nil
}

func cloneRGBA(src *image.RGBA) *image.RGBA {
	dst := image.NewRGBA(src.Bounds())
	draw.Draw(dst, dst.Bounds(), src, src.Bounds().Min, draw.Src)
	return dst
}
