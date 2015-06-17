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
	"encoding/json"
	"flag"
	"fmt"
	"github.com/cloudfoundry/gosigar"
	influxClient "github.com/influxdb/influxdb/client"
	"io/ioutil"
	"net/url"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type chan_ret_t struct {
	series []*influxClient.Point
	err    error
}

const APP_VERSION = "0.5.7"

// Variables storing arguments flags
var verboseFlag bool
var versionFlag bool
var daemonFlag bool
var daemonIntervalFlag time.Duration
var daemonConsistencyFlag time.Duration
var consistencyFactor = 1.0
var collectFlag string
var pidFile string

var sslFlag bool
var hostFlag string
var usernameFlag string
var passwordFlag string
var secretFlag string
var databaseFlag string
var retentionPolicyFlag string

func init() {
	flag.BoolVar(&versionFlag, "version", false, "Print the version number and exit.")
	flag.BoolVar(&versionFlag, "V", false, "Print the version number and exit (shorthand).")

	flag.BoolVar(&verboseFlag, "verbose", false, "Display debug information: choose between text or JSON.")
	flag.BoolVar(&verboseFlag, "v", false, "Display debug information: choose between text or JSON (shorthand).")

	flag.BoolVar(&sslFlag, "ssl", false, "Enable SSL/TLS encryption.")
	flag.BoolVar(&sslFlag, "S", false, "Enable SSL/TLS encryption (shorthand).")
	flag.StringVar(&hostFlag, "host", "localhost:8086", "Connect to host.")
	flag.StringVar(&hostFlag, "h", "localhost:8086", "Connect to host (shorthand).")
	flag.StringVar(&usernameFlag, "username", "root", "User for login.")
	flag.StringVar(&usernameFlag, "u", "root", "User for login (shorthand).")
	flag.StringVar(&passwordFlag, "password", "root", "Password to use when connecting to server.")
	flag.StringVar(&passwordFlag, "p", "root", "Password to use when connecting to server (shorthand).")
	flag.StringVar(&secretFlag, "secret", "", "Absolute path to password file (shorthand). '-p' is ignored if specifed.")
	flag.StringVar(&secretFlag, "s", "", "Absolute path to password file. '-p' is ignored if specifed.")
	flag.StringVar(&databaseFlag, "database", "", "Name of the database to use.")
	flag.StringVar(&databaseFlag, "d", "", "Name of the database to use (shorthand).")
	flag.StringVar(&retentionPolicyFlag, "retentionpolicy", "", "Name of the retention policy to use.")
	flag.StringVar(&retentionPolicyFlag, "rp", "", "Name of the retention policy to use (shorthand).")

	flag.StringVar(&collectFlag, "collect", "cpu,cpus,mem,swap,uptime,load,network,disks,mounts", "Chose which data to collect.")
	flag.StringVar(&collectFlag, "c", "cpu,cpus,mem,swap,uptime,load,network,disks,mounts", "Chose which data to collect (shorthand).")

	flag.BoolVar(&daemonFlag, "daemon", false, "Run in daemon mode.")
	flag.BoolVar(&daemonFlag, "D", false, "Run in daemon mode (shorthand).")
	flag.DurationVar(&daemonIntervalFlag, "interval", time.Second, "With daemon mode, change time between checks.")
	flag.DurationVar(&daemonIntervalFlag, "i", time.Second, "With daemon mode, change time between checks (shorthand).")
	flag.DurationVar(&daemonConsistencyFlag, "consistency", time.Second, "With custom interval, duration to bring back collected values for data consistency (0s to disable).")
	flag.DurationVar(&daemonConsistencyFlag, "C", time.Second, "With daemon mode, duration to bring back collected values for data consistency (shorthand).")

	flag.StringVar(&pidFile, "pidfile", "", "the pid file")
}

func getFqdn() string {
	// Note: We use exec here instead of os.Hostname() because we
	// want the FQDN, and this is the easiest way to get it.
	fqdn, err := exec.Command("hostname", "-f").Output()

	// Fallback on Unqualifed name
	if err != nil {
		hostname, _ := os.Hostname()
		return hostname
	}

	return strings.TrimSpace(string(fqdn))
}

// "in_array" style func for strings
func stringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}

