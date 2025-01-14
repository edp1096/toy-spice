package device

import (
	"fmt"
	"math"
	"toy-spice/pkg/matrix"
)

type Diode struct {
	BaseDevice
	// 기본 모델 파라미터
	Is   float64 // 포화 전류
	N    float64 // 발광계수 (이상계수)
	Rs   float64 // 직렬저항
	Cj0  float64 // 0바이어스에서의 접합 커패시턴스
	M    float64 // 접합 기울기 계수
	Vj   float64 // 접합 전위
	Bv   float64 // 항복 전압
	Gmin float64 // 최소 컨덕턴스

	// 내부 동작점 상태
	vd float64 // 다이오드 전압
	id float64 // 다이오드 전류
	gd float64 // 동작점에서의 컨덕턴스

	// 과도해석을 위한 상태변수
	vdOld      float64 // 이전 시점의 전압
	idOld      float64 // 이전 시점의 전류
	capCurrent float64 // 커패시터 전류
}

// 새 다이오드 생성
func NewDiode(name string, nodeNames []string) *Diode {
	if len(nodeNames) != 2 {
		panic(fmt.Sprintf("diode %s: requires exactly 2 nodes", name))
	}

	d := &Diode{
		BaseDevice: BaseDevice{
			Name:      name,
			Nodes:     make([]int, len(nodeNames)),
			NodeNames: nodeNames,
		},
	}
	d.setDefaultParameters()
	return d
}

func (d *Diode) GetType() string { return "D" }

// 기본 파라미터 설정
func (d *Diode) setDefaultParameters() {
	d.Is = 1e-14   // 1e-14 A
	d.N = 1.0      // 이상계수
	d.Rs = 0.0     // 직렬저항 없음
	d.Cj0 = 0.0    // 접합 커패시턴스 없음
	d.M = 0.5      // 기본 기울기 계수
	d.Vj = 1.0     // 접합 전위
	d.Bv = 100.0   // 항복 전압
	d.Gmin = 1e-12 // 최소 컨덕턴스
}

// 열전압 계산 (kT/q)
func (d *Diode) thermalVoltage(temp float64) float64 {
	return 0.026 // 상온(300K)에서의 열전압
}

// 다이오드 전류 계산
func (d *Diode) calculateCurrent(vd float64, vt float64) float64 {
	// 순방향
	if vd >= -5*vt {
		expArg := vd / (d.N * vt)
		if expArg > 40 { // Prevent overflow - exp(40) ≈ 10^17
			expArg = 40
		}
		expVt := math.Exp(expArg)

		return d.Is * (expVt - 1)
	}

	// 역방향 (항복 포함)
	if vd < -d.Bv {
		// 항복영역
		return -d.Is * (1 + (vd+d.Bv)/vt)
	}
	return -d.Is
}

// 컨덕턴스 계산
func (d *Diode) calculateConductance(vd, id float64, vt float64) float64 {
	// 순방향
	if vd >= -5*vt {
		return (id+d.Is)/(d.N*vt) + d.Gmin
	}

	// 역방향
	if vd < -d.Bv {
		return d.Is/vt + d.Gmin
	}

	return d.Gmin
}

// 접합 커패시턴스 계산
func (d *Diode) calculateJunctionCap(vd float64) float64 {
	if d.Cj0 == 0 {
		return 0
	}

	if vd < 0 {
		arg := 1 - vd/d.Vj
		if arg < 0.1 {
			arg = 0.1
		}
		return d.Cj0 / math.Pow(arg, d.M)
	}

	// 순방향
	return d.Cj0 * (1 + d.M*vd/d.Vj)
}

