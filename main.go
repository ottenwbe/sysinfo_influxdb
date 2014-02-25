// sysinfo_influxdb by Novaquark
//
// To the extent possible under law, the person who associated CC0 with
// sysinfo_influxdb has waived all copyright and related or neighboring rights
// to sysinfo_influxdb.
//
// You should have received a copy of the CC0 legalcode along with this
// work.  If not, see <http://creativecommons.org/publicdomain/zero/1.0/>.

package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"github.com/cloudfoundry/gosigar"
	influxClient "github.com/influxdb/influxdb-go"
)

const APP_VERSION = "0.1.0"


// Variables storing arguments flags
var verboseFlag bool
var versionFlag bool
var prefixFlag string
var collectFlag string

var hostFlag string
var usernameFlag string
var passwordFlag string
var databaseFlag string

func init() {
	flag.BoolVar(&versionFlag, "version", false, "Print the version number and exit.")
	flag.BoolVar(&versionFlag, "V", false, "Print the version number and exit (shorthand).")

	flag.BoolVar(&verboseFlag, "verbose", false, "Display debug information.")
	flag.BoolVar(&verboseFlag, "v", false, "Display debug information (shorthand).")

	hostname, _ := os.Hostname();
	flag.StringVar(&prefixFlag, "prefix", hostname, "Change series name prefix.")
	flag.StringVar(&prefixFlag, "P", hostname, "Change series name prefix (shorthand).")

	flag.StringVar(&hostFlag, "host", "localhost:8086", "Connect to host.")
	flag.StringVar(&hostFlag, "h", "localhost:8086", "Connect to host (shorthand).")
	flag.StringVar(&usernameFlag, "username", "root", "User for login.")
	flag.StringVar(&usernameFlag, "u", "root", "User for login (shorthand).")
	flag.StringVar(&passwordFlag, "password", "root", "Password to use when connecting to server.")
	flag.StringVar(&passwordFlag, "p", "root", "Password to use when connecting to server (shorthand).")
	flag.StringVar(&databaseFlag, "database", "", "Name of the database to use.")
	flag.StringVar(&databaseFlag, "d", "", "Name of the database to use (shorthand).")

	flag.StringVar(&collectFlag, "collect", "cpus,mem,swap,uptime,load,network", "Chose which data to collect.")
	flag.StringVar(&collectFlag, "c", "cpus,mem,swap,uptime,load,network", "Chose which data to collect (shorthand).")
}

func main() {
	flag.Parse() // Scan the arguments list

	if versionFlag {
		fmt.Println("Version:", APP_VERSION)
	} else {
		var collectList []GatherFunc

		for _, c := range strings.Split(collectFlag, ",") {
			switch(strings.Trim(c, " ")) {
			case "cpus":
				collectList = append(collectList, cpus)
			case "mem":
				collectList = append(collectList, mem)
			case "swap":
				collectList = append(collectList, swap)
			case "uptime":
				collectList = append(collectList, uptime)
			case "load":
				collectList = append(collectList, load)
			case "network":
				for _, d := range gen_network() {
					collectList = append(collectList, d)
				}
			}
		}

		// Collect data
		var data []*influxClient.Series;

		for _, c := range collectList {
			if u, err := c(prefixFlag); err == nil {
				data = append(data, u)
			} else {
				fmt.Fprintln(os.Stderr, err)
			}
		}

		// Fill InfluxDB connection settings
		var config *influxClient.ClientConfig = nil;
		if databaseFlag != "" {
			config = new(influxClient.ClientConfig)

			config.Host = hostFlag
			config.Username = usernameFlag
			config.Password = passwordFlag
			config.Database = databaseFlag
		}

		send(config, data)
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

type GatherFunc func(string) (*influxClient.Series, error)

func cpus(prefix string) (*influxClient.Series, error) {
	if prefix != "" {
		prefix += "."
	}

	serie := &influxClient.Series{
		Name:    prefix + "cpu",
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

func mem(prefix string) (*influxClient.Series, error) {
	if prefix != "" {
		prefix += "."
	}

	serie := &influxClient.Series{
		Name:    prefix + "mem",
		Columns: []string{"free", "used", "actualfree", "actualused", "total"},
		Points:  [][]interface{}{},
	}

	mem := sigar.Mem{}
	if err := mem.Get(); err != nil {
		return nil, err
	}
	serie.Points = append(serie.Points, []interface{}{mem.Free, mem.Used, mem.ActualFree, mem.ActualUsed, mem.Total})

	return serie, nil
}

func swap(prefix string) (*influxClient.Series, error) {
	if prefix != "" {
		prefix += "."
	}

	serie := &influxClient.Series{
		Name:    prefix + "swap",
		Columns: []string{"free", "used", "total"},
		Points:  [][]interface{}{},
	}

	swap := sigar.Swap{}
	if err := swap.Get(); err != nil {
		return nil, err
	}
	serie.Points = append(serie.Points, []interface{}{swap.Free, swap.Used, swap.Total})

	return serie, nil
}

func uptime(prefix string) (*influxClient.Series, error) {
	if prefix != "" {
		prefix += "."
	}

	serie := &influxClient.Series{
		Name:    prefix + "uptime",
		Columns: []string{"length"},
		Points:  [][]interface{}{},
	}

	uptime := sigar.Uptime{}
	if err := uptime.Get(); err != nil {
		return nil, err
	}
	serie.Points = append(serie.Points, []interface{}{uptime.Length})

	return serie, nil
}

func load(prefix string) (*influxClient.Series, error) {
	if prefix != "" {
		prefix += "."
	}

	serie := &influxClient.Series{
		Name:    prefix + "load",
		Columns: []string{"one", "five", "fifteen"},
		Points:  [][]interface{}{},
	}

	load := sigar.LoadAverage{}
	if err := load.Get(); err != nil {
		return nil, err
	}
	serie.Points = append(serie.Points, []interface{}{load.One, load.Five, load.Fifteen})

	return serie, nil
}

func gen_network() []GatherFunc {
	var ret []GatherFunc

	fi, err := os.Open("/proc/net/dev")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return ret
	}
	defer fi.Close()

	// Search interface
	skip := 2
	scanner := bufio.NewScanner(fi)
	for scanner.Scan() {
		// Skip headers
		if skip > 0 {
			skip--
			continue
		}

		line := scanner.Text()
		ret = append(ret, func (prefix string) (*influxClient.Series, error) {
			return network_parse_iface_line(prefix, line)
		})
	}

	return ret
}


/**
 * Network data related functions
 */

func network_parse_iface_line(prefix string, line string) (*influxClient.Series, error) {
	if prefix != "" {
		prefix += "."
	}

	tmp := strings.Split(line, ":")
	if len(tmp) < 2 {
		return nil, nil
	}

	iface := strings.Trim(tmp[0], " ")

	serie := &influxClient.Series{
		Name:    prefix + iface,
		Columns: []string{ "recv_bytes", "recv_packets", "recv_errs",
			           "recv_drop", "recv_fifo", "recv_frame",
			           "recv_compressed", "recv_multicast",
			           "trans_bytes", "trans_packets", "trans_errs",
			           "trans_drop", "trans_fifo", "trans_colls",
			           "trans_carrier", "trans_compressed" },
		Points:  [][]interface{}{},
	}

	tmp = strings.Fields(tmp[1])

	var points []interface{}

	for i := 0; i < len(serie.Columns); i++ {
		if v, err := strconv.Atoi(tmp[i]); err == nil {
			points = append(points, v)
		} else {
			points = append(points, 0)
		}
	}

	serie.Points = append(serie.Points, points)

	return serie, nil
}
