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
	AnalysisDC
)

type NetlistData struct {
	Elements  []Element                    // Circuit elements
	Nodes     map[string]int               // Node name and index
	Models    map[string]device.ModelParam // Model parameters
	Analysis  AnalysisType                 // Analysis type
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
	DCParam struct {
		Source1    string
		Start1     float64
		Stop1      float64
		Increment1 float64
		Source2    string
		Start2     float64
		Stop2      float64
		Increment2 float64
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

func ParseNotUse(input string) (*NetlistData, error) {
	scanner := bufio.NewScanner(strings.NewReader(input))
	netlistData := &NetlistData{
		Nodes:  make(map[string]int),
		Models: make(map[string]device.ModelParam),
	}

	// Title or comment
	if scanner.Scan() {
		netlistData.Title = strings.TrimPrefix(scanner.Text(), "*")
		netlistData.Title = strings.TrimSpace(netlistData.Title)
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
			err := parseDotOperator(netlistData, line)
			if err != nil {
				return nil, err
			}
			continue
		}

		element, err := parseElement(line)
		if err != nil {
			return nil, err
		}

		netlistData.Elements = append(netlistData.Elements, *element)

		for _, node := range element.Nodes {
			if _, exists := netlistData.Nodes[node]; !exists {
				netlistData.Nodes[node] = len(netlistData.Nodes)
			}
		}
	}

	return netlistData, nil
}

func Parse(input string) (*NetlistData, error) {
	scanner := bufio.NewScanner(strings.NewReader(input))
	netlistData := &NetlistData{
		Nodes:  make(map[string]int),
		Models: make(map[string]device.ModelParam),
	}

	// Title or comment
	if scanner.Scan() {
		netlistData.Title = strings.TrimPrefix(scanner.Text(), "*")
		netlistData.Title = strings.TrimSpace(netlistData.Title)
	}

	var currentLine string
	for scanner.Scan() {
		line := scanner.Text()
		line = strings.TrimSpace(line)

		if len(line) == 0 { // Empty line
			if currentLine != "" {
				// Process gathered line
				if err := parseLine(netlistData, currentLine); err != nil {
					return nil, err
				}
				currentLine = ""
			}
			continue
		}

		if strings.HasPrefix(line, "*") { // Comment
			if currentLine != "" {
				// Process gathered line before comment
				if err := parseLine(netlistData, currentLine); err != nil {
					return nil, err
				}
				currentLine = ""
			}
			continue
		}

		if strings.HasPrefix(line, "+") { // Line continue
			// Change "+" to space to maintain separation
			line = " " + strings.TrimSpace(line[1:])
			currentLine += line
			continue
		}

		// Process any gathered line before starting new one
		if currentLine != "" {
			if err := parseLine(netlistData, currentLine); err != nil {
				return nil, err
			}
		}
		currentLine = line
	}

	// Process final line if exists
	if currentLine != "" {
		if err := parseLine(netlistData, currentLine); err != nil {
			return nil, err
		}
	}

	return netlistData, nil
}

func parseLine(netlistData *NetlistData, line string) error {
	if strings.HasPrefix(line, ".") { // Analysis type
		return parseDotOperator(netlistData, line)
	}

	element, err := parseElement(line)
	if err != nil {
		return err
	}

	netlistData.Elements = append(netlistData.Elements, *element)
	for _, node := range element.Nodes {
		if _, exists := netlistData.Nodes[node]; !exists {
			netlistData.Nodes[node] = len(netlistData.Nodes)
		}
	}
	return nil
}

// Parse .op, .tran, .ac, .model
func parseDotOperator(netlistData *NetlistData, line string) error {
	var err error

	fields := strings.Fields(line)
	if len(fields) < 1 {
		return fmt.Errorf("invalid analysis command")
	}

	switch strings.ToLower(fields[0]) {
	case ".model":
		return parseModel(netlistData, fields[1:])

	case ".op":
		netlistData.Analysis = AnalysisOP

	case ".tran":
		netlistData.Analysis = AnalysisTRAN
		if len(fields) < 3 {
			return fmt.Errorf("insufficient tran parameters, need at least tstep and tstop")
		}
		netlistData.TranParam.TStep, err = ParseValue(fields[1])
		if err != nil {
			return fmt.Errorf("invalid tstep: %v", err)
		}
		netlistData.TranParam.TStop, err = ParseValue(fields[2])
		if err != nil {
			return fmt.Errorf("invalid tstop: %v", err)
		}

		for i := 3; i < len(fields); i++ {
			if fields[i] == "uic" {
				netlistData.TranParam.UIC = true
				continue
			}
			if i == 3 {
				netlistData.TranParam.TStart, err = ParseValue(fields[i])
				if err != nil {
					return fmt.Errorf("invalid tstart: %v", err)
				}
			}
			if i == 4 {
				netlistData.TranParam.TMax, err = ParseValue(fields[i])
				if err != nil {
					return fmt.Errorf("invalid tmax: %v", err)
				}
			}
		}
		if netlistData.TranParam.TMax == 0 {
			netlistData.TranParam.TMax = netlistData.TranParam.TStep
		}

	case ".ac":
		netlistData.Analysis = AnalysisAC
		if len(fields) < 5 {
			return fmt.Errorf("insufficient AC parameters, need sweep type, points, fstart, and fstop")
		}

		// DEC, OCT, LIN
		netlistData.ACParam.Sweep = strings.ToUpper(fields[1])
		if netlistData.ACParam.Sweep != "DEC" && netlistData.ACParam.Sweep != "OCT" && netlistData.ACParam.Sweep != "LIN" {
			return fmt.Errorf("invalid sweep type: %s", netlistData.ACParam.Sweep)
		}

		netlistData.ACParam.Points, err = strconv.Atoi(fields[2])
		if err != nil {
			return fmt.Errorf("invalid points number: %v", err)
		}
		netlistData.ACParam.FStart, err = ParseValue(fields[3])
		if err != nil {
			return fmt.Errorf("invalid fstart: %v", err)
		}
		netlistData.ACParam.FStop, err = ParseValue(fields[4])
		if err != nil {
			return fmt.Errorf("invalid fstop: %v", err)
		}

	case ".dc":
		netlistData.Analysis = AnalysisDC
		if len(fields) < 5 {
			return fmt.Errorf("insufficient DC sweep parameters")
		}

		// First source sweep
		netlistData.DCParam.Source1 = fields[1]
		var err error
		netlistData.DCParam.Start1, err = ParseValue(fields[2])
		if err != nil {
			return fmt.Errorf("invalid start value: %v", err)
		}
		netlistData.DCParam.Stop1, err = ParseValue(fields[3])
		if err != nil {
			return fmt.Errorf("invalid stop value: %v", err)
		}
		netlistData.DCParam.Increment1, err = ParseValue(fields[4]) // Step1을 Increment1으로 변경
		if err != nil {
			return fmt.Errorf("invalid increment value: %v", err)
		}

	default:
		return fmt.Errorf("unsupported analysis type: %s", fields[0])
	}

	return nil
}

func parseModel(netlistData *NetlistData, fields []string) error {
	if len(fields) < 2 {
		return fmt.Errorf("insufficient model parameters")
	}

	modelName := fields[0]
	modelType := strings.ToUpper(fields[1])

	// Currently D model only
	if modelType != "D" {
		return fmt.Errorf("unsupported model type: %s", modelType)
	}

	// 파라미터 파싱
	params := make(map[string]float64)

	// 기본값 설정
	params["is"] = 1e-14 // Saturation current
	params["n"] = 1.0    // Emission coefficient
	params["rs"] = 0.0   // Series resistance
	params["cj0"] = 0.0  // Zero-bias junction capacitance
	params["m"] = 0.5    // Grading coefficient
	params["vj"] = 1.0   // Junction potential
	params["bv"] = 100.0 // Breakdown voltage
	params["eg"] = 1.11  // Energy gap
	params["xti"] = 3.0  // Saturation current temp exp
	params["tt"] = 0.0   // Transit time
	params["fc"] = 0.5   // Forward-bias depletion capacitance coefficient

	// Parse parameters (name=value pairs)
	for i := 2; i < len(fields); i++ {
		field := strings.TrimRight(fields[i], ")")

		// pair := strings.Split(fields[i], "=")
		pair := strings.Split(field, "=")
		if len(pair) != 2 {
			continue
		}

		paramName := strings.ToLower(pair[0])
		value, err := ParseValue(pair[1])
		if err != nil {
			// return fmt.Errorf("invalid parameter value %s=%s: %v", paramName, pair[1], err)
			return fmt.Errorf("invalid parameter value %s: %v", field, err)
		}
		params[paramName] = value
	}

	netlistData.Models[modelName] = device.ModelParam{
		Type:   modelType,
		Name:   modelName,
		Params: params,
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

	switch elem.Type {
	case "V":
		return parseVoltageSource(fields)
	case "I":
		return parseCurrentSource(fields)
	case "D":
		elem.Nodes = fields[1:3]
		if len(fields) > 3 {
			// 파라미터 처리 나중에
			elem.Params["model"] = fields[3]
		}
		return elem, nil
	default:
		// Parts - RLC..
		elem.Nodes = fields[1 : len(fields)-1]
		valueStr := fields[len(fields)-1]
		value, err := ParseValue(valueStr)
		if err != nil {
			return nil, err
		}
		elem.Value = value

		return elem, nil
	}
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
	remaining = strings.ReplaceAll(remaining, "(", " ( ") // Append whitespace around parentheses
	remaining = strings.ReplaceAll(remaining, ")", " ) ")
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

	case "PULSE":
		elem.Params["type"] = "pulse"
		pulseParams := strings.Join(words[1:], " ")
		pulseParams = strings.Trim(pulseParams, "() ")
		elem.Params["pulse"] = pulseParams

	case "PWL":
		elem.Params["type"] = "pwl"
		pwlParams := strings.Join(words[1:], " ")
		pwlParams = strings.Trim(pwlParams, "() ")
		elem.Params["pwl"] = pwlParams

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

func parseCurrentSource(fields []string) (*Element, error) {
	if len(fields) < 4 {
		return nil, fmt.Errorf("insufficient current source parameters")
	}

	elem := &Element{
		Name:   fields[0],
		Type:   "I",
		Nodes:  []string{fields[1], fields[2]},
		Params: make(map[string]string),
	}

	remaining := strings.Join(fields[3:], " ")
	remaining = strings.ReplaceAll(remaining, "(", " ( ")
	remaining = strings.ReplaceAll(remaining, ")", " ) ")
	words := strings.Fields(remaining)
	if len(words) == 0 {
		return nil, fmt.Errorf("missing current source type")
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

	case "PULSE":
		elem.Params["type"] = "pulse"
		pulseParams := strings.Join(words[1:], " ")
		pulseParams = strings.Trim(pulseParams, "() ")
		elem.Params["pulse"] = pulseParams

	case "PWL":
		elem.Params["type"] = "pwl"
		pwlParams := strings.Join(words[1:], " ")
		pwlParams = strings.Trim(pwlParams, "() ")
		elem.Params["pwl"] = pwlParams

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
			elem.Params["phase"] = "0" // Default phase
		}

	default:
		return nil, fmt.Errorf("unsupported current source type: %s", words[0])
	}

	return elem, nil
}

// ParseValue - Parse value and factor. 1k -> 1000
func ParseValue(val string) (float64, error) {
	// re := regexp.MustCompile(`^([-+]?\d*\.?\d+)([TGMKkmunpf])?s?$`)
	// re := regexp.MustCompile(`^([-+]?\d*\.?\d+)(meg|[TGMKkmunpf])?s?$`)
	re := regexp.MustCompile(`^([-+]?\d*\.?\d+(?:[eE][-+]?\d+)?)(meg|[TGMKkmunpf])?s?$`)
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

// func CreateDevice(elem Element, nodeMap map[string]int) (device.Device, error) {
func CreateDevice(elem Element, nodeMap map[string]int, models map[string]device.ModelParam) (device.Device, error) {
	switch elem.Type {
	case "R":
		return device.NewResistor(elem.Name, elem.Nodes, elem.Value), nil
	case "L":
		return device.NewInductor(elem.Name, elem.Nodes, elem.Value), nil
	case "C":
		return device.NewCapacitor(elem.Name, elem.Nodes, elem.Value), nil
	case "D":
		// return device.NewDiode(elem.Name, elem.Nodes), nil
		diode := device.NewDiode(elem.Name, elem.Nodes)
		if modelName, ok := elem.Params["model"]; ok {
			if model, exists := models[modelName]; exists {
				diode.SetModelParameters(model.Params)
			}
		}
		return diode, nil
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
		case "pulse":
			v1, v2, delay, rise, fall, pWidth, period, err := parsePulseParams(elem.Params["pulse"])
			if err != nil {
				return nil, err
			}
			return device.NewPulseVoltageSource(elem.Name, elem.Nodes, v1, v2, delay, rise, fall, pWidth, period), nil
		case "pwl":
			times, values, err := parsePWLParams(elem.Params["pwl"])
			if err != nil {
				return nil, err
			}
			return device.NewPWLVoltageSource(elem.Name, elem.Nodes, times, values), nil
		case "ac":
			phase, err := ParseValue(elem.Params["phase"])
			if err != nil {
				return nil, fmt.Errorf("invalid AC phase: %v", err)
			}
			return device.NewACVoltageSource(elem.Name, elem.Nodes, 0, elem.Value, phase), nil
		default:
			return nil, fmt.Errorf("unsupported voltage source type: %s", elem.Params["type"])
		}
	case "I":
		switch elem.Params["type"] {
		case "dc":
			return device.NewDCCurrentSource(elem.Name, elem.Nodes, elem.Value), nil
		case "sin":
			offset, amplitude, freq, phase, err := parseSinParams(elem.Params["sin"])
			if err != nil {
				return nil, err
			}
			return device.NewSinCurrentSource(elem.Name, elem.Nodes, offset, amplitude, freq, phase), nil
		case "pulse":
			i1, i2, delay, rise, fall, pWidth, period, err := parsePulseParams(elem.Params["pulse"])
			if err != nil {
				return nil, err
			}
			return device.NewPulseCurrentSource(elem.Name, elem.Nodes, i1, i2, delay, rise, fall, pWidth, period), nil
		case "pwl":
			times, values, err := parsePWLParams(elem.Params["pwl"])
			if err != nil {
				return nil, err
			}
			return device.NewPWLCurrentSource(elem.Name, elem.Nodes, times, values), nil
		case "ac":
			phase, err := ParseValue(elem.Params["phase"])
			if err != nil {
				return nil, fmt.Errorf("invalid AC phase: %v", err)
			}
			return device.NewACCurrentSource(elem.Name, elem.Nodes, 0, elem.Value, phase), nil
		default:
			return nil, fmt.Errorf("unsupported current source type: %s", elem.Params["type"])
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
	offset, err = ParseValue(sinParams[0])
	if err != nil {
		return 0, 0, 0, 0, fmt.Errorf("invalid SIN offset: %v", err)
	}

	// Amplitude
	amplitude, err = ParseValue(sinParams[1])
	if err != nil {
		return 0, 0, 0, 0, fmt.Errorf("invalid SIN amplitude: %v", err)
	}

	// Frequency
	freq, err = ParseValue(sinParams[2])
	if err != nil {
		return 0, 0, 0, 0, fmt.Errorf("invalid SIN frequency: %v", err)
	}

	// Phase
	phase = 0.0
	if len(sinParams) > 3 {
		phase, err = ParseValue(sinParams[3])
		if err != nil {
			return 0, 0, 0, 0, fmt.Errorf("invalid SIN phase: %v", err)
		}
	}

	return offset, amplitude, freq, phase, nil
}

func parsePulseParams(params string) (v1, v2, delay, rise, fall, pWidth, period float64, err error) {
	pulseParams := strings.Fields(params)
	if len(pulseParams) < 7 {
		return 0, 0, 0, 0, 0, 0, 0, fmt.Errorf("insufficient PULSE parameters")
	}

	// V1 - Initial value
	v1, err = ParseValue(pulseParams[0])
	if err != nil {
		return 0, 0, 0, 0, 0, 0, 0, fmt.Errorf("invalid PULSE V1: %v", err)
	}

	// V2 - Pulsed value
	v2, err = ParseValue(pulseParams[1])
	if err != nil {
		return 0, 0, 0, 0, 0, 0, 0, fmt.Errorf("invalid PULSE V2: %v", err)
	}

	// Delay time
	delay, err = ParseValue(pulseParams[2])
	if err != nil {
		return 0, 0, 0, 0, 0, 0, 0, fmt.Errorf("invalid PULSE delay: %v", err)
	}

	// Rise time
	rise, err = ParseValue(pulseParams[3])
	if err != nil {
		return 0, 0, 0, 0, 0, 0, 0, fmt.Errorf("invalid PULSE rise: %v", err)
	}

	// Fall time
	fall, err = ParseValue(pulseParams[4])
	if err != nil {
		return 0, 0, 0, 0, 0, 0, 0, fmt.Errorf("invalid PULSE fall: %v", err)
	}

	// Pulse width
	pWidth, err = ParseValue(pulseParams[5])
	if err != nil {
		return 0, 0, 0, 0, 0, 0, 0, fmt.Errorf("invalid PULSE width: %v", err)
	}

	// Period
	period, err = ParseValue(pulseParams[6])
	if err != nil {
		return 0, 0, 0, 0, 0, 0, 0, fmt.Errorf("invalid PULSE period: %v", err)
	}

	return v1, v2, delay, rise, fall, pWidth, period, nil
}

func parsePWLParams(params string) (times []float64, values []float64, err error) {
	pwlParams := strings.Fields(params)
	if len(pwlParams) < 4 || len(pwlParams)%2 != 0 {
		return nil, nil, fmt.Errorf("insufficient or invalid PWL parameters, need pairs of time-value")
	}

	numPoints := len(pwlParams) / 2
	times = make([]float64, numPoints)
	values = make([]float64, numPoints)

	for i := 0; i < numPoints; i++ {
		// Time point
		times[i], err = ParseValue(pwlParams[2*i])
		if err != nil {
			return nil, nil, fmt.Errorf("invalid PWL time[%d]: %v", i, err)
		}
		// Value point
		values[i], err = ParseValue(pwlParams[2*i+1])
		if err != nil {
			return nil, nil, fmt.Errorf("invalid PWL value[%d]: %v", i, err)
		}

		if i > 0 && times[i] <= times[i-1] {
			return nil, nil, fmt.Errorf("PWL time points must be strictly increasing")
		}
	}

	return times, values, nil
}