// DC/Transient 스탬핑
func (d *Diode) Stamp(matrix matrix.DeviceMatrix, status *CircuitStatus) error {
	if len(d.Nodes) != 2 {
		return fmt.Errorf("diode %s: requires exactly 2 nodes", d.Name)
	}

	n1, n2 := d.Nodes[0], d.Nodes[1]
	vt := d.thermalVoltage(status.Temp)

	// 전류와 컨덕턴스 계산
	d.id = d.calculateCurrent(d.vd, vt)
	d.gd = d.calculateConductance(d.vd, d.id, vt)

	// 행렬 스탬핑
	if n1 != 0 {
		matrix.AddElement(n1, n1, d.gd)
		if n2 != 0 {
			matrix.AddElement(n1, n2, -d.gd)
		}
		matrix.AddRHS(n1, -(d.id - d.gd*d.vd))
	}

	if n2 != 0 {
		if n1 != 0 {
			matrix.AddElement(n2, n1, -d.gd)
		}
		matrix.AddElement(n2, n2, d.gd)
		matrix.AddRHS(n2, (d.id - d.gd*d.vd))
	}

	return nil
}

// AC 분석을 위한 스탬핑
func (d *Diode) StampAC(matrix matrix.DeviceMatrix, status *CircuitStatus) error {
	if len(d.Nodes) != 2 {
		return fmt.Errorf("diode %s: requires exactly 2 nodes", d.Name)
	}

	n1, n2 := d.Nodes[0], d.Nodes[1]
	omega := 2 * math.Pi * status.Frequency

	// 동작점에서의 컨덕턴스와 커패시턴스
	gd := d.gd // DC 동작점에서의 컨덕턴스
	cj := d.calculateJunctionCap(d.vd)

	// 어드미턴스 계산 (G + jωC)
	yeq := complex(gd, omega*cj)

	// 행렬 스탬핑
	if n1 != 0 {
		matrix.AddComplexElement(n1, n1, real(yeq), imag(yeq))
		if n2 != 0 {
			matrix.AddComplexElement(n1, n2, -real(yeq), -imag(yeq))
		}
	}

	if n2 != 0 {
		if n1 != 0 {
			matrix.AddComplexElement(n2, n1, -real(yeq), -imag(yeq))
		}
		matrix.AddComplexElement(n2, n2, real(yeq), imag(yeq))
	}

	return nil
}

// NonLinear 인터페이스 구현
func (d *Diode) LoadConductance(matrix matrix.DeviceMatrix) error {
	n1, n2 := d.Nodes[0], d.Nodes[1]

	if n1 != 0 {
		matrix.AddElement(n1, n1, d.gd)
		if n2 != 0 {
			matrix.AddElement(n1, n2, -d.gd)
		}
	}

	if n2 != 0 {
		if n1 != 0 {
			matrix.AddElement(n2, n1, -d.gd)
		}
		matrix.AddElement(n2, n2, d.gd)
	}

	return nil
}

func (d *Diode) LoadCurrent(matrix matrix.DeviceMatrix) error {
	n1, n2 := d.Nodes[0], d.Nodes[1]

	if n1 != 0 {
		matrix.AddRHS(n1, -(d.id - d.gd*d.vd))
	}
	if n2 != 0 {
		matrix.AddRHS(n2, (d.id - d.gd*d.vd))
	}

	return nil
}

// TimeDependent 인터페이스 구현
func (d *Diode) SetTimeStep(dt float64) {
	// 현재는 시간 스텝만 저장
}

func (d *Diode) UpdateState(voltages []float64, status *CircuitStatus) {
	// 이전 상태 저장
	d.vdOld = d.vd
	d.idOld = d.id
}

func (d *Diode) CalculateLTE(voltages map[string]float64, status *CircuitStatus) float64 {
	// 현재는 단순한 LTE 계산
	return math.Abs(d.vd - d.vdOld)
}

func (d *Diode) UpdateVoltages(voltages []float64) error {
	if len(d.Nodes) != 2 {
		return fmt.Errorf("diode %s: requires exactly 2 nodes", d.Name)
	}

	n1, n2 := d.Nodes[0], d.Nodes[1]
	var v1, v2 float64

	// 노드 전압 가져오기
	if n1 != 0 {
		v1 = voltages[n1]
	}
	if n2 != 0 {
		v2 = voltages[n2]
	}

	// 다이오드 전압 업데이트
	d.vd = v1 - v2
	return nil
}
