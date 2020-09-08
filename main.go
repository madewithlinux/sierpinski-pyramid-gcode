package main

import (
	"fmt"
	"github.com/go-gl/mathgl/mgl64"
	"io"
	"math"
	"os"
)

var pyramidHeight = math.Sqrt(2)
var threshold = 0.0001

func main() {
	//order := 7
	//scale := 100.0
	order := 3
	// TODO: change it so that scale is the size of the square base of the pyramid, not the radius
	scale := 10.0
	fmt.Printf("; size: %f\n", scale/math.Pow(2, float64(order)))
	speedMmS := 55.0
	bedSize2 := 150.0
	zOffset := 0.4
	fanStartLayer := 3
	relativeExtrusion := false
	// TODO extrusion width, layer height options

	//////////////////////////////////////

	writeStartGcode()

	numLayers := int(scale / 0.2)
	// cross-section area of extrusion line, divided by cross-section area of filament
	extrusionPerLinearMM := 0.2 * 0.4 / (math.Pi * math.Pow(1.75/2, 2))
	extruderPosition := 0.0
	fmt.Println("G21 ; set units to mm")
	if relativeExtrusion {
		fmt.Println("M83 ; set relative extrusion")
	} else {
		fmt.Println("M82 ; set absolute extrusion")
	}
	// TODO: write print time estimates
	// TODO: write filament usage
	// print a line to get the extruder primed
	fmt.Printf("G0 X%f Y%f Z%f\n", bedSize2+scale, bedSize2-scale-10, zOffset)
	fmt.Printf("G1 X%f Y%f E%f F%f\n", bedSize2-scale, bedSize2-scale-10, extrusionPerLinearMM*2*scale, speedMmS*60)
	if !relativeExtrusion {
		extruderPosition += extrusionPerLinearMM * 2 * scale
	}
	fmt.Printf("G1 X%f Y%f E%f F%f\n", bedSize2-scale, bedSize2-scale, extruderPosition+extrusionPerLinearMM*10, speedMmS*60)
	if !relativeExtrusion {
		extruderPosition += extrusionPerLinearMM * 10
	}

	for i := 0; i < numLayers; i++ {

		layerZHeight := pyramidHeight * float64(i) / float64(numLayers)
		points := sierpinski(order, layerZHeight)
		for i := range points {
			points[i] = points[i].Mul(scale).Add(mgl64.Vec3{bedSize2, bedSize2, zOffset})
		}
		writeToGcode(os.Stdout, points, extrusionPerLinearMM, speedMmS, &extruderPosition, relativeExtrusion)
		if i == fanStartLayer {
			fmt.Println("M106 P0 S1")
		}
	}

	writeEndGcode()
}

func writeToGcode(
	writer io.Writer,
	pts []mgl64.Vec3,
	extrusionPerLinearMM, speedMmS float64,
	extruderPosition *float64,
	relativeExtrusion bool,
) {
	// G0 travel move to the first point in the perimeter
	_, _ = fmt.Fprintf(writer, "G1 X%f Y%f Z%f F%f\n", pts[0][0], pts[0][1], pts[0][2], speedMmS*60)

	lastPt := pts[0]
	e := *extruderPosition
	for _, pt := range pts {
		if vec4ApproxEq(lastPt, pt) {
			continue
		}
		dist := pt.Sub(lastPt).Len()
		if relativeExtrusion {
			e = dist * extrusionPerLinearMM
		} else {
			e += dist * extrusionPerLinearMM
		}
		_, _ = fmt.Fprintf(writer, "G1 X%f Y%f Z%f E%f F%f\n", pt[0], pt[1], pt[2], e, speedMmS*60)
		lastPt = pt
	}
	if !vec4ApproxEq(pts[0], lastPt) {
		// if the perimieter doesn't loop, we need to make it loop manually
		pt := pts[0]
		dist := pt.Sub(lastPt).Len()
		if relativeExtrusion {
			e = dist * extrusionPerLinearMM
		} else {
			e += dist * extrusionPerLinearMM
		}
		_, _ = fmt.Fprintf(writer, "G1 X%f Y%f Z%f E%f F%f\n", pt[0], pt[1], pt[2], e, speedMmS*60)
	}
	*extruderPosition = e
}

func writeStartGcode() {
	// start gcode
	fmt.Println("G28 ; home all axes")
	fmt.Println("G1 Z15 ; move extruder up")
	fmt.Println("G1 Y80 X80 F6000 ; move bed out of the way of the first binder clip")
	fmt.Println("M203 X4800 Y4800 ; slow max speed")
	fmt.Println("M566 X150 Y150 Z50 E600 ; Set allowable instantaneous speed change (jerk)")
	fmt.Println("T0 ; first (and only) toolhead")
	fmt.Println("M116")
	fmt.Println("M104 S215 ; set extruder temp")
	fmt.Println("M140 S65 ; set bed temp")
	fmt.Println("M116 ; wait for all temperatures")

	// filament gcode
	fmt.Println("M572 D0 S0.064 ; use very low pressure advance (assume printing at or below 80mm/s")
	fmt.Println("M566 E100.00 ; max instantaneous speed change")
	fmt.Println("M201 E500 ; acceleration")
}

func writeEndGcode() {
	// end gcode
	fmt.Println("G1 E-10 F6000 ; retract filament hopefully just enough to allow cold-changing filament")
	fmt.Println("M104 S0 ; turn off extruder")
	fmt.Println("M140 S0 ; turn off bed")
	fmt.Println("G28 X0 Y0  ; home axes")
	fmt.Println("G1 Y300 F6000 ; move the bed out")
	fmt.Println("M84     ; disable motors")
}

/**
standard sierpinski has base on the [-1,1] square in x and y
and z goes from 0 to sqrt(2)
*/
func sierpinski0(height float64) []mgl64.Vec3 {
	scale := (pyramidHeight - height) / pyramidHeight
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
	if height < 0 || height > pyramidHeight {
		panic("height out of range!")
	}

	if height < pyramidHeight/2 { // bottom half

		// assuming that matrix multiplication makes the transformations happen in reverse order
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
		middleTransform := mgl64.Translate3D(0, 0, pyramidHeight/2).Mul4(
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
		middle := sierpinski(order-1, pyramidHeight-height*2)
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
		middleTransform := mgl64.Translate3D(0, 0, pyramidHeight/2).Mul4(
			mgl64.Scale3D(0.5, 0.5, 0.5))
		//middleTransform := mat4.Ident.Scale(0.5).Translate(&mgl64.Vec3{0, 0, 0.5})
		// middle
		middle := sierpinski(order-1, (height-pyramidHeight/2)*2)
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
	return dist < 0.0001
}
