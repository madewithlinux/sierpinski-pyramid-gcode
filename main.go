package main

import (
	"fmt"
	"github.com/go-gl/mathgl/mgl64"
	"gopkg.in/yaml.v2"
	"io"
	"io/ioutil"
	"math"
	"os"
	"strconv"
	"strings"
)

var Version = "development"
var Rev = "HEAD"

var pyramidNominalHeight = math.Sqrt(2)
var thresholdSqr = 0.0001 * 0.0001

type GcodeGenerator struct {
	Order             int     `yaml:"order"`
	Size              float64 `yaml:"size"`
	SpeedMmS          float64 `yaml:"speed"`
	BedSize           float64 `yaml:"bedSize"`
	ZOffset           float64 `yaml:"zOffset"`
	FanStartLayer     int     `yaml:"fanStartLayer"`
	RelativeExtrusion bool    `yaml:"relativeExtrusion"`
	ExtrusionWidth    float64 `yaml:"extrusionWidth"`
	FilamentDiameter  float64 `yaml:"filamentDiameter"`
	LayerHeight       float64 `yaml:"layerHeight"`
	StartGcode        string  `yaml:"startGcode"`
	EndGcode          string  `yaml:"endGcode"`
	OutputFilename    string  `yaml:"outputFilename"`
	GcodeXYDecimals   int     `yaml:"gcodeXYDecimals"`
	GcodeEDecimals    int     `yaml:"gcodeEDecimals"`
	/////////////////////////////////////////////
	inputFilename        string
	inputFileContent     []byte
	pyramidZHeight       float64
	numLayers            int
	extrusionPerLinearMM float64
	extruderPosition     float64
	lastToolheadPosition mgl64.Vec3
	bedCenter            mgl64.Vec3
	writer               io.WriteCloser
	smallestPyramidSize  float64
	filamentUsedMm       float64
}

func DefaultGcodeGenerator(inputFilename string, inputFileContent []byte) GcodeGenerator {
	return GcodeGenerator{
		SpeedMmS:          40,
		ZOffset:           0,
		FanStartLayer:     3,
		RelativeExtrusion: true,
		ExtrusionWidth:    0.4,
		FilamentDiameter:  1.75,
		LayerHeight:       0.2,
		OutputFilename:    inputFilename + ".gcode",
		inputFilename:     inputFilename,
		inputFileContent:  inputFileContent,
		GcodeXYDecimals:   2,
	}
}

func (g *GcodeGenerator) Init() {
	g.pyramidZHeight = pyramidNominalHeight * g.Size / 2
	g.numLayers = int(g.pyramidZHeight / g.LayerHeight)
	g.extrusionPerLinearMM = g.LayerHeight * g.ExtrusionWidth / (math.Pi * math.Pow(g.FilamentDiameter/2, 2))
	g.bedCenter = mgl64.Vec3{g.BedSize / 2, g.BedSize / 2, g.ZOffset}
	g.smallestPyramidSize = g.Size / math.Pow(2, float64(g.Order))
	if g.GcodeEDecimals == 0 {
		g.GcodeEDecimals = int(math.Ceil(math.Log2(math.Exp2(float64(g.GcodeXYDecimals)) / g.extrusionPerLinearMM)))
	}

	if g.smallestPyramidSize < 5.0*g.ExtrusionWidth {
		fmt.Println("warning: the smallest pyramids are very small in comparison to your extrusion width. consider lowering the fractal order, for a better print")
		fmt.Printf("smallestPyramidSize: %f, extrusionWidth: %f\n", g.smallestPyramidSize, g.ExtrusionWidth)
		fmt.Printf("smallestPyramidSize of %f or larger is recommend, for extrusionWidth: %f (such that smallestPyramidSize >= 5 * extrusionWidth)\n",
			g.ExtrusionWidth*5, g.ExtrusionWidth)
	}

	if g.OutputFilename == "-" || g.OutputFilename == "stdout" {
		g.writer = os.Stdout
	} else {
		f, err := os.Create(g.OutputFilename)
		die(err)
		g.writer = f
	}
}

func (g *GcodeGenerator) generateLayers() (layers [][]mgl64.Vec3) {
	layers = make([][]mgl64.Vec3, g.numLayers)
	for i := 0; i < g.numLayers; i++ {
		layerNominalHeight := pyramidNominalHeight * float64(i) / float64(g.numLayers)
		layerActualZHeight := g.ZOffset + float64(i)*g.LayerHeight
		points := sierpinski(g.Order, layerNominalHeight)
		for i := range points {
			points[i] = points[i].Mul(g.Size / 2).Add(g.bedCenter)
			// this is to make sure that Z values are always nice round numbers
			points[i][2] = layerActualZHeight
		}
		layers[i] = points
	}
	return
}

func (g *GcodeGenerator) calculateFilamentUsed(layers [][]mgl64.Vec3) {
	lastPoint := layers[0][0]
	totalDist := 0.0
	for _, layer := range layers {
		for _, point := range layer {
			totalDist += point.Sub(lastPoint).Len()
			lastPoint = point
		}
	}
	g.filamentUsedMm = totalDist * g.extrusionPerLinearMM
}

func (g *GcodeGenerator) Generate() {
	layers := g.generateLayers()
	g.calculateFilamentUsed(layers)

	g.writeConfig()
	g.writeStartGcode()
	g.writePrimeLine()

	g.fprintlnOrFail("; sierpinski pyramid starts now")
	for i, points := range layers {
		g.writeLayer(points)
		if i == g.FanStartLayer {
			g.fanOn()
		}
	}
	g.fanOff()
	g.writeEndGcode()
}

