package main

import (
	"fmt"
	"log"
	"math"
	"strings"

	"github.com/edp1096/toy-spice/pkg/analysis"
	"github.com/edp1096/toy-spice/pkg/circuit"
	"github.com/edp1096/toy-spice/pkg/device"
	"github.com/edp1096/toy-spice/pkg/netlist"
	"github.com/edp1096/toy-spice/pkg/util"
)

func createCircuit() (*circuit.Circuit, error) {
	ckt := circuit.NewWithComplex("BJT Common Emitter Amplifier Circuit", false)

	// BJT model (2N2222 NPN transistor)
	bjtModel := device.ModelParam{
		Name: "Q2N2222",
		Params: map[string]float64{
			"type": 0.0,     // 0 for NPN, 1 for PNP
			"is":   1.8e-14, // Saturation current
			"bf":   100,     // Forward beta
			"vaf":  100,     // Early voltage
			"ikf":  0.3,     // Forward knee current
			"rc":   0.3,     // Collector resistance
			"re":   0.2,     // Emitter resistance
			"rb":   10,      // Base resistance
			"cje":  22e-12,  // Base-Emitter junction capacitance
			"cjc":  8e-12,   // Base-Collector junction capacitance
			"tf":   0.3e-9,  // Forward transit time
		},
	}

	models := map[string]device.ModelParam{
		"Q2N2222": bjtModel,
	}

	elements := []netlist.Element{
		{
			Type:   "V",
			Name:   "Vcc",
			Nodes:  []string{"vcc", "0"},
			Value:  12.0, // DC 12V
			Params: map[string]string{"type": "dc"},
		},
		{
			Type:   "V",
			Name:   "Vin",
			Nodes:  []string{"in", "0"},
			Value:  0.0,                                                   // DC bias
			Params: map[string]string{"type": "sin", "sin": "0 0.1 1k 0"}, // 100mV, 1kHz signal
		},
		// Bias circuit
		{
			Type:   "R",
			Name:   "Rc",
			Nodes:  []string{"vcc", "c"},
			Value:  1000.0, // 1kΩ collector resistor
			Params: map[string]string{},
		},
		{
			Type:   "R",
			Name:   "Rb1",
			Nodes:  []string{"vcc", "b"},
			Value:  10000.0, // 10kΩ base bias resistor
			Params: map[string]string{},
		},
		{
			Type:   "R",
			Name:   "Rb2",
			Nodes:  []string{"b", "0"},
			Value:  2200.0, // 2.2kΩ base bias resistor
			Params: map[string]string{},
		},
		{
			Type:   "R",
			Name:   "Re",
			Nodes:  []string{"e", "0"},
			Value:  220.0, // 220Ω emitter resistor
			Params: map[string]string{},
		},
		// Coupling capacitor
		{
			Type:   "C",
			Name:   "Cin",
			Nodes:  []string{"in", "b"},
			Value:  10e-6, // 10uF input coupling capacitor
			Params: map[string]string{},
		},
		{
			Type:   "C",
			Name:   "Cout",
			Nodes:  []string{"c", "out"},
			Value:  10e-6, // 10uF output coupling capacitor
			Params: map[string]string{},
		},
		// Load resistance
		{
			Type:   "R",
			Name:   "RL",
			Nodes:  []string{"out", "0"},
			Value:  10000.0, // 10kΩ
			Params: map[string]string{},
		},
		// Emitter bypass capacitor
		{
			Type:   "C",
			Name:   "Ce",
			Nodes:  []string{"e", "0"},
			Value:  100e-6, // 100uF
			Params: map[string]string{},
		},
		// BJT
		{
			Type:   "Q",
			Name:   "Q1",
			Nodes:  []string{"c", "b", "e"},
			Params: map[string]string{"model": "Q2N2222"},
		},
	}

	err := ckt.AssignNodeBranchMaps(elements)
	if err != nil {
		return nil, fmt.Errorf("error node, branch map: %v", err)
	}

	ckt.CreateMatrix()

	ckt.Models = models

	err = ckt.SetupDevices(elements)
	if err != nil {
		return nil, fmt.Errorf("error device setup: %v", err)
	}

	return ckt, nil
}

