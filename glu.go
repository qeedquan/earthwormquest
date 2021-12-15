package main

import (
	"fmt"
	"math"
	"strings"
	"unsafe"

	"github.com/go-gl/gl/v2.1/gl"
	"github.com/qeedquan/go-media/math/ga"
	"github.com/qeedquan/go-media/math/ga/vec3"
)

type Quadric struct {
	Normals       int
	TextureCoords bool
	Orientation   int
	DrawStyle     int
}

type PixelStorageMode struct {
	PackAlignment   int32
	PackRowLength   int32
	PackSkipRows    int32
	PackSkipPixels  int32
	PackLSBFirst    int32
	PackSwapBytes   int32
	PackSkipImages  int32
	PackImageHeight int32

	UnpackAlignment   int32
	UnpackRowLength   int32
	UnpackSkipRows    int32
	UnpackSkipPixels  int32
	UnpackLSBFirst    int32
	UnpackSwapBytes   int32
	UnpackSkipImages  int32
	UnpackImageHeight int32
}

const (
	GLU_SMOOTH = 100000
	GLU_FLAT   = 100001
	GLU_NONE   = 100002
)

const (
	GLU_OUTSIDE = 100020
	GLU_INSIDE  = 100021
)

const (
	GLU_POINT      = 100010
	GLU_LINE       = 100011
	GLU_FILL       = 100012
	GLU_SILHOUETTE = 100013
)

func assert(x bool) {
	if !x {
		panic("assert failed")
	}
}

func NewQuadric() *Quadric {
	return &Quadric{
		Normals:     GLU_SMOOTH,
		Orientation: GLU_OUTSIDE,
		DrawStyle:   GLU_FILL,
	}
}

func gluLookAt(eyeX, eyeY, eyeZ, centerX, centerY, centerZ, upX, upY, upZ float64) {
	F := ga.Vec3d{
		centerX - eyeX,
		centerY - eyeY,
		centerZ - eyeZ,
	}
	f := vec3.Normalize(F)
	up := ga.Vec3d{upX, upY, upZ}
	s := vec3.Cross(f, up)
	s = vec3.Normalize(s)
	u := vec3.Cross(s, f)

	m := [16]float64{}
	m[0] = s.X
	m[4] = s.Y
	m[8] = s.Z
	m[1] = u.X
	m[5] = u.Y
	m[9] = u.Z
	m[2] = -f.X
	m[6] = -f.Y
	m[10] = -f.Z
	m[15] = 1

	gl.MultMatrixd(&m[0])
	gl.Translated(-eyeX, -eyeY, -eyeZ)
}

func gluPerspective(fovy, aspect, znear, zfar float64) {
	f := 1 / math.Tan((fovy/2)*math.Pi/180)
	dz := zfar - znear
	m := [16]float64{}
	m[0] = f / aspect
	m[5] = f
	m[10] = -(zfar + znear) / dz
	m[11] = -1
	m[14] = -2 * znear * zfar / dz
	gl.MultMatrixd(&m[0])
}

