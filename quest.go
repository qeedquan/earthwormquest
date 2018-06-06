package main

import (
	"flag"
	"fmt"
	"math"
	"math/rand"
	"os"
	"runtime"
	"time"

	"github.com/go-gl/gl/v2.1/gl"
	"github.com/qeedquan/go-media/sdl"
)

var (
	game     *Game
	window   *sdl.Window
	renderer *sdl.Renderer
)

func main() {
	runtime.LockOSThread()
	rand.Seed(time.Now().UnixNano())
	game = NewGame()
	parseFlags()
	initSDL()
	game.Run()
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: earthwormquest [options]")
	flag.PrintDefaults()
	os.Exit(2)
}

func parseFlags() {
	flag.BoolVar(&game.Fullscreen, "f", false, "fullscreen")
	flag.BoolVar(&game.InfLives, "i", false, "infinite lives")
	flag.Usage = usage
	flag.Parse()
}

func initSDL() {
	err := sdl.Init(sdl.INIT_VIDEO)
	ck(err)

	wflag := sdl.WINDOW_RESIZABLE | sdl.WINDOW_OPENGL
	if game.Fullscreen {
		wflag |= sdl.WINDOW_FULLSCREEN_DESKTOP
	}
	window, renderer, err = sdl.CreateWindowAndRenderer(game.Width, game.Height, wflag)
	ck(err)

	err = gl.Init()
	ck(err)

	sdl.ShowCursor(0)
	window.SetTitle("Earthworm Quest")

	gluQuadricDrawStyle(game.Quad, GLU_FILL)
	gl.BindTexture(gl.TEXTURE_2D, T_GROUND)
	gluBuild2DMipmaps(gl.TEXTURE_2D, gl.RGB, SOIL_WIDTH, SOIL_HEIGHT, gl.RGB, gl.UNSIGNED_BYTE, SOIL_DATA)
	gl.BindTexture(gl.TEXTURE_2D, T_STONES)
	gluBuild2DMipmaps(gl.TEXTURE_2D, gl.RGB, STONES_WIDTH, STONES_HEIGHT, gl.RGB, gl.UNSIGNED_BYTE, STONES_DATA)
	gl.BindTexture(gl.TEXTURE_2D, T_FONT)
	gluBuild2DMipmaps(gl.TEXTURE_2D, gl.RGB, FONT_WIDTH, FONT_HEIGHT, gl.RGB, gl.UNSIGNED_BYTE, FONT_DATA)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP)
	gl.BindTexture(gl.TEXTURE_2D, T_HEADER)
	gluBuild2DMipmaps(gl.TEXTURE_2D, gl.RGB, HEADER_WIDTH, HEADER_HEIGHT, gl.RGB, gl.UNSIGNED_BYTE, HEADER_DATA)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP)

	// The flower texture needs an alpha component, but the pixel data
	// doesn't contain it. This code creates a local copy of the graphics
	// data, in which every black pixel is transparent.
	buf := make([]byte, 64*64*4)
	for y := 0; y < 64; y++ {
		for x := 0; x < 64; x++ {
			r := FLOWER_DATA[y*64*3+x*3+0]
			g := FLOWER_DATA[y*64*3+x*3+1]
			b := FLOWER_DATA[y*64*3+x*3+2]
			a := uint8(255)
			if r == 0 && g == 0 && b == 0 {
				a = 0
			}
			buf[y*64*4+x*4+0] = r
			buf[y*64*4+x*4+1] = g
			buf[y*64*4+x*4+2] = b
			buf[y*64*4+x*4+3] = a
		}
	}
	gl.BindTexture(gl.TEXTURE_2D, T_FLOWER)
	gluBuild2DMipmaps(gl.TEXTURE_2D, gl.RGBA, 64, 64, gl.RGBA, gl.UNSIGNED_BYTE, buf)
}

func ck(err error) {
	if err != nil {
		sdl.LogCritical(sdl.LOG_CATEGORY_APPLICATION, "%v", err)
		sdl.ShowSimpleMessageBox(sdl.MESSAGEBOX_ERROR, "Error", err.Error(), nil)
		os.Exit(1)
	}
}