func main() {
	flag.Parse() // Scan the arguments list

	if versionFlag {
		fmt.Println("Version:", APP_VERSION)
	} else {
		if pidFile != "" {
			pid := strconv.Itoa(os.Getpid())
			if err := ioutil.WriteFile(pidFile, []byte(pid), 0644); err != nil {
				fmt.Fprintf(os.Stderr, "Unable to create pidfile\n")
				panic(err)
			}
		}

		if daemonConsistencyFlag.Seconds() > 0 {
			consistencyFactor = daemonConsistencyFlag.Seconds() / daemonIntervalFlag.Seconds()
		}

		// Fill InfluxDB connection settings
		var client *influxClient.Client = nil
		if databaseFlag != "" {
			var proto string
			if sslFlag {
				proto = "https"
			} else {
				proto = "http"
			}
			var u, _ = url.Parse(fmt.Sprintf("%s://%s/", proto, hostFlag))
			config := influxClient.Config{URL: *u, Username: usernameFlag, UserAgent: "sysinfo_influxdb v" + APP_VERSION}

			// use secret file if present, fallback to CLI password arg
			if secretFlag != "" {
				data, err := ioutil.ReadFile(secretFlag)
				if err != nil {
					panic(err)
				}
				config.Password = strings.Split(string(data), "\n")[0]
			} else {
				config.Password = passwordFlag
			}

			var err error
			client, err = influxClient.NewClient(config)

			if err != nil {
				panic(err)
			}
		}

		// Build collect list
		var collectList []GatherFunc
		for _, c := range strings.Split(collectFlag, ",") {
			switch strings.Trim(c, " ") {
			case "cpu":
				collectList = append(collectList, cpu)
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
				collectList = append(collectList, network)
			case "disks":
				collectList = append(collectList, disks)
			case "mounts":
				collectList = append(collectList, mounts)
			default:
				fmt.Fprintf(os.Stderr, "Unknown collect option `%s'\n", c)
				return
			}
		}

		ch := make(chan chan_ret_t, len(collectList))

		// Without daemon mode, do at least one lap
		first := true

		for first || daemonFlag {
			first = false

			// Collect data
			var data []influxClient.Point

			for _, cl := range collectList {
				go cl(ch)
			}

			for i := len(collectList); i > 0; i-- {
				res := <-ch
				if res.err != nil {
					fmt.Fprintf(os.Stderr, "%s\n", res.err)
				} else if len(res.series) > 0 {
					for _, v := range res.series {
						if v != nil {
							data = append(data, *v)
						} else {
							// Loop if we haven't all data:
							// Since diffed data didn't respond the
							// first time they are collected, loop
							// one more time to have it
							first = true
						}
					}
				} else {
					first = true
				}
			}

			if !first {
				for _, serie := range data {
					serie.Tags["fqdn"] = getFqdn()
				}

				// Show data
				if !first && (databaseFlag == "" || verboseFlag) {
					b, _ := json.Marshal(data)
					fmt.Printf("%s\n", b)
				}

				// Send data
				if client != nil {
					if err := send(client, data); err != nil {
						fmt.Fprintf(os.Stderr, "%s\n", err)
					}
				}
			}

			if daemonFlag || first {
				time.Sleep(daemonIntervalFlag)
			}
		}
	}
}

/**
 * Interactions with InfluxDB
 */

func send(client *influxClient.Client, series []influxClient.Point) error {
	w := influxClient.BatchPoints{Database: databaseFlag, Points: series}

	if retentionPolicyFlag != "" {
		w.RetentionPolicy = retentionPolicyFlag
	}

	_, err := client.Write(w)
	return err
}

/**
 * Diff function
 */

var last_series = make(map[string]map[string]interface{})