func gluSphere(qobj *Quadric, radius float64, slices, stacks int) {
	const CACHE_SIZE = 240
	var (
		sinCache1a, cosCache1a, sinCache2a, cosCache2a [CACHE_SIZE]float64
		sinCache3a, cosCache3a, sinCache1b, cosCache1b [CACHE_SIZE]float64
		sinCache2b, cosCache2b, sinCache3b, cosCache3b [CACHE_SIZE]float64
		sintemp3, costemp3, sintemp4, costemp4         float64
		needCache2, needCache3                         bool
		start, finish                                  int
	)

	if slices >= CACHE_SIZE {
		slices = CACHE_SIZE - 1
	}
	if stacks >= CACHE_SIZE {
		stacks = CACHE_SIZE - 1
	}
	if slices < 2 || stacks < 1 || radius < 0 {
		assert(false)
	}

	if qobj.Normals == GLU_SMOOTH {
		needCache2 = true
	}

	if qobj.Normals == GLU_FLAT {
		if qobj.DrawStyle != GLU_POINT {
			needCache3 = true
		}
		if qobj.DrawStyle == GLU_LINE {
			needCache2 = true
		}
	}

	for i := 0; i < slices; i++ {
		angle := 2 * math.Pi * float64(i) / float64(slices)
		sinCache1a[i] = math.Sin(angle)
		cosCache1a[i] = math.Cos(angle)
		if needCache2 {
			sinCache2a[i] = sinCache1a[i]
			cosCache2a[i] = cosCache1a[i]
		}
	}

	for j := 0; j <= stacks; j++ {
		angle := math.Pi * float64(j) / float64(stacks)
		if needCache2 {
			if qobj.Orientation == GLU_OUTSIDE {
				sinCache2b[j] = math.Sin(angle)
				cosCache2b[j] = math.Cos(angle)
			} else {
				sinCache2b[j] = -math.Sin(angle)
				cosCache2b[j] = -math.Cos(angle)
			}
		}
		sinCache1b[j] = radius * math.Sin(angle)
		cosCache1b[j] = radius * math.Cos(angle)
	}
	// Make sure it comes to a point
	sinCache1b[0] = 0
	sinCache1b[stacks] = 0

	if needCache3 {
		for i := 0; i < slices; i++ {
			angle := 2 * math.Pi * (float64(i) - 0.5) / float64(slices)
			sinCache3a[i] = math.Sin(angle)
			cosCache3a[i] = math.Cos(angle)
		}
		for j := 0; j <= stacks; j++ {
			angle := math.Pi * (float64(j) - 0.5) / float64(stacks)
			if qobj.Orientation == GLU_OUTSIDE {
				sinCache3b[j] = math.Sin(angle)
				cosCache3b[j] = math.Cos(angle)
			} else {
				sinCache3b[j] = -math.Sin(angle)
				cosCache3b[j] = -math.Cos(angle)
			}

		}
	}

	sinCache1a[slices] = sinCache1a[0]
	cosCache1a[slices] = cosCache1a[0]
	if needCache2 {
		sinCache2a[slices] = sinCache2a[0]
		cosCache2a[slices] = cosCache2a[0]
	}
	if needCache3 {
		sinCache3a[slices] = sinCache3a[0]
		cosCache3a[slices] = cosCache3a[0]
	}

	switch qobj.DrawStyle {
	case GLU_FILL:
		// Do ends of sphere as TRIANGLE_FAN's (if not texturing)
		// We don't do it when texturing because we need to respecify the
		// texture coordinates of the apex for every adjacent vertex (because
		// it isn't a constant for that point)
		if !qobj.TextureCoords {
			start = 1
			finish = stacks - 1

			// Low end first (j == 0 iteration)
			sintemp2 := sinCache1b[1]
			zHigh := cosCache1b[1]
			switch qobj.Normals {
			case GLU_FLAT:
				sintemp3 = sinCache3b[1]
				costemp3 = cosCache3b[1]
			case GLU_SMOOTH:
				sintemp3 = sinCache2b[1]
				costemp3 = cosCache2b[1]
				gl.Normal3d(sinCache2a[0]*sinCache2b[0], cosCache2a[0]*sinCache2b[0], cosCache2b[0])
			default:
				assert(false)
			}
			gl.Begin(gl.TRIANGLE_FAN)
			gl.Vertex3d(0.0, 0.0, radius)
			if qobj.Orientation == GLU_OUTSIDE {
				for i := slices; i >= 0; i-- {
					switch qobj.Normals {
					case GLU_SMOOTH:
						gl.Normal3d(sinCache2a[i]*sintemp3, cosCache2a[i]*sintemp3, costemp3)
					case GLU_FLAT:
						if i != slices {
							gl.Normal3d(sinCache3a[i+1]*sintemp3, cosCache3a[i+1]*sintemp3, costemp3)
						}
					case GLU_NONE:
					default:
						assert(false)
					}
					gl.Vertex3d(sintemp2*sinCache1a[i], sintemp2*cosCache1a[i], zHigh)
				}
			} else {
				for i := 0; i <= slices; i++ {
					switch qobj.Normals {
					case GLU_SMOOTH:
						gl.Normal3d(sinCache2a[i]*sintemp3, cosCache2a[i]*sintemp3, costemp3)
					case GLU_FLAT:
						gl.Normal3d(sinCache3a[i]*sintemp3, cosCache3a[i]*sintemp3, costemp3)
					case GLU_NONE:
					default:
						assert(false)
					}
					gl.Vertex3d(sintemp2*sinCache1a[i], sintemp2*cosCache1a[i], zHigh)
				}
			}
			gl.End()

			// High end next (j == stacks-1 iteration)
			sintemp2 = sinCache1b[stacks-1]
			zHigh = cosCache1b[stacks-1]
			switch qobj.Normals {
			case GLU_FLAT:
				sintemp3 = sinCache3b[stacks]
				costemp3 = cosCache3b[stacks]
			case GLU_SMOOTH:
				sintemp3 = sinCache2b[stacks-1]
				costemp3 = cosCache2b[stacks-1]
				gl.Normal3d(sinCache2a[stacks]*sinCache2b[stacks], cosCache2a[stacks]*sinCache2b[stacks], cosCache2b[stacks])
			default:
				assert(false)
			}
			gl.Begin(gl.TRIANGLE_FAN)
			gl.Vertex3d(0.0, 0.0, -radius)
			if qobj.Orientation == GLU_OUTSIDE {
				for i := 0; i <= slices; i++ {
					switch qobj.Normals {
					case GLU_SMOOTH:
						gl.Normal3d(sinCache2a[i]*sintemp3, cosCache2a[i]*sintemp3, costemp3)
					case GLU_FLAT:
						gl.Normal3d(sinCache3a[i]*sintemp3, cosCache3a[i]*sintemp3, costemp3)
					case GLU_NONE:
					default:
						assert(false)
					}
					gl.Vertex3d(sintemp2*sinCache1a[i], sintemp2*cosCache1a[i], zHigh)
				}
			} else {
				for i := slices; i >= 0; i-- {
					switch qobj.Normals {
					case GLU_SMOOTH:
						gl.Normal3d(sinCache2a[i]*sintemp3, cosCache2a[i]*sintemp3, costemp3)
					case GLU_FLAT:
						if i != slices {
							gl.Normal3d(sinCache3a[i+1]*sintemp3, cosCache3a[i+1]*sintemp3, costemp3)
						}
					case GLU_NONE:
					default:
						assert(false)
					}
					gl.Vertex3d(sintemp2*sinCache1a[i], sintemp2*cosCache1a[i], zHigh)
				}
			}
			gl.End()
		} else {
			start = 0
			finish = stacks
		}

		for j := start; j < finish; j++ {
			zLow := cosCache1b[j]
			zHigh := cosCache1b[j+1]
			sintemp1 := sinCache1b[j]
			sintemp2 := sinCache1b[j+1]
			switch qobj.Normals {
			case GLU_FLAT:
				sintemp4 = sinCache3b[j+1]
				costemp4 = cosCache3b[j+1]
			case GLU_SMOOTH:
				if qobj.Orientation == GLU_OUTSIDE {
					sintemp3 = sinCache2b[j+1]
					costemp3 = cosCache2b[j+1]
					sintemp4 = sinCache2b[j]
					costemp4 = cosCache2b[j]
				} else {
					sintemp3 = sinCache2b[j]
					costemp3 = cosCache2b[j]
					sintemp4 = sinCache2b[j+1]
					costemp4 = cosCache2b[j+1]
				}
			default:
				assert(false)
			}

			gl.Begin(gl.QUAD_STRIP)
			for i := 0; i <= slices; i++ {
				switch qobj.Normals {
				case GLU_SMOOTH:
					gl.Normal3d(sinCache2a[i]*sintemp3, cosCache2a[i]*sintemp3, costemp3)
				case GLU_FLAT, GLU_NONE:
				default:
					assert(false)
				}
				if qobj.Orientation == GLU_OUTSIDE {
					if qobj.TextureCoords {
						gl.TexCoord2d(1-float64(i)/float64(slices), 1-float64(j+1)/float64(stacks))
					}
					gl.Vertex3d(sintemp2*sinCache1a[i], sintemp2*cosCache1a[i], zHigh)
				} else {
					if qobj.TextureCoords {
						gl.TexCoord2d(1-float64(i)/float64(slices), 1-float64(j)/float64(stacks))
					}
					gl.Vertex3d(sintemp1*sinCache1a[i], sintemp1*cosCache1a[i], zLow)
				}
				switch qobj.Normals {
				case GLU_SMOOTH:
					gl.Normal3d(sinCache2a[i]*sintemp4, cosCache2a[i]*sintemp4, costemp4)
				case GLU_FLAT:
					gl.Normal3d(sinCache3a[i]*sintemp4, cosCache3a[i]*sintemp4, costemp4)
				case GLU_NONE:
				default:
					assert(false)
				}
				if qobj.Orientation == GLU_OUTSIDE {
					if qobj.TextureCoords {
						gl.TexCoord2d(1-float64(i)/float64(slices), 1-float64(j)/float64(stacks))
					}
					gl.Vertex3d(sintemp1*sinCache1a[i], sintemp1*cosCache1a[i], zLow)
				} else {
					if qobj.TextureCoords {
						gl.TexCoord2d(1-float64(i)/float64(slices), 1-float64(j+1)/float64(stacks))
					}
					gl.Vertex3d(sintemp2*sinCache1a[i], sintemp2*cosCache1a[i], zHigh)
				}
			}
			gl.End()
		}

	default:
		assert(false)
	}
}

