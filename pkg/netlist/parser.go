package netlist

import (
	"bufio"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"toy-spice/pkg/device"
)

type AnalysisType int

const (
	AnalysisOP AnalysisType = iota
	AnalysisTRAN
	AnalysisAC
)

type Circuit struct {
	Elements  []Element      // Circuit elements
	Nodes     map[string]int // Node name and index
	Analysis  AnalysisType   // Analysis type
	TranParam struct {
		TStep  float64 // timestep
		TStop  float64 // stop time
		TStart float64 // start time
		TMax   float64 // max timestep
		UIC    bool    // Use Initial Conditions
	}
	ACParam struct {
		Sweep  string  // DEC, OCT, LIN
		FStart float64 // start frequency
		Points int     // points per decade
		FStop  float64 // stop frequency
	}
	Title string // Circuit title
}

type Element struct {
	Type   string            // Part type (R, L, C, V, etc.)
	Name   string            // Part name
	Nodes  []string          // Node names
	Value  float64           // Part value
	Params map[string]string // Parameter values
}

var unitMap = map[string]float64{
	"T":   1e12,  // tera
	"G":   1e9,   // giga
	"meg": 1e6,   // mega
	"K":   1e3,   // kilo
	"k":   1e3,   // kilo
	"m":   1e-3,  // milli
	"u":   1e-6,  // micro
	"n":   1e-9,  // nano
	"p":   1e-12, // pico
	"f":   1e-15, // femto
}

func Parse(input string) (*Circuit, error) {
	scanner := bufio.NewScanner(strings.NewReader(input))
	circuit := &Circuit{
		Nodes: make(map[string]int),
	}

	// Title or comment
	if scanner.Scan() {
		circuit.Title = strings.TrimPrefix(scanner.Text(), "*")
		circuit.Title = strings.TrimSpace(circuit.Title)
	}

	for scanner.Scan() {
		line := scanner.Text()
		line = strings.TrimSpace(line)

		if len(line) == 0 { // Empty line
			continue
		}

		if strings.HasPrefix(line, "*") { // Comment
			continue
		}

		if strings.HasPrefix(line, ".") { // Analysis type
			err := parseAnalysis(circuit, line)
			if err != nil {
				return nil, err
			}
			continue
		}

		element, err := parseElement(line)
		if err != nil {
			return nil, err
		}

		circuit.Elements = append(circuit.Elements, *element)

		for _, node := range element.Nodes {
			if _, exists := circuit.Nodes[node]; !exists {
				circuit.Nodes[node] = len(circuit.Nodes)
			}
		}
	}

	return circuit, nil
}

// Parse .op, .tran, .ac
func parseAnalysis(ckt *Circuit, line string) error {
	var err error

	fields := strings.Fields(line)
	if len(fields) < 1 {
		return fmt.Errorf("invalid analysis command")
	}

	switch strings.ToLower(fields[0]) {
	case ".op":
		ckt.Analysis = AnalysisOP

	case ".tran":
		ckt.Analysis = AnalysisTRAN
		if len(fields) < 3 {
			return fmt.Errorf("insufficient tran parameters, need at least tstep and tstop")
		}
		if ckt.TranParam.TStep, err = ParseValue(fields[1]); err != nil {
			return fmt.Errorf("invalid tstep: %v", err)
		}
		if ckt.TranParam.TStop, err = ParseValue(fields[2]); err != nil {
			return fmt.Errorf("invalid tstop: %v", err)
		}

		for i := 3; i < len(fields); i++ {
			if fields[i] == "uic" {
				ckt.TranParam.UIC = true
				continue
			}
			if i == 3 {
				if ckt.TranParam.TStart, err = ParseValue(fields[i]); err != nil {
					return fmt.Errorf("invalid tstart: %v", err)
				}
			}
			if i == 4 {
				if ckt.TranParam.TMax, err = ParseValue(fields[i]); err != nil {
					return fmt.Errorf("invalid tmax: %v", err)
				}
			}
		}
		if ckt.TranParam.TMax == 0 {
			ckt.TranParam.TMax = ckt.TranParam.TStep
		}

	case ".ac":
		ckt.Analysis = AnalysisAC
		if len(fields) < 5 {
			return fmt.Errorf("insufficient AC parameters, need sweep type, points, fstart, and fstop")
		}

		// DEC, OCT, LIN
		ckt.ACParam.Sweep = strings.ToUpper(fields[1])
		if ckt.ACParam.Sweep != "DEC" && ckt.ACParam.Sweep != "OCT" && ckt.ACParam.Sweep != "LIN" {
			return fmt.Errorf("invalid sweep type: %s", ckt.ACParam.Sweep)
		}

		if ckt.ACParam.Points, err = strconv.Atoi(fields[2]); err != nil {
			return fmt.Errorf("invalid points number: %v", err)
		}
		if ckt.ACParam.FStart, err = ParseValue(fields[3]); err != nil {
			return fmt.Errorf("invalid fstart: %v", err)
		}
		if ckt.ACParam.FStop, err = ParseValue(fields[4]); err != nil {
			return fmt.Errorf("invalid fstop: %v", err)
		}

	default:
		return fmt.Errorf("unsupported analysis type: %s", fields[0])
	}

	return nil
}

