package main

import (
	"fmt"
	"log"
	"math"

	"github.com/edp1096/toy-spice/pkg/analysis"
	"github.com/edp1096/toy-spice/pkg/circuit"
	"github.com/edp1096/toy-spice/pkg/device"
	"github.com/edp1096/toy-spice/pkg/netlist"
)

func createCircuit() (*circuit.Circuit, error) {
	ckt := circuit.NewWithComplex("Diode DC Sweep Circuit", false)

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
			Name:   "Vsweep",
			Nodes:  []string{"1", "0"},
			Value:  0.0,
			Params: map[string]string{"type": "dc"},
		},
		{
			Type:   "R",
			Name:   "Rs",
			Nodes:  []string{"1", "2"},
			Value:  10.0,
			Params: map[string]string{},
		},
		{
			Type:   "D",
			Name:   "D1",
			Nodes:  []string{"2", "0"},
			Params: map[string]string{"model": "D1N4148"},
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
	fmt.Print("===== Diode DC Sweep Example =====\n\n")

	fmt.Println("Generating circuit...")
	ckt, err := createCircuit()
	if err != nil {
		log.Fatalf("error circuit generation: %v", err)
	}

	fmt.Println("Circuit information:")
	fmt.Printf("  Name: %s\n", ckt.Name())
	fmt.Printf("  Node count: %d (except GND)\n\n", ckt.GetNumNodes())

	// DC Sweep (From 0V to 1.2V, 0.05V step)
	fmt.Println("Setting up DC sweep analysis...")
	sweep := analysis.NewDCSweep(
		[]string{"Vsweep"},
		[]float64{0.0},
		[]float64{1.2},
		[]float64{0.05},
	)

	err = sweep.Setup(ckt)
	if err != nil {
		log.Fatalf("error setting up DC sweep: %v", err)
	}

	fmt.Println("Running DC sweep analysis...")
	err = sweep.Execute()
	if err != nil {
		log.Fatalf("error running DC sweep: %v", err)
	}
	fmt.Println()

	results := sweep.GetResults()

	fmt.Println("DC Sweep Results:")
	fmt.Print("=================\n\n")

	sweepPoints := len(results["SWEEP1"])
	fmt.Printf("Number of sweep points: %d\n\n", sweepPoints)

	fmt.Println("Vsweep(V)    Vdiode(V)    Idiode(mA)    Conductance(mS)")
	fmt.Println("----------------------------------------------------------")

	for i := range sweepPoints {
		vsweep := results["SWEEP1"][i]
		vdiode := results["V(2)"][i]

		var idiode float64
		if id, ok := results["I(D1)"]; ok {
			idiode = id[i]
		} else {
			idiode = -results["I(Vsweep)"][i]
		}

		conductance := 0.0
		if vdiode > 0.01 {
			conductance = idiode / vdiode * 1000.0 // by msec
		}

		fmt.Printf("%8.3f      %8.3f      %8.3f      %8.3f\n", vsweep, vdiode, idiode*1000.0, conductance)
	}

	fmt.Println("\nDiode Characteristics Analysis:")

	thresholdIdx := 0
	for i := range sweepPoints {
		var idiode float64
		if id, ok := results["I(D1)"]; ok {
			idiode = id[i]
		} else {
			idiode = -results["I(Vsweep)"][i]
		}

		if idiode*1000.0 >= 1.0 {
			thresholdIdx = i
			break
		}
	}

	if thresholdIdx > 0 {
		thresholdV := results["V(2)"][thresholdIdx]
		fmt.Printf("  Estimated threshold voltage: %.3f V\n", thresholdV)
	}

	var maxCurrent float64 = 0.0
	maxCurrentIdx := 0

	for i := 0; i < sweepPoints; i++ {
		var idiode float64
		if id, ok := results["I(D1)"]; ok {
			idiode = id[i]
		} else {
			idiode = -results["I(Vsweep)"][i]
		}

		if math.Abs(idiode) > math.Abs(maxCurrent) {
			maxCurrent = idiode
			maxCurrentIdx = i
		}
	}

	fmt.Printf("  Maximum current: %.3f mA at %.3f V\n", maxCurrent*1000.0, results["SWEEP1"][maxCurrentIdx])

	fmt.Println("\nDone!")
}