func legalFormat(format int) bool {
	switch format {
	case gl.COLOR_INDEX, gl.STENCIL_INDEX, gl.DEPTH_COMPONENT, gl.RED, gl.GREEN, gl.BLUE,
		gl.ALPHA, gl.RGB, gl.RGBA, gl.LUMINANCE, gl.LUMINANCE_ALPHA, gl.BGR, gl.BGRA:
		return true
	}
	return false
}

func legalType(typ int) bool {
	switch typ {
	case gl.BITMAP, gl.BYTE, gl.UNSIGNED_BYTE, gl.SHORT, gl.UNSIGNED_SHORT, gl.INT,
		gl.UNSIGNED_INT, gl.FLOAT, gl.UNSIGNED_BYTE_3_3_2, gl.UNSIGNED_BYTE_2_3_3_REV,
		gl.UNSIGNED_SHORT_5_6_5, gl.UNSIGNED_SHORT_5_6_5_REV, gl.UNSIGNED_SHORT_4_4_4_4,
		gl.UNSIGNED_SHORT_4_4_4_4_REV, gl.UNSIGNED_SHORT_5_5_5_1, gl.UNSIGNED_SHORT_1_5_5_5_REV,
		gl.UNSIGNED_INT_8_8_8_8, gl.UNSIGNED_INT_8_8_8_8_REV, gl.UNSIGNED_INT_10_10_10_2,
		gl.UNSIGNED_INT_2_10_10_10_REV:
		return true
	}
	return false
}

func isTypePackedPixel(typ int) bool {
	if !legalType(typ) {
		return false
	}
	switch typ {
	case gl.UNSIGNED_BYTE_3_3_2, gl.UNSIGNED_BYTE_2_3_3_REV, gl.UNSIGNED_SHORT_5_6_5,
		gl.UNSIGNED_SHORT_5_6_5_REV, gl.UNSIGNED_SHORT_4_4_4_4, gl.UNSIGNED_SHORT_4_4_4_4_REV,
		gl.UNSIGNED_SHORT_5_5_5_1, gl.UNSIGNED_SHORT_1_5_5_5_REV, gl.UNSIGNED_INT_8_8_8_8,
		gl.UNSIGNED_INT_8_8_8_8_REV, gl.UNSIGNED_INT_10_10_10_2, gl.UNSIGNED_INT_2_10_10_10_REV:
		return true
	}
	return false
}

func isLegalFormatForPackedPixelType(format, typ int) bool {
	if isTypePackedPixel(typ) {
		return true
	}

	// 3_3_2/2_3_3_REV & 5_6_5/5_6_5_REV are only compatible with RGB
	switch typ {
	case gl.UNSIGNED_BYTE_3_3_2, gl.UNSIGNED_BYTE_2_3_3_REV, gl.UNSIGNED_SHORT_5_6_5, gl.UNSIGNED_SHORT_5_6_5_REV:
		if format != gl.RGB {
			return false
		}
	}

	// 4_4_4_4/4_4_4_4_REV & 5_5_5_1/1_5_5_5_REV & 8_8_8_8/8_8_8_8_REV &
	// 10_10_10_2/2_10_10_10_REV are only compatible with RGBA, BGRA & ABGR_EXT.
	switch typ {
	case gl.UNSIGNED_SHORT_4_4_4_4, gl.UNSIGNED_SHORT_4_4_4_4_REV, gl.UNSIGNED_SHORT_5_5_5_1,
		gl.UNSIGNED_SHORT_1_5_5_5_REV, gl.UNSIGNED_INT_8_8_8_8, gl.UNSIGNED_INT_8_8_8_8_REV,
		gl.UNSIGNED_INT_10_10_10_2, gl.UNSIGNED_INT_2_10_10_10_REV:
		if format != gl.RGBA && format != gl.BGRA {
			return false
		}
	}

	return true
}

