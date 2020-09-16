package main

import (
	sierpinski "../.."
	"fmt"
	"github.com/fogleman/gg"
	"github.com/go-gl/mathgl/mgl64"
	"golang.org/x/image/colornames"
	"image/color"
)

var threshold2 = 0.0001 * 0.0001

type LayerAnimation struct {
	Order                  int
	ImageSize              int
	PyramidSize            float64
	PyramidBaseCenter      mgl64.Vec3
	OutputFileFormatString string
	NumLayers              int
	//////////////////////////////
	ViewTransform mgl64.Mat4
	ctx           *gg.Context
}

var colors = []color.Color{
	colornames.Red,
	colornames.Green,
	colornames.Blue,
	//colornames.Orange,
	//colornames.Yellow,
	//colornames.Indigo,
	//colornames.Violet,
	//colornames.Cyan,
	colornames.Purple,
}

func (g *LayerAnimation) RenderLayer(z float64) {
	points := sierpinski.Sierpinski(g.Order, z)
	hiddenness := sierpinski.SierpinskiHiddenness(g.Order, z)
	if len(points) != len(hiddenness) {
		panic("that doesn't work")
	}
	for i := range points {
		points[i] = points[i].Mul(g.PyramidSize / 2).Add(g.PyramidBaseCenter)
	}

	for i := range points {
		pt0 := points[i]
		pt1 := points[(i+1)%len(points)]
		h := hiddenness[i]
		if vec4ApproxEq(pt0, pt1) {
			continue
		}
		c := colors[h%len(colors)]
		if h >= len(colors) {
			c = color.Black
		}
		g.ctx.SetColor(c)
		g.ctx.DrawLine(pt0[0], pt0[1], pt1[0], pt1[1])
		g.ctx.Stroke()
	}
}

func (g *LayerAnimation) RenderAllLayers() {
	g.ViewTransform = mgl64.Scale3D(0.9, 0.9, 1.0)

	g.ctx = gg.NewContext(g.ImageSize, g.ImageSize)
	g.ctx.SetColor(color.White)
	g.ctx.DrawRectangle(0, 0, float64(g.ImageSize), float64(g.ImageSize))
	g.ctx.Fill()
	//g.ctx.SetColor(color.Black)
	g.ctx.SetLineWidth(1.5)

	for i := 0; i < g.NumLayers; i++ {
		layerNominalHeight := sierpinski.PyramidNominalHeight * float64(i) / float64(g.NumLayers)

		g.ctx.SetColor(color.White)
		g.ctx.DrawRectangle(0, 0, float64(g.ImageSize), float64(g.ImageSize))
		g.ctx.Fill()
		g.RenderLayer(layerNominalHeight)

		outputFileName := fmt.Sprintf(g.OutputFileFormatString, i)
		err := g.ctx.SavePNG(outputFileName)
		die(err)
	}
}

func main() {
	anim := LayerAnimation{
		Order:                  4,
		ImageSize:              500,
		PyramidSize:            500.0,
		PyramidBaseCenter:      mgl64.Vec3{250.0, 250.0},
		OutputFileFormatString: "renders/hiddenness/layer_animation_%05d.png",
		NumLayers:              200,
		//ctx:                    nil,
	}
	anim.RenderAllLayers()
}

func die(err error) {
	if err != nil {
		panic(err)
	}
}

func vec4ApproxEq(a, b mgl64.Vec3) bool {
	dist := a.Sub(b).LenSqr()
	return dist < threshold2
}
