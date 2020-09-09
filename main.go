package main

import (
	"fmt"
	"github.com/go-gl/mathgl/mgl64"
	"gopkg.in/yaml.v2"
	"io"
	"io/ioutil"
	"math"
	"os"
)

var pyramidNominalHeight = math.Sqrt(2)
var threshold = 0.0001

type GcodeGenerator struct {
	Order             int
	Size              float64
	SpeedMmS          float64
	BedSize           float64
	ZOffset           float64
	FanStartLayer     int
	RelativeExtrusion bool
	ExtrusionWidth    float64
	FilamentDiameter  float64
	LayerHeight       float64
	StartGcode        string
	EndGcode          string
	OutputFilename    string
	/////////////////////////////////////////////
	pyramidZHeight       float64
	numLayers            int
	extrusionPerLinearMM float64
	extruderPosition     float64
	lastToolheadPosition mgl64.Vec3
	bedCenter            mgl64.Vec3
	writer               io.Writer
}

func DefaultGcodeGenerator(inputFilename string) GcodeGenerator {
	return GcodeGenerator{
		SpeedMmS:          40,
		ZOffset:           0,
		FanStartLayer:     3,
		RelativeExtrusion: true,
		ExtrusionWidth:    0.4,
		FilamentDiameter:  1.75,
		LayerHeight:       0.2,
		OutputFilename:    inputFilename + ".gcode",
		// TODO make some reasonable defaults for these
		//StartGcode:
		//EndGcode:

	}
}

func (g *GcodeGenerator) Init() {
	g.pyramidZHeight = pyramidNominalHeight * g.Size / 2
	g.numLayers = int(g.pyramidZHeight / g.LayerHeight)
	g.extrusionPerLinearMM = g.LayerHeight * g.ExtrusionWidth / (math.Pi * math.Pow(g.FilamentDiameter/2, 2))
	g.bedCenter = mgl64.Vec3{g.BedSize / 2, g.BedSize / 2, g.ZOffset}

	if g.OutputFilename == "-" || g.OutputFilename == "stdout" {
		g.writer = os.Stdout
	} else {
		f, err := os.Create(g.OutputFilename)
		die(err)
		g.writer = f
	}
}

func (g *GcodeGenerator) Generate() {
	// TODO: print config settings in the gcode (and to stdout)
	g.writeStartGcode()
	g.writePrimeLine()

	for i := 0; i < g.numLayers; i++ {
		layerNominalHeight := pyramidNominalHeight * float64(i) / float64(g.numLayers)
		layerActualZHeight := g.ZOffset + float64(i)*g.LayerHeight
		points := sierpinski(g.Order, layerNominalHeight)
		for i := range points {
			points[i] = points[i].Mul(g.Size / 2).Add(g.bedCenter)
			// this is to make sure that Z values are always nice round numbers
			points[i][2] = layerActualZHeight
		}
		g.writeLayer(points)
		if i == g.FanStartLayer {
			g.fanOn()
		}
	}
	g.fanOff()
	g.writeEndGcode()
}

func (g *GcodeGenerator) travelTo(pt mgl64.Vec3) {
	if vec4ApproxEq(g.lastToolheadPosition, pt) {
		return
	}
	g.fprintfOrFail("G0 X%f Y%f Z%f F%f\n", pt[0], pt[1], pt[2], g.SpeedMmS*60)
	g.lastToolheadPosition = pt
}

func (g *GcodeGenerator) printToXY(x, y float64) {
	g.printTo(mgl64.Vec3{x, y, g.lastToolheadPosition.Z()})
}
func (g *GcodeGenerator) printTo(pt mgl64.Vec3) {
	if vec4ApproxEq(g.lastToolheadPosition, pt) {
		return
	}
	e := pt.Sub(g.lastToolheadPosition).Len() * g.extrusionPerLinearMM
	if !g.RelativeExtrusion {
		g.extruderPosition += e
		e = g.extruderPosition
	}
	g.fprintfOrFail("G1 X%f Y%f Z%f E%f F%f\n", pt[0], pt[1], pt[2], e, g.SpeedMmS*60)
	g.lastToolheadPosition = pt
}

func (g *GcodeGenerator) writeLayer(pts []mgl64.Vec3) {
	if pts == nil || len(pts) == 0 {
		return
	}

	// yes, we are intentionally using a print move (and not a travel move) to change layers.
	// For this sierpinski pyramid, it looks better that way (at least on my printer)
	for _, pt := range pts {
		g.printTo(pt)
	}
	firstPt := pts[0]
	lastPt := pts[len(pts)-1]
	if !vec4ApproxEq(firstPt, lastPt) {
		g.printTo(firstPt)
	}
}

func (g *GcodeGenerator) fprintlnOrFail(s string) {
	_, err := fmt.Fprintln(g.writer, s)
	die(err)
}

func (g *GcodeGenerator) fprintfOrFail(format string, a ...interface{}) {
	_, err := fmt.Fprintf(g.writer, format, a...)
	die(err)
}

func (g *GcodeGenerator) fanOn() {
	g.fprintlnOrFail("M106 S255")
}

func (g *GcodeGenerator) fanOff() {
	g.fprintlnOrFail("M107")
}

func (g *GcodeGenerator) writeStartGcode() {
	g.fprintlnOrFail(g.StartGcode)
	g.fprintlnOrFail("G21 ; set units to mm")
	if g.RelativeExtrusion {
		g.fprintlnOrFail("M83 ; set relative extrusion")
	} else {
		g.fprintlnOrFail("M82 ; set absolute extrusion")
	}
}

