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
var threshold = 0.001
var thresholdSqr = threshold * threshold

type GcodeGenerator struct {
	Order                    int     `yaml:"order"`
	Size                     float64 `yaml:"size"`
	SpeedMmS                 float64 `yaml:"speed"`
	BedSize                  float64 `yaml:"bedSize"`
	ZOffset                  float64 `yaml:"zOffset"`
	FanStartLayer            int     `yaml:"fanStartLayer"`
	RelativeExtrusion        bool    `yaml:"relativeExtrusion"`
	ExtrusionWidth           float64 `yaml:"extrusionWidth"`
	FirstLayerExtrusionWidth float64 `yaml:"firstLayerExtrusionWidth"`
	FilamentDiameter         float64 `yaml:"filamentDiameter"`
	LayerHeight              float64 `yaml:"layerHeight"`
	StartGcode               string  `yaml:"startGcode"`
	EndGcode                 string  `yaml:"endGcode"`
	OutputFilename           string  `yaml:"outputFilename"`
	GcodeXYDecimals          int     `yaml:"gcodeXYDecimals"`
	GcodeEDecimals           int     `yaml:"gcodeEDecimals"`
	PrimeFilamentLength      float64 `yaml:"primeFilamentLength"`
	Octahedron               bool    `yaml:"octahedron"`
	SupportFinHeight         float64 `yaml:"supportFinHeight"`
	SupportExtrusionFactor   float64 `yaml:"supportExtrusionFactor"`
	/////////////////////////////////////////////
	inputFilename                  string
	inputFileContent               []byte
	pyramidZHeight                 float64
	numLayers                      int
	extrusionPerLinearMM           float64
	firstLayerExtrusionPerLinearMM float64
	extruderPosition               float64
	lastToolheadPosition           mgl64.Vec3
	bedCenter                      mgl64.Vec3
	writer                         io.WriteCloser
	smallestPyramidSize            float64
	filamentUsedMm                 float64
	xyMin                          float64
	xyMax                          float64
}

func DefaultGcodeGenerator(inputFilename string, inputFileContent []byte) GcodeGenerator {
	return GcodeGenerator{
		SpeedMmS:               40,
		ZOffset:                0,
		FanStartLayer:          3,
		RelativeExtrusion:      true,
		ExtrusionWidth:         0.4,
		FilamentDiameter:       1.75,
		LayerHeight:            0.2,
		OutputFilename:         inputFilename + ".gcode",
		inputFilename:          inputFilename,
		inputFileContent:       inputFileContent,
		GcodeXYDecimals:        2,
		PrimeFilamentLength:    10,
		Octahedron:             false,
		SupportExtrusionFactor: 0.25,
	}
}

