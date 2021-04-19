package main

import (
	"bufio"
	"encoding/csv"
	"flag"
	"fmt"
	"math"
	"os"
	"strconv"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/goburrow/serial"
	"github.com/MurekL/SmartPi/src/smartpi"
	"github.com/nDenerserve/mbserver"
	log "github.com/sirupsen/logrus"
)

func init() {
	log.SetFormatter(&log.TextFormatter{})
	log.SetOutput(os.Stdout)
	log.SetLevel(log.DebugLevel)
}

var appVersion = "No Version Provided"

// main
func main() {

	config := smartpi.NewConfig()

	version := flag.Bool("v", false, "prints current version information")
	flag.Parse()
	if *version {
		fmt.Println(appVersion)
		os.Exit(0)
	}

	// creates a new file watcher
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal("ERROR", err)
	}
	defer watcher.Close()

	//
	done := make(chan bool)

	serv := mbserver.NewServer()

	if config.ModbusTCPenabled {
		err := serv.ListenTCP(config.ModbusTCPAddress)
		if err != nil {
			log.Fatal("%v\n", err)
		} else {
			log.Info("Modbus TCP started on: ")
			log.Info(config.ModbusTCPAddress)
		}
	}

	if config.ModbusRTUenabled {
		fmt.Println("Device: ", config.ModbusRTUDevice, "  Address: ", config.ModbusRTUAddress)
		err := serv.ListenRTU(&serial.Config{
			Address:  config.ModbusRTUDevice,
			BaudRate: 19200,
			DataBits: 8,
			StopBits: 1,
			Parity:   "N"}, config.ModbusRTUAddress)
		if err != nil {
			log.Fatal("failed to listen, got %v\n", err)
		} else {
			log.Info("Modbus RTU started on address: ")
			log.Info(config.ModbusRTUAddress)
		}
	}
	defer serv.Close()

	var registervalue uint32

	//
	go func() {
		for {
			select {
			// watch for events
			case event := <-watcher.Events:
				log.Debugf("EVENT! %#v\n", event)
				time.Sleep(1 * time.Second)
				file, err := os.Open(config.SharedDir + "/" + config.SharedFile)
				smartpi.Checklog(err)
				defer file.Close()
				reader := csv.NewReader(bufio.NewReader(file))
				reader.Comma = ';'
				records, err := reader.Read()
				log.Debugf("%v", records)
				smartpi.Checklog(err)
				if len(records) >= 19 {
					for i := 1; i < len(records)-1; i++ {
						registervalue = 0
						val, err := strconv.ParseFloat(records[i], 32)
						if err != nil {
							log.Fatal("error converting value", err)
						} else {
							registervalue = math.Float32bits(float32(val))
						}
						serv.HoldingRegisters[2*i-2] = uint16(registervalue >> 16)
						serv.HoldingRegisters[2*i-1] = uint16(registervalue)
					}
				} else {
					log.Fatal("Values not written")
				}

				file.Close()

				// watch for errors
			case err := <-watcher.Errors:
				log.Fatal("ERROR", err)
			}
		}
	}()

	// out of the box fsnotify can watch a single file, or a single directory
	if err := watcher.Add(config.SharedDir + "/" + config.SharedFile); err != nil {
		log.Fatal("ERROR", err)

	}

	<-done
}
