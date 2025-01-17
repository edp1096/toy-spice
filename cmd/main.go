package main // import "spice"

import (
	"flag"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"

	"toy-spice/pkg/analysis"
	"toy-spice/pkg/circuit"
	"toy-spice/pkg/netlist"
	"toy-spice/pkg/util"
)

func getKeys(m map[string][]float64) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func printResults(results map[string][]float64) {
	fmt.Println("\nAnalysis Results:")
	fmt.Println("================")

	// AC
	if freqs, isAC := results["FREQ"]; isAC {
		fmt.Printf("\nAC Analysis Results (%d frequency points):\n", len(freqs))
		fmt.Println("Frequency      Node Voltages (Magnitude/Phase)        Branch Currents (Magnitude/Phase)")
		fmt.Println("-----------------------------------------------------------------------------")

		var voltageNames, currentNames []string
		for name := range results {
			if strings.HasSuffix(name, "_MAG") {
				baseName := strings.TrimSuffix(name, "_MAG")
				if strings.HasPrefix(baseName, "V(") {
					voltageNames = append(voltageNames, baseName)
				} else if strings.HasPrefix(baseName, "I(") {
					currentNames = append(currentNames, baseName)
				}
			}
		}
		sort.Strings(voltageNames)
		sort.Strings(currentNames)

		for i, freq := range freqs {
			fmt.Printf("%-13s", util.FormatFrequency(freq))

			// Node voltage
			for _, name := range voltageNames {
				magName := name + "_MAG"
				phaseName := name + "_PHASE"
				if mag, ok := results[magName]; ok {
					if phase, ok := results[phaseName]; ok {
						magStr := util.FormatMagnitude(mag[i])
						phaseStr := util.FormatPhase(phase[i])
						fmt.Printf("%s=%s<%sdeg  ", name, magStr, phaseStr)
					}
				}
			}

			// Branch current
			for _, name := range currentNames {
				magName := name + "_MAG"
				phaseName := name + "_PHASE"
				if mag, ok := results[magName]; ok {
					if phase, ok := results[phaseName]; ok {
						magStr := util.FormatMagnitude(mag[i])
						phaseStr := util.FormatPhase(phase[i])
						fmt.Printf("%s=%s<%sdeg  ", name, magStr, phaseStr)
					}
				}
			}
			fmt.Println()
		}
		return
	}

	// DC Sweep
	if sweep1, isDC := results["SWEEP1"]; isDC {
		fmt.Printf("\nDC Sweep Analysis Results (%d points):\n", len(sweep1))
		fmt.Println("Sweep Values    Node Voltages        Branch Currents")
		fmt.Println("------------------------------------------------")

		var voltageNames, currentNames []string
		for name := range results {
			if name == "SWEEP1" || name == "SWEEP2" {
				continue
			}
			if strings.HasPrefix(name, "V(") {
				voltageNames = append(voltageNames, name)
			} else if strings.HasPrefix(name, "I(") {
				currentNames = append(currentNames, name)
			}
		}
		sort.Strings(voltageNames)
		sort.Strings(currentNames)

		_, hasNested := results["SWEEP2"]
		for i := range sweep1 {
			if hasNested {
				sweep2 := results["SWEEP2"]
				fmt.Printf("V1=%-9s V2=%-9s  ",
					util.FormatValueFactor(sweep1[i], "V"),
					util.FormatValueFactor(sweep2[i], "V"))
			} else {
				fmt.Printf("V=%-9s  ", util.FormatValueFactor(sweep1[i], "V"))
			}

			for _, name := range voltageNames {
				if values, ok := results[name]; ok {
					fmt.Printf("%s=%s  ", name, util.FormatValueFactor(values[i], "V"))
				}
			}
			for _, name := range currentNames {
				if values, ok := results[name]; ok {
					fmt.Printf("%s=%s  ", name, util.FormatValueFactor(values[i], "A"))
				}
			}
			fmt.Println()
		}
		return
	}

	// Operating point
	if len(results["TIME"]) <= 1 {
		fmt.Println("\nNode Voltages:")
		for name, values := range results {
			if strings.HasPrefix(name, "V(") {
				fmt.Printf("%s = %s\n", name, util.FormatValueFactor(values[0], "V"))
			}
		}
		fmt.Println("\nBranch Currents:")
		for name, values := range results {
			if strings.HasPrefix(name, "I(") {
				fmt.Printf("%s = %s\n", name, util.FormatValueFactor(values[0], "A"))
			}
		}
		return
	}

	// Transient
	times := results["TIME"]
	fmt.Printf("\nTransient Analysis Results (%d time points):\n", len(times))
	fmt.Println("Time        Node Voltages        Branch Currents")
	fmt.Println("------------------------------------------------")

	var voltageNames, currentNames []string
	for name := range results {
		if name == "TIME" {
			continue
		}
		if strings.HasPrefix(name, "V(") {
			voltageNames = append(voltageNames, name)
		} else if strings.HasPrefix(name, "I(") {
			currentNames = append(currentNames, name)
		}
	}
	sort.Strings(voltageNames)
	sort.Strings(currentNames)

	for i, t := range times {
		fmt.Printf("%9s  ", util.FormatValueFactor(t, "s"))

		// Node voltage
		for _, name := range voltageNames {
			if values, ok := results[name]; ok {
				fmt.Printf("%s=%s  ", name, util.FormatValueFactor(values[i], "V"))
			}
		}
		// Branch current
		for _, name := range currentNames {
			if values, ok := results[name]; ok {
				fmt.Printf("%s=%s  ", name, util.FormatValueFactor(values[i], "A"))
			}
		}
		fmt.Println()
	}
}

