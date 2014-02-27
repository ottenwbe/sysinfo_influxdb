**WIP**

A collector of system metrics (CPU, Memory, Load, disks I/Os, Network traffic) to an [InfluxDB](http://influxdb.org) server.

## Usage sample :

To display all metrics without sending them to a server :

    $GOPATH/bin/sysinfo_influxdb

To send metric to an InfluxDB server, only one time :

    $GOPATH/bin/sysinfo_influxdb -h localhost:8086 -u root -p secret -d database

You can ommit `-h`, `-u` and `-p` if you use default settings.

By default, table are prefixed by the hostname, to change or disable the prefix, just do :

    $GOPATH/bin/sysinfo_influxdb -P "" # Disable prefix
    $GOPATH/bin/sysinfo_influxdb -P "koala"

To run in daemon mode (doesn't fork, just loop), use the `-D` option :

    $GOPATH/bin/sysinfo_influxdb -D

To display data even if you send them to a server, use `-v` :

    $GOPATH/bin/sysinfo_influxdb -D -h localhost:8086 -d database -v

Use the `-i` option to change the collect interval; this option alters the consistency of quantities displayed or sent (CPUs, network or disks I/Os) : the amount of outgoing packets in 1 minute is not directly comparable to the same amount outgoing in 1 second when saved in the same table. For example, to collect statistics each minute :

    $GOPATH/bin/sysinfo_influxdb -i 1m

To change data collected, use the `-c` option with one or more metrics type (`cpus`, `mem`, `swap`, `uptime`, `load`, `network`, `disks`) like this :

    $GOPATH/bin/sysinfo_influxdb -c cpus # Collect only CPUs related statistics
    $GOPATH/bin/sysinfo_influxdb -c load,cpus,disks # Collect load average, CPUs and disks I/Os statistics

On hardened kernel, you must be allowed to read `/proc/net/dev` in order to collect networking statistics.

## Building :

	cd $GOPATH
	mkdir -p src/github.com/novaquark/
	cd src/github.com/novaquark/
	git clone https://github.com/novaquark/sysinfo_influxdb.git
	cd sysinfo_influxdb
	go get
	go install

<p xmlns:dct="http://purl.org/dc/terms/">
  <a rel="license"
     href="http://creativecommons.org/publicdomain/zero/1.0/">
    <img src="http://i.creativecommons.org/p/zero/1.0/88x31.png" style="border-style: none;" alt="CC0" />
  </a>
  <br />
  To the extent possible under law,
  <a rel="dct:publisher"
     href="https://github.com/orgs/novaquark">
    <span property="dct:title">Novaquark</span></a>
  has waived all copyright and related or neighboring rights to
  this work.
</p>