func (g *GcodeGenerator) writeEndGcode() {
	g.fprintlnOrFail(g.EndGcode)
}

func (g *GcodeGenerator) writePrimeLine() {
	// TODO: make a prime line pattern that can work for arbitrary prime size needed
	bedSize2 := g.BedSize / 2
	scale := g.Size / 2
	g.travelTo(mgl64.Vec3{bedSize2 + scale, bedSize2 - scale - 10, g.ZOffset})
	g.printToXY(bedSize2-scale, bedSize2-scale-10)
	g.printToXY(bedSize2-scale, bedSize2-scale)
}

func die(err error) {
	if err != nil {
		panic(err)
	}
}

func main() {
	infile := os.Args[1]
	bytes, err := ioutil.ReadFile(infile)
	die(err)

	g := DefaultGcodeGenerator(infile)
	err = yaml.Unmarshal(bytes, &g)
	die(err)

	g.Init()

	fmt.Printf("%+v\n", g)
	fmt.Printf("; size: %f\n", g.Size/2/math.Pow(2, float64(g.Order)))

	g.Generate()
}

/**
standard sierpinski has base on the [-1,1] square in x and y
and z goes from 0 to sqrt(2)
*/
func sierpinski0(height float64) []mgl64.Vec3 {
	scale := (pyramidNominalHeight - height) / pyramidNominalHeight
	points := []mgl64.Vec3{
		{-scale, -scale, height},
		{scale, -scale, height},
		{scale, scale, height},
		{-scale, scale, height},
	}
	return points
}

func sierpinski(order int, height float64) []mgl64.Vec3 {
	if order == 0 {
		return sierpinski0(height)
	} else if order < 0 {
		panic("order < 0")
	}
	if height < 0 || height > pyramidNominalHeight {
		panic("height out of range!")
	}

	if height < pyramidNominalHeight/2 { // bottom half

		// matrix multiplication makes the transformations happen in reverse order
		// TODO: this stuff would probably be way easier if I had just used 2D points instead of 3D, right? Since I'm
		// always returning points for a particular Z, anyway...
		lowerLeftTransform := mgl64.Translate3D(-0.5, -0.5, 0).Mul4(
			mgl64.Scale3D(0.5, 0.5, 0.5))
		lowerRightTransform := mgl64.Translate3D(0.5, -0.5, 0).Mul4(
			mgl64.Scale3D(0.5, 0.5, 0.5)).Mul4(
			mgl64.HomogRotate3DZ(-math.Pi / 2))
		upperRightTransform := mgl64.Translate3D(0.5, 0.5, 0).Mul4(
			mgl64.Scale3D(0.5, 0.5, 0.5))
		upperLeftTransform := mgl64.Translate3D(-0.5, 0.5, 0).Mul4(
			mgl64.Scale3D(0.5, 0.5, 0.5)).Mul4(
			mgl64.HomogRotate3DZ(math.Pi / 2))
		middleTransform := mgl64.Translate3D(0, 0, pyramidNominalHeight/2).Mul4(
			mgl64.Scale3D(1, 1, -1)).Mul4(
			mgl64.Scale3D(0.5, 0.5, 0.5))

		// lower left
		lowerLeft := sierpinski(order-1, height*2)
		assertMod4Len(lowerLeft)
		transformSlice(lowerLeftTransform, lowerLeft)

		// lower right
		lowerRight := sierpinski(order-1, height*2)
		assertMod4Len(lowerRight)
		transformSlice(lowerRightTransform, lowerRight)

		// upper right
		upperRight := sierpinski(order-1, height*2)
		assertMod4Len(upperRight)
		transformSlice(upperRightTransform, upperRight)

		// upper left
		upperLeft := sierpinski(order-1, height*2)
		assertMod4Len(upperLeft)
		transformSlice(upperLeftTransform, upperLeft)

		// middle
		middle := sierpinski(order-1, pyramidNominalHeight-height*2)
		assertMod4Len(middle)
		transformSlice(middleTransform, middle)

		res := make([]mgl64.Vec3, 0, len(lowerLeft)+len(lowerRight)+len(upperRight)+len(upperLeft)+len(middle))
		res = append(res, lowerLeft[:len(lowerLeft)/2]...)
		res = append(res, middle[:len(middle)/4]...)
		res = append(res, lowerRight...)
		res = append(res, middle[len(middle)/4:len(middle)/2]...)
		res = append(res, upperRight...)
		res = append(res, middle[len(middle)/2:3*len(middle)/4]...)
		res = append(res, upperLeft...)
		res = append(res, middle[3*len(middle)/4:]...)
		res = append(res, lowerLeft[len(lowerLeft)/2:]...)
		return res
	} else { // top half
		middleTransform := mgl64.Translate3D(0, 0, pyramidNominalHeight/2).Mul4(
			mgl64.Scale3D(0.5, 0.5, 0.5))
		//middleTransform := mat4.Ident.Scale(0.5).Translate(&mgl64.Vec3{0, 0, 0.5})
		// middle
		middle := sierpinski(order-1, (height-pyramidNominalHeight/2)*2)
		assertMod4Len(middle)
		transformSlice(middleTransform, middle)
		return middle
	}
}

func transformSlice(mat mgl64.Mat4, pts []mgl64.Vec3) {
	for i := range pts {
		pts[i] = mgl64.TransformCoordinate(pts[i], mat)
	}
}

func assertMod4Len(v []mgl64.Vec3) {
	if len(v)%4 != 0 {
		panic("bad length")
	}
}

func vec4ApproxEq(a, b mgl64.Vec3) bool {
	dist := a.Sub(b).Len()
	return dist < threshold
}