type Segment struct {
	X, Z float64
}

type Worm struct {
	Segs        []Segment
	Length      int
	Growto      int
	Angle       int
	Speed       float64
	WobbleCount int
}

type BlamParticle struct {
	X, Y, Z    float64
	Dx, Dy, Dz float64
}

type Game struct {
	Quad         *Quadric
	Width        int
	Height       int
	Fullscreen   bool
	Board        []byte
	FoodPos      [2]float64
	GrowTimer    int
	Score        int
	GameOver     int
	Worm         Worm
	BlamParticle []BlamParticle
	Ticker       *time.Ticker
	Rotation     int
	InfLives     bool
	Quit         bool
	Pause        bool
	LeftPressed  bool
	RightPressed bool
	UpPressed    bool
}

const (
	T_GROUND = iota
	T_STONES
	T_FONT
	T_HEADER
	T_FLOWER
)

func NewGame() *Game {
	const (
		WIDTH     = 800
		HEIGHT    = 600
		MAXSEG    = 300
		NPARTICLE = 80
	)
	return &Game{
		Width:  WIDTH,
		Height: HEIGHT,
		Quad:   NewQuadric(),
		Worm: Worm{
			Segs: make([]Segment, MAXSEG),
		},
		Ticker:       time.NewTicker(20 * time.Millisecond),
		BlamParticle: make([]BlamParticle, NPARTICLE),
		Board: []byte("################" +
			"#..............#" +
			"#..............#" +
			"#..............#" +
			"#..............#" +
			"#.....#........#" +
			"#..............#" +
			"#..............#" +
			"#..............#" +
			"#..............#" +
			"#.......##.....#" +
			"#..............#" +
			"#..............#" +
			"#..............#" +
			"#..............#" +
			"################"),
	}
}

func (c *Game) Event() {
	for {
		ev := sdl.PollEvent()
		if ev == nil {
			break
		}
		switch ev := ev.(type) {
		case sdl.QuitEvent:
			c.Quit = true
		case sdl.KeyUpEvent:
			switch ev.Sym {
			case sdl.K_LEFT:
				c.LeftPressed = false
				if c.RightPressed {
					c.Rotation = 1
				} else {
					c.Rotation = 0
				}
			case sdl.K_RIGHT:
				c.RightPressed = false
				if c.LeftPressed {
					c.Rotation = -1
				} else {
					c.Rotation = 0
				}
			case sdl.K_UP:
				c.UpPressed = false
			}
		case sdl.KeyDownEvent:
			switch ev.Sym {
			case sdl.K_ESCAPE:
				c.Quit = true
			case sdl.K_LEFT:
				c.LeftPressed = true
				c.Rotation = -1
			case sdl.K_RIGHT:
				c.RightPressed = true
				c.Rotation = 1
			case sdl.K_UP:
				c.UpPressed = true
			case sdl.K_SPACE, sdl.K_RETURN:
				c.Pause = !c.Pause
			}
		case sdl.WindowEvent:
			switch ev.Event {
			case sdl.WINDOWEVENT_RESIZED:
				c.Width = int(ev.Data[0])
				c.Height = int(ev.Data[1])
				gl.Viewport(0, 0, int32(c.Width), int32(c.Height))
			}
		}
	}
}

