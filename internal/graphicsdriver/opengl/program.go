// Copyright 2014 Hajime Hoshi
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package opengl

import (
	"fmt"

	"github.com/hajimehoshi/ebiten/internal/affine"
	"github.com/hajimehoshi/ebiten/internal/driver"
	"github.com/hajimehoshi/ebiten/internal/graphics"
	"github.com/hajimehoshi/ebiten/internal/web"
)

// arrayBufferLayoutPart is a part of an array buffer layout.
type arrayBufferLayoutPart struct {
	// TODO: This struct should belong to a program and know it.
	name string
	num  int
}

// arrayBufferLayout is an array buffer layout.
//
// An array buffer in OpenGL is a buffer representing vertices and
// is passed to a vertex shader.
type arrayBufferLayout struct {
	parts []arrayBufferLayoutPart
	total int
}

func (a *arrayBufferLayout) names() []string {
	ns := make([]string, len(a.parts))
	for i, p := range a.parts {
		ns[i] = p.name
	}
	return ns
}

// totalBytes returns the size in bytes for one element of the array buffer.
func (a *arrayBufferLayout) totalBytes() int {
	if a.total != 0 {
		return a.total
	}
	t := 0
	for _, p := range a.parts {
		t += float.SizeInBytes() * p.num
	}
	a.total = t
	return a.total
}

// newArrayBuffer creates OpenGL's buffer object for the array buffer.
func (a *arrayBufferLayout) newArrayBuffer(context *context) buffer {
	return context.newArrayBuffer(a.totalBytes() * graphics.IndicesNum)
}

// enable binds the array buffer the given program to use the array buffer.
func (a *arrayBufferLayout) enable(context *context, program program) {
	for i := range a.parts {
		context.enableVertexAttribArray(program, i)
	}
	total := a.totalBytes()
	offset := 0
	for i, p := range a.parts {
		context.vertexAttribPointer(program, i, p.num, float, total, offset)
		offset += float.SizeInBytes() * p.num
	}
}

// disable stops using the array buffer.
func (a *arrayBufferLayout) disable(context *context, program program) {
	// TODO: Disabling should be done in reversed order?
	for i := range a.parts {
		context.disableVertexAttribArray(program, i)
	}
}

// theArrayBufferLayout is the array buffer layout for Ebiten.
var theArrayBufferLayout = arrayBufferLayout{
	// Note that GL_MAX_VERTEX_ATTRIBS is at least 16.
	parts: []arrayBufferLayoutPart{
		{
			name: "vertex",
			num:  2,
		},
		{
			name: "tex",
			num:  2,
		},
		{
			name: "tex_region",
			num:  4,
		},
		{
			name: "color_scale",
			num:  4,
		},
	},
}

func init() {
	vertexFloatNum := theArrayBufferLayout.totalBytes() / float.SizeInBytes()
	if graphics.VertexFloatNum != vertexFloatNum {
		panic(fmt.Sprintf("vertex float num must be %d but %d", graphics.VertexFloatNum, vertexFloatNum))
	}
}

type programKey struct {
	useColorM bool
	filter    driver.Filter
	address   driver.Address
}

// openGLState is a state for
type openGLState struct {
	// arrayBuffer is OpenGL's array buffer (vertices data).
	arrayBuffer buffer

	// elementArrayBuffer is OpenGL's element array buffer (indices data).
	elementArrayBuffer buffer

	// programs is OpenGL's program for rendering a texture.
	programs map[programKey]program

	lastProgram  program
	lastUniforms map[string][]float32

	source      *Image
	destination *Image
}

var (
	zeroBuffer  buffer
	zeroProgram program
)

