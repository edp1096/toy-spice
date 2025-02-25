package main

import (
	"fmt"
	"log"
	"strings"

	"github.com/edp1096/toy-spice/pkg/analysis"
	"github.com/edp1096/toy-spice/pkg/circuit"
	"github.com/edp1096/toy-spice/pkg/netlist"
	"github.com/edp1096/toy-spice/pkg/util"
)

func createCircuit() (*circuit.Circuit, error) {
	ckt := circuit.NewWithComplex("RR voltage divider circuit", false)

	elements := []netlist.Element{
		{
			Type:   "V",
			Name:   "Vsrc",
			Nodes:  []string{"1", "0"},
			Value:  10.0,
			Params: map[string]string{"type": "dc"},
		},
		{
			Type:   "R",
			Name:   "R1",
			Nodes:  []string{"1", "2"},
			Value:  1000.0,
			Params: map[string]string{},
		},
		{
			Type:   "R",
			Name:   "R2",
			Nodes:  []string{"2", "0"},
			Value:  1000.0,
			Params: map[string]string{},
		},
	}

	err := ckt.AssignNodeBranchMaps(elements)
	if err != nil {
		return nil, fmt.Errorf("error node, branch map: %v", err)
	}

	ckt.CreateMatrix()

	err = ckt.SetupDevices(elements)
	if err != nil {
		return nil, fmt.Errorf("error device setup: %v", err)
	}

	return ckt, nil
}

func main() {
	fmt.Print("===== Example =====\n\n")

	fmt.Println("Generating circuit...")
	ckt, err := createCircuit()
	if err != nil {
		log.Fatalf("error circuit generation: %v", err)
	}

	fmt.Println("Information:")
	fmt.Printf("Circuit name: %s\n", ckt.Name())
	fmt.Printf("Node count: %d (Except 0(GND))\n\n", ckt.GetNumNodes())

	nodeMap := ckt.GetNodeMap()
	fmt.Println("Node map:")
	for name, idx := range nodeMap {
		fmt.Printf("  Node '%s' -> index %d\n", name, idx)
	}
	fmt.Println()

	branchMap := ckt.GetBranchMap()
	fmt.Println("Branch map:")
	for name, idx := range branchMap {
		fmt.Printf("  Branch '%s' -> index %d\n", name, idx)
	}
	fmt.Println()

	fmt.Println("Running bias point...")
	analyzer := analysis.NewOP()
	err = analyzer.Setup(ckt)
	if err != nil {
		log.Fatalf("error bias point: %v", err)
	}

	fmt.Println("Matrix stamping...")
	ckt.Matrix.PrintSystem()
	fmt.Println()

	fmt.Println("Running...")
	err = analyzer.Execute()
	if err != nil {
		log.Fatalf("error running: %v", err)
	}
	fmt.Println()

	results := analyzer.GetResults()

	fmt.Println("Result:")
	fmt.Print("================\n\n")

	fmt.Println(results)

	fmt.Println("Node voltage:")
	for name, values := range results {
		if strings.HasPrefix(name, "V(") {
			fmt.Printf("%s = %s\n", name, util.FormatValueFactor(values[0], "V"))
		}
	}
	fmt.Println()

	fmt.Println("Branch current:")
	for name, values := range results {
		if strings.HasPrefix(name, "I(") {
			fmt.Printf("%s = %s\n", name, util.FormatValueFactor(values[0], "A"))
		}
	}
	fmt.Println()

	if v1, ok := results["V(1)"]; ok {
		if v2, ok := results["V(2)"]; ok {
			fmt.Println("Resistor power consumtion:")

			i_r1 := (v1[0] - v2[0]) / 1000.0
			i_r2 := v2[0] / 1000.0

			p_r1 := (v1[0] - v2[0]) * i_r1
			p_r2 := v2[0] * i_r2
			p_total := p_r1 + p_r2

			fmt.Printf("P(R1) = %s\n", util.FormatValueFactor(p_r1, "W"))
			fmt.Printf("P(R2) = %s\n", util.FormatValueFactor(p_r2, "W"))
			fmt.Printf("P(Total) = %s\n", util.FormatValueFactor(p_total, "W"))
		}
	}
	fmt.Println()

	fmt.Println("Done!")
}