func (c *Game) prepareDraw() {
	// Prepare to draw the ground.

	gl.MatrixMode(gl.PROJECTION)
	gl.LoadIdentity()
	gluPerspective(30, float64(c.Width)/float64(c.Height), 1, 200)
	gl.MatrixMode(gl.MODELVIEW)
	gl.LoadIdentity()

	gl.Enable(gl.DEPTH_TEST)
	gl.Enable(gl.LIGHTING)
	gl.Enable(gl.LIGHT0)
	gl.Enable(gl.NORMALIZE)
	gl.Disable(gl.CULL_FACE)
	gl.Disable(gl.BLEND)
	fv := [4]float32{0, 1, 1, 0}
	gl.Lightfv(gl.LIGHT0, gl.POSITION, &fv[0])
	fv = [4]float32{.5, .4, .3, 0}
	gl.Lightfv(gl.LIGHT0, gl.DIFFUSE, &fv[0])
	fv = [4]float32{.5, .5, .5, 0}
	gl.Lightfv(gl.LIGHT0, gl.SPECULAR, &fv[0])
	segs := c.Worm.Segs[:]
	dir := [2]float64{
		segs[4].X - segs[5].X,
		segs[4].Z - segs[5].Z,
	}
	normalize2(dir[:])

	eye := [2]float64{
		segs[4].X - dir[0]*3,
		segs[4].Z - dir[1]*3,
	}

	lookAt := [2]float64{
		segs[4].X + dir[0]*3,
		segs[4].Z + dir[1]*3,
	}
	gluLookAt(eye[0], .8, eye[1], lookAt[0], .1, lookAt[1], 0, 1, 0)
}

func (c *Game) drawGround() {
	// Draw the ground (brown circle and grass above it).
	fv := [4]float32{.2, 0, 0, 0}
	gl.Materialfv(gl.FRONT_AND_BACK, gl.AMBIENT_AND_DIFFUSE, &fv[0])
	gl.Disable(gl.TEXTURE_2D)
	gl.Normal3d(0, 1, 0)
	gl.Begin(gl.TRIANGLE_FAN)
	gl.Vertex3d(0, 0, 0)
	for i := 0; i <= 128; i++ {
		gl.Vertex3d(5*math.Cos(float64(i)*math.Pi*2/128), -.1, 5*math.Sin(float64(i)*math.Pi*2/128))
	}
	gl.End()

	fv = [4]float32{1, 1, 1, 0}
	gl.Materialfv(gl.FRONT_AND_BACK, gl.AMBIENT_AND_DIFFUSE, &fv[0])
	fv = [4]float32{0, 0, 0, 0}
	gl.Materialfv(gl.FRONT_AND_BACK, gl.SPECULAR, &fv[0])
	gl.Enable(gl.TEXTURE_2D)
	gl.BindTexture(gl.TEXTURE_2D, T_GROUND)

	gl.Normal3d(0, 1, 0)
	gl.Begin(gl.QUADS)
	gl.TexCoord2d(-2, -2)
	gl.Vertex3d(-2, 0, -2)
	gl.TexCoord2d(+2, -2)
	gl.Vertex3d(+2, 0, -2)
	gl.TexCoord2d(+2, +2)
	gl.Vertex3d(+2, 0, +2)
	gl.TexCoord2d(-2, +2)
	gl.Vertex3d(-2, 0, +2)
	gl.End()
}