func main() {
	fmt.Print("===== BJT Common Emitter Amplifier Example =====\n\n")

	fmt.Println("Generating circuit...")
	ckt, err := createCircuit()
	if err != nil {
		log.Fatalf("error circuit generation: %v", err)
	}

	ckt.Matrix.PrintSystem()

	fmt.Println("Circuit information:")
	fmt.Printf("  Name: %s\n", ckt.Name())
	fmt.Printf("  Node count: %d (except GND)\n\n", ckt.GetNumNodes())

	fmt.Println("Running operating point analysis...")
	op := analysis.NewOP()
	err = op.Setup(ckt)
	if err != nil {
		log.Fatalf("error setting up operating point: %v", err)
	}

	err = op.Execute()
	if err != nil {
		log.Fatalf("error running operating point: %v", err)
	}

	bias := op.GetResults()

	fmt.Println("\nOperating Point Results:")
	fmt.Print("=========================\n\n")

	fmt.Println("Node voltages:")
	for name, values := range bias {
		if strings.HasPrefix(name, "V(") {
			fmt.Printf("  %s = %s\n", name, util.FormatValueFactor(values[0], "V"))
		}
	}

	fmt.Println("\nBranch currents:")
	for name, values := range bias {
		if strings.HasPrefix(name, "I(") {
			fmt.Printf("  %s = %s\n", name, util.FormatValueFactor(values[0], "A"))
		}
	}

	vb := bias["V(b)"][0]
	ve := bias["V(e)"][0]
	vc := bias["V(c)"][0]
	vbe := vb - ve
	vce := vc - ve

	var ic float64
	if icValues, ok := bias["I(Q1)"]; ok {
		ic = icValues[0]
	} else {
		ic = (bias["V(vcc)"][0] - bias["V(c)"][0]) / 1000.0
	}

	fmt.Println("\nTransistor Q1 bias point:")
	fmt.Printf("  VBE = %.3f V\n", vbe)
	fmt.Printf("  VCE = %.3f V\n", vce)
	fmt.Printf("  IC = %.3f mA\n", ic*1000.0)

	fmt.Println("\nRunning transient analysis...")
	tran := analysis.NewTransient(0, 5e-3, 5e-6, 20e-6, false)
	err = tran.Setup(ckt)
	if err != nil {
		log.Fatalf("error setting up transient analysis: %v", err)
	}

	err = tran.Execute()
	if err != nil {
		log.Fatalf("error running transient analysis: %v", err)
	}

	results := tran.GetResults()

	timePoints := len(results["TIME"])
	fmt.Printf("\nTransient analysis completed with %d time points\n", timePoints)

	var vin_max, vin_min, vout_max, vout_min float64
	vin_max = results["V(in)"][0]
	vin_min = results["V(in)"][0]
	vout_max = results["V(out)"][0]
	vout_min = results["V(out)"][0]

	steadyStateIdx := 0
	for i, t := range results["TIME"] {
		if t >= 3e-3 {
			steadyStateIdx = i
			break
		}
	}

	for i := steadyStateIdx; i < timePoints; i++ {
		if results["V(in)"][i] > vin_max {
			vin_max = results["V(in)"][i]
		}
		if results["V(in)"][i] < vin_min {
			vin_min = results["V(in)"][i]
		}

		if results["V(out)"][i] > vout_max {
			vout_max = results["V(out)"][i]
		}
		if results["V(out)"][i] < vout_min {
			vout_min = results["V(out)"][i]
		}
	}

	vin_pp := vin_max - vin_min
	vout_pp := vout_max - vout_min

	fmt.Println("\nSignal Analysis (steady state):")
	fmt.Printf("  Input signal: %.3f Vpp\n", vin_pp)
	fmt.Printf("  Output signal: %.3f Vpp\n", vout_pp)

	// Voltage gain
	voltage_gain := vout_pp / vin_pp
	voltage_gain_db := 20.0 * math.Log10(voltage_gain)

	fmt.Printf("  Voltage gain: %.2f (%.2f dB)\n", voltage_gain, voltage_gain_db)

	fmt.Println("\nDone!")
}
