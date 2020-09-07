package main

import (
	"fmt"
	"github.com/go-gl/mathgl/mgl64"
	"io"
	"math"
	"os"
)

/**
standard sierpinski has base on the [-1,1] square in x and y
and z goes from 0 to 1
*/
func sierpinski0(height float64) []mgl64.Vec3 {
	scale := 1 - height
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
	if height < 0 || height > 1 {
		panic("height out of range!")
	}

	if height < 0.5 { // bottom half

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
		middleTransform := mgl64.Translate3D(0, 0, 0.5).Mul4(
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
		middle := sierpinski(order-1, 1-height*2)
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
		middleTransform := mgl64.Translate3D(0, 0, 0.5).Mul4(
			mgl64.Scale3D(0.5, 0.5, 0.5))
		//middleTransform := mat4.Ident.Scale(0.5).Translate(&mgl64.Vec3{0, 0, 0.5})
		// middle
		middle := sierpinski(order-1, (height-0.5)*2)
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

func writeToGcode(writer io.Writer, pts []mgl64.Vec3, extrusionPerLinearMM, speedMmS float64) {
	e := 0.0
	lastPt := pts[0]
	for _, pt := range pts {
		dist := pt.Sub(lastPt).Len()
		if dist < 0.001 {
			continue
		}
		e = dist * extrusionPerLinearMM
		_, _ = fmt.Fprintf(writer, "G1 X%f Y%f Z%f E%f F%f\n",
			pt[0], pt[1], pt[2],
			e,
			speedMmS*60)
		lastPt = pt
	}
}

func main() {
	order := 3
	scale := 50.0
	numLayers := int(scale / 0.2)
	speedMmS := 35.0
	bedSize2 := 150.0
	zOffset := 0.4

	points := []mgl64.Vec3{}
	for i := 0; i < numLayers; i++ {
		height := float64(i) / float64(numLayers)
		points = append(points, sierpinski(order, height)...)
	}
	for i := range points {
		points[i] = points[i].Mul(scale).Add(mgl64.Vec3{bedSize2, bedSize2, zOffset})
	}

	//fmt.Println("G28 ; home")
	//fmt.Println("G21 ; set units to mm")

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

	// cross-section area of extrusion line, divided by cross-section area of filament
	extrusionPerLinearMM := 0.2 * 0.4 / (math.Pi * math.Pow(1.75/2, 2))
	fmt.Println("G21 ; set units to mm")
	//fmt.Println("M82 ; set absolute extrusion")
	fmt.Println("M83 ; set relative extrusion")
	// print a line to get the extruder primed
	fmt.Printf("G0 X%f Y%f Z%f\n", bedSize2+scale, bedSize2-scale-10, zOffset)
	fmt.Printf("G1 X%f Y%f E%f F%f\n", bedSize2-scale, bedSize2-scale-10, extrusionPerLinearMM*2*scale, speedMmS*60)
	fmt.Printf("G1 X%f Y%f E%f F%f\n", bedSize2-scale, bedSize2-scale, extrusionPerLinearMM*10, speedMmS*60)

	writeToGcode(os.Stdout, points, extrusionPerLinearMM, speedMmS)

	// end gcode
	fmt.Println("G1 E-10 F6000 ; retract filament hopefully just enough to allow cold-changing filament")
	fmt.Println("M104 S0 ; turn off extruder")
	fmt.Println("M140 S0 ; turn off bed")
	fmt.Println("G28 X0 Y0  ; home axes")
	fmt.Println("G1 Y300 F6000 ; move the bed out")
	fmt.Println("M84     ; disable motors")
}