func (c *Game) drawStones() {
	h := .06
	gl.BindTexture(gl.TEXTURE_2D, T_STONES)
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			cx := (-7.5 + float64(x)) * 2.0 / 8
			cy := (-7.5 + float64(y)) * 2.0 / 8
			sz := 2.0 / 8 / 2

			if c.Board[y*16+x] == '#' {
				gl.Begin(gl.QUADS)
				fv := [4]float32{1, 1, 1, 0}
				gl.Materialfv(gl.FRONT_AND_BACK, gl.AMBIENT_AND_DIFFUSE, &fv[0])
				gl.Normal3d(0, 1, 0)
				gl.TexCoord2d(cx-sz, cy-sz)
				gl.Vertex3d(cx-sz, h, cy-sz)
				gl.TexCoord2d(cx+sz, cy-sz)
				gl.Vertex3d(cx+sz, h, cy-sz)
				gl.TexCoord2d(cx+sz, cy+sz)
				gl.Vertex3d(cx+sz, h, cy+sz)
				gl.TexCoord2d(cx-sz, cy+sz)
				gl.Vertex3d(cx-sz, h, cy+sz)

				fv = [4]float32{.5, .5, .5, 0}
				gl.Materialfv(gl.FRONT_AND_BACK, gl.AMBIENT_AND_DIFFUSE, &fv[0])
				gl.Normal3d(0, 0, -1)
				gl.TexCoord2d(cx-sz, cy-sz)
				gl.Vertex3d(cx-sz, h, cy-sz)
				gl.TexCoord2d(cx+sz, cy-sz)
				gl.Vertex3d(cx+sz, h, cy-sz)
				gl.TexCoord2d(cx+sz, cy-2*sz)
				gl.Vertex3d(cx+sz, 0, cy-sz)
				gl.TexCoord2d(cx-sz, cy-2*sz)
				gl.Vertex3d(cx-sz, 0, cy-sz)

				gl.Normal3d(0, 0, 1)
				gl.TexCoord2d(cx+sz, cy+sz)
				gl.Vertex3d(cx+sz, h, cy+sz)
				gl.TexCoord2d(cx-sz, cy+sz)
				gl.Vertex3d(cx-sz, h, cy+sz)
				gl.TexCoord2d(cx-sz, cy+2*sz)
				gl.Vertex3d(cx-sz, 0, cy+sz)
				gl.TexCoord2d(cx+sz, cy+2*sz)
				gl.Vertex3d(cx+sz, 0, cy+sz)

				gl.Normal3d(1, 0, 0)
				gl.TexCoord2d(cx+sz, cy-sz)
				gl.Vertex3d(cx+sz, h, cy-sz)
				gl.TexCoord2d(cx+sz, cy+sz)
				gl.Vertex3d(cx+sz, h, cy+sz)
				gl.TexCoord2d(cx+2*sz, cy+sz)
				gl.Vertex3d(cx+sz, 0, cy+sz)
				gl.TexCoord2d(cx+2*sz, cy-sz)
				gl.Vertex3d(cx+sz, 0, cy-sz)

				gl.Normal3d(-1, 0, 0)
				gl.TexCoord2d(cx-sz, cy+sz)
				gl.Vertex3d(cx-sz, h, cy+sz)
				gl.TexCoord2d(cx-sz, cy-sz)
				gl.Vertex3d(cx-sz, h, cy-sz)
				gl.TexCoord2d(cx-2*sz, cy-sz)
				gl.Vertex3d(cx-sz, 0, cy-sz)
				gl.TexCoord2d(cx-2*sz, cy+sz)
				gl.Vertex3d(cx-sz, 0, cy+sz)

				gl.End()
			}
		}
	}
}

func (c *Game) drawWorm() {
	gl.Disable(gl.TEXTURE_2D)
	gl.Enable(gl.CULL_FACE)

	fv := [4]float32{.4, .3, .1, 0}
	gl.Materialfv(gl.FRONT_AND_BACK, gl.AMBIENT_AND_DIFFUSE, &fv[0])
	fv = [4]float32{1, 1, 1, 0}
	gl.Materialfv(gl.FRONT_AND_BACK, gl.SPECULAR, &fv[0])
	fv[0] = 90
	gl.Materialfv(gl.FRONT_AND_BACK, gl.SHININESS, &fv[0])

	w := &c.Worm
	for i := 0; i < w.Length; i++ {
		bevel := 10 + w.Length/10
		bevelsize := w.Length / 30

		radius := .04
		if i <= 15 {
			radius = .02 + .02*math.Sin(float64(i)*math.Pi/2/15)
		} else if i >= w.Length-3 {
			radius = .02 + .02*math.Sin(float64(w.Length-i-1)*math.Pi/2/3)
		}

		if i >= bevel-bevelsize && i <= bevel+bevelsize {
			radius += 0.01
		}

		var wobblex, wobblez float64
		gl.PushMatrix()
		wobble := .02 * math.Sin(float64(i+w.WobbleCount)*math.Pi/8)
		if i != 0 {
			wobblex = w.Segs[i].X - w.Segs[i-1].X
			wobblez = w.Segs[i].Z - w.Segs[i-1].Z
			length := math.Hypot(wobblex, wobblez)
			wobblex /= length
			wobblez /= length
		} else {
			wobblex = w.Segs[1].X - w.Segs[0].X
			wobblez = w.Segs[1].Z - w.Segs[0].Z
			length := math.Hypot(wobblex, wobblez)
			wobblex /= length
			wobblez /= length
		}
		wobblex *= wobble
		wobblez *= wobble
		gl.Translated(w.Segs[i].X+wobblex, .01, w.Segs[i].Z+wobblez)
		gluSphere(c.Quad, radius, 8, 8)
		gl.PopMatrix()
	}
}

