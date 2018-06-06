// +build ignore
// go run dump-rgb.go stones.go flower.go soil.go font.go header.go

package main

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
)

func main() {
	dump("flower.png", FLOWER_WIDTH, FLOWER_HEIGHT, FLOWER_DATA)
	dump("stones.png", STONES_WIDTH, STONES_HEIGHT, STONES_DATA)
	dump("font.png", FONT_WIDTH, FONT_HEIGHT, FONT_DATA)
	dump("header.png", HEADER_WIDTH, HEADER_HEIGHT, HEADER_DATA)
	dump("soil.png", SOIL_WIDTH, SOIL_HEIGHT, SOIL_DATA)
}

func dump(name string, width, height int, col []byte) (err error) {
	defer func() {
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
	}()

	img := image.NewRGBA(image.Rect(0, 0, width, height))
	j := 0
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			c := color.RGBA{
				col[j],
				col[j+1],
				col[j+2],
				255,
			}
			img.Set(x, y, c)
			j += 3
		}
	}

	f, err := os.Create(name)
	if err != nil {
		return err
	}

	err = png.Encode(f, img)
	xerr := f.Close()
	if err == nil {
		err = xerr
	}

	return err
}