func procWithPrint() {
	// 1. Open and read netlist
	fmt.Printf("\n[1] Reading netlist file: %s\n", flag.Arg(0))
	content, err := os.ReadFile(flag.Arg(0))
	if err != nil {
		log.Fatalf("Error reading netlist file: %v", err)
	}
	fmt.Printf("File contents:\n%s\n", string(content))

	// 2. Parse netlist
	fmt.Println("\n[2] Parsing netlist")
	ckt, err := netlist.Parse(string(content))
	if err != nil {
		log.Fatalf("Error parsing netlist: %v", err)
	}
	fmt.Printf("Analysis type: %v\n", ckt.Analysis)
	fmt.Printf("Circuit elements: %d\n", len(ckt.Elements))
	for i, elem := range ckt.Elements {
		fmt.Printf("Element %d: %s (type: %s, nodes: %v)\n",
			i, elem.Name, elem.Type, elem.Nodes)
	}

	// 3. Setup circuit
	fmt.Println("\n[3] Creating circuit structure")
	isComplex := ckt.Analysis == netlist.AnalysisAC
	circuit := circuit.NewWithComplex(ckt.Title, isComplex)

	// 3.1 Map nodes and branches
	if err := circuit.AssignNodeBranchMaps(ckt.Elements); err != nil {
		log.Fatalf("Error creating circuit mappings: %v", err)
	}

	// 3.2 Create matrix
	circuit.CreateMatrix()

	// 3.2.1 Print elements
	fmt.Println("\n=== Circuit Element Details ===")
	for i, elem := range ckt.Elements {
		fmt.Printf("\nElement %d: %s\n", i, elem.Name)
		fmt.Printf("Type: %s\n", elem.Type)
		fmt.Printf("Nodes: %v\n", elem.Nodes)

		// Node mapping information output
		fmt.Printf("Node mapping:\n")
		nodeMap := circuit.GetNodeMap()
		for j, nodeName := range elem.Nodes {
			if nodeName == "0" || nodeName == "gnd" {
				fmt.Printf("  Node %d: %s -> Ground (0)\n", j, nodeName)
			} else {
				fmt.Printf("  Node %d: %s -> %d\n", j, nodeName, nodeMap[nodeName])
			}
		}

		// For voltage sources
		if elem.Type == "V" {
			branchMap := circuit.GetBranchMap()
			branchIdx := branchMap[elem.Name]
			fmt.Printf("Branch index: %d\n", branchIdx)
			fmt.Printf("Expected matrix contributions:\n")
			n1 := nodeMap[elem.Nodes[0]]
			n2 := 0 // Ground case
			if elem.Nodes[1] != "0" && elem.Nodes[1] != "gnd" {
				n2 = nodeMap[elem.Nodes[1]]
			}

			fmt.Printf("  KCL equations:\n")
			if n1 != 0 {
				fmt.Printf("    (%d,%d): +1\n", n1, branchIdx)
			}
			if n2 != 0 {
				fmt.Printf("    (%d,%d): -1\n", n2, branchIdx)
			}

			fmt.Printf("  Branch equations:\n")
			if n1 != 0 {
				fmt.Printf("    (%d,%d): +1\n", branchIdx, n1)
			}
			if n2 != 0 {
				fmt.Printf("    (%d,%d): -1\n", branchIdx, n2)
			}
		}

		// For resistors
		if elem.Type == "R" {
			resistance := elem.Value
			conductance := 1.0 / resistance
			fmt.Printf("Resistance: %g ohm\n", resistance)
			fmt.Printf("Conductance: %g Mho\n", conductance)

			n1 := nodeMap[elem.Nodes[0]]
			n2 := 0
			if elem.Nodes[1] != "0" && elem.Nodes[1] != "gnd" {
				n2 = nodeMap[elem.Nodes[1]]
			}

			fmt.Printf("Expected matrix contributions:\n")
			if n1 != 0 {
				fmt.Printf("  (%d,%d): +%g\n", n1, n1, conductance)
			}
			if n2 != 0 {
				fmt.Printf("  (%d,%d): +%g\n", n2, n2, conductance)
			}
			if n1 != 0 && n2 != 0 {
				fmt.Printf("  (%d,%d): -%g\n", n1, n2, conductance)
				fmt.Printf("  (%d,%d): -%g\n", n2, n1, conductance)
			}
		}
	}

	// 3.3 Create devices and stamp
	if err := circuit.SetupDevices(ckt.Elements); err != nil {
		log.Fatalf("Error setting up devices: %v", err)
	}
	circuit.GetMatrix().PrintSystem() // Print sparse matrix

	// 4. Setup analyzer
	fmt.Println("\n[4] Setting up analyzer")
	var analyzer analysis.Analysis
	switch ckt.Analysis {
	case netlist.AnalysisOP:
		analyzer = analysis.NewOP()
		fmt.Println("Created Operating Point analyzer")
	case netlist.AnalysisTRAN:
		param := ckt.TranParam
		analyzer = analysis.NewTransient(param.TStart, param.TStop, param.TStep, param.TMax, param.UIC)
		fmt.Printf("Created Transient analyzer (step=%g, stop=%g, start=%g, maxstep=%g, uic=%v)\n", param.TStep, param.TStop, param.TStart, param.TMax, param.UIC)
	case netlist.AnalysisAC:
		param := ckt.ACParam
		analyzer = analysis.NewAC(param.FStart, param.FStop, param.Points, param.Sweep)
	case netlist.AnalysisDC:
		param := ckt.DCParam
		if param.Source2 != "" {
			// nested sweep
			analyzer = analysis.NewDCSweep(
				[]string{param.Source1, param.Source2},
				[]float64{param.Start1, param.Start2},
				[]float64{param.Stop1, param.Stop2},
				[]float64{param.Increment1, param.Increment2},
			)
		} else {
			// single sweep
			analyzer = analysis.NewDCSweep(
				[]string{param.Source1},
				[]float64{param.Start1},
				[]float64{param.Stop1},
				[]float64{param.Increment1},
			)
		}
	default:
		log.Fatal("Unsupported analysis type")
	}

	if err := analyzer.Setup(circuit); err != nil {
		log.Fatalf("Analysis setup failed: %v", err)
	}
	fmt.Println("Analyzer setup completed")

	// 5. Run analysis
	fmt.Println("\n[5] Executing analysis")
	if err := analyzer.Execute(); err != nil {
		log.Fatalf("Analysis execution failed: %v", err)
	}

	// 6. Print result
	fmt.Println("\n[6] Analysis completed - Results:")
	printResults(analyzer.GetResults())
}