func (c *Game) drawFlower() {
	h := .08 * float64(c.GrowTimer) / 50
	size := .07 * float64(c.GrowTimer) / 50
	gl.Color3d(.0, .3, .0)
	gl.Disable(gl.LIGHTING)
	gl.LineWidth(5)
	gl.Begin(gl.LINES)
	gl.Vertex3d(c.FoodPos[0], 0, c.FoodPos[1])
	gl.Vertex3d(c.FoodPos[0], h, c.FoodPos[1])
	gl.End()
	gl.Enable(gl.TEXTURE_2D)
	gl.BindTexture(gl.TEXTURE_2D, T_FLOWER)
	gl.Disable(gl.CULL_FACE)
	gl.Enable(gl.BLEND)
	gl.BlendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA)
	gl.Color3d(1, 1, 1)
	gl.Begin(gl.TRIANGLE_FAN)
	gl.TexCoord2d(.5, .5)
	gl.Vertex3d(c.FoodPos[0], h, c.FoodPos[1])
	for i := 0; i <= 8; i++ {
		gl.TexCoord2d(.5+.5*math.Cos(float64(i)*math.Pi*2/8), .5+.5*math.Sin(float64(i)*math.Pi*2/8))
		gl.Vertex3d(c.FoodPos[0]+size*math.Cos(float64(i)*math.Pi*2/8), h*1.1, c.FoodPos[1]+size*math.Sin(float64(i)*math.Pi*2/8))
	}
	gl.End()

}

func (c *Game) drawExplosion() {
	if c.GameOver != 0 {
		gl.Disable(gl.DEPTH_TEST)
		gl.Enable(gl.BLEND)
		gl.BlendFunc(gl.ONE, gl.ONE)
		gl.Enable(gl.LIGHTING)
		gl.Disable(gl.TEXTURE_2D)
		fv := [4]float32{.8, .3, .1, 0}
		gl.Materialfv(gl.FRONT_AND_BACK, gl.AMBIENT_AND_DIFFUSE, &fv[0])
		fv = [4]float32{1, .4, 0, 0}
		gl.Materialfv(gl.FRONT_AND_BACK, gl.SHININESS, &fv[0])
		fv[0] = 90
		gl.Materialfv(gl.FRONT_AND_BACK, gl.SHININESS, &fv[0])
		for i := range c.BlamParticle {
			p := &c.BlamParticle[i]
			gl.PushMatrix()
			gl.Translated(p.X, p.Y, p.Z)
			gluSphere(c.Quad, .03+float64(50-c.GameOver)*.2/50, 8, 8)
			gl.PopMatrix()
		}
	}
}

func (c *Game) drawHeader() {
	gl.MatrixMode(gl.PROJECTION)
	gl.LoadIdentity()
	gl.Ortho(-4, +4, -3, +3, -1, 1)
	gl.MatrixMode(gl.MODELVIEW)
	gl.LoadIdentity()

	gl.Disable(gl.LIGHTING)
	gl.Disable(gl.DEPTH_TEST)
	gl.Enable(gl.TEXTURE_2D)
	gl.Enable(gl.BLEND)
	gl.BlendFunc(gl.ONE, gl.ONE)
	gl.Disable(gl.CULL_FACE)
	gl.Color3d(1, 1, 1)

	gl.BindTexture(gl.TEXTURE_2D, T_HEADER)
	gl.Begin(gl.QUADS)
	gl.TexCoord2d(0, 0)
	gl.Vertex3d(-3.7, 2.6, 0)
	gl.TexCoord2d(1, 0)
	gl.Vertex3d(1, 2.6, 0)
	gl.TexCoord2d(1, 1)
	gl.Vertex3d(1, 2.1, 0)
	gl.TexCoord2d(0, 1)
	gl.Vertex3d(-3.7, 2.1, 0)
	gl.End()
}

