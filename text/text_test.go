// Copyright 2018 The Ebiten Authors
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

package text_test

import (
	"image/color"
	"testing"

	"github.com/hajimehoshi/bitmapfont/v2"

	"github.com/hajimehoshi/ebiten"
	t "github.com/hajimehoshi/ebiten/internal/testing"
	. "github.com/hajimehoshi/ebiten/text"
)

func TestMain(m *testing.M) {
	t.MainWithRunLoop(m)
}

func TestTextColor(t *testing.T) {
	clr := color.RGBA{0x80, 0x80, 0x80, 0x80}
	img, _ := ebiten.NewImage(30, 30, ebiten.FilterNearest)
	Draw(img, "Hello", bitmapfont.Face, 12, 12, clr)

	w, h := img.Size()
	allTransparent := true
	for j := 0; j < h; j++ {
		for i := 0; i < w; i++ {
			got := img.At(i, j)
			want1 := color.RGBA{0x80, 0x80, 0x80, 0x80}
			want2 := color.RGBA{}
			if got != want1 && got != want2 {
				t.Errorf("img At(%d, %d): got %v; want %v or %v", i, j, got, want1, want2)
			}
			if got == want1 {
				allTransparent = false
			}
		}
	}
	if allTransparent {
		t.Fail()
	}
}