func procWithoutPrint() {
	// 1. Open and read netlist
	content, err := os.ReadFile(flag.Arg(0))
	if err != nil {
		log.Fatalf("Error reading netlist file: %v", err)
	}

	// 2. Parse netlist
	ckt, err := netlist.Parse(string(content))
	if err != nil {
		log.Fatalf("Error parsing netlist: %v", err)
	}

	// 3. Setup circuit
	isComplex := ckt.Analysis == netlist.AnalysisAC
	circuit := circuit.NewWithComplex(ckt.Title, isComplex)

	// 3.1 Map nodes and branches
	if err := circuit.AssignNodeBranchMaps(ckt.Elements); err != nil {
		log.Fatalf("Error creating circuit mappings: %v", err)
	}

	// 3.2 Create matrix
	circuit.CreateMatrix()

	// 3.3 Create devices and stamp
	if err := circuit.SetupDevices(ckt.Elements); err != nil {
		log.Fatalf("Error setting up devices: %v", err)
	}
	// circuit.GetMatrix().PrintSystem()

	// 4. Setup analyzer
	var analyzer analysis.Analysis
	switch ckt.Analysis {
	case netlist.AnalysisOP:
		analyzer = analysis.NewOP()
	case netlist.AnalysisTRAN:
		param := ckt.TranParam
		analyzer = analysis.NewTransient(param.TStart, param.TStop, param.TStep, param.TMax, param.UIC)
	case netlist.AnalysisAC:
		param := ckt.ACParam
		analyzer = analysis.NewAC(param.FStart, param.FStop, param.Points, param.Sweep)
	case netlist.AnalysisDC:
		param := ckt.DCParam
		if param.Source2 != "" {
			// nested sweep
			analyzer = analysis.NewDCSweep(
				[]string{param.Source1, param.Source2},
				[]float64{param.Start1, param.Start2},
				[]float64{param.Stop1, param.Stop2},
				[]float64{param.Increment1, param.Increment2},
			)
		} else {
			// single sweep
			analyzer = analysis.NewDCSweep(
				[]string{param.Source1},
				[]float64{param.Start1},
				[]float64{param.Stop1},
				[]float64{param.Increment1},
			)
		}
	default:
		log.Fatal("Unsupported analysis type")
	}

	if err := analyzer.Setup(circuit); err != nil {
		log.Fatalf("Analysis setup failed: %v", err)
	}

	// 5. Run analysis
	if err := analyzer.Execute(); err != nil {
		log.Fatalf("Analysis execution failed: %v", err)
	}

	// 6. Print result
	printResults(analyzer.GetResults())
}

func main() {
	flag.Parse()
	if flag.NArg() != 1 {
		log.Fatal("Usage: spice <netlist_file>")
	}

	// procWithPrint()
	procWithoutPrint()
}