func (c *Game) drawScore() {
	gl.BindTexture(gl.TEXTURE_2D, T_FONT)
	gl.Begin(gl.QUADS)
	i := c.Score / 10
	gl.TexCoord2d(float64(i)/10, 0)
	gl.Vertex3d(2.6, 2.7, 0)
	gl.TexCoord2d((float64(i)+.93)/10, 0)
	gl.Vertex3d(3.2, 2.7, 0)
	gl.TexCoord2d((float64(i)+.93)/10, 1)
	gl.Vertex3d(3.2, 2.1, 0)
	gl.TexCoord2d(float64(i)/10, 1)
	gl.Vertex3d(2.6, 2.1, 0)
	i = c.Score % 10
	gl.TexCoord2d(float64(i)/10, 0)
	gl.Vertex3d(3.2, 2.7, 0)
	gl.TexCoord2d((float64(i)+.93)/10, 0)
	gl.Vertex3d(3.8, 2.7, 0)
	gl.TexCoord2d((float64(i)+.93)/10, 1)
	gl.Vertex3d(3.8, 2.1, 0)
	gl.TexCoord2d(float64(i)/10, 1)
	gl.Vertex3d(3.2, 2.1, 0)
	gl.End()
}

func (c *Game) drawPause() {
	gl.BindTexture(gl.TEXTURE_2D, T_FONT)
	gl.Begin(gl.QUADS)

	i := 5
	x := -3.5
	y := -2.0
	for n := 0; n < 3; n++ {
		gl.TexCoord2d(float64(i)/10, 0)
		gl.Vertex3d(2.6+x, 2.7+y, 0)
		gl.TexCoord2d((float64(i)+.93)/10, 0)
		gl.Vertex3d(3.2+x, 2.7+y, 0)
		gl.TexCoord2d((float64(i)+.93)/10, 1)
		gl.Vertex3d(3.2+x, 2.1+y, 0)
		gl.TexCoord2d(float64(i)/10, 1)
		gl.Vertex3d(2.6+x, 2.1+y, 0)
		x += .5
	}
	gl.End()
}

func (c *Game) Draw() {
	gl.ClearColor(.1, .1, .3, 0)
	gl.Clear(gl.COLOR_BUFFER_BIT | gl.DEPTH_BUFFER_BIT)

	c.prepareDraw()
	c.drawGround()
	c.drawStones()
	c.drawWorm()
	c.drawFlower()
	c.drawExplosion()
	c.drawHeader()
	c.drawScore()
	if c.Pause {
		c.drawPause()
	}
	window.SwapGL()
}

func (c *Game) Update(rotation int, upPressed bool) {
	if c.Pause {
		return
	}

	if c.GameOver != 0 {
		for i := range c.BlamParticle {
			p := &c.BlamParticle[i]
			p.X += p.Dx
			p.Y += p.Dy
			p.Z += p.Dz
			if p.Y < .01 {
				p.Y = .01
			}
			p.Dy -= .0005
		}
		if c.GameOver--; c.GameOver == 0 {
			c.Reset()
		}

		// Since we're in game-over state, there's nothing more for rungame() to do.
		return
	}

	// Animate the flower and do the worm wobbling.
	if c.GrowTimer < 50 {
		c.GrowTimer++
	}
	w := &c.Worm
	w.WobbleCount++

	// Change the current direction according to user input.
	w.Angle += rotation

	// Move the worm.
	speed := w.Speed
	if upPressed {
		speed = 1.9 * w.Speed
	}
	dx := speed * math.Cos(float64(w.Angle)*math.Pi/180)
	dz := speed * math.Sin(float64(w.Angle)*math.Pi/180)

	if w.Growto > w.Length {
		w.Length++
	}
	for i := w.Length - 1; i != 0; i-- {
		w.Segs[i] = w.Segs[i-1]
	}

	w.Segs[0].X += dx
	w.Segs[0].Z += dz

	// Check for collision.
	side := [2]float64{dz, -dx}
	normalize2(side[:])
	side[0] *= .04
	side[1] *= .04

	var i int
	if !c.isFree(w.Segs[0].X, w.Segs[0].Z, false, nil) {
		c.blam(w.Segs[0].X, w.Segs[0].Z)
	} else if !c.isFree(w.Segs[0].X+side[0], w.Segs[0].Z+side[1], true, &i) {
		c.blam(w.Segs[i].X, w.Segs[i].Z)
	} else if !c.isFree(w.Segs[0].X-side[0], w.Segs[0].Z-side[1], true, &i) {
		c.blam(w.Segs[i].X, w.Segs[i].Z)
	}

	// Did the worm hit the flower?
	dx = w.Segs[0].X - c.FoodPos[0]
	dz = w.Segs[0].Z - c.FoodPos[1]
	dist := dx*dx + dz*dz
	if dist < .004 {
		// Yes! Increase score and difficulty.
		if c.Score++; c.Score > 99 {
			c.Score = 99
		}

		w.Growto += 5
		if w.Growto > len(w.Segs) {
			w.Growto = len(w.Segs)
		}
		w.Speed += .0005

		c.placeFood()
	}
}