func checkMipmapArgs(internalFormat, format, typ int) error {
	if !legalFormat(format) || !legalType(typ) {
		return fmt.Errorf("invalid format")
	}
	if format == gl.STENCIL_INDEX {
		return fmt.Errorf("invalid format")
	}
	if !isLegalFormatForPackedPixelType(format, typ) {
		return fmt.Errorf("invalid operation")
	}
	return nil
}

/* Given user requested texture size, determine if it fits. If it
 * doesn't then halve both sides and make the determination again
 * until it does fit (for IR only).
 * Note that proxy textures are not implemented in RE* even though
 * they advertise the texture extension.
 * Note that proxy textures are implemented but not according to spec in
 * IMPACT*.
 */
func closestFit(target, width, height, internalFormat, format, typ int, newWidth, newHeight *int) {
	// Use proxy textures if OpenGL version is >= 1.1
	str := gl.GoStr(gl.GetString(gl.VERSION))
	toks := strings.Split(str, " ")
	version := toks[0]
	if version >= "1.1" {
		widthPowerOf2 := nearestPow2(width)
		heightPowerOf2 := nearestPow2(height)
		proxyWidth := 0
		for {
			widthAtLevelOne := widthPowerOf2
			heightAtLevelOne := heightPowerOf2
			assert(widthAtLevelOne > 0)
			assert(heightAtLevelOne > 0)

			if widthPowerOf2 > 1 {
				widthAtLevelOne = widthPowerOf2 >> 1
			}
			if heightPowerOf2 > 1 {
				heightAtLevelOne = heightPowerOf2 >> 1
			}

			// does width x height at level 1 & all their mipmaps fit?
			var proxyTarget uint32
			if target == gl.TEXTURE_2D || target == gl.PROXY_TEXTURE_2D {
				proxyTarget = gl.PROXY_TEXTURE_2D
				gl.TexImage2D(proxyTarget, 1, int32(internalFormat), int32(widthAtLevelOne), int32(heightAtLevelOne), 0, uint32(format), uint32(typ), nil)
			} else {
				switch target {
				case gl.TEXTURE_CUBE_MAP_POSITIVE_X_ARB, gl.TEXTURE_CUBE_MAP_NEGATIVE_X_ARB,
					gl.TEXTURE_CUBE_MAP_POSITIVE_Y_ARB, gl.TEXTURE_CUBE_MAP_NEGATIVE_Y_ARB,
					gl.TEXTURE_CUBE_MAP_POSITIVE_Z_ARB, gl.TEXTURE_CUBE_MAP_NEGATIVE_Z_ARB:
					proxyTarget = gl.PROXY_TEXTURE_CUBE_MAP_ARB
					gl.TexImage2D(proxyTarget, 1, int32(internalFormat), int32(widthAtLevelOne), int32(heightAtLevelOne), 0, uint32(format), uint32(typ), nil)
				default:
					assert(target == gl.TEXTURE_1D || target == gl.PROXY_TEXTURE_1D)
					proxyTarget = gl.PROXY_TEXTURE_1D
					gl.TexImage1D(proxyTarget, 1, int32(internalFormat), int32(widthAtLevelOne), 0, uint32(format), uint32(typ), nil)
				}
			}

			var width int32
			gl.GetTexLevelParameteriv(proxyTarget, 1, gl.TEXTURE_WIDTH, &width)
			proxyWidth = int(width)

			// does it fit???
			if proxyWidth == 0 {
				// nope, so try again with these sizes
				if widthPowerOf2 == 1 && heightPowerOf2 == 1 {
					// An 1x1 texture couldn't fit for some reason, so
					// break out.  This should never happen. But things
					// happen.  The disadvantage with this if-statement is
					// that we will never be aware of when this happens
					// since it will silently branch out.
					closestFitNoProxyTexture(int(width), height, newWidth, newHeight)
					return
				}
				widthPowerOf2 = widthAtLevelOne
				heightPowerOf2 = heightAtLevelOne
			}

			if proxyWidth != 0 {
				break
			}
		}

		*newWidth = widthPowerOf2
		*newHeight = heightPowerOf2
	} else {
		closestFitNoProxyTexture(width, height, newWidth, newHeight)
	}
}

func closestFitNoProxyTexture(width, height int, newWidth, newHeight *int) {
	var maxSize int32
	gl.GetIntegerv(gl.MAX_TEXTURE_SIZE, &maxSize)

	*newWidth = nearestPow2(width)
	if *newWidth > int(maxSize) {
		*newWidth = int(maxSize)
	}
	*newHeight = nearestPow2(height)
	if *newHeight > int(maxSize) {
		*newHeight = int(maxSize)
	}
}

func gluBuild2DMipmaps(target, internalFormat, width, height, format, typ int, data []byte) {
	err := checkMipmapArgs(internalFormat, format, typ)
	if err != nil {
		panic(err)
	}

	if width < 1 || height < 1 {
		panic("invalid width/height")
	}

	var widthPowerOf2, heightPowerOf2 int
	closestFit(target, width, height, internalFormat, format, typ, &widthPowerOf2, &heightPowerOf2)

	levels := int(math.Log2(float64(widthPowerOf2)))
	level := int(math.Log2(float64(heightPowerOf2)))
	if level > levels {
		levels = level
	}
	gluBuild2DMipmapLevelsCore(target, internalFormat, width, height, widthPowerOf2, heightPowerOf2, format, typ, 0, 0, levels, data)
}

