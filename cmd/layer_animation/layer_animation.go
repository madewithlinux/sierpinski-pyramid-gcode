package main

import (
	sierpinski "../.."
	"fmt"
	"github.com/fogleman/gg"
	"github.com/go-gl/mathgl/mgl64"
	"github.com/lucasb-eyer/go-colorful"
	"image/color"
	"math"
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

func (g *LayerAnimation) RenderLayer(z float64) {
	points := sierpinski.Sierpinski(g.Order, z)
	for i := range points {
		points[i] = mgl64.TransformCoordinate(points[i], g.ViewTransform)
		points[i] = points[i].Mul(g.PyramidSize / 2).Add(g.PyramidBaseCenter)
	}

	lastPt := points[0]
	g.ctx.MoveTo(lastPt[0], lastPt[1])
	for _, pt := range points {
		if vec4ApproxEq(lastPt, pt) {
			continue
		}
		g.ctx.LineTo(pt[0], pt[1])
		lastPt = pt
	}
	g.ctx.LineTo(points[0][0], points[0][1])

	//lastPt := points[0]
	//for _, pt := range points {
	//	if vec4ApproxEq(lastPt, pt) {
	//		continue
	//	}
	//	g.ctx.DrawLine(
	//		lastPt[0], lastPt[1],
	//		pt[0], pt[1],
	//	)
	//	lastPt = pt
	//}
}

func (g *LayerAnimation) RenderAllLayers() {
	//g.ViewTransform = mgl64.Ortho(
	//	g.PyramidSize/2, -g.PyramidSize/2,
	//	0, sierpinski.PyramidNominalHeight*(g.PyramidSize/2),
	//	g.PyramidSize/2, -g.PyramidSize/2,
	//)
	//g.ViewTransform = mgl64.Ident4().Mul4(
	//	mgl64.HomogRotate3DX(-math.Pi / 6)).Mul4(
	//	mgl64.HomogRotate3DY(-math.Pi / 4)).Mul4(
	//	mgl64.Ortho(1, -1, 0, sierpinski.PyramidNominalHeight, 1, -1))
	viewScale := 1.7
	g.ViewTransform = mgl64.Ident4().Mul4(
		//mgl64.Ortho(2, -2, -2, 2, 2, -2)).Mul4(
		mgl64.Ortho(viewScale, -viewScale, -viewScale, viewScale, viewScale, -viewScale)).Mul4(
		//mgl64.HomogRotate3DX(35.264 * (math.Pi / 180))).Mul4(
		mgl64.HomogRotate3DX(30 * (math.Pi / 180))).Mul4(
		mgl64.HomogRotate3DZ(-45 * (math.Pi / 180)))
	//g.ViewTransform = mgl64.Perspective(
	//	120, 1.0, 1, -1,
	//	)

	g.ctx = gg.NewContext(g.ImageSize, g.ImageSize)
	g.ctx.SetColor(color.White)
	g.ctx.DrawRectangle(0, 0, float64(g.ImageSize), float64(g.ImageSize))
	g.ctx.Fill()
	g.ctx.SetColor(color.Black)
	g.ctx.SetLineWidth(1.5)
	for i := 0; i < g.NumLayers; i++ {
		layerNominalHeight := sierpinski.PyramidNominalHeight * float64(i) / float64(g.NumLayers)

		oldCtx := g.ctx
		g.ctx = gg.NewContextForImage(g.ctx.Image())
		//g.ctx.SetColor(color.NRGBA{R: 255, G: 0, B: 0, A: 255})
		g.ctx.SetColor(color.Black)
		g.RenderLayer(layerNominalHeight)
		g.ctx.Stroke()

		outputFileName := fmt.Sprintf(g.OutputFileFormatString, i)
		err := g.ctx.SavePNG(outputFileName)
		die(err)

		g.ctx = oldCtx
		//g.ctx.SetColor(color.Gray{128 + uint8(128*float64(i)/float64(g.NumLayers))})

		//g.ctx.SetColor(colorful.Hsv(360*float64((i*2)%g.NumLayers)/float64(g.NumLayers), 1.0, 1.0))
		g.ctx.SetColor(colorful.Hsv(360*float64(i)/float64(g.NumLayers), 1.0, 1.0))
		g.RenderLayer(layerNominalHeight)
		g.ctx.Stroke()
	}
}

func main() {
	anim := LayerAnimation{
		Order:                  3,
		ImageSize:              500,
		PyramidSize:            500.0,
		PyramidBaseCenter:      mgl64.Vec3{250.0, 250.0},
		OutputFileFormatString: "renders/layer_animation_%05d.png",
		NumLayers:              200,
		//ctx:                    nil,
	}
	anim.RenderAllLayers()

	//points := sierpinski.Sierpinski(3, 0)
	//fmt.Println(points)
	//
	//dc := gg.NewContext(1000, 1000)
	//dc.DrawCircle(500, 500, 400)
	//dc.SetRGB(0, 0, 0)
	//dc.Fill()
	//err := dc.SavePNG("out.png")
	//die(err)
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