func (c *Game) blam(x, z float64) {
	c.GameOver = 50
	for i := range c.BlamParticle {
		p := &c.BlamParticle[i]
		p.X = x
		p.Y = .02
		p.Z = z
		p.Dx = float64(rand.Intn(128)-64) * .0002
		p.Dy = float64(rand.Intn(128)-64) * .0002
		p.Dz = float64(rand.Intn(128)-64) * .0002
	}
}

func (c *Game) Run() {
	c.Reset()
	for !c.Quit {
		c.Event()
		select {
		case <-c.Ticker.C:
			c.Update(c.Rotation*5, c.UpPressed)
		default:
		}
		c.Draw()
	}
}

func (c *Game) Reset() {
	w := &c.Worm
	w.Angle = 0
	w.WobbleCount = 0
	w.Length = 30
	w.Growto = 30
	w.Speed = 0.018
	for i := 0; i < w.Length; i++ {
		w.Segs[i].X = .5 - w.Speed*float64(i)
		w.Segs[i].Z = 0
	}
	if !c.InfLives {
		c.Score = 0
	}
	c.placeFood()
	c.RightPressed = false
	c.LeftPressed = false
	c.UpPressed = false
}

func (c *Game) placeFood() {
	for {
		c.FoodPos[0] = float64(rand.Intn(16384)-8192) / 4096
		c.FoodPos[1] = float64(rand.Intn(16384)-8192) / 4096

		if !(!c.isFree(c.FoodPos[0], c.FoodPos[1], false, nil) ||
			!c.isFree(c.FoodPos[0]+.1, c.FoodPos[1], false, nil) ||
			!c.isFree(c.FoodPos[0]-.1, c.FoodPos[1], false, nil) ||
			!c.isFree(c.FoodPos[0], c.FoodPos[1]+.1, false, nil) ||
			!c.isFree(c.FoodPos[0], c.FoodPos[1]-.1, false, nil)) {
			break
		}
	}
	c.GrowTimer = 0
}

// Check whether a given point collides with a stone block or (possibly,
// depending on "justblocks") by the worm itself. If the point is occupied by
// the worm, the output parameter "hint" is set to the number of the colliding
// segment.
func (c *Game) isFree(x, z float64, justBlocks bool, hint *int) bool {
	if x <= -2 || x >= 2 || z <= -2 || z >= 2 {
		return false
	}
	xblock := int((x + 2) * 4)
	zblock := int((z + 2) * 4)
	if c.Board[zblock*16+xblock] == '#' {
		return false
	}

	if justBlocks {
		return true
	}

	w := &c.Worm
	for i := 20; i < w.Length; i++ {
		dx := x - w.Segs[i].X
		dz := z - w.Segs[i].Z
		dist := dx*dx + dz*dz
		if dist < .01 {
			if hint != nil {
				*hint = i
			}
			return false
		}
	}

	return true
}

func normalize2(v []float64) {
	length := math.Hypot(v[0], v[1])
	v[0] /= length
	v[1] /= length
}