func gluBuild2DMipmapLevelsCore(target, internalFormat, width, height, widthPowerOf2, heightPowerOf2, format, typ, userLevel, baseLevel, maxLevel int, data []byte) {
	err := checkMipmapArgs(internalFormat, format, typ)
	assert(err == nil)
	assert(width >= 1 && height >= 1)
	assert(typ != gl.BITMAP)

	newwidth := widthPowerOf2
	newheight := heightPowerOf2
	levels := int(math.Log2(float64(newwidth)))
	level := int(math.Log2(float64(newheight)))
	if level > levels {
		levels = level
	}
	levels += userLevel

	var (
		psm           PixelStorageMode
		groupsPerLine int
	)
	retrieveStoreModes(&psm)
	cmpts := elementsPerGroup(format, typ)
	if psm.UnpackRowLength > 0 {
		groupsPerLine = int(psm.UnpackRowLength)
	} else {
		groupsPerLine = width
	}

	elementSize := bytesPerElement(typ)
	groupSize := int(elementSize * float64(cmpts))

	rowsize := groupsPerLine * groupSize
	padding := (rowsize % int(psm.UnpackAlignment))
	if padding != 0 {
		rowsize += int(psm.UnpackAlignment) - padding
	}
	usersImage := data[int(psm.UnpackSkipRows)*rowsize+int(psm.UnpackSkipPixels)*groupSize:]

	gl.PixelStorei(gl.UNPACK_SKIP_ROWS, 0)
	gl.PixelStorei(gl.UNPACK_SKIP_PIXELS, 0)
	gl.PixelStorei(gl.UNPACK_ROW_LENGTH, 0)

	level = userLevel
	var (
		srcImage []byte
		dstImage []byte
		memreq   int
	)
	// already power-of-two square
	if width == newwidth && height == newheight {
		// Use usersImage for level userLevel
		if baseLevel <= level && level <= maxLevel {
			gl.PixelStorei(gl.UNPACK_ROW_LENGTH, psm.UnpackRowLength)
			gl.TexImage2D(uint32(target), int32(level), int32(internalFormat), int32(width), int32(height), 0, uint32(format), uint32(typ), unsafe.Pointer(&usersImage[0]))
		}

		gl.PixelStorei(gl.UNPACK_ROW_LENGTH, 0)
		if levels == 0 {
			// we're done. clean up and return
			gl.PixelStorei(gl.UNPACK_ALIGNMENT, psm.UnpackAlignment)
			gl.PixelStorei(gl.UNPACK_SKIP_ROWS, psm.UnpackSkipRows)
			gl.PixelStorei(gl.UNPACK_SKIP_PIXELS, psm.UnpackSkipPixels)
			gl.PixelStorei(gl.UNPACK_ROW_LENGTH, psm.UnpackRowLength)
			gl.PixelStorei(gl.UNPACK_SWAP_BYTES, psm.UnpackSwapBytes)
			return
		}
		{
			nextwidth := newwidth / 2
			nextheight := newheight / 2

			/* clamp to 1 */
			if nextwidth < 1 {
				nextwidth = 1
			}
			if nextheight < 1 {
				nextheight = 1
			}
			memreq = imageSize(nextwidth, nextheight, format, typ)
		}

		switch typ {
		case gl.UNSIGNED_BYTE:
			dstImage = make([]byte, memreq)
		default:
			assert(false)
		}

		switch typ {
		case gl.UNSIGNED_BYTE:
			halveImageUbyte(cmpts, width, height, usersImage, dstImage, int(elementSize), rowsize, groupSize)
		default:
			assert(false)
		}

		newwidth = width / 2
		newheight = height / 2
		/* clamp to 1 */
		if newwidth < 1 {
			newwidth = 1
		}
		if newheight < 1 {
			newheight = 1
		}

		rowsize = newwidth * groupSize
		memreq = imageSize(newwidth, newheight, format, typ)
		srcImage, dstImage = dstImage, srcImage
		switch typ {
		case gl.UNSIGNED_BYTE:
			dstImage = make([]byte, memreq)
		default:
			assert(false)
		}

		// level userLevel+1 is in srcImage; level userLevel already saved
		level = userLevel + 1
	} else {
		// user's image is *not* nice power-of-2 sized square
		memreq = imageSize(newwidth, newheight, format, typ)
		switch typ {
		case gl.UNSIGNED_BYTE:
			dstImage = make([]byte, memreq)
		default:
			assert(false)
		}

		switch typ {
		case gl.UNSIGNED_BYTE:
			scaleInternalUbyte(cmpts, width, height, usersImage, newwidth, newheight, dstImage, int(elementSize), rowsize, groupSize)
		default:
			assert(false)
		}

		rowsize = newwidth * groupSize
		srcImage, dstImage = dstImage, srcImage

		if levels != 0 {
			// use as little memory as possible
			{
				nextWidth := newwidth / 2
				nextHeight := newheight / 2
				if nextWidth < 1 {
					nextWidth = 1
				}
				if nextHeight < 1 {
					nextHeight = 1
				}

				memreq = imageSize(nextWidth, nextHeight, format, typ)
			}

			switch typ {
			case gl.UNSIGNED_BYTE:
				dstImage = make([]byte, memreq)
			default:
				assert(false)
			}
		}
		// level userLevel is in srcImage; nothing saved yet
		level = userLevel
	}

	gl.PixelStorei(gl.UNPACK_SWAP_BYTES, 0)
	if baseLevel <= level && level <= maxLevel {
		gl.TexImage2D(uint32(target), int32(level), int32(internalFormat), int32(newwidth), int32(newheight), 0, uint32(format), uint32(typ), unsafe.Pointer(&srcImage[0]))
	}

	// update current level for the loop
	level++
	for ; level <= levels; level++ {
		switch typ {
		case gl.UNSIGNED_BYTE:
			halveImageUbyte(cmpts, newwidth, newheight, srcImage, dstImage, int(elementSize), rowsize, groupSize)
		default:
			assert(false)
		}

		srcImage, dstImage = dstImage, srcImage
		if newwidth > 1 {
			newwidth /= 2
			rowsize /= 2
		}
		if newheight > 1 {
			newheight /= 2
		}
		{
			// compute amount to pad per row, if any
			rowPad := rowsize % int(psm.UnpackAlignment)

			// should the row be padded
			if rowPad == 0 {
				// nope, row should not be padded
				// call tex image with srcImage untouched since it's not padded
				if baseLevel <= level && level <= maxLevel {
					gl.TexImage2D(uint32(target), int32(level), int32(internalFormat), int32(newwidth), int32(newheight), 0, uint32(format), uint32(typ), unsafe.Pointer(&srcImage[0]))
				}
			} else {
				// compute length of new row in bytes, including padding
				newRowLength := rowsize + int(psm.UnpackAlignment) - rowPad
				newMipmapImage := make([]byte, newRowLength*newheight)

				// copy image from srcImage into newMipmapImage by rows
				dstTrav := newMipmapImage
				srcTrav := srcImage
				for ii := 0; ii < newheight; ii++ {
					copy(dstTrav[:rowsize], srcTrav)
					// make sure the correct distance...
					// is skipped
					dstTrav = dstTrav[newRowLength:]
					srcTrav = srcTrav[rowsize:]
					// note that the pad bytes are not visited and will contain garbage, which is ok.
				}

				// ...and use this new image for mipmapping instead
				if baseLevel <= level && level <= maxLevel {
					gl.TexImage2D(uint32(target), int32(level), int32(internalFormat), int32(newwidth), int32(newheight), 0, uint32(format), uint32(typ), unsafe.Pointer(&newMipmapImage[0]))
				}
			}
		}
	}

	gl.PixelStorei(gl.UNPACK_ALIGNMENT, psm.UnpackAlignment)
	gl.PixelStorei(gl.UNPACK_SKIP_ROWS, psm.UnpackSkipRows)
	gl.PixelStorei(gl.UNPACK_SKIP_PIXELS, psm.UnpackSkipPixels)
	gl.PixelStorei(gl.UNPACK_ROW_LENGTH, psm.UnpackRowLength)
	gl.PixelStorei(gl.UNPACK_SWAP_BYTES, psm.UnpackSwapBytes)
}