func DiffFromLast(serie *influxClient.Point) *influxClient.Point {
	notComplete := false

	var keys []string

	for k := range serie.Tags {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	key := serie.Measurement + "#"
	for _, k := range keys {
		key += k + ":" + serie.Tags[k] + "|"
	}

	if _, ok := last_series[key]; !ok {
		last_series[key] = make(map[string]interface{})
	}

	for i, _ := range serie.Fields {
		var val interface{}
		var ok bool
		if val, ok = last_series[key][i]; !ok {
			notComplete = true
			last_series[key][i] = serie.Fields[i]
			continue
		} else {
			last_series[key][i] = serie.Fields[i]
		}

		switch serie.Fields[i].(type) {
		case int8:
			serie.Fields[i] = int8(float64(serie.Fields[i].(int8)-val.(int8)) * consistencyFactor)
		case int16:
			serie.Fields[i] = int16(float64(serie.Fields[i].(int16)-val.(int16)) * consistencyFactor)
		case int32:
			serie.Fields[i] = int32(float64(serie.Fields[i].(int32)-val.(int32)) * consistencyFactor)
		case int64:
			serie.Fields[i] = int64(float64(serie.Fields[i].(int64)-val.(int64)) * consistencyFactor)
		case uint8:
			serie.Fields[i] = uint8(float64(serie.Fields[i].(uint8)-val.(uint8)) * consistencyFactor)
		case uint16:
			serie.Fields[i] = uint16(float64(serie.Fields[i].(uint16)-val.(uint16)) * consistencyFactor)
		case uint32:
			serie.Fields[i] = uint32(float64(serie.Fields[i].(uint32)-val.(uint32)) * consistencyFactor)
		case uint64:
			serie.Fields[i] = uint64(float64(serie.Fields[i].(uint64)-val.(uint64)) * consistencyFactor)
		case int:
			serie.Fields[i] = int(float64(serie.Fields[i].(int)-val.(int)) * consistencyFactor)
		case uint:
			serie.Fields[i] = uint(float64(serie.Fields[i].(uint)-val.(uint)) * consistencyFactor)
		}
	}

	if notComplete {
		return nil
	} else {
		return serie
	}
}

/**
 * Gathering functions
 */

type GatherFunc func(chan chan_ret_t)

func cpu(ch chan chan_ret_t) {
	cpu := sigar.Cpu{}
	if err := cpu.Get(); err != nil {
		ch <- chan_ret_t{nil, err}
		return
	}

	serie := &influxClient.Point{
		Measurement: "cpu",
		Tags: map[string]string{
			"cpuid": "all",
		},
		Fields: map[string]interface{}{
			"user":  cpu.User,
			"nice":  cpu.Nice,
			"sys":   cpu.Sys,
			"idle":  cpu.Idle,
			"wait":  cpu.Wait,
			"total": cpu.Total(),
		},
	}

	ch <- chan_ret_t{[]*influxClient.Point{DiffFromLast(serie)}, nil}
}

func cpus(ch chan chan_ret_t) {
	var series []*influxClient.Point

	cpus := sigar.CpuList{}
	cpus.Get()
	for i, cpu := range cpus.List {
		serie := &influxClient.Point{
			Measurement: "cpus",
			Tags: map[string]string{
				"cpuid": fmt.Sprint(i),
			},
			Fields: map[string]interface{}{
				"user":  cpu.User,
				"nice":  cpu.Nice,
				"sys":   cpu.Sys,
				"idle":  cpu.Idle,
				"wait":  cpu.Wait,
				"total": cpu.Total(),
			},
		}

		if serie = DiffFromLast(serie); serie != nil {
			series = append(series, serie)
		}
	}

	ch <- chan_ret_t{series, nil}
}

func mem(ch chan chan_ret_t) {
	mem := sigar.Mem{}
	if err := mem.Get(); err != nil {
		ch <- chan_ret_t{nil, err}
	}

	serie := &influxClient.Point{
		Measurement: "mem",
		Tags: map[string]string{},
		Fields: map[string]interface{}{
			"free":       mem.Free,
			"used":       mem.Used,
			"actualfree": mem.ActualFree,
			"actualused": mem.ActualUsed,
			"total":      mem.Total,
		},
	}

	ch <- chan_ret_t{[]*influxClient.Point{serie}, nil}
}

func swap(ch chan chan_ret_t) {
	swap := sigar.Swap{}
	if err := swap.Get(); err != nil {
		ch <- chan_ret_t{nil, err}
		return
	}

	serie := &influxClient.Point{
		Measurement: "swap",
		Tags: map[string]string{},
		Fields: map[string]interface{}{
			"free":  swap.Free,
			"used":  swap.Used,
			"total": swap.Total,
		},
	}

	ch <- chan_ret_t{[]*influxClient.Point{serie}, nil}
}

func uptime(ch chan chan_ret_t) {
	uptime := sigar.Uptime{}
	if err := uptime.Get(); err != nil {
		ch <- chan_ret_t{nil, err}
		return
	}

	serie := &influxClient.Point{
		Measurement: "uptime",
		Tags: map[string]string{},
		Fields: map[string]interface{}{
			"length": uptime.Length,
		},
	}

	ch <- chan_ret_t{[]*influxClient.Point{serie}, nil}
}

func load(ch chan chan_ret_t) {
	load := sigar.LoadAverage{}
	if err := load.Get(); err != nil {
		ch <- chan_ret_t{nil, err}
		return
	}

	serie := &influxClient.Point{
		Measurement: "load",
		Tags: map[string]string{},
		Fields: map[string]interface{}{
			"one":     load.One,
			"five":    load.Five,
			"fifteen": load.Fifteen,
		},
	}

	ch <- chan_ret_t{[]*influxClient.Point{serie}, nil}
}

func network(ch chan chan_ret_t) {
	fi, err := os.Open("/proc/net/dev")
	if err != nil {
		ch <- chan_ret_t{nil, err}
		return
	}
	defer fi.Close()

	var series []*influxClient.Point

	cols := []string{"recv_bytes", "recv_packets", "recv_errs", "recv_drop",
		"recv_fifo", "recv_frame", "recv_compressed",
		"recv_multicast", "trans_bytes", "trans_packets",
		"trans_errs", "trans_drop", "trans_fifo",
		"trans_colls", "trans_carrier", "trans_compressed"}

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
		tmp := strings.Split(line, ":")
		if len(tmp) < 2 {
			ch <- chan_ret_t{nil, nil}
			return
		}

		serie := &influxClient.Point{
			Measurement: "network",
			Tags: map[string]string{
				"iface": strings.Trim(tmp[0], " "),
			},
			Fields: map[string]interface{}{},
		}

		tmpf := strings.Fields(tmp[1])
		for i, vc := range cols {
			if vt, err := strconv.Atoi(tmpf[i]); err == nil {
				serie.Fields[vc] = vt
			} else {
				serie.Fields[vc] = 0
			}
		}

		if serie = DiffFromLast(serie); serie != nil {
			series = append(series, serie)
		}
	}

	ch <- chan_ret_t{series, nil}
}