// Parse circuit element
func parseElement(line string) (*Element, error) {
	fields := strings.Fields(line)
	if len(fields) < 3 {
		return nil, fmt.Errorf("invalid element format: %s", line)
	}

	elem := &Element{
		Name:   fields[0],
		Type:   strings.ToUpper(string(fields[0][0])),
		Params: make(map[string]string),
	}

	// TODO: I
	if elem.Type == "V" {
		return parseVoltageSource(fields)
	}

	// Parts
	elem.Nodes = fields[1 : len(fields)-1]
	valueStr := fields[len(fields)-1]
	value, err := ParseValue(valueStr)
	if err != nil {
		return nil, err
	}
	elem.Value = value

	return elem, nil
}

func parseVoltageSource(fields []string) (*Element, error) {
	if len(fields) < 4 {
		return nil, fmt.Errorf("insufficient voltage source parameters")
	}

	elem := &Element{
		Name:   fields[0],
		Type:   "V",
		Nodes:  []string{fields[1], fields[2]},
		Params: make(map[string]string),
	}

	remaining := strings.Join(fields[3:], " ")
	words := strings.Fields(remaining)
	if len(words) == 0 {
		return nil, fmt.Errorf("missing voltage source type")
	}

	switch strings.ToUpper(words[0]) {
	case "DC":
		if len(words) < 2 {
			return nil, fmt.Errorf("missing DC value")
		}
		elem.Params["type"] = "dc"
		value, err := ParseValue(words[1])
		if err != nil {
			return nil, err
		}
		elem.Value = value

	case "SIN":
		elem.Params["type"] = "sin"
		sinParams := strings.Join(words[1:], " ")
		sinParams = strings.Trim(sinParams, "() ")
		elem.Params["sin"] = sinParams

	case "AC":
		if len(words) < 2 {
			return nil, fmt.Errorf("missing AC magnitude")
		}
		elem.Params["type"] = "ac"
		magnitude, err := ParseValue(words[1])
		if err != nil {
			return nil, fmt.Errorf("invalid AC magnitude: %v", err)
		}
		elem.Value = magnitude

		if len(words) > 2 {
			elem.Params["phase"] = words[2]
		} else {
			elem.Params["phase"] = "0" // Default
		}

	default:
		return nil, fmt.Errorf("unsupported voltage source type: %s", words[0])
	}

	return elem, nil
}

// ParseValue - Parse value and factor. 1k -> 1000
func ParseValue(val string) (float64, error) {
	// re := regexp.MustCompile(`^([-+]?\d*\.?\d+)([TGMKkmunpf])?s?$`)
	re := regexp.MustCompile(`^([-+]?\d*\.?\d+)(meg|[TGMKkmunpf])?s?$`)
	matches := re.FindStringSubmatch(strings.TrimSpace(val))

	if matches == nil {
		return 0, fmt.Errorf("invalid value format: %s", val)
	}

	num, err := strconv.ParseFloat(matches[1], 64)
	if err != nil {
		return 0, err
	}

	// factor
	if len(matches) > 2 && matches[2] != "" {
		if multiplier, ok := unitMap[matches[2]]; ok {
			num *= multiplier
		}
	}

	return num, nil
}

func CreateDevice(elem Element, nodeMap map[string]int) (device.Device, error) {
	switch elem.Type {
	case "R":
		return device.NewResistor(elem.Name, elem.Nodes, elem.Value), nil
	case "L":
		return device.NewInductor(elem.Name, elem.Nodes, elem.Value), nil
	case "C":
		return device.NewCapacitor(elem.Name, elem.Nodes, elem.Value), nil
	case "V":
		switch elem.Params["type"] {
		case "dc":
			return device.NewDCVoltageSource(elem.Name, elem.Nodes, elem.Value), nil
		case "sin":
			offset, amplitude, freq, phase, err := parseSinParams(elem.Params["sin"])
			if err != nil {
				return nil, err
			}
			return device.NewSinVoltageSource(elem.Name, elem.Nodes, offset, amplitude, freq, phase), nil
		case "ac":
			phase, err := ParseValue(elem.Params["phase"])
			if err != nil {
				return nil, fmt.Errorf("invalid AC phase: %v", err)
			}
			return device.NewACVoltageSource(elem.Name, elem.Nodes, 0, elem.Value, phase), nil
		default:
			return nil, fmt.Errorf("unsupported voltage source type: %s", elem.Params["type"])
		}
	}
	return nil, fmt.Errorf("unsupported device type: %s", elem.Type)
}

func parseSinParams(params string) (offset, amplitude, freq, phase float64, err error) {
	sinParams := strings.Fields(params)
	if len(sinParams) < 3 {
		return 0, 0, 0, 0, fmt.Errorf("insufficient SIN parameters")
	}

	// DC offset
	if offset, err = ParseValue(sinParams[0]); err != nil {
		return 0, 0, 0, 0, fmt.Errorf("invalid SIN offset: %v", err)
	}

	// Amplitude
	if amplitude, err = ParseValue(sinParams[1]); err != nil {
		return 0, 0, 0, 0, fmt.Errorf("invalid SIN amplitude: %v", err)
	}

	// Frequency
	if freq, err = ParseValue(sinParams[2]); err != nil {
		return 0, 0, 0, 0, fmt.Errorf("invalid SIN frequency: %v", err)
	}

	// Phase
	phase = 0.0
	if len(sinParams) > 3 {
		if phase, err = ParseValue(sinParams[3]); err != nil {
			return 0, 0, 0, 0, fmt.Errorf("invalid SIN phase: %v", err)
		}
	}

	return offset, amplitude, freq, phase, nil
}
