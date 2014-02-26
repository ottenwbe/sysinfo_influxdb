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
	"time"
	"github.com/cloudfoundry/gosigar"
	influxClient "github.com/influxdb/influxdb-go"
)

const APP_VERSION = "0.1.0"


// Variables storing arguments flags
var verboseFlag bool
var versionFlag bool
var daemonFlag bool
var daemonIntervalFlag time.Duration
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

	flag.BoolVar(&daemonFlag, "daemon", false, "Run in daemon mode.")
	flag.BoolVar(&daemonFlag, "D", false, "Run in daemon mode (shorthand).")
	flag.DurationVar(&daemonIntervalFlag, "interval", time.Second, "With daemon mode, change time between checks.")
	flag.DurationVar(&daemonIntervalFlag, "i", time.Second, "With daemon mode, change time between checks (shorthand).")
}

func main() {
	flag.Parse() // Scan the arguments list

	if versionFlag {
		fmt.Println("Version:", APP_VERSION)
	} else {
		// Fill InfluxDB connection settings
		var client *influxClient.Client = nil;
		if databaseFlag != "" {
			config := new(influxClient.ClientConfig)

			config.Host = hostFlag
			config.Username = usernameFlag
			config.Password = passwordFlag
			config.Database = databaseFlag

			var err error
			client, err = influxClient.NewClient(config)

			if err != nil {
				panic(err)
			}
		}

		// Build collect functions
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

		ch := make(chan *influxClient.Series, len(collectList))

		first := true

		for first || daemonFlag {
			if daemonFlag {
				time.Sleep(daemonIntervalFlag)
			}
			if first {
				first = false
			}

			// Collect data
			var data []*influxClient.Series;

			for _, cl := range collectList {
				go cl(prefixFlag, ch)
			}

			for i := len(collectList); i > 0; i-- {
				res := <-ch
				if res != nil {
					data = append(data, res)
				}
			}

			if databaseFlag == "" || verboseFlag {
				prettyPrinter(data)
			}
			if client != nil {
				if err := send(client, data); err != nil {
					panic(err)
				}
			}
		}
	}
}

func prettyPrinter(series []*influxClient.Series) {
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

/**
 * Interactions with InfluxDB
 */

func send(client *influxClient.Client, series []*influxClient.Series) error {
	return client.WriteSeries(series)
}


/**
 * Gathering functions
 */

type GatherFunc func(string, chan *influxClient.Series) error

func cpus(prefix string, ch chan *influxClient.Series) error {
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
		ch <- nil
		return err
	}
	serie.Points = append(serie.Points, []interface{}{"cpu", cpu.User, cpu.Nice, cpu.Sys, cpu.Idle, cpu.Wait, cpu.Total()})

	cpus := sigar.CpuList{}
	cpus.Get()
	for i, cpu := range cpus.List {
		serie.Points = append(serie.Points, []interface{}{fmt.Sprint("cpu", i), cpu.User, cpu.Nice, cpu.Sys, cpu.Idle, cpu.Wait, cpu.Total()})
	}

	ch <- serie
	return nil;
}

func mem(prefix string, ch chan *influxClient.Series) error {
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
		ch <- nil
		return err
	}
	serie.Points = append(serie.Points, []interface{}{mem.Free, mem.Used, mem.ActualFree, mem.ActualUsed, mem.Total})

	ch <- serie
	return nil
}

func swap(prefix string, ch chan *influxClient.Series) error {
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
		ch <- nil
		return err
	}
	serie.Points = append(serie.Points, []interface{}{swap.Free, swap.Used, swap.Total})

	ch <- serie
	return nil
}

func uptime(prefix string, ch chan *influxClient.Series) error {
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
		ch <- nil
		return err
	}
	serie.Points = append(serie.Points, []interface{}{uptime.Length})

	ch <- serie
	return nil
}

func load(prefix string, ch chan *influxClient.Series) error {
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
		ch <- nil
		return err
	}
	serie.Points = append(serie.Points, []interface{}{load.One, load.Five, load.Fifteen})

	ch <- serie
	return nil
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
		ret = append(ret, func (prefix string, ch chan *influxClient.Series) error {
			return network_parse_iface_line(prefix, ch, line)
		})
	}

	return ret
}


/**
 * Network data related functions
 */

func network_parse_iface_line(prefix string, ch chan *influxClient.Series, line string) error {
	if prefix != "" {
		prefix += "."
	}

	tmp := strings.Split(line, ":")
	if len(tmp) < 2 {
		ch <- nil
		return nil
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

	ch <- serie
	return nil
}