func disks(ch chan chan_ret_t) {
	fi, err := os.Open("/proc/diskstats")
	if err != nil {
		ch <- chan_ret_t{nil, err}
		return
	}
	defer fi.Close()

	var series []*influxClient.Point

	cols := []string{"read_ios", "read_merges", "read_sectors", "read_ticks",
		"write_ios", "write_merges", "write_sectors", "write_ticks",
		"in_flight", "io_ticks", "time_in_queue"}

	// Search device
	scanner := bufio.NewScanner(fi)
	for scanner.Scan() {
		tmp := strings.Fields(scanner.Text())
		if len(tmp) < 14 {
			ch <- chan_ret_t{nil, nil}
			return
		}

		serie := &influxClient.Point{
			Measurement: "disks",
			Tags: map[string]string{
				"device": strings.Trim(tmp[2], " "),
			},
			Fields: map[string]interface{}{},
		}

		for i, vc := range cols {
			if vt, err := strconv.Atoi(tmp[3+i]); err == nil {
				serie.Fields[vc] = vt
			} else {
				serie.Fields[vc] = 0
			}
		}

		if serie = DiffFromLast(serie); serie != nil {
			series = append(series, serie)
		}
	}

	ch <- chan_ret_t{series, nil}
}

func mounts(ch chan chan_ret_t) {
	fi, err := os.Open("/proc/mounts")
	if err != nil {
		ch <- chan_ret_t{nil, err}
		return
	}
	defer fi.Close()

	var series []*influxClient.Point

	// Exclude virtual & system fstype
	sysfs := []string{"binfmt_misc", "cgroup", "configfs", "debugfs",
		"devpts", "devtmpfs", "efivarfs", "fusectl", "mqueue",
		"none", "proc", "rootfs", "securityfs", "sysfs",
		"rpc_pipefs", "fuse.gvfsd-fuse", "tmpfs"}

	scanner := bufio.NewScanner(fi)
	for scanner.Scan() {
		tmp := strings.Fields(scanner.Text())

		// Some hack needed to remove "none" virtual mountpoints
		if (stringInSlice(tmp[2], sysfs) == false) && (tmp[0] != "none") {
			fs := syscall.Statfs_t{}

			err := syscall.Statfs(tmp[1], &fs)
			if err != nil {
				ch <- chan_ret_t{nil, err}
				return
			}

			serie := &influxClient.Point{
				Measurement: "mounts",
				Tags: map[string]string{
					"disk":       tmp[0],
					"mountpoint": tmp[1],
				},
				Fields: map[string]interface{}{
					"free":  fs.Bfree * uint64(fs.Bsize),
					"total": fs.Blocks * uint64(fs.Bsize),
				},
			}

			if serie = DiffFromLast(serie); serie != nil {
				series = append(series, serie)
			}
		}
	}

	ch <- chan_ret_t{series, nil}
}
