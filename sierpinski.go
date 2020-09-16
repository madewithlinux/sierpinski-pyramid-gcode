package sierpinski

import (
	"github.com/go-gl/mathgl/mgl64"
	"math"
)

var PyramidNominalHeight = math.Sqrt(2)

/**
standard sierpinski has base on the [-1,1] square in x and y
and z goes from 0 to sqrt(2)
*/
func sierpinski0(height float64) []mgl64.Vec3 {
	scale := (PyramidNominalHeight - height) / PyramidNominalHeight
	points := []mgl64.Vec3{
		{-scale, -scale, height},
		{scale, -scale, height},
		{scale, scale, height},
		{-scale, scale, height},
	}
	return points
}

func SierpinskiHiddenness(order int, height float64) []int {
	if order == 0 {
		return []int{0, 0, 0, 0}
	} else if order < 0 {
		panic("order < 0")
	}
	if height < 0 || height > PyramidNominalHeight {
		panic("height out of range!")
	}

	if height < PyramidNominalHeight/2 { // bottom half
		lowerLeft := SierpinskiHiddenness(order-1, height*2)
		lowerRight := SierpinskiHiddenness(order-1, height*2)
		upperRight := SierpinskiHiddenness(order-1, height*2)
		upperLeft := SierpinskiHiddenness(order-1, height*2)
		middle := SierpinskiHiddenness(order-1, PyramidNominalHeight-height*2)

		for i := len(lowerLeft) / 4; i < 3*len(lowerLeft)/4; i++ {
			lowerLeft[i]++
		}

		for i := 0; i < len(lowerRight)/4; i++ {
			lowerRight[i] = lowerRight[i] + 1
			upperRight[i] = upperRight[i] + 1
			upperLeft[i] = upperLeft[i] + 1
		}
		for i := 3 * len(lowerRight) / 4; i < len(lowerRight); i++ {
			lowerRight[i] = lowerRight[i] + 1
			upperRight[i] = upperRight[i] + 1
			upperLeft[i] = upperLeft[i] + 1
		}

		res := make([]int, 0, len(lowerLeft)+len(lowerRight)+len(upperRight)+len(upperLeft)+len(middle))
		res = append(res, lowerLeft[:len(lowerLeft)/2]...)
		res = append(res, middle[:len(middle)/4]...)
		res = append(res, lowerRight...)
		res = append(res, middle[len(middle)/4:len(middle)/2]...)
		res = append(res, upperRight...)
		res = append(res, middle[len(middle)/2:3*len(middle)/4]...)
		res = append(res, upperLeft...)
		res = append(res, middle[3*len(middle)/4:]...)
		res = append(res, lowerLeft[len(lowerLeft)/2:]...)
		if len(res) != len(lowerLeft)+len(lowerRight)+len(upperRight)+len(upperLeft)+len(middle) {
			panic("that's a problem")
		}
		return res
	} else { // top half
		middle := SierpinskiHiddenness(order-1, (height-PyramidNominalHeight/2)*2)
		return middle
	}
}

func Sierpinski(order int, height float64) []mgl64.Vec3 {
	if order == 0 {
		return sierpinski0(height)
	} else if order < 0 {
		panic("order < 0")
	}
	if height < 0 || height > PyramidNominalHeight {
		panic("height out of range!")
	}

	if height < PyramidNominalHeight/2 { // bottom half

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
		middleTransform := mgl64.Translate3D(0, 0, PyramidNominalHeight/2).Mul4(
			mgl64.Scale3D(1, 1, -1)).Mul4(
			mgl64.Scale3D(0.5, 0.5, 0.5))

		// lower left
		lowerLeft := Sierpinski(order-1, height*2)
		assertMod4Len(lowerLeft)
		transformSlice(lowerLeftTransform, lowerLeft)

		// lower right
		lowerRight := Sierpinski(order-1, height*2)
		assertMod4Len(lowerRight)
		transformSlice(lowerRightTransform, lowerRight)

		// upper right
		upperRight := Sierpinski(order-1, height*2)
		assertMod4Len(upperRight)
		transformSlice(upperRightTransform, upperRight)

		// upper left
		upperLeft := Sierpinski(order-1, height*2)
		assertMod4Len(upperLeft)
		transformSlice(upperLeftTransform, upperLeft)

		// middle
		middle := Sierpinski(order-1, PyramidNominalHeight-height*2)
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
		middleTransform := mgl64.Translate3D(0, 0, PyramidNominalHeight/2).Mul4(
			mgl64.Scale3D(0.5, 0.5, 0.5))
		//middleTransform := mat4.Ident.Scale(0.5).Translate(&mgl64.Vec3{0, 0, 0.5})
		// middle
		middle := Sierpinski(order-1, (height-PyramidNominalHeight/2)*2)
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