func retrieveStoreModes(psm *PixelStorageMode) {
	gl.GetIntegerv(gl.UNPACK_ALIGNMENT, &psm.UnpackAlignment)
	gl.GetIntegerv(gl.UNPACK_ROW_LENGTH, &psm.UnpackRowLength)
	gl.GetIntegerv(gl.UNPACK_SKIP_ROWS, &psm.UnpackSkipRows)
	gl.GetIntegerv(gl.UNPACK_SKIP_PIXELS, &psm.UnpackSkipPixels)
	gl.GetIntegerv(gl.UNPACK_LSB_FIRST, &psm.UnpackLSBFirst)
	gl.GetIntegerv(gl.UNPACK_SWAP_BYTES, &psm.UnpackSwapBytes)

	gl.GetIntegerv(gl.PACK_ALIGNMENT, &psm.PackAlignment)
	gl.GetIntegerv(gl.PACK_ROW_LENGTH, &psm.PackRowLength)
	gl.GetIntegerv(gl.PACK_SKIP_ROWS, &psm.PackSkipRows)
	gl.GetIntegerv(gl.PACK_SKIP_PIXELS, &psm.PackSkipPixels)
	gl.GetIntegerv(gl.PACK_LSB_FIRST, &psm.PackLSBFirst)
	gl.GetIntegerv(gl.PACK_SWAP_BYTES, &psm.PackSwapBytes)
}

// Compute memory required for internal packed array of data of given type
// and format.
func imageSize(width, height, format, typ int) int {
	assert(width > 0)
	assert(height > 0)

	components := elementsPerGroup(format, typ)
	var bytesPerRow int
	if typ == gl.BITMAP {
		bytesPerRow = (width + 7) / 8
	} else {
		bytesPerRow = int(bytesPerElement(typ) * float64(width))
	}
	return bytesPerRow * height * components
}

// Return the number of bytes per element, based on the element type
func bytesPerElement(typ int) float64 {
	switch typ {
	case gl.BITMAP:
		return 1 / 8.0
	case gl.UNSIGNED_SHORT:
		return 2
	case gl.SHORT:
		return 2
	case gl.UNSIGNED_BYTE:
		return 1
	case gl.BYTE:
		return 1
	case gl.INT:
		return 4
	case gl.UNSIGNED_INT:
		return 4
	case gl.FLOAT:
		return 4
	case gl.UNSIGNED_BYTE_3_3_2, gl.UNSIGNED_BYTE_2_3_3_REV:
		return 1
	case gl.UNSIGNED_SHORT_5_6_5, gl.UNSIGNED_SHORT_5_6_5_REV, gl.UNSIGNED_SHORT_4_4_4_4,
		gl.UNSIGNED_SHORT_4_4_4_4_REV, gl.UNSIGNED_SHORT_5_5_5_1, gl.UNSIGNED_SHORT_1_5_5_5_REV:
		return 2
	case gl.UNSIGNED_INT_8_8_8_8, gl.UNSIGNED_INT_8_8_8_8_REV, gl.UNSIGNED_INT_10_10_10_2, gl.UNSIGNED_INT_2_10_10_10_REV:
		return 4
	}
	return 4
}

