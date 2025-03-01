package netlist

import (
	"bufio"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/edp1096/toy-spice/pkg/device"
	"github.com/edp1096/toy-spice/pkg/util"
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
	var continuationMode bool

	for scanner.Scan() {
		line := scanner.Text()

		// 앞뒤 공백 제거
		line = strings.TrimSpace(line)

		// 빈 줄 처리
		if len(line) == 0 {
			if currentLine != "" {
				if err := parseLine(netlistData, currentLine); err != nil {
					return nil, err
				}
				currentLine = ""
				continuationMode = false
			}
			continue
		}

		// 라인 내 주석 제거
		if idx := strings.Index(line, "*"); idx >= 0 {
			line = strings.TrimSpace(line[:idx])
			if len(line) == 0 {
				continue
			}
		}

		// 전체 주석 라인 처리
		if strings.HasPrefix(line, "*") {
			if currentLine != "" {
				if err := parseLine(netlistData, currentLine); err != nil {
					return nil, err
				}
				currentLine = ""
				continuationMode = false
			}
			continue
		}

		// 라인 이어짐 처리
		if strings.HasPrefix(line, "+") {
			line = strings.TrimPrefix(line, "+")
			line = strings.TrimSpace(line)
			if currentLine != "" {
				currentLine += " " + line
			}
			continuationMode = true
			continue
		}

		// 들여쓰기로 이어진 라인 처리
		if continuationMode && strings.HasPrefix(scanner.Text(), " ") {
			line = strings.TrimSpace(line)
			if currentLine != "" {
				currentLine += " " + line
			}
			continue
		}

		// 새로운 라인 시작
		if currentLine != "" {
			if err := parseLine(netlistData, currentLine); err != nil {
				return nil, err
			}
		}
		currentLine = line
		continuationMode = false
	}

	// 마지막 라인 처리
	if currentLine != "" {
		if err := parseLine(netlistData, currentLine); err != nil {
			return nil, err
		}
	}

	return netlistData, nil
}

