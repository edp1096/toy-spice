package circuit

import (
	"fmt"
	"toy-spice/pkg/device"
	"toy-spice/pkg/matrix"
	"toy-spice/pkg/netlist"
)

type Circuit struct {
	name             string
	nodeMap          map[string]int
	branchMap        map[string]int
	devices          []device.Device
	numNodes         int
	matrix           *matrix.CircuitMatrix
	Status           *device.CircuitStatus
	Time             float64
	timeStep         float64
	isComplex        bool
	prevSolution     map[string]float64
	nonlinearDevices []device.NonLinear
	Models           map[string]device.ModelParam
}

func New(name string) *Circuit {
	return NewWithComplex(name, false)
}

func NewWithComplex(name string, isComplex bool) *Circuit {
	return &Circuit{
		name:         name,
		nodeMap:      make(map[string]int),
		branchMap:    make(map[string]int),
		devices:      make([]device.Device, 0),
		Status:       &device.CircuitStatus{},
		prevSolution: make(map[string]float64),
		isComplex:    isComplex,
		Models:       make(map[string]device.ModelParam),
	}
}

func (c *Circuit) SetModels(models map[string]device.ModelParam) {
	c.Models = models
}

func (c *Circuit) AssignNodeBranchMaps(elements []netlist.Element) error {
	for _, elem := range elements {
		for _, nodeName := range elem.Nodes {
			if nodeName == "0" || nodeName == "gnd" {
				continue
			}
			if _, exists := c.nodeMap[nodeName]; !exists {
				idx := len(c.nodeMap) + 1
				c.nodeMap[nodeName] = idx
			}
		}
	}

	branchStart := len(c.nodeMap) + 1
	for _, elem := range elements {
		if elem.Type == "V" {
			c.branchMap[elem.Name] = branchStart
			branchStart++
		}
	}

	c.numNodes = len(c.nodeMap)
	return nil
}

func (c *Circuit) CreateMatrix() {
	matrixSize := len(c.nodeMap) + len(c.branchMap)
	c.matrix = matrix.NewMatrix(matrixSize, c.isComplex)
}

func (c *Circuit) SetupDevices(elements []netlist.Element) error {
	var err error

	for _, elem := range elements {
		// dev, err := netlist.CreateDevice(elem, c.nodeMap)
		dev, err := netlist.CreateDevice(elem, c.nodeMap, c.Models)
		if err != nil {
			return fmt.Errorf("creating device %s: %v", elem.Name, err)
		}

		// Node index
		nodeIndices := make([]int, len(elem.Nodes))
		for i, nodeName := range elem.Nodes {
			if nodeName == "0" || nodeName == "gnd" {
				nodeIndices[i] = 0
				continue
			}
			nodeIndices[i] = c.nodeMap[nodeName]
		}
		dev.SetNodes(nodeIndices)

		if v, ok := dev.(*device.VoltageSource); ok {
			v.SetBranchIndex(c.branchMap[elem.Name])
		}

		if nl, ok := dev.(device.NonLinear); ok {
			c.nonlinearDevices = append(c.nonlinearDevices, nl)
		}

		c.devices = append(c.devices, dev)
	}

	// Initial stamp
	cktStatus := &device.CircuitStatus{Time: 0}
	err = c.Stamp(cktStatus)
	if err != nil {
		return fmt.Errorf("initial stamping failed: %v", err)
	}
	c.matrix.SetupElements()

	return nil
}

func (c *Circuit) Stamp(status *device.CircuitStatus) error {
	var err error

	for _, dev := range c.devices {
		err = dev.Stamp(c.matrix, status)
		if err != nil {
			return fmt.Errorf("stamping device %s: %v", dev.GetName(), err)
		}
	}
	return nil
}

func (c *Circuit) SetTimeStep(dt float64) {
	c.timeStep = dt
	for _, dev := range c.devices {
		if td, ok := dev.(device.TimeDependent); ok {
			td.SetTimeStep(dt)
		}
	}
}

func (c *Circuit) Update() {
	solution := c.matrix.Solution()

	c.Status = &device.CircuitStatus{
		Time:     c.Time,
		TimeStep: c.timeStep,
		Gmin:     1e-12,
		Mode:     device.TransientAnalysis,
		Temp:     300.15, // 27 = 300.15K
	}

	for _, dev := range c.devices {
		if td, ok := dev.(device.TimeDependent); ok {
			td.SetTimeStep(c.timeStep)
			td.UpdateState(solution, c.Status)
		}
	}
}

func (c *Circuit) GetMatrix() *matrix.CircuitMatrix {
	return c.matrix
}

func (c *Circuit) GetNodeMap() map[string]int {
	return c.nodeMap
}

func (c *Circuit) GetBranchMap() map[string]int {
	return c.branchMap
}

func (c *Circuit) GetDevices() []device.Device {
	return c.devices
}

func (c *Circuit) GetSolution() map[string]float64 {
	solution := make(map[string]float64)
	matrixSolution := c.matrix.Solution()

	// Node voltage
	for name, idx := range c.nodeMap {
		solution[fmt.Sprintf("V(%s)", name)] = matrixSolution[idx]
	}

	// Branch current of voltage source
	for name, idx := range c.branchMap {
		solution[fmt.Sprintf("I(%s)", name)] = -matrixSolution[idx]
	}

	// V = IR -> I = V/R
	for _, dev := range c.devices {
		if dev.GetType() == "R" {
			nodes := dev.GetNodes()
			v1, v2 := 0.0, 0.0
			if nodes[0] > 0 {
				v1 = matrixSolution[nodes[0]]
			}
			if nodes[1] > 0 {
				v2 = matrixSolution[nodes[1]]
			}
			current := (v1 - v2) / dev.GetValue()
			solution[fmt.Sprintf("I(%s)", dev.GetName())] = current
		}
	}

	return solution
}

func (c *Circuit) Destroy() {
	if c.matrix != nil {
		c.matrix.Destroy()
	}
}

func (c *Circuit) Name() string {
	return c.name
}

func (c *Circuit) GetNumNodes() int {
	return c.numNodes
}

func (c *Circuit) GetNodeVoltage(nodeIdx int) float64 {
	if nodeIdx <= 0 { // ground or invalid node
		return 0
	}

	solution := c.matrix.Solution()
	if nodeIdx >= len(solution) {
		return 0
	}

	return solution[nodeIdx]
}

func (c *Circuit) UpdateNonlinearVoltages(solution []float64) error {
	var err error

	for _, dev := range c.nonlinearDevices {
		err = dev.UpdateVoltages(solution)
		if err != nil {
			return fmt.Errorf("updating voltages: %v", err)
		}
	}
	return nil
}