// Return the number of elements per group of a specified format
func elementsPerGroup(format, typ int) int {
	switch typ {
	case gl.UNSIGNED_BYTE_3_3_2, gl.UNSIGNED_BYTE_2_3_3_REV, gl.UNSIGNED_SHORT_5_6_5,
		gl.UNSIGNED_SHORT_5_6_5_REV, gl.UNSIGNED_SHORT_4_4_4_4, gl.UNSIGNED_SHORT_4_4_4_4_REV,
		gl.UNSIGNED_SHORT_5_5_5_1, gl.UNSIGNED_SHORT_1_5_5_5_REV, gl.UNSIGNED_INT_8_8_8_8,
		gl.UNSIGNED_INT_8_8_8_8_REV, gl.UNSIGNED_INT_10_10_10_2, gl.UNSIGNED_INT_2_10_10_10_REV:
		return 1
	}

	// Types are not packed pixels, so get elements per group
	switch format {
	case gl.RGB, gl.BGR:
		return 3
	case gl.LUMINANCE_ALPHA:
		return 2
	case gl.RGBA, gl.BGRA:
		return 4
	}
	return 1
}

func halveImageUbyte(components, width, height int, datain, dataout []byte, elementSize, ysize, groupSize int) {
	// handle case where there is only 1 column/row
	if width == 1 || height == 1 {
		// can't be 1x1
		assert(!(width == 1 && height == 1))
		halve1DimageUbyte(components, width, height, datain, dataout, elementSize, ysize, groupSize)
		return
	}

	newwidth := width / 2
	newheight := height / 2
	padBytes := ysize - (width * groupSize)
	s := dataout
	t := datain

	// Piece o' cake!
	for i := 0; i < newheight; i++ {
		for j := 0; j < newwidth; j++ {
			for k := 0; k < components; k++ {
				s[0] = byte((int(t[0]) + int(t[groupSize]) + int(t[ysize]) + int(t[ysize+groupSize]) + 2) / 4)
				s = s[1:]
				t = t[elementSize:]
			}
			t = t[groupSize:]
		}
		t = t[padBytes:]
		t = t[ysize:]
	}
}

func halve1DimageUbyte(components, width, height int, datain, dataout []byte, elementSize, ysize, groupSize int) {
	// must be 1D
	assert(width == 1 || height == 1)
	// can't be square
	assert(width != height)

	halfWidth := width / 2
	halfHeight := height / 2
	src := datain
	dest := dataout
	// 1 row
	if height == 1 {
		// width x height can't be 1x1
		assert(width != 1)
		halfHeight = 1

		for jj := 0; jj < halfWidth; jj++ {
			for kk := 0; kk < components; kk++ {
				dest[0] = (src[0] + src[groupSize]) / 2
				src = src[elementSize:]
				dest = dest[1:]
			}
			// skip to next 2
			src = src[groupSize:]
		}
		{
			padBytes := ysize - (width * groupSize)
			// for assertion only
			src = src[padBytes:]
		}
	} else if width == 1 {
		// 1 column
		padBytes := ysize - (width * groupSize)
		// width x height can't be 1x1
		assert(height != 1)

		// one vertical column with possible pad bytes per row
		// average two at a time
		for jj := 0; jj < halfHeight; jj++ {
			for kk := 0; kk < components; kk++ {
				dest[0] = (src[0] + src[ysize]) / 2
				src = src[elementSize:]
				dest = dest[1:]
			}
			// add pad bytes, if any, to get to end to row
			src = src[padBytes:]
			src = src[ysize:]
		}
	}

	assert(&src[0] == &datain[ysize*height])
	assert(&dest[0] == &dataout[components*elementSize*halfWidth*halfHeight])
}