// reset resets or initializes the OpenGL state.
func (s *openGLState) reset(context *context) error {
	if err := context.reset(); err != nil {
		return err
	}

	s.lastProgram = zeroProgram
	s.lastUniforms = map[string][]float32{}

	// When context lost happens, deleting programs or buffers is not necessary.
	// However, it is not assumed that reset is called only when context lost happens.
	// Let's delete them explicitly.
	if s.programs == nil {
		s.programs = map[programKey]program{}
	} else {
		for k, p := range s.programs {
			context.deleteProgram(p)
			delete(s.programs, k)
		}
	}

	// On browsers (at least Chrome), buffers are already detached from the context
	// and must not be deleted by DeleteBuffer.
	if !web.IsBrowser() {
		if !s.arrayBuffer.equal(zeroBuffer) {
			context.deleteBuffer(s.arrayBuffer)
		}
		if !s.elementArrayBuffer.equal(zeroBuffer) {
			context.deleteBuffer(s.elementArrayBuffer)
		}
	}

	shaderVertexModelviewNative, err := context.newShader(vertexShader, vertexShaderStr())
	if err != nil {
		panic(fmt.Sprintf("graphics: shader compiling error:\n%s", err))
	}
	defer context.deleteShader(shaderVertexModelviewNative)

	for _, c := range []bool{false, true} {
		for _, a := range []driver.Address{
			driver.AddressClampToZero,
			driver.AddressRepeat,
		} {
			for _, f := range []driver.Filter{
				driver.FilterNearest,
				driver.FilterLinear,
				driver.FilterScreen,
			} {
				shaderFragmentColorMatrixNative, err := context.newShader(fragmentShader, fragmentShaderStr(c, f, a))
				if err != nil {
					panic(fmt.Sprintf("graphics: shader compiling error:\n%s", err))
				}
				defer context.deleteShader(shaderFragmentColorMatrixNative)

				program, err := context.newProgram([]shader{
					shaderVertexModelviewNative,
					shaderFragmentColorMatrixNative,
				}, theArrayBufferLayout.names())

				if err != nil {
					return err
				}

				s.programs[programKey{
					useColorM: c,
					filter:    f,
					address:   a,
				}] = program
			}
		}
	}

	s.arrayBuffer = theArrayBufferLayout.newArrayBuffer(context)

	// Note that the indices passed to NewElementArrayBuffer is not under GC management
	// in opengl package due to unsafe-way.
	// See NewElementArrayBuffer in context_mobile.go.
	s.elementArrayBuffer = context.newElementArrayBuffer(graphics.IndicesNum * 2)

	return nil
}

// areSameFloat32Array returns a boolean indicating if a and b are deeply equal.
func areSameFloat32Array(a, b []float32) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// useProgram uses the program (programTexture).
func (g *Graphics) useProgram(colorM *affine.ColorM, filter driver.Filter, address driver.Address) error {
	destination := g.state.destination
	if destination == nil {
		panic("destination image is not set")
	}
	source := g.state.source
	if source == nil {
		panic("source image is not set")
	}

	if err := destination.setViewport(); err != nil {
		return err
	}
	dstW := destination.width
	srcW, srcH := source.width, source.height

	program := g.state.programs[programKey{
		useColorM: colorM != nil,
		filter:    filter,
		address:   address,
	}]
	if !g.state.lastProgram.equal(program) {
		g.context.useProgram(program)
		if g.state.lastProgram.equal(zeroProgram) {
			theArrayBufferLayout.enable(&g.context, program)
			g.context.bindBuffer(arrayBuffer, g.state.arrayBuffer)
			g.context.bindBuffer(elementArrayBuffer, g.state.elementArrayBuffer)
			g.context.uniformInt(program, "texture", 0)
		}

		g.state.lastProgram = program
		g.state.lastUniforms = map[string][]float32{}
	}

	vw := destination.framebuffer.width
	vh := destination.framebuffer.height
	vs := []float32{float32(vw), float32(vh)}
	if !areSameFloat32Array(g.state.lastUniforms["viewport_size"], vs) {
		g.context.uniformFloats(program, "viewport_size", vs)
		g.state.lastUniforms["viewport_size"] = vs
	}

	if colorM != nil {
		esBody, esTranslate := colorM.UnsafeElements()
		if !areSameFloat32Array(g.state.lastUniforms["color_matrix_body"], esBody) {
			g.context.uniformFloats(program, "color_matrix_body", esBody)
			// ColorM's elements are immutable. It's OK to hold the reference without copying.
			g.state.lastUniforms["color_matrix_body"] = esBody
		}
		if !areSameFloat32Array(g.state.lastUniforms["color_matrix_translation"], esTranslate) {
			g.context.uniformFloats(program, "color_matrix_translation", esTranslate)
			// ColorM's elements are immutable. It's OK to hold the reference without copying.
			g.state.lastUniforms["color_matrix_translation"] = esTranslate
		}
	}

	if filter != driver.FilterNearest {
		sw := graphics.InternalImageSize(srcW)
		sh := graphics.InternalImageSize(srcH)
		s := []float32{float32(sw), float32(sh)}
		if !areSameFloat32Array(g.state.lastUniforms["source_size"], s) {
			g.context.uniformFloats(program, "source_size", s)
			g.state.lastUniforms["source_size"] = s
		}
	}

	if filter == driver.FilterScreen {
		scale := float32(dstW) / float32(srcW)
		g.context.uniformFloat(program, "scale", scale)
	}

	// We don't have to call gl.ActiveTexture here: GL_TEXTURE0 is the default active texture
	// See also: https://www.opengl.org/sdk/docs/man2/xhtml/glActiveTexture.xml
	g.context.bindTexture(source.textureNative)

	g.state.source = nil
	g.state.destination = nil
	return nil
}