func parseLine(netlistData *NetlistData, line string) error {
	// 라인 내 연속된 공백을 단일 공백으로 변환
	line = regexp.MustCompile(`\s+`).ReplaceAllString(line, " ")

	if strings.HasPrefix(line, ".") {
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

	// 두번째 필드에서 타입과 괄호 시작 분리
	typeField := fields[1]
	modelType := ""
	hasOpenParen := false

	if strings.Contains(typeField, "(") {
		parts := strings.SplitN(typeField, "(", 2)
		modelType = strings.ToUpper(parts[0])
		hasOpenParen = true
		// 나머지 부분을 다시 fields에 추가
		if len(parts) > 1 {
			fields = append(fields[:2], append([]string{parts[1]}, fields[2:]...)...)
		}
	} else {
		modelType = strings.ToUpper(typeField)
	}

	var supportedModelTypes = []string{"D", "CORE", "NPN", "PNP", "NMOS", "PMOS"}

	if !util.SliceContains(supportedModelTypes, modelType) {
		return fmt.Errorf("unsupported model type: %s", modelType)
	}

	// 파라미터 문자열 구성
	var paramStr string
	if hasOpenParen {
		// 괄호가 있는 경우 나머지 필드들을 결합
		paramParts := fields[2:]
		if len(paramParts) > 0 {
			// 마지막 필드의 닫는 괄호 처리
			last := paramParts[len(paramParts)-1]
			if strings.HasSuffix(last, ")") {
				paramParts[len(paramParts)-1] = strings.TrimSuffix(last, ")")
			}
		}
		paramStr = strings.Join(paramParts, " ")
	} else if len(fields) > 2 {
		// 괄호가 없는 경우 나머지 필드들을 결합
		paramStr = strings.Join(fields[2:], " ")
		// 마지막 닫는 괄호 제거
		paramStr = strings.TrimSuffix(paramStr, ")")
	}

	// 파라미터 문자열에서 주석 제거
	paramStr = regexp.MustCompile(`\*.*$`).ReplaceAllString(paramStr, "")
	paramStr = strings.TrimSpace(paramStr)

	params := make(map[string]float64)

	// 기본값 설정
	switch modelType {
	case "D":
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

	case "CORE":
		// Jiles-Atherton model
		params["ms"] = 1.6e6   // Saturation magnetization
		params["alpha"] = 1e-3 // Domain coupling
		params["a"] = 1000.0   // Shape parameter
		params["c"] = 0.1      // Reversibility
		params["k"] = 2000.0   // Pinning
		params["tc"] = 1043.0  // Curie temperature
		params["beta"] = 0.0   // Temperature coefficient
		params["area"] = 1e-4  // Cross-sectional area
		params["len"] = 0.1    // Mean path length

	case "NPN", "PNP":
		// BJT 기본 파라미터 설정
		params["is"] = 1e-16  // Transport saturation current
		params["bf"] = 100.0  // Ideal maximum forward beta
		params["br"] = 1.0    // Ideal maximum reverse beta
		params["nf"] = 1.0    // Forward emission coefficient
		params["nr"] = 1.0    // Reverse emission coefficient
		params["vaf"] = 100.0 // Forward Early voltage
		params["var"] = 100.0 // Reverse Early voltage
		params["ikf"] = 0.01  // Forward knee current
		params["ikr"] = 0.01  // Reverse knee current
		params["rc"] = 0.0    // Collector resistance
		params["re"] = 0.0    // Emitter resistance
		params["rb"] = 0.0    // Base resistance
		params["cje"] = 0.0   // B-E junction capacitance
		params["vje"] = 0.75  // B-E built-in potential
		params["mje"] = 0.33  // B-E junction grading coefficient
		params["cjc"] = 0.0   // B-C junction capacitance
		params["vjc"] = 0.75  // B-C built-in potential
		params["mjc"] = 0.33  // B-C junction grading coefficient
		params["tf"] = 0.0    // Forward transit time
		params["tr"] = 0.0    // Reverse transit time
		params["xtb"] = 0.0   // Forward and reverse beta temp. exp
		params["eg"] = 1.11   // Energy gap
		params["xti"] = 3.0   // Temp. exponent for Is

		if modelType == "PNP" {
			params["type"] = 1.0 // PNP = 1, NPN = 0
		}

	case "NMOS", "PMOS":
		params["level"] = 1     // 기본 레벨 1
		params["vto"] = 0.7     // 문턱 전압
		params["kp"] = 2e-5     // 트랜스컨덕턴스 파라미터
		params["gamma"] = 0.5   // 기판 효과 계수
		params["phi"] = 0.6     // 표면 포텐셜
		params["lambda"] = 0.01 // 채널 길이 변조 파라미터
		params["rd"] = 0.0      // 드레인 저항
		params["rs"] = 0.0      // 소스 저항
		params["cbd"] = 0.0     // 벌크-드레인 접합 캐패시턴스
		params["cbs"] = 0.0     // 벌크-소스 접합 캐패시턴스
		params["is"] = 1e-14    // 벌크 접합 포화 전류
		params["pb"] = 0.8      // 벌크 접합 전위
		params["cgso"] = 0.0    // 게이트-소스 오버랩 캐패시턴스
		params["cgdo"] = 0.0    // 게이트-드레인 오버랩 캐패시턴스
		params["cgbo"] = 0.0    // 게이트-벌크 오버랩 캐패시턴스
		params["cj"] = 0.0      // 벌크 접합 캐패시턴스
		params["mj"] = 0.5      // 벌크 접합 기울기 계수
		params["cjsw"] = 0.0    // 벌크 접합 측벽 캐패시턴스
		params["mjsw"] = 0.33   // 벌크 접합 측벽 기울기 계수
		params["tox"] = 1e-7    // 산화막 두께
		params["l"] = 10e-6     // 채널 길이
		params["w"] = 10e-6     // 채널 폭

		if modelType == "PMOS" {
			params["type"] = 1.0 // PMOS = 1, NMOS = 0
		}
	}

	// Parse parameters
	paramPairs := strings.Fields(paramStr)
	for _, pair := range paramPairs {
		parts := strings.Split(pair, "=")
		if len(parts) != 2 {
			continue
		}

		paramName := strings.ToLower(strings.TrimSpace(parts[0]))
		value, err := ParseValue(strings.TrimSpace(parts[1]))
		if err != nil {
			return fmt.Errorf("invalid parameter value %s: %v", pair, err)
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

	case "L":
		elem.Nodes = fields[1:3]
		elem.Params = make(map[string]string)

		for i := 3; i < len(fields); i++ {
			pair := strings.Split(fields[i], "=")
			if len(pair) == 2 {
				paramName := strings.ToLower(pair[0])
				elem.Params[paramName] = pair[1]
			} else {
				if !strings.Contains(fields[i], "=") {
					value, err := ParseValue(fields[i])
					if err != nil {
						return nil, err
					}
					elem.Value = value
				}
			}
		}

		return elem, nil

	case "K": // 상호 인덕턴스
		if len(fields) < 4 {
			return nil, fmt.Errorf("insufficient mutual coupling parameters: need coupling name, inductors and coefficient")
		}

		// 마지막 필드가 결합 계수
		coefficient, err := ParseValue(fields[len(fields)-1])
		if err != nil {
			return nil, fmt.Errorf("invalid coupling coefficient: %v", err)
		}
		if coefficient < -1 || coefficient > 1 {
			return nil, fmt.Errorf("coupling coefficient must be between -1 and 1: %f", coefficient)
		}

		// 중간의 모든 필드들은 인덕터 이름들
		indNames := fields[1 : len(fields)-1]
		if len(indNames) < 2 {
			return nil, fmt.Errorf("mutual coupling requires at least two inductors")
		}

		elem.Params = make(map[string]string)
		for i, name := range indNames {
			elem.Params[fmt.Sprintf("ind%d", i+1)] = name
		}
		elem.Value = coefficient
		return elem, nil

	case "D":
		elem.Nodes = fields[1:3]
		if len(fields) > 3 {
			// 인라인 파라미터 나중에
			elem.Params["model"] = fields[3]
		}

		return elem, nil

	case "Q":
		if len(fields) < 4 {
			return nil, fmt.Errorf("insufficient BJT parameters: need nodes and model name")
		}
		elem.Nodes = fields[1:4] // Collector, Base, Emitter
		if len(fields) > 4 {
			elem.Params["model"] = fields[4]
		}
		return elem, nil

	case "M":
		if len(fields) < 6 {
			return nil, fmt.Errorf("insufficient MOSFET parameters: need nodes and model name")
		}

		elem.Nodes = fields[1:5] // Drain, Gate, Source, Bulk
		elem.Params = make(map[string]string)
		elem.Params["model"] = fields[5] // Model name

		// Parameters eg. L=2u W=20u ...
		for i := 6; i < len(fields); i++ {
			parts := strings.Split(fields[i], "=")
			if len(parts) == 2 {
				elem.Params[strings.ToLower(parts[0])] = parts[1]
			}
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

var magneticCores = make(map[string]*device.MagneticCore)

func CreateDevice(elem Element, nodeMap map[string]int, models map[string]device.ModelParam) (device.Device, error) {
	switch elem.Type {
	case "R":
		return device.NewResistor(elem.Name, elem.Nodes, elem.Value), nil

	case "L":
		// Transformer - Magnetic Core
		if coreName, ok := elem.Params["core"]; ok {
			if model, exists := models[coreName]; exists {
				if model.Type == "CORE" {
					// Parse turns of winding
					turns := 100 // Default winding
					if turnsStr, ok := elem.Params["turns"]; ok {
						if t, err := strconv.Atoi(turnsStr); err == nil {
							turns = t
						}
					}

					inductor := device.NewMagneticInductor(elem.Name, elem.Nodes, turns)

					if core, exists := magneticCores[coreName]; exists {
						inductor.SetCore(model.Params)
						core.AddInductor(inductor)
					} else {
						inductor.SetCore(model.Params)
						magneticCores[coreName] = inductor.GetCore()
					}

					return inductor, nil
				}
				return nil, fmt.Errorf("invalid core model type for inductor %s: %s", elem.Name, model.Type)
			}
			return nil, fmt.Errorf("undefined core model for inductor %s: %s", elem.Name, coreName)
		}

		// Inductor
		return device.NewInductor(elem.Name, elem.Nodes, elem.Value), nil

	case "C":
		return device.NewCapacitor(elem.Name, elem.Nodes, elem.Value), nil

	case "K":
		var indNames []string
		for i := 1; ; i++ {
			if name, ok := elem.Params[fmt.Sprintf("ind%d", i)]; ok {
				indNames = append(indNames, name)
			} else {
				break
			}
		}
		if len(indNames) < 2 {
			return nil, fmt.Errorf("mutual coupling %s requires at least two inductors", elem.Name)
		}
		return device.NewMutual(elem.Name, indNames, elem.Value), nil

	case "D":
		diode := device.NewDiode(elem.Name, elem.Nodes)
		if modelName, ok := elem.Params["model"]; ok {
			if model, exists := models[modelName]; exists {
				diode.SetModelParameters(model.Params)
			}
		}
		return diode, nil

	case "Q":
		bjt := device.NewBJT(elem.Name, elem.Nodes)
		if modelName, ok := elem.Params["model"]; ok {
			if model, exists := models[modelName]; exists {
				bjt.SetModelParameters(model.Params)
			}
		}
		return bjt, nil

	case "M":
		if modelName, ok := elem.Params["model"]; ok {
			mosfet := device.NewMosfet(elem.Name, elem.Nodes)
			if model, exists := models[modelName]; exists {
				mosfet.SetModelParameters(model.Params)
			}

			if l, ok := elem.Params["l"]; ok {
				if lVal, err := ParseValue(l); err == nil {
					mosfet.L = lVal
				}
			}
			if w, ok := elem.Params["w"]; ok {
				if wVal, err := ParseValue(w); err == nil {
					mosfet.W = wVal
				}
			}

			return mosfet, nil
		}

		return nil, fmt.Errorf("mosfet %s: model not specified", elem.Name)

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

	for i := range numPoints {
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