func (g *GcodeGenerator) Init() {
	g.pyramidZHeight = pyramidNominalHeight * g.Size / 2
	g.numLayers = int(g.pyramidZHeight / g.LayerHeight)
	g.extrusionPerLinearMM = g.LayerHeight * g.ExtrusionWidth / (math.Pi * math.Pow(g.FilamentDiameter/2, 2))
	g.bedCenter = mgl64.Vec3{g.BedSize / 2, g.BedSize / 2, g.ZOffset}
	g.xyMin = g.BedSize/2 - g.Size/2
	g.xyMax = g.BedSize/2 + g.Size/2
	g.smallestPyramidSize = g.Size / math.Pow(2, float64(g.Order))
	if g.GcodeEDecimals == 0 {
		g.GcodeEDecimals = int(math.Ceil(math.Log2(math.Exp2(float64(g.GcodeXYDecimals)) / g.extrusionPerLinearMM)))
		if !g.RelativeExtrusion {
			g.GcodeEDecimals += g.GcodeXYDecimals
		}
	}
	if g.FirstLayerExtrusionWidth == 0 {
		g.FirstLayerExtrusionWidth = g.ExtrusionWidth * 1.5
	}
	g.firstLayerExtrusionPerLinearMM = g.LayerHeight * g.FirstLayerExtrusionWidth / (math.Pi * math.Pow(g.FilamentDiameter/2, 2))
	if g.SupportFinHeight == 0 {
		g.SupportFinHeight = math.Min(g.pyramidZHeight-20*g.LayerHeight, g.pyramidZHeight*0.85)
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
	g.filamentUsedMm = totalDist*g.extrusionPerLinearMM + g.PrimeFilamentLength
}

func (g *GcodeGenerator) Generate() {
	layers := g.generateLayers()
	g.calculateFilamentUsed(layers)

	g.writeConfig()
	g.writeStartGcode()
	g.writePrimeLine()

	g.fprintlnOrFail("; sierpinski pyramid starts now")
	if g.Octahedron {
		// print a raft/brim type thing, to print the subsequent support fins on
		numRaftSections := int(math.Ceil(g.Size / g.FirstLayerExtrusionWidth))
		for i := 0; i < (numRaftSections+1)/2; i++ {
			g.printToXY(g.xyMin, g.xyMin+float64(2*i)*g.FirstLayerExtrusionWidth)
			g.printToXY(g.xyMax, g.xyMin+float64(2*i)*g.FirstLayerExtrusionWidth)
			g.printToXY(g.xyMax, g.xyMin+float64(2*i+1)*g.FirstLayerExtrusionWidth)
			g.printToXY(g.xyMin, g.xyMin+float64(2*i+1)*g.FirstLayerExtrusionWidth)
		}
		// return to the start of the print
		g.printToXY(g.xyMin, g.xyMax+g.FirstLayerExtrusionWidth)
		g.printToXY(g.xyMin-g.FirstLayerExtrusionWidth, g.xyMax+g.FirstLayerExtrusionWidth)
		g.printToXY(g.xyMin-g.FirstLayerExtrusionWidth, g.xyMin)
		g.printTo(mgl64.Vec3{g.xyMin, g.xyMin, g.ZOffset + g.LayerHeight})

		// now all subsequent prints must be shifted one layer higher
		for i := range layers {
			points := layers[len(layers)-i-1]
			zOverride := layers[i][0][2] + g.LayerHeight
			g.writeLayerWithSupportFins(points, zOverride, layers[i][0][2] < g.SupportFinHeight)
			if i == g.FanStartLayer {
				g.fanOn()
			}
		}
		for i, points := range layers {
			zOverride := points[0][2] + layers[len(layers)-1][0][2] - g.ZOffset + g.LayerHeight
			g.writeLayerWithSupportFins(points, zOverride, false)
			if i == g.FanStartLayer {
				g.fanOn()
			}
		}
	} else {
		for i, points := range layers {
			g.writeLayer(points)
			if i == g.FanStartLayer {
				g.fanOn()
			}
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
	extrusionPerLinearMM := g.extrusionPerLinearMM
	speedMmS := g.SpeedMmS
	if pt[2] <= (g.ZOffset+g.LayerHeight)*(threshold+1) {
		extrusionPerLinearMM = g.firstLayerExtrusionPerLinearMM
		speedMmS /= 2
	}
	g.printToAdvanced(pt, speedMmS, extrusionPerLinearMM)
}
func (g *GcodeGenerator) printToAdvanced(pt mgl64.Vec3, speedMmS float64, extrusionPerLinearMM float64) {
	e := pt.Sub(g.lastToolheadPosition).Len() * extrusionPerLinearMM
	feedrate := speedMmS * 60
	if !g.RelativeExtrusion {
		g.extruderPosition += e
		e = g.extruderPosition
	}
	g.fprintfOrFail("G1 X%f Y%f Z%f E%f F%f\n", pt[0], pt[1], pt[2], e, feedrate)
	g.lastToolheadPosition = pt
}
func (g *GcodeGenerator) printToXYMinimal(pt mgl64.Vec3) {
	// this outputs the shortest possible gcode text (to reduce the total gcode file size)
	// and assumes that only X, Y, and E need to be moved. (Z movement is not handled)
	if vec4ApproxEq(g.lastToolheadPosition, pt) {
		return
	}
	e := pt.Sub(g.lastToolheadPosition).Len() * g.extrusionPerLinearMM
	if pt[2] <= (g.ZOffset+g.LayerHeight)*(threshold+1) {
		e = pt.Sub(g.lastToolheadPosition).Len() * g.firstLayerExtrusionPerLinearMM
	}
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

func (g *GcodeGenerator) writeLayerWithSupportFins(pts []mgl64.Vec3, zOverride float64, supportFins bool) {
	if pts == nil || len(pts) == 0 {
		return
	}

	firstPt := pts[0]
	lastPt := pts[len(pts)-1]
	firstPt[2] = zOverride
	lastPt[2] = zOverride

	finOffset := g.ExtrusionWidth * 4
	finExtrusionPerLinearMM := g.extrusionPerLinearMM * g.SupportExtrusionFactor
	finSpeedMmS := g.SpeedMmS * 2

	if supportFins {
		if len(pts) == 4 {
			g.printToAdvanced(mgl64.Vec3{g.xyMin + finOffset, g.xyMin, zOverride}, finSpeedMmS, finExtrusionPerLinearMM)
			g.printToAdvanced(mgl64.Vec3{g.xyMin, g.xyMin + finOffset, zOverride}, finSpeedMmS, finExtrusionPerLinearMM)
			g.printToAdvanced(mgl64.Vec3{firstPt[0] - finOffset, firstPt[1], zOverride}, finSpeedMmS, finExtrusionPerLinearMM)
		} else {
			g.printToAdvanced(mgl64.Vec3{g.xyMin + finOffset, g.xyMin, zOverride}, finSpeedMmS, finExtrusionPerLinearMM)
			g.printToAdvanced(mgl64.Vec3{firstPt[0], firstPt[1] - finOffset, zOverride}, finSpeedMmS, finExtrusionPerLinearMM)
		}
		g.printToAdvanced(mgl64.Vec3{firstPt[0], firstPt[1], zOverride}, finSpeedMmS, finExtrusionPerLinearMM)
		g.printToAdvanced(mgl64.Vec3{firstPt[0], firstPt[1], zOverride}, g.SpeedMmS, g.extrusionPerLinearMM)
	} else {
		// yes, we are intentionally using a print move (and not a travel move) to change layers.
		// For this sierpinski pyramid, it looks better that way (at least on my printer)
		g.printTo(mgl64.Vec3{firstPt[0], firstPt[1], zOverride})
	}
	for i, pt := range pts {
		pt[2] = zOverride
		// move is a x move if y axis does not move. and vice versa
		xMove := math.Abs(pt[1]-g.lastToolheadPosition[1]) < threshold
		yMove := math.Abs(pt[0]-g.lastToolheadPosition[0]) < threshold
		g.printToXYMinimal(pt)
		if supportFins && (i%(len(pts)/4)) == 0 && i > 0 {
			if i == len(pts)/4 { // lower right fin
				// in each of these cases, if the move that ends the layer isn't the kind of move that we expect, we just
				// have to reverse the direction that we print the fin
				if yMove {
					g.printToAdvanced(mgl64.Vec3{pt[0], pt[1] - finOffset, zOverride}, finSpeedMmS, finExtrusionPerLinearMM)
					g.printToAdvanced(mgl64.Vec3{g.xyMax - finOffset, g.xyMin, zOverride}, finSpeedMmS, finExtrusionPerLinearMM)
					g.printToAdvanced(mgl64.Vec3{g.xyMax, g.xyMin + finOffset, zOverride}, finSpeedMmS, finExtrusionPerLinearMM)
					g.printToAdvanced(mgl64.Vec3{pt[0] + finOffset, pt[1], zOverride}, finSpeedMmS, finExtrusionPerLinearMM)
				} else {
					g.printToAdvanced(mgl64.Vec3{pt[0] + finOffset, pt[1], zOverride}, finSpeedMmS, finExtrusionPerLinearMM)
					g.printToAdvanced(mgl64.Vec3{g.xyMax, g.xyMin + finOffset, zOverride}, finSpeedMmS, finExtrusionPerLinearMM)
					g.printToAdvanced(mgl64.Vec3{g.xyMax - finOffset, g.xyMin, zOverride}, finSpeedMmS, finExtrusionPerLinearMM)
					g.printToAdvanced(mgl64.Vec3{pt[0], pt[1] - finOffset, zOverride}, finSpeedMmS, finExtrusionPerLinearMM)
				}
			} else if i == len(pts)/2 { // upper right fin
				if xMove {
					g.printToAdvanced(mgl64.Vec3{pt[0] + finOffset, pt[1], zOverride}, finSpeedMmS, finExtrusionPerLinearMM)
					g.printToAdvanced(mgl64.Vec3{g.xyMax, g.xyMax - finOffset, zOverride}, finSpeedMmS, finExtrusionPerLinearMM)
					g.printToAdvanced(mgl64.Vec3{g.xyMax - finOffset, g.xyMax, zOverride}, finSpeedMmS, finExtrusionPerLinearMM)
					g.printToAdvanced(mgl64.Vec3{pt[0], pt[1] + finOffset, zOverride}, finSpeedMmS, finExtrusionPerLinearMM)
				} else {
					g.printToAdvanced(mgl64.Vec3{pt[0], pt[1] + finOffset, zOverride}, finSpeedMmS, finExtrusionPerLinearMM)
					g.printToAdvanced(mgl64.Vec3{g.xyMax - finOffset, g.xyMax, zOverride}, finSpeedMmS, finExtrusionPerLinearMM)
					g.printToAdvanced(mgl64.Vec3{g.xyMax, g.xyMax - finOffset, zOverride}, finSpeedMmS, finExtrusionPerLinearMM)
					g.printToAdvanced(mgl64.Vec3{pt[0] + finOffset, pt[1], zOverride}, finSpeedMmS, finExtrusionPerLinearMM)
				}
			} else if i == 3*len(pts)/4 { // upper left fin
				if yMove {
					g.printToAdvanced(mgl64.Vec3{pt[0], pt[1] + finOffset, zOverride}, finSpeedMmS, finExtrusionPerLinearMM)
					g.printToAdvanced(mgl64.Vec3{g.xyMin + finOffset, g.xyMax, zOverride}, finSpeedMmS, finExtrusionPerLinearMM)
					g.printToAdvanced(mgl64.Vec3{g.xyMin, g.xyMax - finOffset, zOverride}, finSpeedMmS, finExtrusionPerLinearMM)
					g.printToAdvanced(mgl64.Vec3{pt[0] - finOffset, pt[1], zOverride}, finSpeedMmS, finExtrusionPerLinearMM)
				} else {
					g.printToAdvanced(mgl64.Vec3{pt[0] - finOffset, pt[1], zOverride}, finSpeedMmS, finExtrusionPerLinearMM)
					g.printToAdvanced(mgl64.Vec3{g.xyMin, g.xyMax - finOffset, zOverride}, finSpeedMmS, finExtrusionPerLinearMM)
					g.printToAdvanced(mgl64.Vec3{g.xyMin + finOffset, g.xyMax, zOverride}, finSpeedMmS, finExtrusionPerLinearMM)
					g.printToAdvanced(mgl64.Vec3{pt[0], pt[1] + finOffset, zOverride}, finSpeedMmS, finExtrusionPerLinearMM)
				}
			}
			// lower left fin is handled before/after this loop, as it starts and ends the layer
			g.printToAdvanced(pt, finSpeedMmS, finExtrusionPerLinearMM)
			g.printToAdvanced(pt, g.SpeedMmS, g.extrusionPerLinearMM)
		}
	}
	if !vec4ApproxEq(firstPt, lastPt) {
		g.printTo(firstPt)
	}
	if supportFins {
		if len(pts) == 4 {
			// special case for base-case layers, which need fins to be printed in the opposite direction
			g.printToAdvanced(mgl64.Vec3{firstPt[0], firstPt[1] - finOffset, zOverride}, finSpeedMmS, finExtrusionPerLinearMM)
			g.printToAdvanced(mgl64.Vec3{g.xyMin + finOffset, g.xyMin, zOverride}, finSpeedMmS, finExtrusionPerLinearMM)
		} else {
			g.printToAdvanced(mgl64.Vec3{firstPt[0] - finOffset, firstPt[1], zOverride}, finSpeedMmS, finExtrusionPerLinearMM)
			g.printToAdvanced(mgl64.Vec3{g.xyMin, g.xyMin + finOffset, zOverride}, finSpeedMmS, finExtrusionPerLinearMM)
			g.printToAdvanced(mgl64.Vec3{g.xyMin + finOffset, g.xyMin, zOverride}, finSpeedMmS, finExtrusionPerLinearMM)
		}
	}
}

func (g *GcodeGenerator) writeLayer(pts []mgl64.Vec3) {
	if pts == nil || len(pts) == 0 {
		return
	}

	// yes, we are intentionally using a print move (and not a travel move) to change layers.
	// For this sierpinski pyramid, it looks better that way (at least on my printer)
	g.printTo(pts[0])

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
	g.fprintlnOrFail("; prime the nozzle")
	bedSize2 := g.BedSize / 2
	scale := g.Size / 2
	numPrimeLines := int(math.Ceil(g.PrimeFilamentLength / g.firstLayerExtrusionPerLinearMM / g.Size))
	if numPrimeLines%2 == 1 {
		numPrimeLines += 1
	}
	primeLineSeparation := g.FirstLayerExtrusionWidth * 2
	distFromObject := math.Max(5.0, 2*primeLineSeparation)

	lineMinX := bedSize2 - scale
	lineMaxX := bedSize2 + scale
	linesStartMinY := bedSize2 - scale - distFromObject - primeLineSeparation*float64(numPrimeLines)

	g.travelTo(mgl64.Vec3{lineMinX, linesStartMinY, g.ZOffset})
	for i := 0; i < numPrimeLines/2; i++ {
		/*
			print the lines two-at-a-time, in this shape:
			|
			|__________________________
			__________________________|
			|
			|
		*/
		g.printToXY(lineMinX, linesStartMinY+float64(2*i)*primeLineSeparation)
		g.printToXY(lineMaxX, linesStartMinY+float64(2*i)*primeLineSeparation)
		g.printToXY(lineMaxX, linesStartMinY+float64(2*i+1)*primeLineSeparation)
		g.printToXY(lineMinX, linesStartMinY+float64(2*i+1)*primeLineSeparation)
	}
	// finally, go to the start of the print
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

var flipXYmat4 = [16]float64{
	0, 1, 0, 0,
	1, 0, 0, 0,
	0, 0, 1, 0,
	0, 0, 0, 1,
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
		if order == 1 {
			lowerLeftTransform = lowerLeftTransform.Mul4(flipXYmat4)
			lowerRightTransform = lowerRightTransform.Mul4(flipXYmat4)
			upperRightTransform = upperRightTransform.Mul4(flipXYmat4)
			upperLeftTransform = upperLeftTransform.Mul4(flipXYmat4)
		}

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

	// 3d printers should be able to interpret less-than-1 numbers without leading zeros (like ".01" instead of "0.01")
	// RepRapFirmware should be fine (ref. https://github.com/Duet3D/RRFLibraries/blob/master/src/General/SafeStrtod.cpp)
	// Marlin should be fine, too: https://github.com/MarlinFirmware/Marlin/blob/2.0.x/Marlin/src/gcode/parser.h#L248
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