func scaleInternalUbyte(components, widthin, heightin int, datain []byte, widthout, heightout int, dataout []byte, elementSize, ysize, groupSize int) {
	if widthin == widthout*2 && heightin == heightout*2 {
		halveImageUbyte(components, widthin, heightin, datain, dataout, elementSize, ysize, groupSize)
		return
	}

	convy := float64(heightin) / float64(heightout)
	convx := float64(widthin) / float64(widthout)
	convy_int := int(math.Floor(convy))
	convy_float := convy - float64(convy_int)
	convx_int := int(math.Floor(convx))
	convx_float := convx - float64(convx_int)

	area := convx * convy

	lowy_int := 0
	lowy_float := 0.0
	highy_int := int(convy_int)
	highy_float := convy_float

	var totals [4]float64

	for i := 0; i < heightout; i++ {
		// Clamp here to be sure we don't read beyond input buffer.
		if highy_int >= heightin {
			highy_int = heightin - 1
		}

		lowx_int := 0
		lowx_float := 0.0
		highx_int := convx_int
		highx_float := convx_float

		for j := 0; j < widthout; j++ {
			// Ok, now apply box filter to box that goes from (lowx, lowy)
			// to (highx, highy) on input data into this pixel on output
			// data.

			for n := range totals {
				totals[n] = 0.0
			}

			// calculate the value for pixels in the 1st row
			xindex := lowx_int * groupSize
			if highy_int > lowy_int && highx_int > lowx_int {
				y_percent := 1 - lowy_float
				temp := datain[xindex+int(lowy_int)*ysize:]
				percent := y_percent * (1 - lowx_float)
				temp_index := temp
				for k := 0; k < components; k++ {
					totals[k] += float64(temp_index[0]) * percent
					temp_index = temp_index[elementSize:]
				}
				left := temp
				for l := lowx_int + 1; l < highx_int; l++ {
					temp = temp[groupSize:]
					temp_index := temp
					for k := 0; k < components; k++ {
						totals[k] += float64(temp_index[0]) * y_percent
						temp_index = temp_index[elementSize:]
					}
				}
				temp = temp[groupSize:]
				right := temp
				percent = y_percent * highx_float
				temp_index = temp
				for k := 0; k < components; k++ {
					totals[k] += float64(temp_index[0]) * percent
					temp_index = temp_index[elementSize:]
				}

				// calculate the value for pixels in the last row
				y_percent = highy_float
				percent = y_percent * (1 - lowx_float)
				temp = datain[xindex+int(highy_int)*ysize:]
				temp_index = temp
				for k := 0; k < components; k++ {
					totals[k] += float64(temp_index[0]) * percent
					temp_index = temp_index[elementSize:]
				}
				for l := lowx_int + 1; l < highx_int; l++ {
					temp = temp[groupSize:]
					for k := 0; k < components; k++ {
						totals[k] += float64(temp_index[0]) * y_percent
						temp_index = temp_index[elementSize:]
					}
				}
				temp = temp[groupSize:]
				percent = y_percent * highx_float
				for k := 0; k < components; k++ {
					totals[k] += float64(temp_index[0]) * percent
					temp_index = temp_index[elementSize:]
				}

				// calculate the value for pixels in the 1st and last column
				for m := lowy_int + 1; m < highy_int; m++ {
					left = left[ysize:]
					right = right[ysize:]
					for k := 0; k < components; k++ {
						totals[k] += float64(left[0])*(1-lowx_float) + float64(right[0])*highx_float
						left = left[elementSize:]
						right = right[elementSize:]
					}
				}
			} else if highy_int > lowy_int {
				x_percent := highx_float - lowx_float
				percent := (1 - lowy_float) * x_percent
				temp := datain[xindex+int(lowy_int)*ysize:]
				temp_index := temp
				for k := 0; k < components; k++ {
					totals[k] += float64(temp_index[0]) * percent
				}
				for m := lowy_int + 1; m < highy_int; m++ {
					temp = temp[ysize:]
					for k := 0; k < components; k++ {
						totals[k] += float64(temp_index[0]) * x_percent
						temp_index = temp_index[elementSize:]
					}
				}
				percent = x_percent * highy_float
				temp = temp[ysize:]
				temp_index = temp
				for k := 0; k < components; k++ {
					totals[k] += float64(temp_index[0]) * percent
					temp_index = temp_index[elementSize:]
				}
			} else if highx_int > lowx_int {
				y_percent := highy_float - lowy_float
				percent := (1 - lowx_float) * y_percent
				temp := datain[xindex+int(lowy_int)*ysize:]
				temp_index := temp
				for k := 0; k < components; k++ {
					totals[k] += float64(temp_index[0]) * percent
					temp_index = temp_index[elementSize:]
				}
				for l := lowx_int + 1; l < highx_int; l++ {
					temp = temp[groupSize:]
					temp_index = temp
					for k := 0; k < components; k++ {
						totals[k] += float64(temp_index[0]) * percent
						temp_index = temp_index[elementSize:]
					}
				}
				temp = temp[groupSize:]
				percent = y_percent * highx_float
				temp_index = temp
				for k := 0; k < components; k++ {
					totals[k] += float64(temp_index[0]) * percent
					temp_index = temp_index[elementSize:]
				}
			} else {
				percent := (highy_float - lowy_float) * (highx_float - lowx_float)
				temp := datain[xindex+int(lowy_int)*ysize:]
				temp_index := temp
				for k := 0; k < components; k++ {
					totals[k] += float64(temp_index[0]) * percent
					temp_index = temp_index[elementSize:]
				}
			}

			// this is for the pixels in the body
			var temp0 []byte
			if lowy_int+1 < highy_int {
				temp0 = datain[xindex+groupSize+(lowy_int+1)*ysize:]
			}
			for m := lowy_int + 1; m < highy_int; m++ {
				temp := temp0
				for l := lowx_int + 1; l < highx_int; l++ {
					temp_index := temp
					for k := 0; k < components; k++ {
						totals[k] += float64(temp_index[0])
						temp_index = temp_index[elementSize:]
					}
					temp = temp[groupSize:]
				}
				temp0 = temp0[ysize:]
			}

			outindex := (j + (i * widthout)) * components
			for k := 0; k < components; k++ {
				dataout[outindex+k] = byte(totals[k] / area)
			}

			lowx_int = highx_int
			lowx_float = highx_float
			highx_int += convx_int
			highx_float += convx_float
			if highx_float > 1 {
				highx_float -= 1.0
				highx_int++
			}
		}
		lowy_int = highy_int
		lowy_float = highy_float
		highy_int += convy_int
		highy_float += convy_float
		if highy_float > 1 {
			highy_float -= 1.0
			highy_int++
		}
	}
}

func gluQuadricDrawStyle(quad *Quadric, drawStyle int) {
	switch drawStyle {
	case GLU_POINT, GLU_LINE, GLU_FILL, GLU_SILHOUETTE:
		quad.DrawStyle = drawStyle
	default:
		panic("unreachable")
	}
}

func nearestPow2(v int) int {
	if v <= 0 {
		return -1
	}
	for i := 1; ; {
		if v == 1 {
			return i
		}
		if v == 3 {
			return i * 4
		}
		v >>= 1
		i *= 2
	}
}