func (g *GcodeGenerator) Close() error {
	return g.writer.Close()
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
func (g *GcodeGenerator) printToXYMinimal(pt mgl64.Vec3) {
	// this outputs the shortest possible gcode text (to reduce the total gcode file size)
	// and assumes that only X, Y, and E need to be moved. (Z movement is not handled)
	if vec4ApproxEq(g.lastToolheadPosition, pt) {
		return
	}
	e := pt.Sub(g.lastToolheadPosition).Len() * g.extrusionPerLinearMM
	if !g.RelativeExtrusion {
		g.extruderPosition += e
		e = g.extruderPosition
	}
	g.fprintfOrFail("G1 X%s Y%s E%s\n",
		FloatToSmallestString(pt[0], g.GcodeXYDecimals),
		FloatToSmallestString(pt[1], g.GcodeXYDecimals),
		FloatToSmallestString(e, g.GcodeEDecimals),
	)
	g.lastToolheadPosition = pt
}

func (g *GcodeGenerator) writeConfig() {
	// TODO: also print this info to stdout?
	g.fprintlnOrFail("; generated by sierpinski-pyramid-gcode")
	g.fprintlnOrFail("; github.com/madewithlinux/sierpinski-pyramid-gcode")
	g.fprintfOrFail("; version: %s (commit %s)\n", Version, Rev)
	g.fprintlnOrFail("")

	g.fprintlnOrFail("; input config file:")
	for _, line := range strings.Split(string(g.inputFileContent), "\n") {
		g.fprintfOrFail("; %s\n", line)
	}
	g.fprintlnOrFail("")
	g.fprintlnOrFail("; calculated variables")
	g.fprintfOrFail("; pyramidZHeight: %f\n", g.pyramidZHeight)
	g.fprintfOrFail("; numLayers: %d\n", g.numLayers)
	g.fprintfOrFail("; extrusionPerLinearMM: %f\n", g.extrusionPerLinearMM)
	g.fprintfOrFail("; smallestPyramidSize: %f\n", g.smallestPyramidSize)
	g.fprintlnOrFail("")

	// strings used by PrusaSlicer or kisslicer and recognized by RepRapFirmware:
	// (ref https://github.com/Duet3D/RepRapFirmware/blob/2.03/src/Storage/FileInfoParser.cpp)
	g.fprintfOrFail("; estimated printing time (normal mode) = %fs\n", g.filamentUsedMm/g.extrusionPerLinearMM/g.SpeedMmS)
	g.fprintfOrFail("; filament used [mm] = %f\n", g.filamentUsedMm)
	g.fprintfOrFail("; layer_height = %f\n", g.LayerHeight)
	g.fprintfOrFail("; END_LAYER_OBJECT z=%f\n", g.pyramidZHeight)
	g.fprintlnOrFail("")
}

func (g *GcodeGenerator) writeLayer(pts []mgl64.Vec3) {
	if pts == nil || len(pts) == 0 {
		return
	}

	g.printTo(pts[0])

	// yes, we are intentionally using a print move (and not a travel move) to change layers.
	// For this sierpinski pyramid, it looks better that way (at least on my printer)
	for _, pt := range pts {
		g.printToXYMinimal(pt)
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
	g.fprintlnOrFail("; start gcode:")
	g.fprintlnOrFail(g.StartGcode)
	g.fprintlnOrFail("G21 ; set units to mm")
	if g.RelativeExtrusion {
		g.fprintlnOrFail("M83 ; set relative extrusion")
	} else {
		g.fprintlnOrFail("M82 ; set absolute extrusion")
	}
	g.fprintlnOrFail("")
}

func (g *GcodeGenerator) writeEndGcode() {
	g.fprintlnOrFail("")
	g.fprintlnOrFail("; end gcode:")
	g.fprintlnOrFail(g.EndGcode)
	g.fprintlnOrFail("")
}

func (g *GcodeGenerator) writePrimeLine() {
	// TODO: make a prime line pattern that can work for arbitrary prime size needed
	g.fprintlnOrFail("; prime the nozzle")
	bedSize2 := g.BedSize / 2
	scale := g.Size / 2
	g.travelTo(mgl64.Vec3{bedSize2 + scale, bedSize2 - scale - 10, g.ZOffset})
	g.printToXY(bedSize2-scale, bedSize2-scale-10)
	g.printToXY(bedSize2-scale, bedSize2-scale)
	g.fprintlnOrFail("")
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

	g := DefaultGcodeGenerator(infile, bytes)
	err = yaml.UnmarshalStrict(bytes, &g)
	die(err)

	g.Init()
	g.Generate()
	die(g.Close())
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
	dist := a.Sub(b).LenSqr()
	return dist < thresholdSqr
}

func FloatToSmallestString(f float64, decimals int) string {
	s := strconv.FormatFloat(f, 'f', decimals, 64)
	if !strings.Contains(s, ".") {
		return s
	}
	// this is faster than using strings.TrimRight()
	if s[0] == '0' {
		s = s[1:]
	}
	for s[len(s)-1] == '0' {
		s = s[:len(s)-1]
	}
	if s[len(s)-1] == '.' {
		s = s[:len(s)-1]
	}
	if len(s) == 0 {
		// special case, to make sure we still have a number after all this
		return "0"
	}
	return s
}
