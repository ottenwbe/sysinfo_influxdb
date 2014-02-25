package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/cloudfoundry/gosigar"
	influxClient "github.com/influxdb/influxdb-go"
)
// use of cat /proc/net/dev for basic network traffic else create an export for collectl
// http://stackoverflow.com/questions/15825007/golang-how-to-ignore-fields-with-sscanf-is-rejected
// http://stackoverflow.com/questions/1052589/how-can-i-parse-the-output-of-proc-net-dev-into-keyvalue-pairs-per-interface-u

const APP_VERSION = "0.1.0"

// The flag package provides a default help printer via -h switch
var versionFlag *bool = flag.Bool("v", false, "Print the version number.")

func main() {
	flag.Parse() // Scan the arguments list

	if *versionFlag {
		fmt.Println("Version:", APP_VERSION)
	}
	cpus()
}

func cpus() error {
	serie := &influxClient.Series{
		Name:    "test",
		Columns: []string{"id", "user", "nice", "sys", "idle", "wait", "total"},
		Points:  [][]interface{}{},
	}
	
	cpu := sigar.Cpu{}
	if err := cpu.Get(); err != nil {
		return err
	}
	serie.Points = append(serie.Points, []interface{}{"cpu", cpu.User, cpu.Nice, cpu.Sys, cpu.Idle, cpu.Wait, cpu.Total()})

	cpus := sigar.CpuList{}
	cpus.Get()
	for i, cpu := range cpus.List {
		serie.Points = append(serie.Points, []interface{}{fmt.Sprint("cpu", i), cpu.User, cpu.Nice, cpu.Sys, cpu.Idle, cpu.Wait, cpu.Total()})
	}

	b, _ := json.Marshal(serie)
	fmt.Printf("%s", b)
	//fmt.Printf("%2d %5d %5d %5d %5d %5d %5d\n",)
	return nil;
}
