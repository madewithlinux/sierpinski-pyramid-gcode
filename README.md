# sierpinski pyramid gcode generator

Skip to the [usage/tutorial](#usagetutorial) section if you just want to get printing.

- [sierpinski pyramid gcode generator](#sierpinski-pyramid-gcode-generator)
- [motivation](#motivation)
- [usage/tutorial](#usagetutorial)
- [implementation details (how does the code work?)](#implementation-details-how-does-the-code-work)
- [mathematics explanation](#mathematics-explanation)
- [see also/references](#see-alsoreferences)


# motivation
The sierpinski pyramid is a really cool fractal.
(Technically, it's more correctly referred to as the top-half of a sierpinski octahedron, but whatever)
It is formed by taking a "base case" object, and iterating it through a series of geometric transformations and clone operations.
Each iteration/order shrinks the previous one.
In the limit to infinity, it doesn't matter what shape you started out with, since it has been shrunk down to a point at each position.
(usually, if we're going to render or print it, we limit the number of iterations (also called the order) of the fractal, and we set the "base case" shape to be a simplified shape as the fractal, for consistency)
In the sierpinski pyramid, the base of the pyramid always has `2^i` sections, where `i` is the iteration (or order) of the fractal.

The sierpinski pyramid is uniquely well-suited for 3D printing, because it has a continuous cross section.
This means we can print it in vase mode, with no travel moves or retraction!

This fractal has other properties that make it hard to model in most CAD software and slicers, though.
1. It has very high surface area and very low volume (in the limit, it has infinite surface area and zero volume).
2. The different "mini-pyramids" are connected by infinitely thin sections.
3. The complexity is 6x more with each iteration of the fractal.


There are many 3D models to print of this fractal (see the references section below).
These all make valiant efforts to solve issues (1) and (2) mentioned above, but the issue of total complexity still remains.
**As of this writing, I have yet to find a 3D model of an iteration-7 sierpinski pyramid.**
It's just too complex to practically do.
(Most of these models are built with OpenSCAD, and it just crashes if you try to make an iteration-7 fractal)

So, my solution (implemented in this repository) is to simply skip the CAD and slicer steps! I do the fractal math, and then generate gcode directly from that.


# usage/tutorial
TODO




# implementation details (how does the code work?)
TODO


# mathematics explanation
TODO



# see also/references
* 3d models
  * https://www.thingiverse.com/thing:1356547
  * https://www.thingiverse.com/thing:2573402
  * https://www.thingiverse.com/thing:2613568
* https://duet3d.dozuki.com/Wiki/Gcode
* https://github.com/Duet3D/RepRapFirmware/blob/2.03/src/Storage/FileInfoParser.cpp
