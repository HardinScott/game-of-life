package main

import (
	"fmt"
	"github.com/go-gl/gl/v4.1-core/gl"
	"github.com/go-gl/glfw/v3.3/glfw"
	"math/rand"
	"runtime"
	"strings"
	"time"
)

const (
	xBound             = 1280
	yBound             = 720
	vertexShaderSource = `
    #version 410
    in vec3 vp;
    void main() {
        gl_Position = vec4(vp, 1.0);
    }
` + "\x00"
	fragmentShaderSource = `
    #version 410
    out vec4 frag_colour;
    void main() {
        frag_colour = vec4(0.5, 0.7, 0.1, 1);
    }
` + "\x00"
	rows         = 200
	columns      = 200
	chanceToLive = 0.12
	fps          = 60
)

var (
	square = []float32{
		-0.5, 0.5, 0,
		-0.5, -0.5, 0,
		0.5, -0.5, 0,

		-0.5, 0.5, 0,
		0.5, 0.5, 0,
		0.5, -0.5, 0,
	}
	mainWindow *glfw.Window
	cells      [][]*cell
	program    uint32
)

type cell struct {
	drawable      uint32
	alive         bool
	aliveNextTurn bool
	x             int
	y             int
}

func main() {
	runtime.LockOSThread()

	mainWindow = glfwInit()
	defer glfw.Terminate()

	//mainWindow.SetMouseButtonCallback(mouseClick)

	program = initOpenGL()
	cells = createCells()

	for !mainWindow.ShouldClose() {
		t := time.Now()
		draw()
		time.Sleep(time.Second/time.Duration(fps) - time.Since(t))
	}
}

//func mouseClick(window *glfw.Window, button glfw.MouseButton, action glfw.Action, mods glfw.ModifierKey) {
//TODO: make an init state where you pick which cells should be alive by clicking on them
//}

func glfwInit() *glfw.Window {
	err := glfw.Init()
	if err != nil {
		return nil
	}
	glfw.WindowHint(glfw.ContextVersionMajor, 4)
	glfw.WindowHint(glfw.OpenGLProfile, glfw.OpenGLCoreProfile)
	glfw.WindowHint(glfw.OpenGLForwardCompatible, glfw.True)

	window, _ := glfw.CreateWindow(xBound, yBound, "Game of Life", nil, nil)

	window.MakeContextCurrent()

	return window
}

func initOpenGL() uint32 {
	if err := gl.Init(); err != nil {
		panic(any(err))
	}

	vertexShader, err := compileShader(vertexShaderSource, gl.VERTEX_SHADER)
	if err != nil {
		panic(any(err))
	}
	fragmentShader, err := compileShader(fragmentShaderSource, gl.FRAGMENT_SHADER)
	if err != nil {
		panic(any(err))
	}

	program := gl.CreateProgram()
	gl.AttachShader(program, vertexShader)
	gl.AttachShader(program, fragmentShader)
	gl.LinkProgram(program)
	return program
}

func draw() {
	gl.Clear(gl.COLOR_BUFFER_BIT | gl.DEPTH_BUFFER_BIT)
	gl.UseProgram(program)
	go func() {
		for i := range cells {
			for _, c := range cells[i] {
				c.checkState(cells)
			}
		}
	}()
	for i := range cells {
		for _, c := range cells[i] {
			c.draw()
		}
	}
	mainWindow.SwapBuffers()
	glfw.PollEvents()
}

func createVAO(matrix []float32) uint32 {
	var vbo uint32
	var vao uint32

	gl.GenBuffers(1, &vbo)
	gl.BindBuffer(gl.ARRAY_BUFFER, vbo)
	gl.BufferData(gl.ARRAY_BUFFER, 4*len(matrix), gl.Ptr(matrix), gl.STATIC_DRAW)

	gl.GenVertexArrays(1, &vao)
	gl.BindVertexArray(vao)
	gl.EnableVertexAttribArray(0)
	gl.BindBuffer(gl.ARRAY_BUFFER, vbo)
	gl.VertexAttribPointer(0, 3, gl.FLOAT, false, 0, nil)

	return vao
}

func compileShader(source string, shaderType uint32) (uint32, error) {
	shader := gl.CreateShader(shaderType)

	csources, free := gl.Strs(source)
	gl.ShaderSource(shader, 1, csources, nil)
	free()
	gl.CompileShader(shader)

	var status int32
	gl.GetShaderiv(shader, gl.COMPILE_STATUS, &status)
	if status == gl.FALSE {
		var logLength int32
		gl.GetShaderiv(shader, gl.INFO_LOG_LENGTH, &logLength)

		log := strings.Repeat("\x00", int(logLength+1))
		gl.GetShaderInfoLog(shader, logLength, nil, gl.Str(log))

		return 0, fmt.Errorf("failed to compile %v: %v", source, log)
	}

	return shader, nil
}

func createCells() [][]*cell {
	rand.Seed(time.Now().UnixNano())

	cells := make([][]*cell, rows, rows)
	for x := 0; x < rows; x++ {
		for y := 0; y < columns; y++ {
			c := createCell(x, y)
			c.alive = rand.Float64() < chanceToLive
			c.aliveNextTurn = c.alive

			cells[x] = append(cells[x], c)
		}
	}

	return cells
}

func createCell(x, y int) *cell {
	points := make([]float32, len(square), len(square))
	copy(points, square)

	for i := 0; i < len(points); i++ {
		var position float32
		var size float32
		switch i % 3 {
		case 0:
			size = 1.0 / float32(columns)
			position = float32(x) * size
		case 1:
			size = 1.0 / float32(rows)
			position = float32(y) * size
		default:
			continue
		}

		if points[i] < 0 {
			points[i] = (position * 2) - 1
		} else {
			points[i] = ((position + size) * 2) - 1
		}
	}

	return &cell{
		drawable: createVAO(points),
		x:        x,
		y:        y,
	}
}

func (c *cell) draw() {
	if !c.alive {
		return
	}
	gl.BindVertexArray(c.drawable)
	gl.DrawArrays(gl.TRIANGLES, 0, int32(len(square)/3))
}

func (c *cell) checkState(cells [][]*cell) {
	c.alive = c.aliveNextTurn
	c.aliveNextTurn = c.alive

	liveCount := c.liveNeighbors(cells)
	if c.alive {
		if liveCount < 2 {
			c.aliveNextTurn = false
		}

		if liveCount == 2 || liveCount == 3 {
			c.aliveNextTurn = true
		}

		if liveCount > 3 {
			c.aliveNextTurn = false
		}
	} else {
		if liveCount == 3 {
			c.aliveNextTurn = true
		}
	}
}

func (c *cell) liveNeighbors(cells [][]*cell) int {
	var liveNeighbors int
	count := func(x, y int) {
		if x == len(cells) {
			x = 0
		} else if x == -1 {
			x = len(cells) - 1
		}
		if y == len(cells[x]) {
			y = 0
		} else if y == -1 {
			y = len(cells[x]) - 1
		}

		if cells[x][y].alive {
			liveNeighbors++
		}
	}

	count(c.x-1, c.y)
	count(c.x+1, c.y)
	count(c.x, c.y+1)
	count(c.x, c.y-1)
	count(c.x-1, c.y+1)
	count(c.x+1, c.y+1)
	count(c.x-1, c.y-1)
	count(c.x+1, c.y-1)

	return liveNeighbors
}
