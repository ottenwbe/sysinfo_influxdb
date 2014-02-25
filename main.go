package main

import (
	"flag"
	"fmt"
	"github.com/cloudfoundry/gosigar"
	influxClient "github.com/influxdb/influxdb-go"
)
// use of cat /proc/net/dev for basic network traffic else create an export for collectl
// http://stackoverflow.com/questions/15825007/golang-how-to-ignore-fields-with-sscanf-is-rejected
// http://stackoverflow.com/questions/1052589/how-can-i-parse-the-output-of-proc-net-dev-into-keyvalue-pairs-per-interface-u

const APP_VERSION = "0.1.0"


// Variables storing arguments flags
var verboseFlag bool
var versionFlag bool

func init() {
	flag.BoolVar(&versionFlag, "version", false, "Print the version number and exit.")
	flag.BoolVar(&versionFlag, "V", false, "Print the version number and exit (shorthand).")

	flag.BoolVar(&verboseFlag, "verbose", false, "Display debug information.")
	flag.BoolVar(&verboseFlag, "v", false, "Display debug information (shorthand).")
}

func main() {
	flag.Parse() // Scan the arguments list

	if versionFlag {
		fmt.Println("Version:", APP_VERSION)
	} else {
		var data []*influxClient.Series;

		u, _ := cpus()

		data = append(data, u)

		send(nil, data)
	}
}

/**
 * Interactions with InfluxDB
 */

func send(config *influxClient.ClientConfig, series []*influxClient.Series) error {
	// Pretty printer
	if config == nil || verboseFlag {
		for ks, serie := range series {
			nbCols := len(serie.Columns)

			fmt.Printf("\n#%d: %s\n", ks, serie.Name)

			for _, col := range serie.Columns {
				fmt.Printf("| %s\t", col)
			}
			fmt.Println("|")

			for _, value := range serie.Points {
				fmt.Print("| ")
				for i := 0; i < nbCols; i++ {
					fmt.Print(value[i], "\t| ")
				}
				fmt.Print("\n")
			}
		}
	}

	// Write to InfluxDB
	if config != nil {
		client, err := influxClient.NewClient(config)

		if err != nil {
			return err
		}

		client.WriteSeries(series)
	}

	return nil
}

/**
 * Gathering functions
 */

func cpus() (*influxClient.Series, error) {
	serie := &influxClient.Series{
		Name:    "cpu",
		Columns: []string{"id", "user", "nice", "sys", "idle", "wait", "total"},
		Points:  [][]interface{}{},
	}

	cpu := sigar.Cpu{}
	if err := cpu.Get(); err != nil {
		return nil, err
	}
	serie.Points = append(serie.Points, []interface{}{"cpu", cpu.User, cpu.Nice, cpu.Sys, cpu.Idle, cpu.Wait, cpu.Total()})

	cpus := sigar.CpuList{}
	cpus.Get()
	for i, cpu := range cpus.List {
		serie.Points = append(serie.Points, []interface{}{fmt.Sprint("cpu", i), cpu.User, cpu.Nice, cpu.Sys, cpu.Idle, cpu.Wait, cpu.Total()})
	}

	return serie, nil;
}
