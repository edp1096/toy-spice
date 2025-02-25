package main

import (
	"fmt"
	"log"
	"strings"

	"toy-spice/pkg/analysis"
	"toy-spice/pkg/circuit"
	"toy-spice/pkg/device"
	"toy-spice/pkg/netlist"
	"toy-spice/pkg/util"
)

func createCircuit() (*circuit.Circuit, error) {
	ckt := circuit.NewWithComplex("Diode Rectifier Circuit", false)

	diodeModel := device.ModelParam{
		Type: "D",
		Name: "D1N4148",
		Params: map[string]float64{
			"is":  2.52e-9, // Saturation current
			"n":   1.752,   // Ideality factor
			"rs":  0.568,   // Serial resistance
			"cj0": 4e-12,   // Junction capacitance
			"vj":  0.7,     // Internal voltage
			"bv":  100.0,   // Breakdown voltage
		},
	}

	models := map[string]device.ModelParam{
		"D1N4148": diodeModel,
	}

	elements := []netlist.Element{
		{
			Type:   "V",
			Name:   "Vin",
			Nodes:  []string{"1", "0"},
			Value:  5.0,
			Params: map[string]string{"type": "sin", "sin": "0 5 1k 0"},
		},
		{
			Type:   "R",
			Name:   "R1",
			Nodes:  []string{"1", "2"},
			Value:  100.0,
			Params: map[string]string{},
		},
		{
			Type:   "D",
			Name:   "D1",
			Nodes:  []string{"2", "3"},
			Params: map[string]string{"model": "D1N4148"},
		},
		{
			Type:   "C",
			Name:   "C1",
			Nodes:  []string{"3", "0"},
			Value:  10e-6,
			Params: map[string]string{},
		},
		{
			Type:   "R",
			Name:   "RL",
			Nodes:  []string{"3", "0"},
			Value:  1000.0,
			Params: map[string]string{},
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
	fmt.Print("===== Diode Rectifier Example =====\n\n")

	fmt.Println("Generating circuit...")
	ckt, err := createCircuit()
	if err != nil {
		log.Fatalf("error circuit generation: %v", err)
	}

	fmt.Println("Circuit information:")
	fmt.Printf("  Name: %s\n", ckt.Name())
	fmt.Printf("  Node count: %d (except GND)\n\n", ckt.GetNumNodes())

	fmt.Println("Setting up transient analysis...")
	tran := analysis.NewTransient(0, 5e-3, 10e-6, 50e-6, false)
	err = tran.Setup(ckt)
	if err != nil {
		log.Fatalf("error setting up transient analysis: %v", err)
	}

	fmt.Println("Running transient analysis...")
	err = tran.Execute()
	if err != nil {
		log.Fatalf("error running transient analysis: %v", err)
	}
	fmt.Println()

	results := tran.GetResults()

	fmt.Println("Transient Analysis Results:")
	fmt.Print("==========================\n\n")

	timePoints := len(results["TIME"])
	fmt.Printf("Number of time points: %d\n", timePoints)

	lastIdx := timePoints - 1
	fmt.Printf("\nResults at final time point (t = %s):\n",
		util.FormatValueFactor(results["TIME"][lastIdx], "s"))

	fmt.Println("Node voltages:")
	for name, values := range results {
		if strings.HasPrefix(name, "V(") {
			fmt.Printf("  %s = %s\n", name, util.FormatValueFactor(values[lastIdx], "V"))
		}
	}

	fmt.Println("\nBranch currents:")
	for name, values := range results {
		if strings.HasPrefix(name, "I(") {
			fmt.Printf("  %s = %s\n", name, util.FormatValueFactor(values[lastIdx], "A"))
		}
	}

	var vin_min, vin_max, vout_min, vout_max float64
	vin_min = results["V(1)"][0]
	vin_max = results["V(1)"][0]
	vout_min = results["V(3)"][0]
	vout_max = results["V(3)"][0]

	for i := 1; i < timePoints; i++ {
		if results["V(1)"][i] < vin_min {
			vin_min = results["V(1)"][i]
		}
		if results["V(1)"][i] > vin_max {
			vin_max = results["V(1)"][i]
		}

		if results["V(3)"][i] < vout_min {
			vout_min = results["V(3)"][i]
		}
		if results["V(3)"][i] > vout_max {
			vout_max = results["V(3)"][i]
		}
	}

	fmt.Println("\nVoltage Analysis:")
	fmt.Printf("  Input voltage range: %s to %s\n", util.FormatValueFactor(vin_min, "V"), util.FormatValueFactor(vin_max, "V"))
	fmt.Printf("  Output voltage range: %s to %s\n", util.FormatValueFactor(vout_min, "V"), util.FormatValueFactor(vout_max, "V"))
	fmt.Printf("  Ripple voltage: %s\n", util.FormatValueFactor(vout_max-vout_min, "V"))

	fmt.Println("\nDiode Characteristics:")
	vd := results["V(2)"][lastIdx] - results["V(3)"][lastIdx]
	id := 0.0
	if id_values, ok := results["I(D1)"]; ok {
		id = id_values[lastIdx]
	} else {
		id = (results["V(2)"][lastIdx] - results["V(3)"][lastIdx]) / 0.568
	}

	fmt.Printf("  Diode voltage: %s\n", util.FormatValueFactor(vd, "V"))
	fmt.Printf("  Diode current: %s\n", util.FormatValueFactor(id, "A"))

	fmt.Println("\nDone!")
}
